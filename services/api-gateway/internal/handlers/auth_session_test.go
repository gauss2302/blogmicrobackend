package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"api-gateway/internal/clients"
	"api-gateway/internal/config"
	authv1 "github.com/gauss2302/blogmicrobackend/proto/auth/v1"
)

// sessionAuth answers Login and RefreshToken with a fixed session, records
// the last Logout and fails Logout and RefreshToken with err when set.
type sessionAuth struct {
	authv1.UnimplementedAuthServiceServer
	err    error
	logout *authv1.LogoutRequest
}

var testSession = &authv1.TokenPair{AccessToken: "access-1", RefreshToken: "refresh-1", TokenType: "Bearer", ExpiresIn: 900}
var testUser = &authv1.UserInfo{Id: "u1", Email: "u1@example.com"}

func (*sessionAuth) Login(context.Context, *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	return &authv1.LoginResponse{User: testUser, Tokens: testSession}, nil
}

func (f *sessionAuth) RefreshToken(context.Context, *authv1.RefreshTokenRequest) (*authv1.RefreshTokenResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
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
		h := newSessionHandler(t, &sessionAuth{}, cookieMode)
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

func (f *sessionAuth) Logout(_ context.Context, req *authv1.LogoutRequest) (*emptypb.Empty, error) {
	f.logout = req
	if f.err != nil {
		return nil, f.err
	}
	return &emptypb.Empty{}, nil
}

func postWithCookie(h gin.HandlerFunc, header, cookie, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/", h)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: cookie})
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func clearsRefreshCookie(w *httptest.ResponseRecorder) bool {
	c := w.Header().Get("Set-Cookie")
	return strings.Contains(c, "refresh_token=;") && strings.Contains(c, "Max-Age=0")
}

func TestLogoutWithExpiredAccessRevokesRefreshCookie(t *testing.T) {
	fake := &sessionAuth{}
	w := postWithCookie(newSessionHandler(t, fake, true).Logout, "Bearer expired-access", "refresh-1", "")

	if w.Code != http.StatusOK || !clearsRefreshCookie(w) {
		t.Fatalf("status %d Set-Cookie %q, want 200 and a cleared cookie", w.Code, w.Header().Get("Set-Cookie"))
	}
	if fake.logout.GetAccessToken() != "expired-access" || fake.logout.GetRefreshToken() != "refresh-1" {
		t.Fatalf("auth-service got %+v, want both tokens", fake.logout)
	}
}

func TestLogoutWithRefreshInBody(t *testing.T) {
	fake := &sessionAuth{}
	w := postWithCookie(newSessionHandler(t, fake, false).Logout, "", "", `{"refresh_token":"refresh-1"}`)

	if w.Code != http.StatusOK || fake.logout.GetRefreshToken() != "refresh-1" {
		t.Fatalf("status %d, auth-service got %+v; want 200 and the body token", w.Code, fake.logout)
	}
}

func TestLogoutClearsCookieWhenRevokeFails(t *testing.T) {
	fake := &sessionAuth{err: status.Error(codes.Unauthenticated, "both tokens dead")}
	w := postWithCookie(newSessionHandler(t, fake, true).Logout, "", "refresh-1", "")

	if w.Code != http.StatusUnauthorized || !clearsRefreshCookie(w) {
		t.Fatalf("status %d Set-Cookie %q, want 401 and a cleared cookie", w.Code, w.Header().Get("Set-Cookie"))
	}
}

func TestLogoutWithoutTokensSucceeds(t *testing.T) {
	fake := &sessionAuth{}
	w := postWithCookie(newSessionHandler(t, fake, true).Logout, "", "", "")

	if w.Code != http.StatusOK || fake.logout != nil {
		t.Fatalf("status %d, auth-service called with %+v; want 200 and no call", w.Code, fake.logout)
	}
}

func TestFailedRefreshClearsCookieOnlyWhenTokenIsDead(t *testing.T) {
	for code, wantCleared := range map[codes.Code]bool{codes.Unauthenticated: true, codes.Unavailable: false} {
		fake := &sessionAuth{err: status.Error(code, "x")}
		w := postWithCookie(newSessionHandler(t, fake, true).RefreshToken, "", "refresh-1", "")

		if clearsRefreshCookie(w) != wantCleared {
			t.Errorf("%v: status %d Set-Cookie %q, want cleared=%v", code, w.Code, w.Header().Get("Set-Cookie"), wantCleared)
		}
	}
}
