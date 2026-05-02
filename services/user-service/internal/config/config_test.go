package config

import "testing"

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
