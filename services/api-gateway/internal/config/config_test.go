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

// A-29: app_mtls must mean mutual TLS. Each missing piece used to start the
// service with one-way TLS or none.
func TestValidateTransportSecurityModeAppMTLS(t *testing.T) {
	// The gateway is only a client, so GRPC_TLS_REQUIRE_CLIENT_CERT does not apply.
	complete := GRPCTLSConfig{Enabled: true, CAFile: "ca.pem", CertFile: "tls.crt", KeyFile: "tls.key"}
	for _, tc := range []struct {
		name    string
		mutate  func(*GRPCTLSConfig)
		wantErr string
	}{
		{"complete", func(*GRPCTLSConfig) {}, ""},
		{"tls disabled", func(c *GRPCTLSConfig) { c.Enabled = false }, "GRPC_TLS_ENABLED"},
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

// A-71: /metrics must not share the public port.
func TestLoadRejectsMetricsOnPublicPort(t *testing.T) {
	t.Setenv("PORT", "8080")
	t.Setenv("METRICS_HTTP_PORT", "8080")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "METRICS_HTTP_PORT") {
		t.Fatalf("expected METRICS_HTTP_PORT error, got %v", err)
	}
}
