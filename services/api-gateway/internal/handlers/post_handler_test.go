package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"

	"api-gateway/internal/clients"
	"api-gateway/internal/config"
	postv1 "github.com/gauss2302/blogmicrobackend/proto/post/v1"
)

// fakeUserPosts records the last GetUserPosts request.
type fakeUserPosts struct {
	postv1.UnimplementedPostServiceServer
	mu   sync.Mutex
	last *postv1.GetUserPostsRequest
}

func (f *fakeUserPosts) GetUserPosts(_ context.Context, req *postv1.GetUserPostsRequest) (*postv1.ListPostsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.last = req
	return &postv1.ListPostsResponse{}, nil
}

func TestGetUserPostsPassesCallerAsActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &fakeUserPosts{}
	addr := startGRPC(t, func(s *grpc.Server) { postv1.RegisterPostServiceServer(s, fake) })
	client, err := clients.NewPostClient(addr, config.GRPCTLSConfig{}, testLogger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	h := NewPostHandler(client, testLogger)

	r := gin.New()
	// Stands in for OptionalAuthMiddleware: userID is set only for a valid token.
	r.GET("/posts/user/:userId", func(c *gin.Context) {
		if id := c.GetHeader("X-Test-User"); id != "" {
			c.Set("userID", id)
		}
		h.GetUserPosts(c)
	})

	for _, tt := range []struct{ caller, wantActor string }{
		{"author", "author"},
		{"someone-else", "someone-else"},
		{"", ""}, // anonymous
	} {
		req := httptest.NewRequest(http.MethodGet, "/posts/user/author", nil)
		if tt.caller != "" {
			req.Header.Set("X-Test-User", tt.caller)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("caller %q: status = %d (%s)", tt.caller, w.Code, w.Body)
		}
		fake.mu.Lock()
		got := fake.last
		fake.mu.Unlock()
		if got.GetUserId() != "author" || got.GetActorId() != tt.wantActor {
			t.Errorf("caller %q: sent user_id=%q actor_id=%q, want author/%q", tt.caller, got.GetUserId(), got.GetActorId(), tt.wantActor)
		}
	}
}
