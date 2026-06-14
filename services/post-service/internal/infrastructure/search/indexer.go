// Package search publishes post change events to a Kafka topic that search-service
// consumes to index posts into OpenSearch. The wire format must match
// search-service's kafka.IndexEvent / PostPayload contract.
package search

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"

	"post-service/internal/domain/entities"
	"post-service/pkg/logger"
)

// indexEvent mirrors search-service/internal/infrastructure/kafka.IndexEvent.
type indexEvent struct {
	EntityType string      `json:"entity_type"` // always "post" here
	EventType  string      `json:"event_type"`  // "created" | "updated" | "deleted"
	EntityID   string      `json:"entity_id"`
	Payload    interface{} `json:"payload"`
	Timestamp  string      `json:"timestamp"` // ISO8601
	MessageID  string      `json:"message_id"`
}

// postPayload mirrors search-service's expected post payload.
type postPayload struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Title     string `json:"title"`
	Slug      string `json:"slug"`
	Content   string `json:"content"`
	Published bool   `json:"published"`
}

// Indexer publishes post index events to Kafka. Best-effort: failures are logged,
// never propagated to the caller (search indexing must not break writes).
type Indexer struct {
	writer *kafka.Writer
	log    *logger.Logger
}

// NewIndexer creates a Kafka-backed search indexer for the given topic.
func NewIndexer(brokers []string, topic string, log *logger.Logger) *Indexer {
	return &Indexer{
		writer: &kafka.Writer{
			Addr:                   kafka.TCP(brokers...),
			Topic:                  topic,
			Balancer:               &kafka.Hash{}, // key by entity id -> stable partition per post
			RequiredAcks:           kafka.RequireOne,
			AllowAutoTopicCreation: true,
		},
		log: log,
	}
}

func (i *Indexer) PostCreated(ctx context.Context, p *entities.Post) {
	i.publish(ctx, "created", p.ID, fromPost(p))
}

func (i *Indexer) PostUpdated(ctx context.Context, p *entities.Post) {
	i.publish(ctx, "updated", p.ID, fromPost(p))
}

// PostDeleted only needs the id; search-service deletes the document by entity_id.
func (i *Indexer) PostDeleted(ctx context.Context, postID string) {
	i.publish(ctx, "deleted", postID, postPayload{ID: postID})
}

func fromPost(p *entities.Post) postPayload {
	return postPayload{
		ID:        p.ID,
		UserID:    p.UserID,
		Title:     p.Title,
		Slug:      p.Slug,
		Content:   p.Content,
		Published: p.Published,
	}
}

func (i *Indexer) publish(ctx context.Context, eventType, entityID string, payload postPayload) {
	body, err := json.Marshal(indexEvent{
		EntityType: "post",
		EventType:  eventType,
		EntityID:   entityID,
		Payload:    payload,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		MessageID:  uuid.New().String(),
	})
	if err != nil {
		i.log.Error(fmt.Sprintf("search index marshal (%s %s): %v", eventType, entityID, err))
		return
	}

	wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := i.writer.WriteMessages(wctx, kafka.Message{Key: []byte(entityID), Value: body}); err != nil {
		i.log.Error(fmt.Sprintf("search index publish (%s %s): %v", eventType, entityID, err))
		return
	}
	i.log.Info(fmt.Sprintf("search index published: post %s %s", eventType, entityID))
}

// Close flushes and closes the underlying Kafka writer.
func (i *Indexer) Close() error {
	if i.writer != nil {
		return i.writer.Close()
	}
	return nil
}
