package search

import (
	"testing"
	"time"

	"post-service/pkg/logger"
)

func TestNewIndexerSetsShortBatchTimeout(t *testing.T) {
	indexer := NewIndexer([]string{"127.0.0.1:9092"}, "search.posts", logger.New("error"))
	t.Cleanup(func() { _ = indexer.Close() })

	if indexer.writer.BatchTimeout != 10*time.Millisecond {
		t.Fatalf("BatchTimeout = %s, want 10ms", indexer.writer.BatchTimeout)
	}
}
