package config

import (
	"strings"
	"testing"
)

func TestLoad_rejectsMetricsPortSameAsGRPC(t *testing.T) {
	t.Setenv("GRPC_PORT", "50054")
	t.Setenv("METRICS_HTTP_PORT", "50054")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when METRICS_HTTP_PORT == GRPC_PORT")
	}
}

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
