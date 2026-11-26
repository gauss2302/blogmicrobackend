package handlers

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestListOffsetLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var offset atomic.Int32
	users, posts := newTestListHandlers(t, &offset)

	r := gin.New()
	r.GET("/posts", posts.ListPosts)
	r.GET("/users", func(c *gin.Context) { c.Set("userID", "u1"); users.ListUsers(c) })

	tests := []struct {
		path       string
		want       int
		wantOffset int32 // offset that reached the backend, -1 for none
	}{
		{"/posts?offset=5000", http.StatusOK, 5000},
		{"/posts?offset=5001", http.StatusBadRequest, -1},
		{"/users?offset=5000", http.StatusOK, 5000},
		{"/users?offset=5001", http.StatusBadRequest, -1},
	}
	for _, tt := range tests {
		offset.Store(-1)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.path, nil))

		if w.Code != tt.want {
			t.Fatalf("%s: status = %d, want %d (%s)", tt.path, w.Code, tt.want, w.Body)
		}
		if got := offset.Load(); got != tt.wantOffset {
			t.Fatalf("%s: backend offset = %d, want %d", tt.path, got, tt.wantOffset)
		}
	}
}
