package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"api-gateway/internal/config"
)

func TestRefreshCookieOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		CORS: config.CORSConfig{AllowedOrigins: []string{"https://blog.example.com"}},
		Auth: config.AuthConfig{UseRefreshTokenCookie: true, RefreshTokenCookieName: "refresh_token"},
	}

	cases := []struct {
		name    string
		cookie  bool
		origin  string
		want    int
		reached bool
	}{
		{"cookie, foreign origin", true, "https://evil.example", http.StatusForbidden, false},
		{"cookie, null origin", true, "null", http.StatusForbidden, false},
		{"cookie, own origin", true, "https://BLOG.example.com", http.StatusOK, true},
		{"cookie, no origin", true, "", http.StatusOK, true},
		{"body token, foreign origin", false, "https://evil.example", http.StatusOK, true},
	}
	for _, tc := range cases {
		for _, path := range []string{"/refresh", "/logout"} {
			t.Run(tc.name+" "+path, func(t *testing.T) {
				reached := false
				r := gin.New()
				r.POST(path, RefreshCookieOrigin(cfg), func(c *gin.Context) {
					reached = true
					c.Status(http.StatusOK)
				})

				req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"refresh_token":"t"}`))
				if tc.cookie {
					req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "t"})
				}
				if tc.origin != "" {
					req.Header.Set("Origin", tc.origin)
				}
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				if w.Code != tc.want || reached != tc.reached {
					t.Fatalf("got %d reached=%v, want %d reached=%v", w.Code, reached, tc.want, tc.reached)
				}
			})
		}
	}
}
