package config

import (
	"strings"
	"testing"
)

func setRequiredAuthEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GOOGLE_CLIENT_ID", "client-id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "client-secret")
	t.Setenv("GOOGLE_REDIRECT_URL", "https://api.example.com/api/v1/auth/google/callback")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
}

func TestLoadProductionRequiresTransportSecurityMode(t *testing.T) {
	setRequiredAuthEnv(t)
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("REDIS_PASSWORD", "redis-password")
	// Isolate from parent process env (e.g. CI / docker-compose exports).
	t.Setenv("SERVICE_TRANSPORT_SECURITY", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "SERVICE_TRANSPORT_SECURITY") {
		t.Fatalf("expected SERVICE_TRANSPORT_SECURITY error, got %v", err)
	}
}

func TestLoadProductionRequiresRedisPassword(t *testing.T) {
	setRequiredAuthEnv(t)
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("SERVICE_TRANSPORT_SECURITY", "mesh")
	t.Setenv("INTERNAL_HTTP_TRUST_MODE", "private_network")
	t.Setenv("REDIS_PASSWORD", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "REDIS_PASSWORD") {
		t.Fatalf("expected REDIS_PASSWORD error, got %v", err)
	}
}

func TestLoadProductionAllowsMeshTransportMode(t *testing.T) {
	setRequiredAuthEnv(t)
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("SERVICE_TRANSPORT_SECURITY", "mesh")
	t.Setenv("INTERNAL_HTTP_TRUST_MODE", "private_network")
	t.Setenv("REDIS_PASSWORD", "redis-password")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ServiceTransportSecurity != "mesh" {
		t.Fatalf("expected mesh transport mode, got %q", cfg.ServiceTransportSecurity)
	}
}
