package config

import (
	"strings"
	"testing"
)

func TestValidateInternalHTTPTrustMode(t *testing.T) {
	if err := validateInternalHTTPTrustMode("production", ""); err == nil {
		t.Fatal("expected production to require INTERNAL_HTTP_TRUST_MODE")
	}
	if err := validateInternalHTTPTrustMode("production", "private_network"); err != nil {
		t.Fatalf("private_network should be valid in production: %v", err)
	}
	if err := validateInternalHTTPTrustMode("production", "insecure_dev"); err == nil {
		t.Fatal("expected production to reject insecure_dev")
	}
}

func TestLoadRejectsInvalidNotificationCleanupDays(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:password@localhost:5432/notificationdb")
	t.Setenv("RABBITMQ_URL", "amqp://user:password@localhost:5672/vhost")
	t.Setenv("NOTIFICATION_CLEANUP_DAYS", "0")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "NOTIFICATION_CLEANUP_DAYS") {
		t.Fatalf("expected NOTIFICATION_CLEANUP_DAYS error, got %v", err)
	}
}
