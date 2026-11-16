package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"user-service/pkg/auth"
	"user-service/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

const testSecret = "test-secret-at-least-32-characters-long"

func signToken(t *testing.T, secret, userID, tokenType string, ttl time.Duration) string {
	t.Helper()

	claims := auth.Claims{
		UserID: userID,
		Type:   tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

// runRequest sends a request through AuthMiddleware and reports the status code
// plus the actor the middleware authenticated (empty when it rejected).
type allowRevocations struct{}

func (allowRevocations) Revoked(context.Context, string) (bool, error) { return false, nil }

type revokedTokens struct{}

func (revokedTokens) Revoked(context.Context, string) (bool, error) { return true, nil }

func runRequest(trustMode string, validator *auth.Validator, headers map[string]string) (int, string) {
	return runRequestWithRevocations(trustMode, validator, allowRevocations{}, headers)
}

func runRequestWithRevocations(trustMode string, validator *auth.Validator, revocations auth.RevocationChecker, headers map[string]string) (int, string) {
	gin.SetMode(gin.TestMode)

	var actor string
	router := gin.New()
	router.GET("/protected", AuthMiddleware(validator, revocations, trustMode, logger.New("error")), func(c *gin.Context) {
		actor = ActorID(c)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec.Code, actor
}

// The regression this guards: before the fix, any caller that could reach the
// service's HTTP port could act as any user by setting X-User-ID.
func TestAuthMiddlewareRejectsSpoofedUserIDHeader(t *testing.T) {
	validator := auth.NewValidator(testSecret)

	for _, trustMode := range []string{"private_network", "disabled", ""} {
		status, actor := runRequest(trustMode, validator, map[string]string{"X-User-ID": "victim-id"})
		if status != http.StatusForbidden {
			t.Errorf("trustMode %q: got status %d, want %d", trustMode, status, http.StatusForbidden)
		}
		if actor != "" {
			t.Errorf("trustMode %q: handler ran with actor %q, want no handler execution", trustMode, actor)
		}
	}
}

func TestAuthMiddlewareAcceptsValidAccessToken(t *testing.T) {
	validator := auth.NewValidator(testSecret)
	token := signToken(t, testSecret, "user-1", "access", time.Hour)

	status, actor := runRequest("private_network", validator, map[string]string{
		"Authorization": "Bearer " + token,
		// A spoofed header must not override the identity proven by the token.
		"X-User-ID": "victim-id",
	})

	if status != http.StatusOK {
		t.Fatalf("got status %d, want %d", status, http.StatusOK)
	}
	if actor != "user-1" {
		t.Errorf("got actor %q, want %q", actor, "user-1")
	}
}

func TestAuthMiddlewareRejectsRevokedAccessToken(t *testing.T) {
	validator := auth.NewValidator(testSecret)
	token := signToken(t, testSecret, "user-1", "access", time.Hour)

	status, actor := runRequestWithRevocations("private_network", validator, revokedTokens{}, map[string]string{
		"Authorization": "Bearer " + token,
	})
	if status != http.StatusForbidden {
		t.Fatalf("got status %d, want %d", status, http.StatusForbidden)
	}
	if actor != "" {
		t.Errorf("handler ran with actor %q", actor)
	}
}

func TestAuthMiddlewareRejectsBadTokens(t *testing.T) {
	validator := auth.NewValidator(testSecret)

	cases := map[string]string{
		"wrong signing key": signToken(t, "another-secret-at-least-32-characters", "user-1", "access", time.Hour),
		"refresh token":     signToken(t, testSecret, "user-1", "refresh", time.Hour),
		"expired token":     signToken(t, testSecret, "user-1", "access", -time.Minute),
		"garbage":           "not-a-jwt",
	}

	for name, token := range cases {
		status, actor := runRequest("private_network", validator, map[string]string{"Authorization": "Bearer " + token})
		if status != http.StatusForbidden {
			t.Errorf("%s: got status %d, want %d", name, status, http.StatusForbidden)
		}
		if actor != "" {
			t.Errorf("%s: handler ran with actor %q, want no handler execution", name, actor)
		}
	}
}

// insecure_dev keeps the header fallback so the service can be exercised
// locally without minting tokens; config validation forbids it in production.
func TestAuthMiddlewareInsecureDevAllowsHeader(t *testing.T) {
	status, actor := runRequest(trustModeInsecureDev, nil, map[string]string{"X-User-ID": "dev-user"})

	if status != http.StatusOK {
		t.Fatalf("got status %d, want %d", status, http.StatusOK)
	}
	if actor != "dev-user" {
		t.Errorf("got actor %q, want %q", actor, "dev-user")
	}
}

// Even in insecure_dev, a presented token must verify: a caller may not corrupt
// its own token to fall back onto a spoofed header identity.
func TestAuthMiddlewareInsecureDevStillRejectsInvalidToken(t *testing.T) {
	validator := auth.NewValidator(testSecret)

	status, actor := runRequest(trustModeInsecureDev, validator, map[string]string{
		"Authorization": "Bearer not-a-jwt",
		"X-User-ID":     "victim-id",
	})

	if status != http.StatusForbidden {
		t.Errorf("got status %d, want %d", status, http.StatusForbidden)
	}
	if actor != "" {
		t.Errorf("handler ran with actor %q, want no handler execution", actor)
	}
}
