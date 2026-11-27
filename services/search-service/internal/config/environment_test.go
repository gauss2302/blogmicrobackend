package config

import (
	"strings"
	"testing"
)

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
