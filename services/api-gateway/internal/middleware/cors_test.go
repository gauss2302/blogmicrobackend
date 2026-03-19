package middleware

import "testing"

func TestResolveAllowedOrigin(t *testing.T) {
	tests := []struct {
		name             string
		origin           string
		allowedOrigins   []string
		allowCredentials bool
		expected         string
	}{
		{
			name:             "wildcard with credentials echoes request origin",
			origin:           "http://localhost:3000",
			allowedOrigins:   []string{"*"},
			allowCredentials: true,
			expected:         "http://localhost:3000",
		},
		{
			name:             "wildcard without credentials returns star",
			origin:           "http://localhost:3000",
			allowedOrigins:   []string{"*"},
			allowCredentials: false,
			expected:         "*",
		},
		{
			name:             "exact origin match",
			origin:           "https://app.example.com",
			allowedOrigins:   []string{"https://app.example.com"},
			allowCredentials: true,
			expected:         "https://app.example.com",
		},
		{
			name:             "missing origin returns empty when no wildcard",
			origin:           "",
			allowedOrigins:   []string{"https://app.example.com"},
			allowCredentials: true,
			expected:         "https://app.example.com",
		},
		{
			name:             "origin not allowed",
			origin:           "https://evil.example.com",
			allowedOrigins:   []string{"https://app.example.com"},
			allowCredentials: true,
			expected:         "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := resolveAllowedOrigin(tc.origin, tc.allowedOrigins, tc.allowCredentials)
			if got != tc.expected {
				t.Fatalf("expected origin %q, got %q", tc.expected, got)
			}
		})
	}
}

