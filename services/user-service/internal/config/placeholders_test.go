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

// The production runbook (docs/production.md) uses a different placeholder
// style than .env.example — "<strong-random>", "postgres://postgres:<pw>@..." —
// which contains none of the literal markers. The guard matches the shape too.
func TestValidateRejectsAngleBracketPlaceholders(t *testing.T) {
	cases := map[string]string{
		"runbook secret":  "<strong-random>",
		"runbook dsn":     "postgres://postgres:<pw>@postgres-user:5432/userdb?sslmode=disable",
		"runbook jwt":     "<>= 32 random chars>",
		"oauth client id": "<your-oauth-client-id>",
	}
	for name, value := range cases {
		if !looksUnfilled(strings.ToLower(value)) {
			t.Errorf("%s: %q should be recognised as an unfilled template slot", name, value)
		}
	}

	// Real secrets must not trip it.
	for _, real := range []string{"aB3xK9mQ2pL7vR4t", "kJ8s+Zq1/Wm4NpX2yTb6Rd==", "postgres://postgres:s3cr3t@postgres-user:5432/userdb"} {
		if looksUnfilled(strings.ToLower(real)) {
			t.Errorf("%q must not be treated as a placeholder", real)
		}
	}
}
