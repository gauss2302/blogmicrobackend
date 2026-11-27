// Package search keeps search-service's posts index in step with the posts
// table. Triggers add a row to search_outbox in the same transaction as every
// write that changes what search sees (OutboxDDL). Relay sends the current
// state of those posts to Kafka and deletes the rows only once Kafka has them,
// so an event survives a Kafka outage and a restart, and no write waits for
// Kafka. The wire format matches search-service's kafka.IndexEvent / PostPayload.
package search

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"

	"post-service/pkg/logger"
)

// stateColumns are the posts columns search sees: the payload plus published,
// which decides between indexing and deleting. The UPDATE trigger fires only
// when one of them changes, and the relay reads exactly these, so the two
// cannot drift apart. TestStateColumnsMatchPayload ties them to postPayload.
var stateColumns = []string{"user_id", "title", "slug", "content", "published"}

const (
	batchSize    = 100
	writeTimeout = 10 * time.Second
	pollInterval = 500 * time.Millisecond
	minBackoff   = time.Second
	maxBackoff   = 30 * time.Second
	// relayLockKey is the advisory lock that lets one relay at a time send, so
	// overlapping replicas cannot reorder a post's events. Any constant works:
	// the posts database holds nothing else that uses advisory locks.
	relayLockKey = 7_200_001
)

// OutboxDDL creates search_outbox and the triggers that fill it. Pass it to
// postgres.RunMigrations: it needs the posts table and runs in that migration's
// transaction and lock (two separately locked migrations deadlock on posts).
// Safe to run again.
func OutboxDDL() string {
	oldCols := "OLD." + strings.Join(stateColumns, ", OLD.")
	newCols := "NEW." + strings.Join(stateColumns, ", NEW.")
	return fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS search_outbox (
		id BIGSERIAL PRIMARY KEY,
		entity_id VARCHAR(255) NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT now()
	);

	CREATE OR REPLACE FUNCTION search_outbox_enqueue() RETURNS TRIGGER AS $$
	BEGIN
		IF TG_OP = 'DELETE' THEN
			INSERT INTO search_outbox (entity_id) VALUES (OLD.id);
		ELSE
			INSERT INTO search_outbox (entity_id) VALUES (NEW.id);
		END IF;
		RETURN NULL;
	END;
	$$ LANGUAGE plpgsql;

	DROP TRIGGER IF EXISTS posts_search_outbox_insert_delete ON posts;
	CREATE TRIGGER posts_search_outbox_insert_delete
		AFTER INSERT OR DELETE ON posts
		FOR EACH ROW EXECUTE FUNCTION search_outbox_enqueue();

	DROP TRIGGER IF EXISTS posts_search_outbox_update ON posts;
	CREATE TRIGGER posts_search_outbox_update
		AFTER UPDATE ON posts
		FOR EACH ROW WHEN ((%s) IS DISTINCT FROM (%s))
		EXECUTE FUNCTION search_outbox_enqueue();
	`, oldCols, newCols)
}

// messageWriter is the part of *kafka.Writer the relay uses; tests fake it.
type messageWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

type indexEvent struct {
	EntityType string      `json:"entity_type"` // always "post" here
	EventType  string      `json:"event_type"`  // "updated" | "deleted"
	EntityID   string      `json:"entity_id"`
	Payload    interface{} `json:"payload"`
	Timestamp  string      `json:"timestamp"` // ISO8601
	MessageID  string      `json:"message_id"`
}

type postPayload struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Title     string `json:"title"`
	Slug      string `json:"slug"`
	Content   string `json:"content"`
	Published bool   `json:"published"`
}

// Relay drains search_outbox into Kafka. Run it in one goroutine per process.
type Relay struct {
	db     *sql.DB
	writer messageWriter
	log    *logger.Logger
	poll   time.Duration
}

func NewRelay(db *sql.DB, brokers []string, topic, saslUsername, saslPassword string, log *logger.Logger) *Relay {
	return newRelay(db, &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Topic:                  topic,
		Balancer:               &kafka.Hash{}, // key by post id -> stable partition per post
		RequiredAcks:           kafka.RequireOne,
		AllowAutoTopicCreation: true,
		// kafka-go's default BatchTimeout is 1s; a small batch would wait for it.
		BatchTimeout: 10 * time.Millisecond,
		Transport: &kafka.Transport{
			SASL: plain.Mechanism{Username: saslUsername, Password: saslPassword},
		},
	}, log)
}

func newRelay(db *sql.DB, w messageWriter, log *logger.Logger) *Relay {
	return &Relay{db: db, writer: w, log: log, poll: pollInterval}
}

// Run sends outbox batches until ctx is done. A failed batch stays in the
// outbox and is retried with a growing pause.
func (r *Relay) Run(ctx context.Context) {
	backoff := minBackoff
	for {
		n, err := r.relayOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		wait := r.poll
		switch {
		case err != nil:
			r.log.Error(fmt.Sprintf("search outbox: %v; %s pending, retrying in %s", err, r.pendingText(), backoff))
			wait = backoff
			backoff = min(backoff*2, maxBackoff)
		case n == batchSize:
			wait = 0 // more rows are waiting
			backoff = minBackoff
		default:
			backoff = minBackoff
		}
		if wait > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
		}
	}
}

// relayOnce sends one batch and returns how many outbox rows it consumed.
func (r *Relay) relayOnce(ctx context.Context) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // a no-op after Commit

	var locked bool
	if err := tx.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock($1)`, relayLockKey).Scan(&locked); err != nil {
		return 0, fmt.Errorf("lock: %w", err)
	}
	if !locked {
		return 0, nil // another replica is sending
	}

	rowIDs, postIDs, err := pendingRows(ctx, tx)
	if err != nil || len(rowIDs) == 0 {
		return 0, err
	}
	msgs, err := r.messages(ctx, tx, postIDs)
	if err != nil {
		return 0, err
	}

	wctx, cancel := context.WithTimeout(ctx, writeTimeout)
	err = r.writer.WriteMessages(wctx, msgs...)
	cancel()
	if err != nil {
		return 0, fmt.Errorf("kafka write (%d posts): %w", len(msgs), err)
	}

	// Exactly the rows read: a lower id from a transaction that commits later
	// must not be deleted unsent.
	if _, err := tx.ExecContext(ctx, `DELETE FROM search_outbox WHERE id = ANY($1)`, pq.Array(rowIDs)); err != nil {
		return 0, fmt.Errorf("delete sent rows: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	r.log.Info(fmt.Sprintf("search outbox: sent %d posts", len(msgs)))
	return len(rowIDs), nil
}

// pendingRows reads the oldest batch: outbox row ids, and the distinct post
// ids in first-seen order.
func pendingRows(ctx context.Context, tx *sql.Tx) ([]int64, []string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, entity_id FROM search_outbox ORDER BY id LIMIT $1`, batchSize)
	if err != nil {
		return nil, nil, fmt.Errorf("read outbox: %w", err)
	}
	defer rows.Close()

	var rowIDs []int64
	var entityIDs []string
	seen := map[string]bool{}
	for rows.Next() {
		var id int64
		var entityID string
		if err := rows.Scan(&id, &entityID); err != nil {
			return nil, nil, fmt.Errorf("read outbox: %w", err)
		}
		rowIDs = append(rowIDs, id)
		if !seen[entityID] {
			seen[entityID] = true
			entityIDs = append(entityIDs, entityID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("read outbox: %w", err)
	}
	return rowIDs, entityIDs, nil
}

// messages builds one message per post from its state now: a published post
// is indexed, a draft or a deleted post is removed from the index.
func (r *Relay) messages(ctx context.Context, tx *sql.Tx, postIDs []string) ([]kafka.Message, error) {
	query := `SELECT id, ` + strings.Join(stateColumns, ", ") + ` FROM posts WHERE id = ANY($1)`
	rows, err := tx.QueryContext(ctx, query, pq.Array(postIDs))
	if err != nil {
		return nil, fmt.Errorf("read posts: %w", err)
	}
	defer rows.Close()

	current := map[string]postPayload{}
	for rows.Next() {
		var p postPayload
		// Same order as stateColumns.
		if err := rows.Scan(&p.ID, &p.UserID, &p.Title, &p.Slug, &p.Content, &p.Published); err != nil {
			return nil, fmt.Errorf("read posts: %w", err)
		}
		current[p.ID] = p
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read posts: %w", err)
	}

	msgs := make([]kafka.Message, 0, len(postIDs))
	for _, id := range postIDs {
		eventType, payload := "deleted", postPayload{ID: id}
		if p, ok := current[id]; ok && p.Published {
			eventType, payload = "updated", p
		}
		msg, err := message(eventType, id, payload)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, msg)
	}
	return msgs, nil
}

func message(eventType, entityID string, payload postPayload) (kafka.Message, error) {
	body, err := json.Marshal(indexEvent{
		EntityType: "post",
		EventType:  eventType,
		EntityID:   entityID,
		Payload:    payload,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		MessageID:  uuid.New().String(),
	})
	if err != nil {
		return kafka.Message{}, fmt.Errorf("marshal %s %s: %w", eventType, entityID, err)
	}
	return kafka.Message{Key: []byte(entityID), Value: body}, nil
}

// pendingText is the outbox backlog for logs; "?" when it cannot be read.
func (r *Relay) pendingText() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var n int64
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM search_outbox`).Scan(&n); err != nil {
		return "?"
	}
	return fmt.Sprint(n)
}

// Close closes the Kafka writer. Stop Run (cancel its context) first.
func (r *Relay) Close() error {
	return r.writer.Close()
}
