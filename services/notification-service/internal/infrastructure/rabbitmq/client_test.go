package rabbitmq

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"notification-service/internal/config"
	"notification-service/pkg/logger"
)

// Integration test against a real broker. Start one with
//
//	docker run -d --rm -p 127.0.0.1::5672 rabbitmq:3-alpine
//
// and set TEST_RABBITMQ_URL=amqp://guest:guest@127.0.0.1:<port>/.

// TestConsumerResumesAfterBrokerCancel deletes the queue under a running
// consumer: the broker cancels the consumer (basic.cancel) while the channel
// and connection stay open. The client must notice, reconnect (which declares
// the queue again) and keep delivering.
func TestConsumerResumesAfterBrokerCancel(t *testing.T) {
	url := os.Getenv("TEST_RABBITMQ_URL")
	if url == "" {
		t.Skip("TEST_RABBITMQ_URL not set")
	}
	id := fmt.Sprintf("t11.%d", time.Now().UnixNano())
	cfg := config.RabbitMQConfig{
		URL:            url,
		ExchangeName:   id + ".events",
		QueueName:      id + ".notifications",
		RoutingKey:     "post.*",
		DLXName:        id + ".dlx",
		DLQName:        id + ".dlq",
		DLQRoutingKey:  "post.failed",
		PrefetchCount:  1,
		ReconnectDelay: 1,
		MaxRetries:     3,
	}

	admin, err := amqp.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	adminCh, err := admin.Channel()
	if err != nil {
		t.Fatal(err)
	}

	client := NewClient(cfg, logger.New("error"))
	if err := client.Connect(); err != nil {
		t.Fatal(err)
	}
	got := make(chan string, 100)
	if err := client.StartConsuming(func(_, messageID string, _ []byte) error {
		got <- messageID
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = client.Close()
		for _, q := range []string{cfg.QueueName, cfg.DLQName} {
			_, _ = adminCh.QueueDelete(q, false, false, false)
		}
		_ = adminCh.ExchangeDelete(cfg.ExchangeName, false, false)
		_ = adminCh.ExchangeDelete(cfg.DLXName, false, false)
	})

	// publishUntilDelivered publishes until a message reaches the handler: one
	// published while the queue is missing is unroutable and dropped.
	publishUntilDelivered := func(what string) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for i := 0; time.Now().Before(deadline); i++ {
			body := fmt.Sprintf("%s-%d", what, i)
			msg := amqp.Publishing{MessageId: body, Body: []byte(body)}
			if err := adminCh.Publish(cfg.ExchangeName, "post.updated", false, false, msg); err != nil {
				t.Fatal(err)
			}
			select {
			case id := <-got:
				if !strings.HasPrefix(id, what+"-") {
					t.Fatalf("handler got message id %q, want the published %s-N", id, what)
				}
				return
			case <-time.After(200 * time.Millisecond):
			}
		}
		t.Fatalf("no %q message reached the handler within 10s", what)
	}

	publishUntilDelivered("before")

	if _, err := adminCh.QueueDelete(cfg.QueueName, false, false, false); err != nil {
		t.Fatal(err)
	}
	publishUntilDelivered("after-cancel")
}
