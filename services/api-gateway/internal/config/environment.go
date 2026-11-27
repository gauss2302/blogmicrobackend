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
