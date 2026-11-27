package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"search-service/internal/infrastructure/opensearch"
	"search-service/pkg/logger"
)

// Pause before retrying a message in place: the base (KAFKA_RETRY_BACKOFF_MS,
// at least minRetryBackoff) doubles per attempt up to maxRetryBackoff.
const (
	minRetryBackoff = 100 * time.Millisecond
	maxRetryBackoff = 30 * time.Second
)

// indexCallTimeout bounds one OpenSearch call for one event; a timeout is a
// transient error and the event is retried.
const indexCallTimeout = 10 * time.Second

// messageReader is the part of *kafka.Reader the consumer uses.
type messageReader interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
}

// messageWriter is the part of *kafka.Writer the consumer uses for the DLQ.
type messageWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

type Consumer struct {
	readerUsers  *kafka.Reader
	readerPosts  *kafka.Reader
	dlqWriter    messageWriter // nil when no DLQ topic is configured
	os           *opensearch.Client
	usersIndex   string
	postsIndex   string
	dlqTopic     string
	retryBackoff time.Duration
	log          *logger.Logger
}

func NewConsumer(
	brokers []string,
	groupID, topicUsers, topicPosts, dlqTopic, usersIndex, postsIndex string,
	saslUsername, saslPassword string,
	retryBackoff time.Duration,
	osClient *opensearch.Client,
	log *logger.Logger,
) *Consumer {
	dialer := &kafka.Dialer{
		Timeout:       10 * time.Second,
		SASLMechanism: plain.Mechanism{Username: saslUsername, Password: saslPassword},
	}
	readerUsers := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		GroupID:        groupID,
		Topic:          topicUsers,
		Dialer:         dialer,
		MinBytes:       1,
		MaxBytes:       10e6,
		MaxWait:        1 * time.Second,
		CommitInterval: 0,
	})
	readerPosts := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		GroupID:        groupID,
		Topic:          topicPosts,
		Dialer:         dialer,
		MinBytes:       1,
		MaxBytes:       10e6,
		MaxWait:        1 * time.Second,
		CommitInterval: 0,
	})

	c := &Consumer{
		readerUsers:  readerUsers,
		readerPosts:  readerPosts,
		os:           osClient,
		usersIndex:   usersIndex,
		postsIndex:   postsIndex,
		dlqTopic:     dlqTopic,
		retryBackoff: max(retryBackoff, minRetryBackoff),
		log:          log,
	}
	if dlqTopic != "" {
		c.dlqWriter = &kafka.Writer{
			Addr:                   kafka.TCP(brokers...),
			Topic:                  dlqTopic,
			Balancer:               &kafka.LeastBytes{},
			RequiredAcks:           kafka.RequireAll,
			Async:                  false,
			BatchTimeout:           10 * time.Millisecond,
			AllowAutoTopicCreation: true, // nothing else creates the DLQ topic
			Transport: &kafka.Transport{
				SASL: plain.Mechanism{Username: saslUsername, Password: saslPassword},
			},
		}
	}
	return c
}

func (c *Consumer) Run(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		c.consume(ctx, c.readerUsers, "users", c.handleUserEvent)
	}()
	go func() {
		defer wg.Done()
		c.consume(ctx, c.readerPosts, "posts", c.handlePostEvent)
	}()
	wg.Wait()
}

func (c *Consumer) Close() error {
	_ = c.readerUsers.Close()
	_ = c.readerPosts.Close()
	if c.dlqWriter != nil {
		_ = c.dlqWriter.Close()
	}
	return nil
}

// consume handles one message at a time and commits it only once it is indexed
// or parked, so a commit never covers a message that was skipped.
func (c *Consumer) consume(ctx context.Context, reader messageReader, name string, handle func(context.Context, IndexEvent) error) {
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.log.Warn(fmt.Sprintf("kafka %s fetch: %v", name, err))
			if !sleep(ctx, time.Second) {
				return
			}
			continue
		}
		if !c.process(ctx, name, msg, handle) {
			return // shutting down: msg stays uncommitted and is delivered again
		}
		if err := reader.CommitMessages(ctx, msg); err != nil {
			// Already indexed or parked; a lost commit only means a redelivery,
			// and indexing by id is idempotent.
			c.log.Warn(fmt.Sprintf("kafka %s commit offset %d: %v", name, msg.Offset, err))
		}
	}
}

// process works on msg until it is indexed or parked in the DLQ: transient
// failures and failed parks are retried on this same message. It returns false
// only when ctx is done first.
func (c *Consumer) process(ctx context.Context, name string, msg kafka.Message, handle func(context.Context, IndexEvent) error) bool {
	var ev IndexEvent
	var parsed *IndexEvent
	err := json.Unmarshal(msg.Value, &ev)
	if err != nil {
		err = permanentError{fmt.Errorf("unmarshal event: %w", err)}
	} else {
		parsed = &ev
		err = c.handleWithRetry(ctx, name, handle, ev)
	}
	if err == nil {
		return true
	}
	if ctx.Err() != nil {
		return false
	}

	c.log.Warn(fmt.Sprintf("kafka %s offset %d: parking: %v", name, msg.Offset, err))
	for attempt := 0; ; attempt++ {
		parkErr := c.parkFailedMessage(ctx, name, msg, parsed, err)
		if parkErr == nil {
			return true
		}
		if ctx.Err() != nil {
			return false
		}
		c.log.Error(fmt.Sprintf("kafka %s offset %d: failed to park, retrying: %v", name, msg.Offset, parkErr))
		if !c.wait(ctx, attempt) {
			return false
		}
	}
}

// handleWithRetry runs handle until it succeeds, fails permanently or ctx is
// done. Transient failures (network, timeouts, OpenSearch 5xx and 429) are
// retried without limit: losing an event is worse than stalling the partition.
func (c *Consumer) handleWithRetry(ctx context.Context, name string, handle func(context.Context, IndexEvent) error, ev IndexEvent) error {
	for attempt := 0; ; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, indexCallTimeout)
		err := handle(callCtx, ev)
		cancel()
		if err == nil || isPermanent(err) || ctx.Err() != nil {
			return err
		}
		c.log.Warn(fmt.Sprintf("kafka %s %s %s: retrying: %v", name, ev.EventType, ev.EntityID, err))
		if !c.wait(ctx, attempt) {
			return ctx.Err()
		}
	}
}

// permanentError marks a failure that retrying cannot fix; the message is parked.
type permanentError struct{ err error }

func (e permanentError) Error() string { return e.err.Error() }
func (e permanentError) Unwrap() error { return e.err }

func isPermanent(err error) bool {
	var p permanentError
	return errors.As(err, &p) || opensearch.IsPermanent(err)
}

func (c *Consumer) handleUserEvent(ctx context.Context, ev IndexEvent) error {
	switch ev.EventType {
	case "created", "updated":
		var p UserPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return permanentError{fmt.Errorf("unmarshal payload: %w", err)}
		}
		doc := map[string]interface{}{
			"id":      p.ID,
			"name":    p.Name,
			"picture": p.Picture,
			"bio":     p.Bio,
		}
		return c.os.IndexDocument(ctx, c.usersIndex, p.ID, doc)
	case "deleted":
		return c.os.DeleteDocument(ctx, c.usersIndex, ev.EntityID)
	default:
		return nil
	}
}

func (c *Consumer) handlePostEvent(ctx context.Context, ev IndexEvent) error {
	switch ev.EventType {
	case "created", "updated":
		var p PostPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return permanentError{fmt.Errorf("unmarshal payload: %w", err)}
		}
		doc := map[string]interface{}{
			"id":        p.ID,
			"user_id":   p.UserID,
			"title":     p.Title,
			"slug":      p.Slug,
			"content":   p.Content,
			"published": p.Published,
		}
		return c.os.IndexDocument(ctx, c.postsIndex, p.ID, doc)
	case "deleted":
		return c.os.DeleteDocument(ctx, c.postsIndex, ev.EntityID)
	default:
		return nil
	}
}

func (c *Consumer) parkFailedMessage(ctx context.Context, source string, msg kafka.Message, ev *IndexEvent, processErr error) error {
	if c.dlqWriter == nil {
		return fmt.Errorf("DLQ is not configured: %w", processErr)
	}

	dlqMessage := map[string]interface{}{
		"source":      source,
		"topic":       msg.Topic,
		"partition":   msg.Partition,
		"offset":      msg.Offset,
		"error":       processErr.Error(),
		"failed_at":   time.Now().UTC().Format(time.RFC3339Nano),
		"raw_payload": string(msg.Value),
	}
	if ev != nil {
		dlqMessage["entity_type"] = ev.EntityType
		dlqMessage["event_type"] = ev.EventType
		dlqMessage["entity_id"] = ev.EntityID
		dlqMessage["message_id"] = ev.MessageID
	}

	payload, err := json.Marshal(dlqMessage)
	if err != nil {
		return fmt.Errorf("marshal DLQ message: %w", err)
	}

	// The writer already carries the DLQ topic. Setting Topic on the message as
	// well makes kafka-go reject the write, so failed events never park.
	return c.dlqWriter.WriteMessages(ctx, newDLQMessage(msg, payload))
}

func newDLQMessage(msg kafka.Message, payload []byte) kafka.Message {
	return kafka.Message{
		Key:   msg.Key,
		Value: payload,
	}
}

// wait sleeps before retry attempt n (from 0). It returns false if ctx is done first.
func (c *Consumer) wait(ctx context.Context, attempt int) bool {
	return sleep(ctx, c.backoff(attempt))
}

// backoff is retryBackoff doubled attempt times, capped at maxRetryBackoff.
func (c *Consumer) backoff(attempt int) time.Duration {
	d := c.retryBackoff
	for i := 0; i < attempt && d < maxRetryBackoff; i++ {
		d *= 2
	}
	return min(d, maxRetryBackoff)
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
