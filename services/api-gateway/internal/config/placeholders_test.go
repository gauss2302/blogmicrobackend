package config

import (
	"strings"
	"testing"
)

// The production runbook (docs/production.md) uses a different placeholder
// style than .env.example — "<strong-random>", "postgres://postgres:<pw>@..." —
// which contains none of the literal markers. The guard matches the shape too.
func TestValidateRejectsAngleBracketPlaceholders(t *testing.T) {
	cases := map[string]string{
		"runbook secret":  "<strong-random>",
		"runbook dsn":     "postgres://postgres:<pw>@postgres-user:5432/userdb?sslmode=disable",
		"runbook jwt":     "<>= 32 random chars>",
		"oauth client id": "<your-oauth-client-id>",
	}
	for name, value := range cases {
		if !looksUnfilled(strings.ToLower(value)) {
			t.Errorf("%s: %q should be recognised as an unfilled template slot", name, value)
		}
	}

	// Real secrets must not trip it.
	for _, real := range []string{"aB3xK9mQ2pL7vR4t", "kJ8s+Zq1/Wm4NpX2yTb6Rd==", "postgres://postgres:s3cr3t@postgres-user:5432/userdb"} {
		if looksUnfilled(strings.ToLower(real)) {
			t.Errorf("%q must not be treated as a placeholder", real)
		}
	}
}
