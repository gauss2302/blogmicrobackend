package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
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
