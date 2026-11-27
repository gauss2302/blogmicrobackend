package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"search-service/internal/infrastructure/opensearch"
	"search-service/pkg/logger"
)

// trace records fetches, handler calls, parks and commits in order, so a test
// can check that nothing after a failed message is fetched or committed first.
type trace struct {
	mu     sync.Mutex
	events []string
}

func (t *trace) add(format string, args ...any) {
	t.mu.Lock()
	t.events = append(t.events, fmt.Sprintf(format, args...))
	t.mu.Unlock()
}

func (t *trace) get() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.events...)
}

// fakeReader serves msgs in order, then cancels the consumer's context.
type fakeReader struct {
	tr     *trace
	msgs   []kafkago.Message
	cancel context.CancelFunc
}

func (r *fakeReader) FetchMessage(ctx context.Context) (kafkago.Message, error) {
	if len(r.msgs) == 0 {
		r.cancel()
		return kafkago.Message{}, ctx.Err()
	}
	m := r.msgs[0]
	r.msgs = r.msgs[1:]
	r.tr.add("fetch %d", m.Offset)
	return m, nil
}

func (r *fakeReader) CommitMessages(_ context.Context, msgs ...kafkago.Message) error {
	for _, m := range msgs {
		r.tr.add("commit %d", m.Offset)
	}
	return nil
}

// fakeDLQ fails the first `fail` writes.
type fakeDLQ struct {
	tr   *trace
	fail int
}

func (w *fakeDLQ) WriteMessages(_ context.Context, msgs ...kafkago.Message) error {
	var parked struct {
		Offset int64 `json:"offset"`
	}
	_ = json.Unmarshal(msgs[0].Value, &parked)
	if w.fail > 0 {
		w.fail--
		w.tr.add("park-fail %d", parked.Offset)
		return errors.New("kafka: Unknown Topic Or Partition")
	}
	w.tr.add("park %d", parked.Offset)
	return nil
}

func (w *fakeDLQ) Close() error { return nil }

func event(offset int64, id string) kafkago.Message {
	v, _ := json.Marshal(IndexEvent{EntityType: "post", EventType: "created", EntityID: id, Payload: json.RawMessage(`{}`)})
	return kafkago.Message{Topic: "search.posts", Offset: offset, Value: v}
}

// run consumes msgs with handle until the reader runs dry and returns the trace.
func run(t *testing.T, dlqFailures int, handle func(context.Context, IndexEvent) error, msgs ...kafkago.Message) []string {
	t.Helper()
	tr := &trace{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := &Consumer{dlqWriter: &fakeDLQ{tr: tr, fail: dlqFailures}, retryBackoff: time.Millisecond, log: logger.New("fatal")}
	reader := &fakeReader{tr: tr, msgs: msgs, cancel: cancel}

	done := make(chan struct{})
	go func() {
		c.consume(ctx, reader, "posts", func(ctx context.Context, ev IndexEvent) error {
			tr.add("handle %s", ev.EntityID)
			return handle(ctx, ev)
		})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatalf("consume did not finish; trace so far: %v", tr.get())
	}
	return tr.get()
}

// failFirst makes handle fail with err the first n times it sees entity id.
func failFirst(id string, n int, err error) func(context.Context, IndexEvent) error {
	var mu sync.Mutex
	return func(_ context.Context, ev IndexEvent) error {
		mu.Lock()
		defer mu.Unlock()
		if ev.EntityID == id && n > 0 {
			n--
			return err
		}
		return nil
	}
}

func expect(t *testing.T, got []string, want ...string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("trace\n got: %v\nwant: %v", got, want)
	}
}

var badRequest = &opensearch.StatusError{Op: "index document", Status: 400, Body: "mapper_parsing_exception"}

func TestFailedParkRetriesSameMessage(t *testing.T) {
	got := run(t, 2, failFirst("a", 1, badRequest), event(0, "a"), event(1, "b"))
	expect(t, got,
		"fetch 0", "handle a", "park-fail 0", "park-fail 0", "park 0", "commit 0",
		"fetch 1", "handle b", "commit 1")
}

func TestTransientErrorRetriesWithoutCommitOrPark(t *testing.T) {
	for name, err := range map[string]error{
		"503":      &opensearch.StatusError{Op: "index document", Status: 503},
		"429":      &opensearch.StatusError{Op: "index document", Status: 429},
		"network":  &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")},
		"deadline": fmt.Errorf("index: %w", context.DeadlineExceeded),
	} {
		t.Run(name, func(t *testing.T) {
			got := run(t, 0, failFirst("a", 3, err), event(0, "a"), event(1, "b"))
			expect(t, got,
				"fetch 0", "handle a", "handle a", "handle a", "handle a", "commit 0",
				"fetch 1", "handle b", "commit 1")
		})
	}
}

func TestPermanentErrorParkedOnce(t *testing.T) {
	got := run(t, 0, failFirst("a", 100, badRequest), event(0, "a"), event(1, "b"))
	expect(t, got, "fetch 0", "handle a", "park 0", "commit 0", "fetch 1", "handle b", "commit 1")
}

func TestMalformedEventParkedOnce(t *testing.T) {
	bad := kafkago.Message{Topic: "search.posts", Offset: 0, Value: []byte("{not json")}
	got := run(t, 0, failFirst("", 0, nil), bad, event(1, "b"))
	expect(t, got, "fetch 0", "park 0", "commit 0", "fetch 1", "handle b", "commit 1")
}

func TestMalformedPayloadIsPermanent(t *testing.T) {
	c := &Consumer{}
	ev := IndexEvent{EventType: "created", Payload: json.RawMessage(`[1]`)}
	for name, handle := range map[string]func(context.Context, IndexEvent) error{
		"users": c.handleUserEvent,
		"posts": c.handlePostEvent,
	} {
		if err := handle(context.Background(), ev); !isPermanent(err) {
			t.Errorf("%s: err = %v, want a permanent error", name, err)
		}
	}
}

func TestShutdownDuringRetryLeavesMessageUncommitted(t *testing.T) {
	tr := &trace{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := &Consumer{dlqWriter: &fakeDLQ{tr: tr}, retryBackoff: time.Millisecond, log: logger.New("fatal")}
	reader := &fakeReader{tr: tr, msgs: []kafkago.Message{event(0, "a"), event(1, "b")}, cancel: cancel}

	calls := 0
	done := make(chan struct{})
	go func() {
		c.consume(ctx, reader, "posts", func(context.Context, IndexEvent) error {
			tr.add("handle")
			if calls++; calls == 3 {
				cancel()
			}
			return &opensearch.StatusError{Op: "index document", Status: 503}
		})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("consume did not stop after cancel")
	}
	expect(t, tr.get(), "fetch 0", "handle", "handle", "handle")
}

func TestRetryBackoffGrowsToCap(t *testing.T) {
	c := &Consumer{retryBackoff: 500 * time.Millisecond}
	for attempt, want := range map[int]time.Duration{
		0:    500 * time.Millisecond,
		1:    time.Second,
		5:    16 * time.Second,
		6:    maxRetryBackoff, // 32s capped
		1000: maxRetryBackoff,
	} {
		if got := c.backoff(attempt); got != want {
			t.Errorf("backoff(%d) = %s, want %s", attempt, got, want)
		}
	}
}

func TestEachIndexCallHasTimeout(t *testing.T) {
	got := run(t, 0, func(ctx context.Context, _ IndexEvent) error {
		dl, ok := ctx.Deadline()
		if !ok || time.Until(dl) > indexCallTimeout {
			return permanentError{errors.New("handler context has no per-call deadline")}
		}
		return nil
	}, event(0, "a"))
	expect(t, got, "fetch 0", "handle a", "commit 0")
}
