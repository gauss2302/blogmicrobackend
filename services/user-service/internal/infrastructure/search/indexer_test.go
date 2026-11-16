package search

import (
	"testing"
	"time"

	"user-service/pkg/logger"
)

func TestNewIndexerSetsShortBatchTimeout(t *testing.T) {
	indexer := NewIndexer([]string{"127.0.0.1:9092"}, "search.users", logger.New("error"))
	t.Cleanup(func() { _ = indexer.Close() })

	if indexer.writer.BatchTimeout != kafkaBatchTimeout {
		t.Fatalf("BatchTimeout = %s, want %s", indexer.writer.BatchTimeout, kafkaBatchTimeout)
	}
	if indexer.writer.BatchTimeout >= time.Second {
		t.Fatalf("BatchTimeout %s still waits out the kafka-go default", indexer.writer.BatchTimeout)
	}
}
