package config

import (
	"fmt"
	"strings"
)

// placeholderMarkers are fragments of the template values shipped in
// .env.example and k8s/overlays/local/secret.yaml (e.g.
// "replace-with-at-least-32-random-characters"). Those values satisfy every
// length and format check, so without this guard a stack brought up from an
// unedited template would run in production with credentials that are public
// in the repository if ENVIRONMENT=production. .env.example sets development
// so a local copy boots; production deployments still hit this guard.
var placeholderMarkers = []string{"replace-with", "change-me", "your-google"}

// looksUnfilled reports whether a value still has the shape of a fill-in-me
// template slot, e.g. the "<strong-random>" and "postgres://postgres:<pw>@..."
// forms in docs/production.md. Matching the shape rather than only known
// strings means a new template does not silently escape the guard; real
// secrets (random, base64, hex) do not contain angle brackets.
func looksUnfilled(value string) bool {
	open := strings.Index(value, "<")
	return open >= 0 && strings.Index(value[open+1:], ">") >= 0
}

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
		if looksUnfilled(value) {
			return fmt.Errorf("%s still holds a template placeholder value; set a real secret before running in production", secret.name)
		}
		for _, marker := range placeholderMarkers {
			if strings.Contains(value, marker) {
				return fmt.Errorf("%s still holds a template placeholder value; set a real secret before running in production", secret.name)
			}
		}
	}
	return nil
}
