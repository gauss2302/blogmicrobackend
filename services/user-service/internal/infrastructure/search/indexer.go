// Package search publishes user change events to the Kafka topic search-service
// consumes to index users into OpenSearch. The wire format matches
// search-service's kafka.IndexEvent / UserPayload contract. Email is not part
// of that contract and is never published.
package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"

	"user-service/internal/domain/entities"
	"user-service/pkg/logger"
)

// kafkaBatchTimeout overrides kafka-go's 1s default. A single-message write
// otherwise waits for a batch that never fills, adding about a second to every
// user create, update, and delete.
const kafkaBatchTimeout = 10 * time.Millisecond

const (
	// queueSize bounds memory while Kafka is down; request events past it are
	// dropped and logged.
	queueSize    = 1024
	writeTimeout = 5 * time.Second
	// drainTimeout bounds Close: queued events get this long to reach Kafka.
	drainTimeout = 10 * time.Second
)

var errIndexerClosed = errors.New("search indexer closed")

// messageWriter is the part of *kafka.Writer the indexer uses; tests fake it.
type messageWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

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
//
// Events go through a queue drained by one goroutine, so a user write never
// waits for Kafka and never depends on the request context, while one sender
// keeps each user's created/updated/deleted events in order. kafka.Writer's
// Async mode is not a substitute: in kafka-go v0.4.47 WriteMessages fetches
// partition metadata (writer.go:651) before it checks Async (writer.go:667), so
// a down broker would still block the request.
type Indexer struct {
	writer       messageWriter
	log          *logger.Logger
	queue        chan kafka.Message
	stop         chan struct{} // closed by Close
	done         chan struct{} // closed when run returns
	drainTimeout time.Duration
	ctx          context.Context // cancelled by Close after drainTimeout
	cancel       context.CancelFunc
	dropped      int // owned by run
	closeOnce    sync.Once
	closeErr     error
}

func NewIndexer(brokers []string, topic, saslUsername, saslPassword string, log *logger.Logger) *Indexer {
	return newIndexer(&kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Topic:                  topic,
		Balancer:               &kafka.Hash{},
		RequiredAcks:           kafka.RequireOne,
		AllowAutoTopicCreation: true,
		BatchTimeout:           kafkaBatchTimeout,
		Transport: &kafka.Transport{
			SASL: plain.Mechanism{Username: saslUsername, Password: saslPassword},
		},
	}, log, drainTimeout)
}

func newIndexer(w messageWriter, log *logger.Logger, drain time.Duration) *Indexer {
	i := &Indexer{
		writer:       w,
		log:          log,
		queue:        make(chan kafka.Message, queueSize),
		stop:         make(chan struct{}),
		done:         make(chan struct{}),
		drainTimeout: drain,
	}
	i.ctx, i.cancel = context.WithCancel(context.Background())
	go i.run()
	return i
}

// The request context is deliberately unused: a cancelled request must not
// lose an event for a write that already committed.

func (i *Indexer) UserCreated(_ context.Context, user *entities.User) {
	i.enqueue("created", user.ID, fromUser(user))
}

func (i *Indexer) UserUpdated(_ context.Context, user *entities.User) {
	i.enqueue("updated", user.ID, fromUser(user))
}

func (i *Indexer) UserDeleted(_ context.Context, userID string) {
	i.enqueue("deleted", userID, userPayload{ID: userID})
}

// BackfillUser queues a "created" event and, unlike the request path, waits
// for room in the queue, so a backfill larger than the queue loses nothing.
// It gives up when ctx is done or the indexer is closed.
func (i *Indexer) BackfillUser(ctx context.Context, user *entities.User) error {
	msg, ok := i.message("created", user.ID, fromUser(user))
	if !ok {
		return nil
	}
	select {
	case i.queue <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-i.stop:
		return errIndexerClosed
	}
}

func fromUser(user *entities.User) userPayload {
	return userPayload{
		ID:      user.ID,
		Name:    user.Name,
		Picture: user.Picture,
		Bio:     user.Bio,
	}
}

func (i *Indexer) message(eventType, entityID string, payload userPayload) (kafka.Message, bool) {
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
		return kafka.Message{}, false
	}
	return kafka.Message{Key: []byte(entityID), Value: body}, true
}

// enqueue never blocks: with the queue full or the indexer closed the event is
// dropped and logged.
func (i *Indexer) enqueue(eventType, entityID string, payload userPayload) {
	msg, ok := i.message(eventType, entityID, payload)
	if !ok {
		return
	}
	select {
	case <-i.stop:
		i.log.Error(fmt.Sprintf("search index dropped (%s %s): indexer closed", eventType, entityID))
		return
	default:
	}
	select {
	case i.queue <- msg:
	default:
		i.log.Error(fmt.Sprintf("search index dropped (%s %s): queue full", eventType, entityID))
	}
}

func (i *Indexer) run() {
	defer close(i.done)
	for {
		select {
		case msg := <-i.queue:
			i.write(msg)
		case <-i.stop:
			for {
				select {
				case msg := <-i.queue:
					i.write(msg)
				default:
					if i.dropped > 0 {
						i.log.Error(fmt.Sprintf("search index: %d events dropped at shutdown", i.dropped))
					}
					return
				}
			}
		}
	}
}

func (i *Indexer) write(msg kafka.Message) {
	if i.ctx.Err() != nil {
		i.dropped++
		return
	}
	wctx, cancel := context.WithTimeout(i.ctx, writeTimeout)
	defer cancel()
	if err := i.writer.WriteMessages(wctx, msg); err != nil {
		i.log.Error(fmt.Sprintf("search index publish (user %s): %v", msg.Key, err))
		return
	}
	i.log.Info(fmt.Sprintf("search index published: user %s", msg.Key))
}

// Close stops accepting events, sends what is queued within drainTimeout, and
// closes the Kafka writer. Safe to call more than once.
func (i *Indexer) Close() error {
	i.closeOnce.Do(func() {
		close(i.stop)
		// The deadline also cancels a write already in flight.
		deadline := time.AfterFunc(i.drainTimeout, i.cancel)
		<-i.done
		deadline.Stop()
		i.cancel()
		i.closeErr = i.writer.Close()
	})
	return i.closeErr
}
