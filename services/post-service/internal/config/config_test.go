package config

import (
	"strings"
	"testing"
)

func TestValidateTransportSecurityMode(t *testing.T) {
	if err := validateTransportSecurityMode("production", "", GRPCTLSConfig{}); err == nil {
		t.Fatal("expected production to require SERVICE_TRANSPORT_SECURITY")
	}
	if err := validateTransportSecurityMode("production", "mesh", GRPCTLSConfig{}); err != nil {
		t.Fatalf("mesh mode should be valid without app TLS: %v", err)
	}
	if err := validateTransportSecurityMode("production", "insecure_dev", GRPCTLSConfig{}); err == nil {
		t.Fatal("expected production to reject insecure_dev")
	}
	if err := validateTransportSecurityMode("production", "app_mtls", GRPCTLSConfig{}); err == nil {
		t.Fatal("expected app_mtls to require GRPC_TLS_ENABLED")
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

// The HTTP API verifies access tokens with this secret, so any mode other than
// the explicitly insecure local-dev one must refuse to start without it.
func TestValidateRequiresJWTSecretOutsideInsecureDev(t *testing.T) {
	base := func(trustMode, secret string) *Config {
		return &Config{
			Environment:              "production",
			GRPCPort:                 "50053",
			JWTSecret:                secret,
			Database:                 DatabaseConfig{URL: "postgres://localhost:5432/postdb"},
			Redis:                    RedisConfig{URL: "redis:6379", Password: "real-redis-password"},
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

// A-29: app_mtls must mean mutual TLS. Each missing piece used to start the
// service with one-way TLS or none.
func TestValidateTransportSecurityModeAppMTLS(t *testing.T) {
	complete := GRPCTLSConfig{Enabled: true, CAFile: "ca.pem", CertFile: "tls.crt", KeyFile: "tls.key", RequireClientCert: true}
	for _, tc := range []struct {
		name    string
		mutate  func(*GRPCTLSConfig)
		wantErr string
	}{
		{"complete", func(*GRPCTLSConfig) {}, ""},
		{"tls disabled", func(c *GRPCTLSConfig) { c.Enabled = false }, "GRPC_TLS_ENABLED"},
		{"client cert not required", func(c *GRPCTLSConfig) { c.RequireClientCert = false }, "GRPC_TLS_REQUIRE_CLIENT_CERT"},
		{"no CA", func(c *GRPCTLSConfig) { c.CAFile = "" }, "GRPC_TLS_CA_FILE"},
		{"no cert", func(c *GRPCTLSConfig) { c.CertFile = "" }, "GRPC_TLS_CERT_FILE"},
		{"no key", func(c *GRPCTLSConfig) { c.KeyFile = "" }, "GRPC_TLS_KEY_FILE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tlsCfg := complete
			tc.mutate(&tlsCfg)
			err := validateTransportSecurityMode("production", "app_mtls", tlsCfg)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want an error about %s, got %v", tc.wantErr, err)
			}
		})
	}
}
