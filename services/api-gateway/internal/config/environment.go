package config

import "strings"

// normalizeEnvironment canonicalises ENVIRONMENT once at load. Every
// production-only guard in this package compares the result with ==, so without
// this a deployment that spells it "Production" or leaves a trailing space
// silently skips all of them.
func normalizeEnvironment(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
