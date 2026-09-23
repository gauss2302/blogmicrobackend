package config

import (
	"strings"
	"testing"
)

// SEC: .env.example ships ENVIRONMENT=production alongside placeholder secrets
// that are long enough to satisfy every length check, so copying it verbatim
// would start a production stack whose signing key is public in the repo.
func TestValidateRejectsTemplatePlaceholdersInProduction(t *testing.T) {
	real := strings.Repeat("s", 40)
	base := func() *Config {
		return &Config{
			Environment:              "production",
			Google:                   GoogleConfig{ClientID: "real-id", ClientSecret: "real-client-secret", RedirectURL: "https://api.example.com/cb", DefaultWebRedirectURI: "https://app.example.com/cb"},
			JWT:                      JWTConfig{Secret: real},
			Redis:                    RedisConfig{Password: "real-redis-password"},
			ServiceTransportSecurity: "mesh",
			InternalHTTPTrustMode:    "private_network",
		}
	}

	if err := base().validate(); err != nil {
		t.Fatalf("a fully configured production config should validate: %v", err)
	}

	// The literal value from .env.example.
	cfg := base()
	cfg.JWT.Secret = "replace-with-at-least-32-random-characters"
	if err := cfg.validate(); err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("expected the placeholder JWT_SECRET to be rejected, got %v", err)
	}

	cfg = base()
	cfg.Google.ClientSecret = "replace-with-google-client-secret"
	if err := cfg.validate(); err == nil || !strings.Contains(err.Error(), "GOOGLE_CLIENT_SECRET") {
		t.Fatalf("expected the placeholder GOOGLE_CLIENT_SECRET to be rejected, got %v", err)
	}

	cfg = base()
	cfg.Redis.Password = "replace-with-redis-password"
	if err := cfg.validate(); err == nil || !strings.Contains(err.Error(), "REDIS_PASSWORD") {
		t.Fatalf("expected the placeholder REDIS_PASSWORD to be rejected, got %v", err)
	}

	// Outside production the template is expected to be usable as-is.
	cfg = base()
	cfg.Environment = "development"
	cfg.JWT.Secret = "replace-with-at-least-32-random-characters"
	cfg.ServiceTransportSecurity = "insecure_dev"
	cfg.InternalHTTPTrustMode = "insecure_dev"
	if err := cfg.validate(); err != nil {
		t.Fatalf("development should tolerate template placeholders: %v", err)
	}
}
