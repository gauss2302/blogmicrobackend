// Package search keeps search-service's users index in step with the users
// table. Triggers add a row to search_outbox in the same transaction as every
// write that changes what search sees (OutboxDDL). Relay sends the current
// state of those users to Kafka and deletes the rows only once Kafka has them,
// so an event survives a Kafka outage and a restart, and no write waits for
// Kafka. The wire format matches search-service's kafka.IndexEvent / UserPayload.
// Email is not part of that contract and is never published.
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

	"user-service/pkg/logger"
)

// stateColumns are the users columns search sees: the payload plus is_active,
// which decides between indexing and deleting (users are deleted softly). The
// UPDATE trigger fires only when one of them changes, and the relay reads
// exactly these, so the two cannot drift apart. TestStateColumnsMatchPayload
// ties them to userPayload.
var stateColumns = []string{"name", "picture", "bio", "is_active"}

const (
	batchSize    = 100
	writeTimeout = 10 * time.Second
	pollInterval = 500 * time.Millisecond
	minBackoff   = time.Second
	maxBackoff   = 30 * time.Second
	// relayLockKey is the advisory lock that lets one relay at a time send, so
	// overlapping replicas cannot reorder a user's events. Any constant works:
	// the users database holds nothing else that uses advisory locks.
	relayLockKey = 7_200_001
)

// OutboxDDL creates search_outbox and the triggers that fill it. Pass it to
// postgres.RunMigrations: it needs the users table and runs in that migration's
// transaction and lock (two separately locked migrations deadlock on users).
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

	DROP TRIGGER IF EXISTS users_search_outbox_insert_delete ON users;
	CREATE TRIGGER users_search_outbox_insert_delete
		AFTER INSERT OR DELETE ON users
		FOR EACH ROW EXECUTE FUNCTION search_outbox_enqueue();

	DROP TRIGGER IF EXISTS users_search_outbox_update ON users;
	CREATE TRIGGER users_search_outbox_update
		AFTER UPDATE ON users
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
	EntityType string      `json:"entity_type"` // always "user" here
	EventType  string      `json:"event_type"`  // "updated" | "deleted"
	EntityID   string      `json:"entity_id"`
	Payload    interface{} `json:"payload"`
	Timestamp  string      `json:"timestamp"` // ISO8601
	MessageID  string      `json:"message_id"`
}

type userPayload struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
	Bio     string `json:"bio"`
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
		Balancer:               &kafka.Hash{}, // key by user id -> stable partition per user
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

	rowIDs, userIDs, err := pendingRows(ctx, tx)
	if err != nil || len(rowIDs) == 0 {
		return 0, err
	}
	msgs, err := r.messages(ctx, tx, userIDs)
	if err != nil {
		return 0, err
	}

	wctx, cancel := context.WithTimeout(ctx, writeTimeout)
	err = r.writer.WriteMessages(wctx, msgs...)
	cancel()
	if err != nil {
		return 0, fmt.Errorf("kafka write (%d users): %w", len(msgs), err)
	}

	// Exactly the rows read: a lower id from a transaction that commits later
	// must not be deleted unsent.
	if _, err := tx.ExecContext(ctx, `DELETE FROM search_outbox WHERE id = ANY($1)`, pq.Array(rowIDs)); err != nil {
		return 0, fmt.Errorf("delete sent rows: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	r.log.Info(fmt.Sprintf("search outbox: sent %d users", len(msgs)))
	return len(rowIDs), nil
}

// pendingRows reads the oldest batch: outbox row ids, and the distinct user
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

// messages builds one message per user from its state now: an active user is
// indexed, a deleted (inactive or missing) one is removed from the index.
func (r *Relay) messages(ctx context.Context, tx *sql.Tx, userIDs []string) ([]kafka.Message, error) {
	query := `SELECT id, ` + strings.Join(stateColumns, ", ") + ` FROM users WHERE id = ANY($1)`
	rows, err := tx.QueryContext(ctx, query, pq.Array(userIDs))
	if err != nil {
		return nil, fmt.Errorf("read users: %w", err)
	}
	defer rows.Close()

	type state struct {
		payload userPayload
		active  bool
	}
	current := map[string]state{}
	for rows.Next() {
		var s state
		var name, picture, bio sql.NullString // picture and bio may be NULL
		// Same order as stateColumns.
		if err := rows.Scan(&s.payload.ID, &name, &picture, &bio, &s.active); err != nil {
			return nil, fmt.Errorf("read users: %w", err)
		}
		s.payload.Name, s.payload.Picture, s.payload.Bio = name.String, picture.String, bio.String
		current[s.payload.ID] = s
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read users: %w", err)
	}

	msgs := make([]kafka.Message, 0, len(userIDs))
	for _, id := range userIDs {
		eventType, payload := "deleted", userPayload{ID: id}
		if s, ok := current[id]; ok && s.active {
			eventType, payload = "updated", s.payload
		}
		msg, err := message(eventType, id, payload)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, msg)
	}
	return msgs, nil
}

func message(eventType, entityID string, payload userPayload) (kafka.Message, error) {
	body, err := json.Marshal(indexEvent{
		EntityType: "user",
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
