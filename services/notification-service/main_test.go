package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
)

func TestRedactURLHidesPassword(t *testing.T) {
	_, pqErr := pq.ParseURL("postgres://notif:s3cret@db:5432x/notifications")
	_, amqpErr := amqp.ParseURI("amqp://guest:s3cret@rabbit:56x72/")
	for name, err := range map[string]error{
		"lib/pq":           pqErr,
		"amqp091, wrapped": fmt.Errorf("failed to connect to rabbit: %w", amqpErr),
	} {
		if err == nil || !strings.Contains(err.Error(), "s3cret") {
			t.Fatalf("%s: expected the library error to leak the URL, got %v", name, err)
		}
		got := redactURL(err)
		if strings.Contains(got, "s3cret") {
			t.Errorf("%s: password still in %q", name, got)
		}
		if !strings.Contains(got, "invalid port") || !strings.Contains(got, `"***"`) {
			t.Errorf("%s: redacted error lost its reason: %q", name, got)
		}
	}
	if got := redactURL(fmt.Errorf("dial tcp: connection refused")); got != "dial tcp: connection refused" {
		t.Errorf("error without a URL changed: %q", got)
	}
}
