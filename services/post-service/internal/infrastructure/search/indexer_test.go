package search

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"

	"post-service/pkg/logger"
)

func TestNewIndexerSetsShortBatchTimeout(t *testing.T) {
	indexer := NewIndexer([]string{"127.0.0.1:9092"}, "search.posts", "app", "secret", logger.New("error"))
	t.Cleanup(func() { _ = indexer.Close() })

	if indexer.writer.BatchTimeout != 10*time.Millisecond {
		t.Fatalf("BatchTimeout = %s, want 10ms", indexer.writer.BatchTimeout)
	}
	transport, ok := indexer.writer.Transport.(*kafka.Transport)
	if !ok || transport.SASL == nil {
		t.Fatal("writer is not using SASL")
	}
}

func TestPostsDeletedReturnsBrokerErrors(t *testing.T) {
	indexer := NewIndexer([]string{"127.0.0.1:1"}, "search.posts", "", "", logger.New("fatal")) // nothing listens there
	t.Cleanup(func() { _ = indexer.Close() })

	if err := indexer.PostsDeleted(context.Background(), nil); err != nil {
		t.Errorf("no posts: err = %v, want nil without touching the broker", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := indexer.PostsDeleted(ctx, []string{"p1", "p2"}); err == nil {
		t.Error("broker down: err = nil, want the write error")
	}
}

func TestDeletedMessageIsKeyedByPost(t *testing.T) {
	msg, err := message("deleted", "p1", postPayload{ID: "p1"})
	if err != nil {
		t.Fatal(err)
	}
	var ev indexEvent
	if err := json.Unmarshal(msg.Value, &ev); err != nil {
		t.Fatal(err)
	}
	if string(msg.Key) != "p1" || ev.EntityType != "post" || ev.EventType != "deleted" || ev.EntityID != "p1" || ev.MessageID == "" {
		t.Errorf("key %q, event %+v", msg.Key, ev)
	}
}
