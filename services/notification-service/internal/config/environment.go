package config

import (
	"fmt"
	"slices"
	"strings"
)

// normalizeEnvironment canonicalises ENVIRONMENT once at load. Every
// production-only guard in this package compares the result with ==, so without
// this a deployment that spells it "Production" or leaves a trailing space
// silently skips all of them.
func normalizeEnvironment(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// isDevEnvironment reports whether this is a recognised local/development
// environment. Only those may *imply* an insecure default: an unknown or
// misspelt environment ("prod", "staging") must fail closed rather than quietly
// resolve to insecure_dev, which would re-enable the unauthenticated X-User-ID
// fallback and plaintext service-to-service gRPC.
func isDevEnvironment(environment string) bool {
	switch environment {
	case "development", "dev", "local", "test":
		return true
	default:
		return false
	}
}

// supportedEnvironments is the one set of ENVIRONMENT values every service
// accepts; the copy in each of the six services must stay identical.
// development, dev, local and test are local; every deployed environment runs
// as production. There is deliberately no "staging": code outside config
// (cookie Secure flag, HSTS, gin release mode) checks for "production" only.
var supportedEnvironments = []string{"development", "dev", "local", "test", "production"}

// validateEnvironment refuses to start on an ENVIRONMENT outside
// supportedEnvironments. Every production guard compares with "production", so
// an unsupported spelling such as "prod" or "staging" would otherwise run with
// none of them.
func validateEnvironment(environment string) error {
	if slices.Contains(supportedEnvironments, environment) {
		return nil
	}
	return fmt.Errorf("ENVIRONMENT=%q is not supported; use one of %s", environment, strings.Join(supportedEnvironments, ", "))
}
