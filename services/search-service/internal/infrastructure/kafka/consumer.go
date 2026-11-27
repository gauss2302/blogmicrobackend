package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
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

// groupReader is a consumer-group member: Close leaves the group.
type groupReader interface {
	messageReader
	Close() error
}

// messageWriter is the part of *kafka.Writer the consumer uses for the DLQ.
type messageWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

// metadataTimeout bounds one topic metadata request while waiting for the topics.
const metadataTimeout = 10 * time.Second

type Consumer struct {
	topicUsers   string
	topicPosts   string
	topicsReady  func(ctx context.Context) error // nil once both topics have partitions
	newReader    func(topic string) groupReader  // joins the consumer group at once
	dlqWriter    messageWriter                   // nil when no DLQ topic is configured
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
	c := &Consumer{
		topicUsers: topicUsers,
		topicPosts: topicPosts,
		topicsReady: func(ctx context.Context) error {
			return readTopics(ctx, dialer, brokers, topicUsers, topicPosts)
		},
		newReader: func(topic string) groupReader {
			return kafka.NewReader(kafka.ReaderConfig{
				Brokers:        brokers,
				GroupID:        groupID,
				Topic:          topic,
				Dialer:         dialer,
				MinBytes:       1,
				MaxBytes:       10e6,
				MaxWait:        1 * time.Second,
				CommitInterval: 0,
			})
		},
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

// Run indexes both topics until ctx is done, then leaves the group. It joins
// the group only once the broker reports both topics and indicesReady is
// closed. A member that joins while one of the group's topics is unknown gets
// no partitions: kafka-go's leader hands out an empty assignment when its
// metadata request fails, and nothing rebalances once the topic appears.
func (c *Consumer) Run(ctx context.Context, indicesReady <-chan struct{}) {
	if !c.waitForTopics(ctx) {
		return
	}
	select {
	case <-indicesReady:
	case <-ctx.Done():
		return
	}

	users, posts := c.newReader(c.topicUsers), c.newReader(c.topicPosts)
	defer func() {
		_ = users.Close()
		_ = posts.Close()
	}()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		c.consume(ctx, users, "users", c.handleUserEvent)
	}()
	go func() {
		defer wg.Done()
		c.consume(ctx, posts, "posts", c.handlePostEvent)
	}()
	wg.Wait()
}

// Close closes the DLQ writer. Run closes the readers itself, so call Close
// after Run has returned.
func (c *Consumer) Close() error {
	if c.dlqWriter != nil {
		return c.dlqWriter.Close()
	}
	return nil
}

// waitForTopics retries topicsReady with a growing pause until it succeeds
// (true) or ctx is done (false).
func (c *Consumer) waitForTopics(ctx context.Context) bool {
	for attempt := 0; ; attempt++ {
		err := c.topicsReady(ctx)
		if err == nil {
			if attempt > 0 {
				c.log.Info(fmt.Sprintf("kafka topics %s, %s ready", c.topicUsers, c.topicPosts))
			}
			return true
		}
		if ctx.Err() != nil {
			return false
		}
		d := c.backoff(attempt)
		c.log.Warn(fmt.Sprintf("kafka topics %s, %s not ready, retrying in %s: %v", c.topicUsers, c.topicPosts, d, err))
		if !sleep(ctx, d) {
			return false
		}
	}
}

// readTopics asks the brokers in turn for the topics' partitions with the same
// request a group leader makes before assigning them (Conn.ReadPartitions:
// metadata with AllowAutoTopicCreation, any topic error fails the whole call),
// so nil means the leader sees every topic too. That request also creates a
// missing topic when the broker allows auto-creation.
//
// ponytail: asks one broker; in a multi-broker cluster the group coordinator
// may learn about a new topic a moment later and still assign nothing. Compose
// and k8s run a single broker.
func readTopics(ctx context.Context, dialer *kafka.Dialer, brokers []string, topics ...string) error {
	err := errors.New("no brokers configured")
	for _, broker := range brokers {
		var conn *kafka.Conn
		if conn, err = dialer.DialContext(ctx, "tcp", broker); err != nil {
			continue
		}
		_ = conn.SetDeadline(time.Now().Add(metadataTimeout))
		var parts []kafka.Partition
		parts, err = conn.ReadPartitions(topics...)
		_ = conn.Close()
		if err == nil {
			if missing := missingTopics(parts, topics); len(missing) > 0 {
				return fmt.Errorf("no partitions for %v", missing)
			}
			return nil
		}
	}
	return err
}

// missingTopics returns the topics that have no partition in parts.
func missingTopics(parts []kafka.Partition, topics []string) []string {
	var missing []string
	for _, t := range topics {
		if !slices.ContainsFunc(parts, func(p kafka.Partition) bool { return p.Topic == t }) {
			missing = append(missing, t)
		}
	}
	return missing
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
