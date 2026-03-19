package handlers

import (
	"net/http"
	"testing"

	"api-gateway/internal/config"
)

func TestGetRefreshCookieSameSite(t *testing.T) {
	tests := []struct {
		name     string
		handler  *AuthHandler
		expected http.SameSite
	}{
		{
			name:     "nil config defaults to lax",
			handler:  &AuthHandler{},
			expected: http.SameSiteLaxMode,
		},
		{
			name: "strict value",
			handler: &AuthHandler{
				cfg: &config.Config{
					Auth: config.AuthConfig{RefreshTokenCookieSameSite: "Strict"},
				},
			},
			expected: http.SameSiteStrictMode,
		},
		{
			name: "none value",
			handler: &AuthHandler{
				cfg: &config.Config{
					Auth: config.AuthConfig{RefreshTokenCookieSameSite: "none"},
				},
			},
			expected: http.SameSiteNoneMode,
		},
		{
			name: "invalid value falls back to lax",
			handler: &AuthHandler{
				cfg: &config.Config{
					Auth: config.AuthConfig{RefreshTokenCookieSameSite: "unsupported"},
				},
			},
			expected: http.SameSiteLaxMode,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := tc.handler.getRefreshCookieSameSite()
			if got != tc.expected {
				t.Fatalf("expected SameSite %v, got %v", tc.expected, got)
			}
		})
	}
}

