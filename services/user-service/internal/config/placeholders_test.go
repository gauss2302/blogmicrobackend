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
			Environment:              "production",
			GRPCPort:                 "50052",
			JWTSecret:                strings.Repeat("s", 40),
			Database:                 DatabaseConfig{URL: "postgres://postgres:real-pw@postgres-user:5432/userdb"},
			ServiceTransportSecurity: "mesh",
			InternalHTTPTrustMode:    "private_network",
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

	// The k8s local overlay embeds the password in the DSN.
	cfg = base()
	cfg.Database.URL = "postgres://postgres:replace-with-user-db-password@postgres-user:5432/userdb?sslmode=disable"
	if err := cfg.validate(); err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("expected the placeholder DATABASE_URL to be rejected, got %v", err)
	}
}
