package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetGoogleAuthURLClientStateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		size, want  int
		reachesAuth bool
	}{
		{512, http.StatusOK, true},
		{513, http.StatusBadRequest, false},
	}
	for _, tt := range tests {
		fake := &fakeAuth{}
		r := gin.New()
		r.GET("/", newTestAuthHandler(t, fake).GetGoogleAuthURL)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/?client_state="+strings.Repeat("a", tt.size), nil))

		if w.Code != tt.want {
			t.Fatalf("%d bytes: status = %d, want %d (%s)", tt.size, w.Code, tt.want, w.Body)
		}
		if got := fake.calls.Load() == 1; got != tt.reachesAuth {
			t.Fatalf("%d bytes: auth-service called = %v, want %v", tt.size, got, tt.reachesAuth)
		}
	}
}

func TestRefreshTokenStatusMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		code codes.Code
		want int
	}{
		{codes.Unauthenticated, http.StatusUnauthorized},
		{codes.InvalidArgument, http.StatusUnauthorized},
		{codes.Unavailable, http.StatusServiceUnavailable},
		{codes.DeadlineExceeded, http.StatusGatewayTimeout},
		{codes.Internal, http.StatusInternalServerError},
	}
	for _, tt := range tests {
		r := gin.New()
		r.POST("/", newTestAuthHandler(t, &fakeAuth{err: status.Error(tt.code, "x")}).RefreshToken)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"refresh_token":"t"}`)))

		if w.Code != tt.want {
			t.Errorf("%v: status = %d, want %d", tt.code, w.Code, tt.want)
		}
	}
}

func TestRegisterPasswordByteLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		size, want  int
		reachesAuth bool
	}{
		{72, http.StatusCreated, true},
		{73, http.StatusBadRequest, false},
	}
	for _, tt := range tests {
		fake := &fakeAuth{}
		r := gin.New()
		r.POST("/", newTestAuthHandler(t, fake).Register)

		body := fmt.Sprintf(`{"email":"a@example.com","name":"A","password":%q}`, strings.Repeat("p", tt.size))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))

		if w.Code != tt.want {
			t.Fatalf("%d bytes: status = %d, want %d (%s)", tt.size, w.Code, tt.want, w.Body)
		}
		if got := fake.calls.Load() == 1; got != tt.reachesAuth {
			t.Fatalf("%d bytes: auth-service called = %v, want %v", tt.size, got, tt.reachesAuth)
		}
	}
}

func TestBackendErrorStatusMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		code codes.Code
		want int
	}{
		{codes.NotFound, http.StatusNotFound},
		{codes.AlreadyExists, http.StatusConflict},
		{codes.InvalidArgument, http.StatusBadRequest},
		{codes.PermissionDenied, http.StatusForbidden},
		{codes.Unauthenticated, http.StatusUnauthorized},
		{codes.Unavailable, http.StatusServiceUnavailable},
		{codes.DeadlineExceeded, http.StatusGatewayTimeout},
		{codes.Internal, http.StatusInternalServerError},
	}
	mappers := map[string]func(*gin.Context, error, string, string){
		"user": (&UserHandler{logger: testLogger}).handleUserError,
		"post": (&PostHandler{logger: testLogger}).handlePostError,
	}
	for name, handle := range mappers {
		for _, tt := range tests {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			handle(c, status.Error(tt.code, "x"), "FAILED", "failed")
			if w.Code != tt.want {
				t.Errorf("%s %v: status = %d, want %d", name, tt.code, w.Code, tt.want)
			}
		}
	}
}

// Login and Register: lockout, outages and rejected input are not 500, and the auth-service
// text never reaches the client.
func TestPasswordAuthErrorMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		code     codes.Code
		want     int
		wantCode string
	}{
		{codes.ResourceExhausted, http.StatusTooManyRequests, "TOO_MANY_ATTEMPTS"},
		{codes.Unavailable, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE"},
		{codes.InvalidArgument, http.StatusBadRequest, "INVALID_REQUEST"},
		{codes.Internal, http.StatusInternalServerError, ""},
	}
	endpoints := []struct {
		name     string
		body     string
		handler  func(*AuthHandler) gin.HandlerFunc
		fallback string
	}{
		{"login", `{"email":"a@example.com","password":"p"}`, func(h *AuthHandler) gin.HandlerFunc { return h.Login }, "LOGIN_FAILED"},
		{"register", `{"email":"a@example.com","name":"A","password":"password1"}`, func(h *AuthHandler) gin.HandlerFunc { return h.Register }, "REGISTER_FAILED"},
	}
	for _, ep := range endpoints {
		for _, tt := range tests {
			r := gin.New()
			r.POST("/", ep.handler(newTestAuthHandler(t, &fakeAuth{err: status.Error(tt.code, "raw auth-service text")})))

			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(ep.body)))

			wantCode := tt.wantCode
			if wantCode == "" {
				wantCode = ep.fallback
			}
			if w.Code != tt.want || !strings.Contains(w.Body.String(), `"code":"`+wantCode+`"`) {
				t.Errorf("%s %v: got %d %s, want %d %s", ep.name, tt.code, w.Code, w.Body, tt.want, wantCode)
			}
			if strings.Contains(w.Body.String(), "raw auth-service text") {
				t.Errorf("%s %v: auth-service message leaked: %s", ep.name, tt.code, w.Body)
			}
		}
	}
}
