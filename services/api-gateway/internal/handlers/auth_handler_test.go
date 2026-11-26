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
