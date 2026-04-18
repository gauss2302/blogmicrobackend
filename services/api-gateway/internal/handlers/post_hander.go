package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"api-gateway/internal/clients"
	"api-gateway/internal/models"
	"api-gateway/pkg/logger"
	"api-gateway/pkg/utils"
)

type PostHandler struct {
	postClient *clients.PostClient
	logger     *logger.Logger
}

func NewPostHandler(postClient *clients.PostClient, logger *logger.Logger) *PostHandler {
	return &PostHandler{
		postClient: postClient,
		logger:     logger,
	}
}

func (h *PostHandler) CreatePost(c *gin.Context) {
	var req models.CreatePostRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid create post request: " + err.Error())
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format")
		return
	}

	userID, exists := c.Get("userID")
	if !exists {
		utils.ErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	input := &clients.CreatePostInput{
		UserID:    userID.(string),
		Title:     req.Title,
		Content:   req.Content,
		Slug:      req.Slug,
		Published: req.Published,
	}

	response, err := h.postClient.CreatePost(c.Request.Context(), input)
	if err != nil {
		h.handlePostError(c, err, "CREATE_FAILED", "Failed to create post")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Post created successfully", response)
}

func (h *PostHandler) GetPost(c *gin.Context) {
	id := c.Param("id")

	if id == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "Post ID is required")
		return
	}

	userID, _ := c.Get("userID")
	var userIDStr string
	if userID != nil {
		userIDStr = userID.(string)
	}

	response, err := h.postClient.GetPost(c.Request.Context(), id, userIDStr)
	if err != nil {
		h.handlePostError(c, err, "POST_NOT_FOUND", "Post not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Post retrieved successfully", response)
}

func (h *PostHandler) GetPostBySlug(c *gin.Context) {
	slug := c.Param("slug")

	if slug == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "Post slug is required")
		return
	}

	response, err := h.postClient.GetPostBySlug(c.Request.Context(), slug)
	if err != nil {
		h.handlePostError(c, err, "POST_NOT_FOUND", "Post not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Post retrieved successfully", response)
}

func (h *PostHandler) UpdatePost(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "Post ID is required")
		return
	}

	var req models.UpdatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid update post request: " + err.Error())
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format")
		return
	}

	userID, exists := c.Get("userID")
	if !exists {
		utils.ErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	input := &clients.UpdatePostInput{
		ID:        id,
		UserID:    userID.(string),
		Title:     req.Title,
		Content:   req.Content,
		Slug:      req.Slug,
		Published: req.Published,
	}

	response, err := h.postClient.UpdatePost(c.Request.Context(), input)
	if err != nil {
		h.handlePostError(c, err, "UPDATE_FAILED", "Failed to update post")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Post updated successfully", response)
}

func (h *PostHandler) DeletePost(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "Post ID is required")
		return
	}

	userID, exists := c.Get("userID")
	if !exists {
		utils.ErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	if err := h.postClient.DeletePost(c.Request.Context(), id, userID.(string)); err != nil {
		h.handlePostError(c, err, "DELETE_FAILED", "Failed to delete post")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Post deleted successfully", nil)
}

func (h *PostHandler) ListPosts(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 20
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 || offset > maxOffset {
		offset = 0
	}

	// Public route must never expose drafts, ignore client override.
	publishedOnly := true

	response, err := h.postClient.ListPosts(c.Request.Context(), limit, offset, publishedOnly)
	if err != nil {
		h.handlePostError(c, err, "LIST_FAILED", "Failed to retrieve posts")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Posts retrieved successfully", response)
}

func (h *PostHandler) GetUserPosts(c *gin.Context) {
	userID := c.Param("userId")
	if userID == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "User ID is required")
		return
	}

	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 20
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 || offset > maxOffset {
		offset = 0
	}

	response, err := h.postClient.GetUserPosts(c.Request.Context(), userID, limit, offset)
	if err != nil {
		h.handlePostError(c, err, "USER_POSTS_FAILED", "Failed to retrieve user posts")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "User posts retrieved successfully", response)
}

func (h *PostHandler) SearchPosts(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "MISSING_QUERY", "Search query is required")
		return
	}

	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 20
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 || offset > maxOffset {
		offset = 0
	}

	// Public route must never expose drafts, ignore client override.
	publishedOnly := true

	response, err := h.postClient.SearchPosts(c.Request.Context(), query, limit, offset, publishedOnly)
	if err != nil {
		h.handlePostError(c, err, "SEARCH_FAILED", "Failed to search posts")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Post search completed successfully", response)
}

func (h *PostHandler) GetStats(c *gin.Context) {
	userID := ""
	if uid, exists := c.Get("userID"); exists {
		userID = uid.(string)
	}

	response, err := h.postClient.GetStats(c.Request.Context(), userID)
	if err != nil {
		h.handlePostError(c, err, "STATS_FAILED", "Failed to retrieve post statistics")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Post statistics retrieved successfully", response)
}

func (h *PostHandler) HealthCheck(c *gin.Context) {
	utils.SuccessResponse(c, http.StatusOK, "Post service is healthy", gin.H{
		"service": "post-service",
		"status":  "running",
	})
}

func (h *PostHandler) handlePostError(c *gin.Context, err error, code, message string) {
	if err == nil {
		return
	}

	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.NotFound:
			utils.ErrorResponse(c, http.StatusNotFound, code, message)
			return
		case codes.AlreadyExists:
			utils.ErrorResponse(c, http.StatusConflict, code, message)
			return
		case codes.InvalidArgument:
			utils.ErrorResponse(c, http.StatusBadRequest, code, message)
			return
		case codes.PermissionDenied:
			utils.ErrorResponse(c, http.StatusForbidden, code, message)
			return
		case codes.Unauthenticated:
			utils.ErrorResponse(c, http.StatusUnauthorized, code, message)
			return
		case codes.Unavailable:
			utils.ErrorResponse(c, http.StatusServiceUnavailable, code, message)
			return
		}
	}

	h.logger.Error("Post service operation failed: " + err.Error())
	utils.ErrorResponse(c, http.StatusInternalServerError, code, message)
}
