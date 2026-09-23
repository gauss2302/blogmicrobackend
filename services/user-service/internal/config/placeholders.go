package config

import (
	"fmt"
	"strings"
)

// placeholderMarkers are fragments of the template values shipped in
// .env.example and k8s/overlays/local/secret.yaml (e.g.
// "replace-with-at-least-32-random-characters"). Those values satisfy every
// length and format check, so without this guard a stack brought up from an
// unedited template — which declares ENVIRONMENT=production — would run in
// production with credentials that are public in the repository.
var placeholderMarkers = []string{"replace-with", "change-me", "your-google"}

// namedSecret pairs a configuration value with the environment variable it came
// from, so the error names what the operator has to fix.
type namedSecret struct {
	name  string
	value string
}

// rejectPlaceholderSecrets fails startup when a secret still holds a template
// value. Callers apply it in production only: local development is expected to
// run with the template as-is.
func rejectPlaceholderSecrets(secrets ...namedSecret) error {
	for _, secret := range secrets {
		value := strings.ToLower(secret.value)
		for _, marker := range placeholderMarkers {
			if strings.Contains(value, marker) {
				return fmt.Errorf("%s still holds a template placeholder value; set a real secret before running in production", secret.name)
			}
		}
	}
	return nil
}
