package config

import (
	"strings"
	"testing"
)

func TestValidateTransportSecurityMode(t *testing.T) {
	if err := validateTransportSecurityMode("production", "", false); err == nil {
		t.Fatal("expected production to require SERVICE_TRANSPORT_SECURITY")
	}
	if err := validateTransportSecurityMode("production", "insecure_dev", false); err == nil {
		t.Fatal("expected production to reject insecure_dev")
	}
	if err := validateTransportSecurityMode("production", "mesh", false); err != nil {
		t.Fatalf("mesh mode should be valid without app TLS: %v", err)
	}
	if err := validateTransportSecurityMode("production", "app_mtls", false); err == nil {
		t.Fatal("expected app_mtls to require GRPC_TLS_ENABLED")
	}
}

// The HTTP API verifies access tokens with this secret, so any mode other than
// the explicitly insecure local-dev one must refuse to start without it.
func TestValidateRequiresJWTSecretOutsideInsecureDev(t *testing.T) {
	base := func(trustMode, secret string) *Config {
		return &Config{
			Environment:              "production",
			GRPCPort:                 "50052",
			JWTSecret:                secret,
			Database:                 DatabaseConfig{URL: "postgres://localhost:5432/userdb"},
			ServiceTransportSecurity: "mesh",
			InternalHTTPTrustMode:    trustMode,
		}
	}

	if err := base("private_network", "").validate(); err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("expected JWT_SECRET to be required, got %v", err)
	}
	if err := base("private_network", "too-short").validate(); err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("expected short JWT_SECRET to be rejected, got %v", err)
	}
	if err := base("private_network", strings.Repeat("a", 32)).validate(); err != nil {
		t.Fatalf("expected a 32-character secret to be accepted: %v", err)
	}

	devCfg := base("insecure_dev", "")
	devCfg.Environment = "development"
	if err := devCfg.validate(); err != nil {
		t.Fatalf("insecure_dev should not require a JWT secret: %v", err)
	}
}

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
