package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"

	"api-gateway/internal/clients"
	"api-gateway/internal/config"
	authv1 "github.com/gauss2302/blogmicrobackend/proto/auth/v1"
)

// sessionAuth answers Login and RefreshToken with a fixed session.
type sessionAuth struct {
	authv1.UnimplementedAuthServiceServer
}

var testSession = &authv1.TokenPair{AccessToken: "access-1", RefreshToken: "refresh-1", TokenType: "Bearer", ExpiresIn: 900}
var testUser = &authv1.UserInfo{Id: "u1", Email: "u1@example.com"}

func (sessionAuth) Login(context.Context, *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	return &authv1.LoginResponse{User: testUser, Tokens: testSession}, nil
}

func (sessionAuth) RefreshToken(context.Context, *authv1.RefreshTokenRequest) (*authv1.RefreshTokenResponse, error) {
	return &authv1.RefreshTokenResponse{User: testUser, Tokens: testSession}, nil
}

func newSessionHandler(t *testing.T, srv authv1.AuthServiceServer, cookieMode bool) *AuthHandler {
	t.Helper()
	addr := startGRPC(t, func(s *grpc.Server) { authv1.RegisterAuthServiceServer(s, srv) })
	client, err := clients.NewAuthClient(addr, config.GRPCTLSConfig{}, testLogger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	cfg := &config.Config{}
	cfg.Auth.UseRefreshTokenCookie = cookieMode
	cfg.Auth.RefreshTokenCookieName = "refresh_token"
	return NewAuthHandler(client, cfg, testLogger)
}

func TestRefreshTokenInJSONOnlyWithoutCookieMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, cookieMode := range []bool{true, false} {
		h := newSessionHandler(t, sessionAuth{}, cookieMode)
		r := gin.New()
		r.POST("/login", h.Login)
		r.POST("/refresh", h.RefreshToken)

		for path, body := range map[string]string{
			"/login":   `{"email":"u1@example.com","password":"secret"}`,
			"/refresh": `{"refresh_token":"refresh-0"}`,
		} {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
			if w.Code != http.StatusOK {
				t.Fatalf("cookie mode %v %s: status %d", cookieMode, path, w.Code)
			}
			inJSON := strings.Contains(w.Body.String(), `"refresh_token"`)
			inCookie := strings.Contains(w.Header().Get("Set-Cookie"), "refresh_token=refresh-1")
			if inJSON == cookieMode || inCookie != cookieMode {
				t.Errorf("cookie mode %v %s: refresh_token in JSON %v, in cookie %v", cookieMode, path, inJSON, inCookie)
			}
			if !strings.Contains(w.Body.String(), `"access_token":"access-1"`) {
				t.Errorf("cookie mode %v %s: access token missing: %s", cookieMode, path, w.Body)
			}
		}
	}
}
