package config

import (
	"strings"
	"testing"
)

// SEC: see the auth-service counterpart — placeholder secrets from .env.example
// satisfy the length checks, so production must refuse them explicitly.
func TestValidateRejectsTemplatePlaceholdersInProduction(t *testing.T) {
	base := func() *Config {
		return &Config{
			Environment:           "production",
			JWTSecret:             strings.Repeat("s", 40),
			Database:              DatabaseConfig{URL: "postgres://postgres:real-pw@postgres-notification:5432/notificationdb"},
			RabbitMQ:              RabbitMQConfig{URL: "amqp://user:real-pw@rabbitmq:5672/vhost", DLXName: "dlx", DLQName: "dlq", DLQRoutingKey: "post.failed"},
			InternalHTTPTrustMode: "private_network",
			Notification:          NotificationConfig{CleanupDays: 30, BatchSize: 100},
		}
	}

	if err := base().validate(); err != nil {
		t.Fatalf("a fully configured production config should validate: %v", err)
	}

	cfg := base()
	cfg.JWTSecret = "replace-with-at-least-32-random-characters"
	if err := cfg.validate(); err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("expected the placeholder JWT_SECRET to be rejected, got %v", err)
	}
}
