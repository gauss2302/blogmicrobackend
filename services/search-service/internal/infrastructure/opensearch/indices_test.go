package opensearch

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"search-service/pkg/logger"
)

func closedPortURL(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return "http://" + addr
}

func TestNewClientDoesNotNeedOpenSearch(t *testing.T) {
	if _, err := NewClient(closedPortURL(t), "u", "p", logger.New("fatal")); err != nil {
		t.Fatalf("NewClient with OpenSearch down: %v", err)
	}
	if _, err := NewClient("http://x", "", "", logger.New("fatal")); err == nil {
		t.Error("NewClient accepted empty credentials")
	}
}

// TestWaitForIndicesRetriesUntilUp: OpenSearch answers 500 until it is "up",
// then reports both indices as existing.
func TestWaitForIndicesRetriesUntilUp(t *testing.T) {
	var up atomic.Bool
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if !up.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK) // HEAD /users, HEAD /posts: index exists
	}))
	defer srv.Close()
	c, err := NewClient(srv.URL, "u", "p", logger.New("fatal"))
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- c.WaitForIndices(context.Background(), "users", "posts", 10*time.Millisecond) }()

	deadline := time.Now().Add(2 * time.Second)
	for requests.Load() < 4 { // at least two failed attempts
		if time.Now().After(deadline) {
			t.Fatal("WaitForIndices is not retrying")
		}
		time.Sleep(5 * time.Millisecond)
	}
	select {
	case err := <-done:
		t.Fatalf("WaitForIndices returned while OpenSearch was down: %v", err)
	default:
	}

	up.Store(true)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("WaitForIndices: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForIndices did not return after OpenSearch came up")
	}
}

func TestWaitForIndicesStopsOnCancel(t *testing.T) {
	c, err := NewClient(closedPortURL(t), "u", "p", logger.New("fatal"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.WaitForIndices(ctx, "users", "posts", time.Hour) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForIndices did not stop on cancel")
	}
}
