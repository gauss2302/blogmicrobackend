// Package search publishes user change events to the Kafka topic search-service
// consumes to index users into OpenSearch. The wire format matches
// search-service's kafka.IndexEvent / UserPayload contract. Email is not part
// of that contract and is never published.
package search

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"

	"user-service/internal/domain/entities"
	"user-service/pkg/logger"
)

// kafkaBatchTimeout overrides kafka-go's 1s default. A single-message write
// otherwise waits for a batch that never fills, adding about a second to every
// user create, update, and delete.
const kafkaBatchTimeout = 10 * time.Millisecond

type indexEvent struct {
	EntityType string      `json:"entity_type"`
	EventType  string      `json:"event_type"`
	EntityID   string      `json:"entity_id"`
	Payload    interface{} `json:"payload"`
	Timestamp  string      `json:"timestamp"`
	MessageID  string      `json:"message_id"`
}

type userPayload struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
	Bio     string `json:"bio"`
}

// Indexer publishes user index events to Kafka. Best-effort: failures are logged
// and never returned to the caller, so a broker outage cannot fail a user write.
type Indexer struct {
	writer *kafka.Writer
	log    *logger.Logger
}

func NewIndexer(brokers []string, topic string, log *logger.Logger) *Indexer {
	return &Indexer{
		writer: &kafka.Writer{
			Addr:                   kafka.TCP(brokers...),
			Topic:                  topic,
			Balancer:               &kafka.Hash{},
			RequiredAcks:           kafka.RequireOne,
			AllowAutoTopicCreation: true,
			BatchTimeout:           kafkaBatchTimeout,
		},
		log: log,
	}
}

func (i *Indexer) UserCreated(ctx context.Context, user *entities.User) {
	i.publish(ctx, "created", user.ID, fromUser(user))
}

func (i *Indexer) UserUpdated(ctx context.Context, user *entities.User) {
	i.publish(ctx, "updated", user.ID, fromUser(user))
}

func (i *Indexer) UserDeleted(ctx context.Context, userID string) {
	i.publish(ctx, "deleted", userID, userPayload{ID: userID})
}

func fromUser(user *entities.User) userPayload {
	return userPayload{
		ID:      user.ID,
		Name:    user.Name,
		Picture: user.Picture,
		Bio:     user.Bio,
	}
}

func (i *Indexer) publish(ctx context.Context, eventType, entityID string, payload userPayload) {
	body, err := json.Marshal(indexEvent{
		EntityType: "user",
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
	i.log.Info(fmt.Sprintf("search index published: user %s %s", eventType, entityID))
}

func (i *Indexer) Close() error {
	if i.writer != nil {
		return i.writer.Close()
	}
	return nil
}
