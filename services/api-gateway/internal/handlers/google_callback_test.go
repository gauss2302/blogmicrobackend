package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// T-10 rejects Google sign-in for an email that has a password account; the
// browser must land on the login page with a fixed code, not on raw JSON.
func TestGoogleCallbackAccountExistsRedirectsToLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name     string
		origins  []string
		code     codes.Code
		want     int
		location string
	}{
		{"redirect", []string{"https://blog.example.com", "http://localhost:3000"}, codes.AlreadyExists, http.StatusSeeOther, "https://blog.example.com/auth/login?error=account_exists_use_password"},
		{"no web origin", []string{"*"}, codes.AlreadyExists, http.StatusConflict, ""},
		{"other failure unchanged", []string{"https://blog.example.com"}, codes.Internal, http.StatusInternalServerError, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestAuthHandler(t, &fakeAuth{err: status.Error(tt.code, "raw auth-service text")})
			h.cfg.CORS.AllowedOrigins = tt.origins
			r := gin.New()
			r.GET("/cb", h.GoogleCallback)

			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/cb?state=s&code=c", nil))

			if w.Code != tt.want || w.Header().Get("Location") != tt.location {
				t.Fatalf("got %d Location=%q, want %d %q (%s)", w.Code, w.Header().Get("Location"), tt.want, tt.location, w.Body)
			}
			if tt.code == codes.AlreadyExists && strings.Contains(w.Body.String(), "raw auth-service text") {
				t.Fatalf("auth-service message leaked: %s", w.Body)
			}
		})
	}
}
