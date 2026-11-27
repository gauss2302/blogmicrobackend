package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"notification-service/internal/domain/entities"
	"notification-service/internal/domain/repositories"
)

// Integration test against a real Postgres. It works in a schema of its own,
// dropped at the end. Start one with
//
//	docker run -d --rm -e POSTGRES_PASSWORD=test -p 127.0.0.1::5432 postgres:16-alpine
//
// and set TEST_DATABASE_URL=postgres://postgres:test@127.0.0.1:<port>/postgres?sslmode=disable.

// oldNotificationsTable is the table as created before event_id existed.
const oldNotificationsTable = `
CREATE TABLE notifications (
	id VARCHAR(255) PRIMARY KEY,
	user_id VARCHAR(255) NOT NULL,
	type VARCHAR(50) NOT NULL,
	title VARCHAR(200) NOT NULL,
	message VARCHAR(1000) NOT NULL,
	data JSONB,
	read BOOLEAN DEFAULT false,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	read_at TIMESTAMP NULL
)`

func TestCreateIsIdempotentPerEvent(t *testing.T) {
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db := openTestSchema(t, raw)
	ctx := context.Background()

	// A row from before the migration must survive it, with a NULL event_id.
	if _, err := db.Exec(oldNotificationsTable); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO notifications (id, user_id, type, title, message) VALUES ('old', 'u', 'post_created', 't', 'm')`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ { // the migration is idempotent
		if err := RunMigrations(db); err != nil {
			t.Fatalf("RunMigrations #%d: %v", i+1, err)
		}
	}
	if n := count(t, db, `id = 'old' AND event_id IS NULL`); n != 1 {
		t.Fatalf("pre-migration row: found %d, want 1 with NULL event_id", n)
	}

	repo := NewNotificationRepository(db)
	newNotification := func(eventID string) *entities.Notification {
		return &entities.Notification{ID: uuid.NewString(), UserID: "u", Type: entities.NotificationTypePostCreated, Title: "t", Message: "m", EventID: eventID}
	}

	if err := repo.Create(ctx, newNotification("post.created-1")); err != nil {
		t.Fatalf("first delivery: %v", err)
	}
	if err := repo.Create(ctx, newNotification("post.created-1")); !errors.Is(err, repositories.ErrDuplicateEvent) {
		t.Fatalf("redelivery: err = %v, want ErrDuplicateEvent", err)
	}
	if n := count(t, db, `event_id = 'post.created-1'`); n != 1 {
		t.Errorf("rows for one event delivered twice = %d, want 1", n)
	}

	// No event ID (API-created): no deduplication.
	for i := 0; i < 2; i++ {
		if err := repo.Create(ctx, newNotification("")); err != nil {
			t.Fatalf("create without event id #%d: %v", i+1, err)
		}
	}
	if n := count(t, db, `event_id IS NULL`); n != 3 {
		t.Errorf("rows without event id = %d, want 3 (old + 2 new)", n)
	}
}

// openTestSchema creates a fresh schema, returns a pool whose search_path
// points at it, and drops the schema when the test ends.
func openTestSchema(t *testing.T, raw string) *sql.DB {
	t.Helper()
	admin, err := sql.Open("postgres", raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	schema := fmt.Sprintf("t11_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE") })

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("TEST_DATABASE_URL must be a URL: %v", err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func count(t *testing.T, db *sql.DB, where string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM notifications WHERE " + where).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
