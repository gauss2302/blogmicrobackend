package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"search-service/internal/infrastructure/opensearch"
	"search-service/pkg/logger"
)

type Consumer struct {
	readerUsers          *kafka.Reader
	readerPosts          *kafka.Reader
	dlqWriter            *kafka.Writer
	os                   *opensearch.Client
	usersIndex           string
	postsIndex           string
	dlqTopic             string
	maxProcessingRetries int
	retryBackoff         time.Duration
	log                  *logger.Logger
}

func NewConsumer(
	brokers []string,
	groupID, topicUsers, topicPosts, dlqTopic, usersIndex, postsIndex string,
	maxProcessingRetries int,
	retryBackoff time.Duration,
	osClient *opensearch.Client,
	log *logger.Logger,
) *Consumer {
	readerUsers := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		GroupID:        groupID,
		Topic:          topicUsers,
		MinBytes:       1,
		MaxBytes:       10e6,
		MaxWait:        1 * time.Second,
		CommitInterval: 0,
	})
	readerPosts := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		GroupID:        groupID,
		Topic:          topicPosts,
		MinBytes:       1,
		MaxBytes:       10e6,
		MaxWait:        1 * time.Second,
		CommitInterval: 0,
	})

	var dlqWriter *kafka.Writer
	if dlqTopic != "" {
		dlqWriter = &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        dlqTopic,
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireAll,
			Async:        false,
		}
	}

	if maxProcessingRetries < 1 {
		maxProcessingRetries = 1
	}

	return &Consumer{
		readerUsers:          readerUsers,
		readerPosts:          readerPosts,
		dlqWriter:            dlqWriter,
		os:                   osClient,
		usersIndex:           usersIndex,
		postsIndex:           postsIndex,
		dlqTopic:             dlqTopic,
		maxProcessingRetries: maxProcessingRetries,
		retryBackoff:         retryBackoff,
		log:                  log,
	}
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

func (c *Consumer) consume(ctx context.Context, reader *kafka.Reader, name string, handle func(context.Context, IndexEvent) error) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.log.Warn(fmt.Sprintf("kafka %s fetch: %v", name, err))
			time.Sleep(time.Second)
			continue
		}
		var ev IndexEvent
		if err := json.Unmarshal(msg.Value, &ev); err != nil {
			c.log.Warn(fmt.Sprintf("kafka %s unmarshal: %v", name, err))
			if parkErr := c.parkFailedMessage(ctx, name, msg, nil, err); parkErr != nil {
				c.log.Error(fmt.Sprintf("kafka %s failed to park malformed event: %v", name, parkErr))
				c.waitBeforeRetry()
				continue
			}
			if commitErr := reader.CommitMessages(ctx, msg); commitErr != nil {
				c.log.Warn(fmt.Sprintf("kafka %s commit malformed event: %v", name, commitErr))
			}
			continue
		}

		if err := c.handleWithRetry(ctx, handle, ev); err != nil {
			c.log.Warn(fmt.Sprintf("kafka %s handle %s %s: %v", name, ev.EventType, ev.EntityID, err))
			if parkErr := c.parkFailedMessage(ctx, name, msg, &ev, err); parkErr != nil {
				c.log.Error(fmt.Sprintf("kafka %s failed to park event: %v", name, parkErr))
				c.waitBeforeRetry()
				continue
			}
		}

		if commitErr := reader.CommitMessages(ctx, msg); commitErr != nil {
			c.log.Warn(fmt.Sprintf("kafka %s commit message: %v", name, commitErr))
			c.waitBeforeRetry()
			continue
		}
	}
}

func (c *Consumer) handleUserEvent(ctx context.Context, ev IndexEvent) error {
	switch ev.EventType {
	case "created", "updated":
		var p UserPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
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
			return err
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

func (c *Consumer) handleWithRetry(ctx context.Context, handle func(context.Context, IndexEvent) error, ev IndexEvent) error {
	var lastErr error
	for attempt := 1; attempt <= c.maxProcessingRetries; attempt++ {
		lastErr = handle(ctx, ev)
		if lastErr == nil {
			return nil
		}
		if attempt < c.maxProcessingRetries {
			c.log.Warn(fmt.Sprintf(
				"kafka retry %d/%d for event %s %s: %v",
				attempt,
				c.maxProcessingRetries,
				ev.EventType,
				ev.EntityID,
				lastErr,
			))
			c.waitBeforeRetry()
		}
	}
	return fmt.Errorf("processing failed after %d attempts: %w", c.maxProcessingRetries, lastErr)
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

	return c.dlqWriter.WriteMessages(ctx, kafka.Message{
		Topic: c.dlqTopic,
		Key:   msg.Key,
		Value: payload,
	})
}

func (c *Consumer) waitBeforeRetry() {
	if c.retryBackoff <= 0 {
		return
	}
	time.Sleep(c.retryBackoff)
}
