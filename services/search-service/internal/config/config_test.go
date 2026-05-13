package config

import (
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
	if err := validateTransportSecurityMode("production", "", false); err == nil {
		t.Fatal("expected production to require SERVICE_TRANSPORT_SECURITY")
	}
	if err := validateTransportSecurityMode("production", "mesh", false); err != nil {
		t.Fatalf("mesh mode should be valid without app TLS: %v", err)
	}
	if err := validateTransportSecurityMode("production", "insecure_dev", false); err == nil {
		t.Fatal("expected production to reject insecure_dev")
	}
	if err := validateTransportSecurityMode("production", "app_mtls", false); err == nil {
		t.Fatal("expected app_mtls to require GRPC_TLS_ENABLED")
	}
}
