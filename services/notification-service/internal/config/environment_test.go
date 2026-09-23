package config

import "testing"

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
