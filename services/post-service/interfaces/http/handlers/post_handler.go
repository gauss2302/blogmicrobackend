package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"post-service/interfaces/validators"
	"post-service/internal/application/dto"
	"post-service/internal/application/errors"
	"post-service/internal/application/services"
	"post-service/pkg/logger"
	"post-service/pkg/utils"
)

type PostHandler struct {
	postService *services.PostService
	validator   *validators.PostValidator
	logger      *logger.Logger
}

func NewPostHandler(postService *services.PostService, logger *logger.Logger) *PostHandler {
	return &PostHandler{
		postService: postService,
		validator:   validators.NewPostValidator(),
		logger:      logger,
	}
}

func (h *PostHandler) CreatePost(c *gin.Context) {
	var req dto.CreatePostRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid create post request: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	if err := h.validator.ValidateCreatePostRequest(&req); err != nil {
		h.logger.Warn("Create post validation failed: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidPostData)
		return
	}

	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		utils.ErrorResponse(c, errors.ErrUnauthorizedAccess)
		return
	}

	response, err := h.postService.CreatePost(c.Request.Context(), &req, userID)
	if err != nil {
		if postErr, ok := err.(*errors.PostError); ok {
			utils.ErrorResponse(c, postErr)
		} else {
			h.logger.Error("Unexpected error in create post: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Post created successfully", response)
}

func (h *PostHandler) GetPost(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetHeader("X-User-ID")

	if id == "" {
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	response, err := h.postService.GetPost(c.Request.Context(), id, userID)
	if err != nil {
		if postErr, ok := err.(*errors.PostError); ok {
			utils.ErrorResponse(c, postErr)
		} else {
			h.logger.Error("Unexpected error in get post: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Post retrieved successfully", response)
}

func (h *PostHandler) GetPostBySlug(c *gin.Context) {
	slug := c.Param("slug")

	if slug == "" {
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	response, err := h.postService.GetPostBySlug(c.Request.Context(), slug)
	if err != nil {
		if postErr, ok := err.(*errors.PostError); ok {
			utils.ErrorResponse(c, postErr)
		} else {
			h.logger.Error("Unexpected error in get post by slug: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Post retrieved successfully", response)
}

func (h *PostHandler) UpdatePost(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetHeader("X-User-ID")

	if id == "" || userID == "" {
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	var req dto.UpdatePostRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid update post request: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	if err := h.validator.ValidateUpdatePostRequest(&req); err != nil {
		h.logger.Warn("Update post validation failed: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidPostData)
		return
	}

	response, err := h.postService.UpdatePost(c.Request.Context(), id, &req, userID)
	if err != nil {
		if postErr, ok := err.(*errors.PostError); ok {
			utils.ErrorResponse(c, postErr)
		} else {
			h.logger.Error("Unexpected error in update post: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Post updated successfully", response)
}

func (h *PostHandler) DeletePost(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetHeader("X-User-ID")

	if id == "" || userID == "" {
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	err := h.postService.DeletePost(c.Request.Context(), id, userID)
	if err != nil {
		if postErr, ok := err.(*errors.PostError); ok {
			utils.ErrorResponse(c, postErr)
		} else {
			h.logger.Error("Unexpected error in delete post: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Post deleted successfully", nil)
}

func (h *PostHandler) ListPosts(c *gin.Context) {
	var req dto.ListPostsRequest

	if err := c.ShouldBindQuery(&req); err != nil {
		h.logger.Warn("Invalid list posts request: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	// Set defaults
	if req.Limit == 0 {
		req.Limit = 20
	}

	response, err := h.postService.ListPosts(c.Request.Context(), &req)
	if err != nil {
		if postErr, ok := err.(*errors.PostError); ok {
			utils.ErrorResponse(c, postErr)
		} else {
			h.logger.Error("Unexpected error in list posts: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Posts retrieved successfully", response)
}

func (h *PostHandler) GetUserPosts(c *gin.Context) {
	userID := c.Param("userId")
	if userID == "" {
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	var req dto.UserPostsRequest

	if err := c.ShouldBindQuery(&req); err != nil {
		h.logger.Warn("Invalid user posts request: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	// Set defaults
	if req.Limit == 0 {
		req.Limit = 20
	}

	response, err := h.postService.GetUserPosts(c.Request.Context(), userID, &req)
	if err != nil {
		if postErr, ok := err.(*errors.PostError); ok {
			utils.ErrorResponse(c, postErr)
		} else {
			h.logger.Error("Unexpected error in get user posts: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "User posts retrieved successfully", response)
}

func (h *PostHandler) SearchPosts(c *gin.Context) {
	var req dto.SearchPostsRequest

	if err := c.ShouldBindQuery(&req); err != nil {
		h.logger.Warn("Invalid search posts request: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	// Set defaults
	if req.Limit == 0 {
		req.Limit = 20
	}

	if err := h.validator.ValidateSearchPostsRequest(&req); err != nil {
		h.logger.Warn("Search posts validation failed: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	response, err := h.postService.SearchPosts(c.Request.Context(), &req)
	if err != nil {
		if postErr, ok := err.(*errors.PostError); ok {
			utils.ErrorResponse(c, postErr)
		} else {
			h.logger.Error("Unexpected error in search posts: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Post search completed successfully", response)
}

func (h *PostHandler) GetStats(c *gin.Context) {
	userID := c.GetHeader("X-User-ID") // Optional for public stats

	response, err := h.postService.GetStats(c.Request.Context(), userID)
	if err != nil {
		if postErr, ok := err.(*errors.PostError); ok {
			utils.ErrorResponse(c, postErr)
		} else {
			h.logger.Error("Unexpected error in get stats: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
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