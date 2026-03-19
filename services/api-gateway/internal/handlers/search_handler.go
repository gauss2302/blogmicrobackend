package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"api-gateway/pkg/logger"
	"api-gateway/pkg/utils"

	searchv1 "github.com/nikitashilov/microblog_grpc/proto/search/v1"
)

// SearchClient is the minimal interface used by SearchHandler for testability.
type SearchClient interface {
	Search(ctx context.Context, query, requestingUserID string, usersLimit, postsLimit int32, usersCursor, postsCursor string) (*searchv1.SearchResponse, error)
}

type SearchHandler struct {
	searchClient SearchClient
	logger       *logger.Logger
}

func NewSearchHandler(searchClient SearchClient, logger *logger.Logger) *SearchHandler {
	return &SearchHandler{searchClient: searchClient, logger: logger}
}

func (h *SearchHandler) Search(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		utils.ErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}
	requestingUserID := userID.(string)

	query := c.Query("q")
	if query == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "MISSING_QUERY", "Search query is required")
		return
	}

	usersLimit := 20
	if v := c.Query("users_limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 50 {
			usersLimit = n
		}
	}
	postsLimit := 20
	if v := c.Query("posts_limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 50 {
			postsLimit = n
		}
	}
	usersCursor := c.Query("users_cursor")
	postsCursor := c.Query("posts_cursor")

	resp, err := h.searchClient.Search(c.Request.Context(), query, requestingUserID, int32(usersLimit), int32(postsLimit), usersCursor, postsCursor)
	if err != nil {
		h.handleSearchError(c, err)
		return
	}

	data := map[string]interface{}{
		"users":               protoUsersToMap(resp.GetUsers()),
		"posts":               protoPostsToMap(resp.GetPosts()),
		"users_next_cursor":   resp.GetUsersNextCursor(),
		"posts_next_cursor":  resp.GetPostsNextCursor(),
		"users_partial":      resp.GetUsersPartial(),
		"posts_partial":     resp.GetPostsPartial(),
	}
	utils.SuccessResponse(c, http.StatusOK, "Search completed successfully", data)
}

func protoUsersToMap(users []*searchv1.SearchUserHit) []map[string]interface{} {
	if users == nil {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(users))
	for _, u := range users {
		out = append(out, map[string]interface{}{
			"id":      u.GetId(),
			"name":    u.GetName(),
			"picture": u.GetPicture(),
			"bio":     u.GetBio(),
		})
	}
	return out
}

func protoPostsToMap(posts []*searchv1.SearchPostHit) []map[string]interface{} {
	if posts == nil {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(posts))
	for _, p := range posts {
		out = append(out, map[string]interface{}{
			"id":              p.GetId(),
			"user_id":         p.GetUserId(),
			"title":           p.GetTitle(),
			"slug":            p.GetSlug(),
			"content_preview": p.GetContentPreview(),
			"published":       p.GetPublished(),
		})
	}
	return out
}

func (h *SearchHandler) handleSearchError(c *gin.Context, err error) {
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.InvalidArgument:
			utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", st.Message())
			return
		case codes.Unavailable:
			utils.ErrorResponse(c, http.StatusServiceUnavailable, "SEARCH_UNAVAILABLE", "Search service temporarily unavailable")
			return
		}
	}
	h.logger.Error("Search failed: " + err.Error())
	utils.ErrorResponse(c, http.StatusInternalServerError, "SEARCH_FAILED", "Failed to search")
}
