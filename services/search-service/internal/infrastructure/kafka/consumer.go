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
	readerUsers *kafka.Reader
	readerPosts *kafka.Reader
	os          *opensearch.Client
	usersIndex  string
	postsIndex  string
	log         *logger.Logger
}

func NewConsumer(brokers []string, groupID, topicUsers, topicPosts, usersIndex, postsIndex string, osClient *opensearch.Client, log *logger.Logger) *Consumer {
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
	return &Consumer{
		readerUsers: readerUsers,
		readerPosts: readerPosts,
		os:          osClient,
		usersIndex:  usersIndex,
		postsIndex:  postsIndex,
		log:         log,
	}
}

func (c *Consumer) Run(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		c.consume(ctx, c.readerUsers, "users", c.usersIndex, c.handleUserEvent)
	}()
	go func() {
		defer wg.Done()
		c.consume(ctx, c.readerPosts, "posts", c.postsIndex, c.handlePostEvent)
	}()
	wg.Wait()
}

func (c *Consumer) Close() error {
	_ = c.readerUsers.Close()
	_ = c.readerPosts.Close()
	return nil
}

func (c *Consumer) consume(ctx context.Context, reader *kafka.Reader, name, index string, handle func(context.Context, IndexEvent) error) {
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
			_ = reader.CommitMessages(ctx, msg)
			continue
		}
		if err := handle(ctx, ev); err != nil {
			c.log.Warn(fmt.Sprintf("kafka %s handle %s %s: %v", name, ev.EventType, ev.EntityID, err))
		}
		_ = reader.CommitMessages(ctx, msg)
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
