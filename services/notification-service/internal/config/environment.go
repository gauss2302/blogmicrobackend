package config

import "strings"

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
