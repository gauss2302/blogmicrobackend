package kafka

import (
	"testing"

	kafkago "github.com/segmentio/kafka-go"
)

func TestDLQMessageDoesNotRepeatWriterTopic(t *testing.T) {
	source := kafkago.Message{Topic: "search.posts", Key: []byte("post-1"), Value: []byte("body")}
	got := newDLQMessage(source, []byte(`{"source":"posts"}`))

	if got.Topic != "" {
		t.Fatalf("DLQ message Topic = %q; the writer already has a topic and kafka-go rejects both", got.Topic)
	}
	if string(got.Key) != "post-1" {
		t.Fatalf("key = %q", got.Key)
	}
	if string(got.Value) != `{"source":"posts"}` {
		t.Fatalf("value = %s", got.Value)
	}
}
