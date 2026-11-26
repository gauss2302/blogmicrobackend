package search

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"

	"user-service/internal/domain/entities"
	"user-service/pkg/logger"
)

// fakeWriter records messages. Until release is closed, every write blocks
// until its context ends, like kafka-go against an unreachable broker.
type fakeWriter struct {
	mu      sync.Mutex
	got     []kafka.Message
	release chan struct{}
}

func newFakeWriter(hanging bool) *fakeWriter {
	w := &fakeWriter{release: make(chan struct{})}
	if !hanging {
		close(w.release)
	}
	return w
}

func (w *fakeWriter) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	select {
	case <-w.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.got = append(w.got, msgs...)
	return nil
}

func (w *fakeWriter) Close() error { return nil }

func (w *fakeWriter) events() []indexEvent {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]indexEvent, 0, len(w.got))
	for _, m := range w.got {
		var e indexEvent
		_ = json.Unmarshal(m.Value, &e)
		out = append(out, e)
	}
	return out
}

func testLogger() *logger.Logger { return logger.New("fatal") }

func TestPublishDoesNotWaitForKafka(t *testing.T) {
	w := newFakeWriter(true)
	indexer := newIndexer(w, testLogger(), 50*time.Millisecond)
	t.Cleanup(func() { _ = indexer.Close() })

	user := &entities.User{ID: "u1", Name: "Alice"}
	start := time.Now()
	indexer.UserCreated(context.Background(), user)
	indexer.UserUpdated(context.Background(), user)
	indexer.UserDeleted(context.Background(), user.ID)
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("publishing took %s with Kafka hanging; want it to return at once", elapsed)
	}
}

func TestCloseIsBoundedWhenKafkaHangs(t *testing.T) {
	indexer := newIndexer(newFakeWriter(true), testLogger(), 100*time.Millisecond)
	for n := 0; n < 10; n++ {
		indexer.UserCreated(context.Background(), &entities.User{ID: "u1"})
	}

	start := time.Now()
	_ = indexer.Close()
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Close took %s; want about the 100ms drain timeout", elapsed)
	}
}

func TestEventsKeepOrderAndOutliveRequest(t *testing.T) {
	w := newFakeWriter(false)
	indexer := newIndexer(w, testLogger(), time.Second)

	// A request that is already cancelled must not lose its committed event.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	user := &entities.User{ID: "u1", Name: "Alice"}
	indexer.UserCreated(ctx, user)
	indexer.UserCreated(ctx, &entities.User{ID: "u2"})
	indexer.UserUpdated(ctx, user)
	indexer.UserDeleted(ctx, "u2")
	indexer.UserDeleted(ctx, user.ID)
	if err := indexer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var u1 []string
	for _, e := range w.events() {
		if e.EntityID == "u1" {
			u1 = append(u1, e.EventType)
		}
	}
	want := []string{"created", "updated", "deleted"}
	if len(u1) != len(want) {
		t.Fatalf("u1 events = %v, want %v", u1, want)
	}
	for n := range want {
		if u1[n] != want[n] {
			t.Fatalf("u1 events = %v, want %v", u1, want)
		}
	}
	if total := len(w.events()); total != 5 {
		t.Fatalf("sent %d events, want 5", total)
	}
}

func TestBackfillWaitsForQueueInsteadOfDropping(t *testing.T) {
	w := newFakeWriter(true)
	indexer := newIndexer(w, testLogger(), 5*time.Second)

	const n = 2*queueSize + 10
	done := make(chan error, 1)
	go func() {
		for k := 0; k < n; k++ {
			if err := indexer.BackfillUser(context.Background(), &entities.User{ID: "u"}); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()

	time.Sleep(20 * time.Millisecond) // let the queue fill while Kafka hangs
	close(w.release)
	if err := <-done; err != nil {
		t.Fatalf("BackfillUser: %v", err)
	}
	if err := indexer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := len(w.events()); got != n {
		t.Fatalf("sent %d events, want %d", got, n)
	}
}
