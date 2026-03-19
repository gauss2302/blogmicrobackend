package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/pkg/logger"

	searchv1 "github.com/nikitashilov/microblog_grpc/proto/search/v1"
	"github.com/gin-gonic/gin"
)

type mockSearchClient struct {
	resp *searchv1.SearchResponse
	err  error
}

func (m *mockSearchClient) Search(ctx context.Context, query, requestingUserID string, usersLimit, postsLimit int32, usersCursor, postsCursor string) (*searchv1.SearchResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}

func TestSearchHandler_Search_HappyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log := logger.New("info")
	mock := &mockSearchClient{
		resp: &searchv1.SearchResponse{
			Users: []*searchv1.SearchUserHit{
				{Id: "u1", Name: "Alice", Picture: "", Bio: ""},
			},
			Posts: []*searchv1.SearchPostHit{
				{Id: "p1", UserId: "u1", Title: "Hello", Slug: "hello", ContentPreview: "Hi", Published: true},
			},
			UsersNextCursor:   "",
			PostsNextCursor:  "",
			UsersPartial:     false,
			PostsPartial:    false,
		},
	}
	h := NewSearchHandler(mock, log)

	r := gin.New()
	r.GET("/search", func(c *gin.Context) {
		c.Set("userID", "requesting-user")
		h.Search(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/search?q=alice", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d body %s", rec.Code, rec.Body.String())
	}
}

func TestSearchHandler_Search_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log := logger.New("info")
	h := NewSearchHandler(&mockSearchClient{}, log)

	r := gin.New()
	r.GET("/search", h.Search)

	req := httptest.NewRequest(http.MethodGet, "/search?q=alice", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 when userID not set, got %d", rec.Code)
	}
}

func TestSearchHandler_Search_MissingQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log := logger.New("info")
	h := NewSearchHandler(&mockSearchClient{}, log)

	r := gin.New()
	r.GET("/search", func(c *gin.Context) {
		c.Set("userID", "user1")
		h.Search(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/search", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 when q missing, got %d", rec.Code)
	}
}
