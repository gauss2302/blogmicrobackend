package search

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"

	"user-service/internal/domain/entities"
	"user-service/internal/infrastructure/postgres"
	"user-service/pkg/logger"
)

// testDB gives each test its own database, so relays of tests running in
// parallel (and other packages' writes) never see each other's outbox rows.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	admin, err := sql.Open("postgres", raw)
	if err != nil {
		t.Fatal(err)
	}
	name := "relay_" + strings.ReplaceAll(uuid.NewString()[:13], "-", "")
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		_, _ = admin.Exec("DROP DATABASE " + name + " WITH (FORCE)")
		admin.Close()
	})
	if err := postgres.RunMigrations(db, OutboxDDL()); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return db
}

// fakeWriter records sent messages; fail makes writes fail, block makes them
// wait for release (or the context).
type fakeWriter struct {
	mu      sync.Mutex
	fail    error
	block   chan struct{}
	started chan struct{} // closed when a blocked write begins
	sent    []kafka.Message
}

func (w *fakeWriter) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	if w.block != nil {
		close(w.started)
		select {
		case <-w.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.fail != nil {
		return w.fail
	}
	w.sent = append(w.sent, msgs...)
	return nil
}

func (w *fakeWriter) Close() error { return nil }

// events decodes what was sent: user id -> event type and payload, in order.
func (w *fakeWriter) events(t *testing.T) []sentEvent {
	t.Helper()
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []sentEvent
	for _, m := range w.sent {
		var ev struct {
			EntityType string          `json:"entity_type"`
			EventType  string          `json:"event_type"`
			EntityID   string          `json:"entity_id"`
			Payload    json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(m.Value, &ev); err != nil {
			t.Fatal(err)
		}
		var p userPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			t.Fatal(err)
		}
		if ev.EntityType != "user" || string(m.Key) != ev.EntityID || p.ID != ev.EntityID {
			t.Fatalf("malformed message: key %q, %+v", m.Key, ev)
		}
		if strings.Contains(string(m.Value), "email") || strings.Contains(string(m.Value), "@example.com") {
			t.Fatalf("email published: %s", m.Value)
		}
		out = append(out, sentEvent{ev.EventType, p})
	}
	return out
}

type sentEvent struct {
	Type    string
	Payload userPayload
}

// waitStarted waits for the blocked write to begin, failing fast if the relay
// returned first (an error before sending) instead of hanging.
func waitStarted(t *testing.T, w *fakeWriter, done <-chan error) {
	t.Helper()
	select {
	case <-w.started:
	case err := <-done:
		t.Fatalf("relay returned before sending: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("relay never started sending")
	}
}

func newTestRelay(db *sql.DB, w *fakeWriter) *Relay {
	r := newRelay(db, w, logger.New("fatal"))
	r.poll = 10 * time.Millisecond
	return r
}

// createUser adds an active user (soft-deleted when active is false) with
// every text column set, so changing any of them is a real change.
func createUser(t *testing.T, db *sql.DB, active bool) *entities.User {
	t.Helper()
	id := uuid.NewString()
	u := &entities.User{ID: id, Email: id[:8] + "@example.com", Name: "Name", Picture: "https://img.example.com/p.png", PasswordHash: "hash", Bio: "Bio", Location: "City", Website: "https://example.com", IsActive: true}
	repo := postgres.NewUserRepository(db)
	if err := repo.Create(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if !active {
		if err := repo.Delete(context.Background(), u.ID); err != nil {
			t.Fatal(err)
		}
		u.IsActive = false
	}
	return u
}

func outboxCount(t *testing.T, db *sql.DB, userID string) int {
	t.Helper()
	var n int
	q := `SELECT count(*) FROM search_outbox`
	args := []any{}
	if userID != "" {
		q += ` WHERE entity_id = $1`
		args = append(args, userID)
	}
	if err := db.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func drain(t *testing.T, r *Relay) {
	t.Helper()
	for {
		n, err := r.relayOnce(context.Background())
		if err != nil {
			t.Fatalf("relayOnce: %v", err)
		}
		if n == 0 {
			return
		}
	}
}

// The payload sent to search and the columns that wake the UPDATE trigger are
// one list; this keeps userPayload in it too. is_active is the one state column
// outside the payload: it decides between indexing and deleting.
func TestStateColumnsMatchPayload(t *testing.T) {
	tags := []string{"is_active"}
	typ := reflect.TypeOf(userPayload{})
	for i := 0; i < typ.NumField(); i++ {
		if tag := typ.Field(i).Tag.Get("json"); tag != "id" {
			tags = append(tags, tag)
		}
	}
	cols := slices.Clone(stateColumns)
	slices.Sort(tags)
	slices.Sort(cols)
	if !slices.Equal(tags, cols) {
		t.Fatalf("userPayload fields + is_active %v != stateColumns %v", tags, cols)
	}
}

// Changing any column search sees queues the user; changing any other column
// (email, password, location...) does not. Every users column is tried, so a
// new column is covered too.
func TestTriggerFiresOnlyForSearchColumns(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	p := createUser(t, db, true)
	if got := outboxCount(t, db, p.ID); got != 1 {
		t.Fatalf("insert queued %d rows, want 1", got)
	}

	rows, err := db.QueryContext(ctx, `SELECT column_name, data_type FROM information_schema.columns WHERE table_name = 'users' AND column_name <> 'id'`)
	if err != nil {
		t.Fatal(err)
	}
	type column struct{ name, typ string }
	var columns []column
	for rows.Next() {
		var c column
		if err := rows.Scan(&c.name, &c.typ); err != nil {
			t.Fatal(err)
		}
		columns = append(columns, c)
	}
	rows.Close()
	if len(columns) < len(stateColumns)+2 {
		t.Fatalf("read only %d users columns", len(columns))
	}

	for _, c := range columns {
		var change string
		switch {
		case c.typ == "boolean":
			change = "NOT " + c.name
		case strings.HasPrefix(c.typ, "timestamp"):
			change = c.name + " + interval '1 second'"
		default:
			change = c.name + " || 'x'"
		}
		if _, err := db.ExecContext(ctx, `DELETE FROM search_outbox`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`UPDATE users SET %s = %s WHERE id = $1`, c.name, change), p.ID); err != nil {
			t.Fatalf("update %s: %v", c.name, err)
		}
		want := 0
		if slices.Contains(stateColumns, c.name) {
			want = 1
		}
		if got := outboxCount(t, db, p.ID); got != want {
			t.Errorf("changing %s queued %d rows, want %d", c.name, got, want)
		}
	}

	if _, err := db.ExecContext(ctx, `DELETE FROM search_outbox`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, p.ID); err != nil {
		t.Fatal(err)
	}
	if got := outboxCount(t, db, p.ID); got != 1 {
		t.Errorf("delete queued %d rows, want 1", got)
	}
}

func TestMigrateTwiceAndConcurrently(t *testing.T) {
	db := testDB(t) // migrated once already
	if err := postgres.RunMigrations(db, OutboxDDL()); err != nil {
		t.Fatalf("second run: %v", err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 4; i++ { // replicas starting together
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- postgres.RunMigrations(db, OutboxDDL())
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent migration: %v", err)
		}
	}
	if p := createUser(t, db, true); outboxCount(t, db, p.ID) != 1 {
		t.Error("triggers missing after repeated migrations")
	}
}

// The relay sends what the user is now: several edits make one message, and
// soft-deleted and removed users leave the index.
func TestRelaySendsCurrentState(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	live := createUser(t, db, true)
	for _, name := range []string{"One", "Two", "Three"} {
		if _, err := db.ExecContext(ctx, `UPDATE users SET name = $2 WHERE id = $1`, live.ID, name); err != nil {
			t.Fatal(err)
		}
	}
	softDeleted := createUser(t, db, false) // UserService.DeleteUser path
	removed := createUser(t, db, true)
	if _, err := db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, removed.ID); err != nil {
		t.Fatal(err)
	}
	nullFields := createUser(t, db, true)
	if _, err := db.ExecContext(ctx, `UPDATE users SET picture = NULL, bio = NULL WHERE id = $1`, nullFields.ID); err != nil {
		t.Fatal(err)
	}

	w := &fakeWriter{}
	drain(t, newTestRelay(db, w))

	got := map[string]sentEvent{}
	for _, ev := range w.events(t) {
		if _, dup := got[ev.Payload.ID]; dup {
			t.Errorf("user %s sent twice", ev.Payload.ID)
		}
		got[ev.Payload.ID] = ev
	}
	want := map[string]sentEvent{
		live.ID:        {"updated", userPayload{ID: live.ID, Name: "Three", Picture: live.Picture, Bio: live.Bio}},
		softDeleted.ID: {"deleted", userPayload{ID: softDeleted.ID}},
		removed.ID:     {"deleted", userPayload{ID: removed.ID}},
		nullFields.ID:  {"updated", userPayload{ID: nullFields.ID, Name: "Name"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sent\n%+v\nwant\n%+v", got, want)
	}
	if n := outboxCount(t, db, ""); n != 0 {
		t.Errorf("%d outbox rows left", n)
	}
}

func TestRelayKeepsRowsWhileKafkaFails(t *testing.T) {
	db := testDB(t)
	p := createUser(t, db, true)

	w := &fakeWriter{fail: errors.New("kafka down")}
	r := newTestRelay(db, w)
	if _, err := r.relayOnce(context.Background()); err == nil {
		t.Fatal("relayOnce succeeded with Kafka down")
	}
	if n := outboxCount(t, db, p.ID); n != 1 {
		t.Fatalf("after a failed send %d rows left, want 1", n)
	}

	w.fail = nil
	drain(t, r)
	if ev := w.events(t); len(ev) != 1 || ev[0].Type != "updated" || ev[0].Payload.ID != p.ID {
		t.Errorf("retry sent %+v", ev)
	}
	if n := outboxCount(t, db, ""); n != 0 {
		t.Errorf("%d outbox rows left after the retry", n)
	}
}

// A process stopped in the middle of a send loses nothing: its batch rolls
// back and the next process sends it.
func TestRelayStoppedMidSendLeavesRowsForTheNextOne(t *testing.T) {
	db := testDB(t)
	p := createUser(t, db, true)

	stuck := &fakeWriter{block: make(chan struct{}), started: make(chan struct{})}
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error)
	go func() {
		_, err := newTestRelay(db, stuck).relayOnce(ctx)
		done <- err
	}()
	waitStarted(t, stuck, done)
	stop() // shutdown while Kafka is slow
	if err := <-done; err == nil {
		t.Fatal("interrupted send reported success")
	}
	if n := outboxCount(t, db, p.ID); n != 1 {
		t.Fatalf("after an interrupted send %d rows left, want 1", n)
	}

	next := &fakeWriter{}
	drain(t, newTestRelay(db, next))
	if ev := next.events(t); len(ev) != 1 || ev[0].Payload.ID != p.ID {
		t.Errorf("next relay sent %+v", ev)
	}
}

// While one relay is sending, another (an overlapping replica) sends nothing,
// so a user's events cannot be reordered.
func TestRelayOneSenderAtATime(t *testing.T) {
	db := testDB(t)
	createUser(t, db, true)

	first := &fakeWriter{block: make(chan struct{}), started: make(chan struct{})}
	done := make(chan error)
	go func() {
		_, err := newTestRelay(db, first).relayOnce(context.Background())
		done <- err
	}()
	waitStarted(t, first, done)

	second := &fakeWriter{}
	n, err := newTestRelay(db, second).relayOnce(context.Background())
	if err != nil || n != 0 || len(second.events(t)) != 0 {
		t.Errorf("second relay: n=%d err=%v sent=%d; want 0, nil, 0", n, err, len(second.events(t)))
	}

	close(first.block)
	if err := <-done; err != nil {
		t.Fatalf("first relay: %v", err)
	}
	if len(first.events(t)) != 1 {
		t.Errorf("first relay sent %d messages, want 1", len(first.events(t)))
	}
}

func TestRunSendsUntilStopped(t *testing.T) {
	db := testDB(t)
	w := &fakeWriter{}
	r := newTestRelay(db, w)
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { r.Run(ctx); close(done) }()

	p := createUser(t, db, true) // written while Run is polling
	deadline := time.Now().Add(5 * time.Second)
	for len(w.events(t)) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if ev := w.events(t); len(ev) != 1 || ev[0].Payload.ID != p.ID {
		t.Errorf("Run sent %+v", ev)
	}
	stop()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop")
	}
}

func TestNewRelaySetsShortBatchTimeoutAndSASL(t *testing.T) {
	r := NewRelay(nil, []string{"127.0.0.1:9092"}, "search.users", "app", "secret", logger.New("fatal"))
	t.Cleanup(func() { _ = r.Close() })
	w, ok := r.writer.(*kafka.Writer)
	if !ok {
		t.Fatalf("writer is %T, want *kafka.Writer", r.writer)
	}
	if w.BatchTimeout != 10*time.Millisecond {
		t.Errorf("BatchTimeout = %s, want 10ms", w.BatchTimeout)
	}
	if tr, ok := w.Transport.(*kafka.Transport); !ok || tr.SASL == nil {
		t.Error("writer is not using SASL")
	}
}

// A row committed while a batch is being sent was not in that batch and must
// survive its DELETE (deleting "id <= max" would drop it unsent).
func TestRelayKeepsRowsCommittedDuringSend(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	late, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer late.Rollback() //nolint:errcheck
	lateID := uuid.NewString()
	if _, err := late.ExecContext(ctx, `INSERT INTO users (id, email, name, is_active) VALUES ($1, $2, 'Late', true)`, lateID, lateID[:8]+"@example.com"); err != nil {
		t.Fatal(err) // its outbox row gets the lower id but is not committed yet
	}
	early := createUser(t, db, true)

	w := &fakeWriter{block: make(chan struct{}), started: make(chan struct{})}
	done := make(chan error)
	go func() {
		_, err := newTestRelay(db, w).relayOnce(ctx)
		done <- err
	}()
	waitStarted(t, w, done)
	if err := late.Commit(); err != nil {
		t.Fatal(err)
	}
	close(w.block)
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	if ev := w.events(t); len(ev) != 1 || ev[0].Payload.ID != early.ID {
		t.Errorf("first batch sent %+v, want only %s", ev, early.ID)
	}
	if n := outboxCount(t, db, lateID); n != 1 {
		t.Errorf("row committed during the send: %d left, want 1", n)
	}
}
