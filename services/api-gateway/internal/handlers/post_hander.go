package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

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

	// Convert to map for client
	reqMap := map[string]interface{}{
		"title":     req.Title,
		"content":   req.Content,
		"published": req.Published,
	}
	if req.Slug != "" {
		reqMap["slug"] = req.Slug
	}

	response, err := h.postClient.CreatePost(c.Request.Context(), reqMap, userID.(string))
	if err != nil {
		h.logger.Error("Create post failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusInternalServerError, "CREATE_FAILED", "Failed to create post")
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
		h.logger.Error("Get post failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusNotFound, "POST_NOT_FOUND", "Post not found")
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
		h.logger.Error("Get post by slug failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusNotFound, "POST_NOT_FOUND", "Post not found")
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

	// Convert to map for client - only include non-nil fields
	reqMap := make(map[string]interface{})
	if req.Title != nil {
		reqMap["title"] = *req.Title
	}
	if req.Content != nil {
		reqMap["content"] = *req.Content
	}
	if req.Slug != nil {
		reqMap["slug"] = *req.Slug
	}
	if req.Published != nil {
		reqMap["published"] = *req.Published
	}

	response, err := h.postClient.UpdatePost(c.Request.Context(), id, reqMap, userID.(string))
	if err != nil {
		h.logger.Error("Update post failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusInternalServerError, "UPDATE_FAILED", "Failed to update post")
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

	err := h.postClient.DeletePost(c.Request.Context(), id, userID.(string))
	if err != nil {
		h.logger.Error("Delete post failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusInternalServerError, "DELETE_FAILED", "Failed to delete post")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Post deleted successfully", nil)
}

func (h *PostHandler) ListPosts(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")
	publishedOnlyStr := c.DefaultQuery("published_only", "true")
	
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 20
	}
	
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	publishedOnly, err := strconv.ParseBool(publishedOnlyStr)
	if err != nil {
		publishedOnly = true
	}

	response, err := h.postClient.ListPosts(c.Request.Context(), limit, offset, publishedOnly)
	if err != nil {
		h.logger.Error("List posts failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusInternalServerError, "LIST_FAILED", "Failed to retrieve posts")
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
	if err != nil || offset < 0 {
		offset = 0
	}

	response, err := h.postClient.GetUserPosts(c.Request.Context(), userID, limit, offset)
	if err != nil {
		h.logger.Error("Get user posts failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusInternalServerError, "LIST_FAILED", "Failed to retrieve user posts")
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
	publishedOnlyStr := c.DefaultQuery("published_only", "true")
	
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 20
	}
	
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	publishedOnly, err := strconv.ParseBool(publishedOnlyStr)
	if err != nil {
		publishedOnly = true
	}

	response, err := h.postClient.SearchPosts(c.Request.Context(), query, limit, offset, publishedOnly)
	if err != nil {
		h.logger.Error("Search posts failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusInternalServerError, "SEARCH_FAILED", "Failed to search posts")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Post search completed successfully", response)
}

func (h *PostHandler) GetPostStats(c *gin.Context) {
	userID, _ := c.Get("userID")
	var userIDStr string
	if userID != nil {
		userIDStr = userID.(string)
	}

	response, err := h.postClient.GetStats(c.Request.Context(), userIDStr)
	if err != nil {
		h.logger.Error("Get post stats failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusInternalServerError, "STATS_FAILED", "Failed to retrieve post statistics")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Post statistics retrieved successfully", response)
}