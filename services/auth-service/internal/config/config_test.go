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
