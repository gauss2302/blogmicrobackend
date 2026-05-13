package config

import (
	"strings"
	"testing"
)

func TestLoadProductionRequiresTransportSecurityMode(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("REDIS_PASSWORD", "redis-password")
	// Isolate from parent process env (e.g. CI / docker-compose exports).
	t.Setenv("SERVICE_TRANSPORT_SECURITY", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "SERVICE_TRANSPORT_SECURITY") {
		t.Fatalf("expected SERVICE_TRANSPORT_SECURITY error, got %v", err)
	}
}

func TestLoadProductionAllowsMeshTransportMode(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("SERVICE_TRANSPORT_SECURITY", "mesh")
	t.Setenv("REDIS_PASSWORD", "redis-password")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ServiceTransportSecurity != "mesh" {
		t.Fatalf("expected mesh transport mode, got %q", cfg.ServiceTransportSecurity)
	}
}

func TestLoadProductionRejectsWildcardCredentialsCORS(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("SERVICE_TRANSPORT_SECURITY", "mesh")
	t.Setenv("REDIS_PASSWORD", "redis-password")
	t.Setenv("CORS_ALLOWED_ORIGINS", "*")
	t.Setenv("CORS_ALLOW_CREDENTIALS", "true")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "CORS_ALLOWED_ORIGINS") {
		t.Fatalf("expected CORS wildcard error, got %v", err)
	}
}

func TestLoadRejectsInvalidRequestBodyLimit(t *testing.T) {
	t.Setenv("REQUEST_MAX_BODY_BYTES", "0")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "REQUEST_MAX_BODY_BYTES") {
		t.Fatalf("expected REQUEST_MAX_BODY_BYTES error, got %v", err)
	}
}
