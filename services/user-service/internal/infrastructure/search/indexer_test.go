package search

import (
	"testing"

	"github.com/segmentio/kafka-go"

	"user-service/pkg/logger"
)

func TestNewIndexerSetsShortBatchTimeout(t *testing.T) {
	indexer := NewIndexer([]string{"127.0.0.1:9092"}, "search.users", "app", "secret", logger.New("error"))
	t.Cleanup(func() { _ = indexer.Close() })

	writer, ok := indexer.writer.(*kafka.Writer)
	if !ok {
		t.Fatalf("writer is %T, want *kafka.Writer", indexer.writer)
	}
	if writer.BatchTimeout != kafkaBatchTimeout {
		t.Fatalf("BatchTimeout = %s, want %s", writer.BatchTimeout, kafkaBatchTimeout)
	}
	transport, ok := writer.Transport.(*kafka.Transport)
	if !ok || transport.SASL == nil {
		t.Fatal("writer is not using SASL")
	}
}
