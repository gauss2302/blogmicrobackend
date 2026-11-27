package config

import (
	"strings"
	"testing"
)

// SEC: every production-only guard compares ENVIRONMENT with ==, and an
// unrecognised environment used to imply insecure_dev — which re-enabled the
// unauthenticated X-User-ID fallback and made JWT_SECRET optional. A misspelt
// environment must never be the difference between open and closed.
func TestEnvironmentSpellingCannotOpenTheHeaderFallback(t *testing.T) {
	for _, spelling := range []string{"Production", "PRODUCTION", "production ", " production", "prod", "staging"} {
		t.Setenv("ENVIRONMENT", spelling)
		t.Setenv("INTERNAL_HTTP_TRUST_MODE", "")

		environment := normalizeEnvironment(spelling)
		if mode := resolveInternalHTTPTrustMode("", environment); mode == "insecure_dev" {
			t.Errorf("ENVIRONMENT=%q resolved to insecure_dev: the X-User-ID fallback would be live", spelling)
		}
		if err := validateInternalHTTPTrustMode(environment, "insecure_dev"); err == nil {
			t.Errorf("ENVIRONMENT=%q accepted an explicit insecure_dev trust mode", spelling)
		}
	}

	// Development still gets the convenience fallback.
	if mode := resolveInternalHTTPTrustMode("", normalizeEnvironment("development")); mode != "insecure_dev" {
		t.Errorf("development should still imply insecure_dev, got %q", mode)
	}
	if err := validateInternalHTTPTrustMode("development", "insecure_dev"); err != nil {
		t.Errorf("development should accept insecure_dev: %v", err)
	}
}

// Case and whitespace variants must reach the production branch, not bypass it.
func TestNormalizeEnvironmentReachesProductionGuards(t *testing.T) {
	for _, spelling := range []string{"production", "Production", "PRODUCTION", " production ", "\tproduction"} {
		if got := normalizeEnvironment(spelling); got != "production" {
			t.Errorf("normalizeEnvironment(%q) = %q, want \"production\"", spelling, got)
		}
	}
}

// A-28: ENVIRONMENT is a closed set shared by all six services. Any other value
// must stop startup: "prod" and "staging" used to run with every production
// guard skipped.
func TestLoadRejectsUnsupportedEnvironment(t *testing.T) {
	for _, tc := range []struct {
		value    string
		accepted bool
	}{
		{"", true}, // unset defaults to development
		{"development", true},
		{"dev", true},
		{"local", true},
		{"test", true},
		{"production", true},
		{" Production ", true},
		{"prod", false},
		{"staging", false},
		{"stage", false},
		{"qa", false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("ENVIRONMENT", tc.value)
			_, err := Load()
			rejected := err != nil && strings.Contains(err.Error(), "ENVIRONMENT=")
			if rejected == tc.accepted {
				t.Fatalf("ENVIRONMENT=%q: want accepted=%v, got err %v", tc.value, tc.accepted, err)
			}
		})
	}
}
