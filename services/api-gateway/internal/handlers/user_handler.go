package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"api-gateway/internal/clients"
	"api-gateway/pkg/logger"
	"api-gateway/pkg/utils"
)

type UserHandler struct {
	userClient *clients.UserClient
	logger     *logger.Logger
}

const maxOffset = 5000

func NewUserHandler(userClient *clients.UserClient, logger *logger.Logger) *UserHandler {
	return &UserHandler{
		userClient: userClient,
		logger:     logger,
	}
}

func (h *UserHandler) CreateUser(c *gin.Context) {
	var req clients.CreateUserInput

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid create user request: " + err.Error())
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format")
		return
	}

	userID, exists := c.Get("userID")
	if !exists {
		utils.ErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Enforce created user ID matches authenticated user if provided
	if req.ID == "" {
		req.ID = userID.(string)
	}
	if req.Name == "" {
		req.Name = req.Email
	}

	response, err := h.userClient.CreateUser(c.Request.Context(), &req)
	if err != nil {
		h.handleUserError(c, err, "CREATE_FAILED", "Failed to create user")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "User created successfully", response)
}

func (h *UserHandler) GetUser(c *gin.Context) {
	id := c.Param("id")

	if _, exists := c.Get("userID"); !exists {
		utils.ErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	response, err := h.userClient.GetUser(c.Request.Context(), id)
	if err != nil {
		h.handleUserError(c, err, "USER_NOT_FOUND", "User not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "User retrieved successfully", response)
}

func (h *UserHandler) UpdateUser(c *gin.Context) {
	id := c.Param("id")

	var req clients.UpdateUserInput
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid update user request: " + err.Error())
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format")
		return
	}

	userID, exists := c.Get("userID")
	if !exists {
		utils.ErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	req.ID = id
	req.ActorID = userID.(string)

	response, err := h.userClient.UpdateUser(c.Request.Context(), &req)
	if err != nil {
		h.handleUserError(c, err, "UPDATE_FAILED", "Failed to update user")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "User updated successfully", response)
}

func (h *UserHandler) DeleteUser(c *gin.Context) {
	id := c.Param("id")

	userID, exists := c.Get("userID")
	if !exists {
		utils.ErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	if err := h.userClient.DeleteUser(c.Request.Context(), id, userID.(string)); err != nil {
		h.handleUserError(c, err, "DELETE_FAILED", "Failed to delete user")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "User deleted successfully", nil)
}

func (h *UserHandler) ListUsers(c *gin.Context) {
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

	if _, exists := c.Get("userID"); !exists {
		utils.ErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	response, err := h.userClient.ListUsers(c.Request.Context(), limit, offset)
	if err != nil {
		h.handleUserError(c, err, "LIST_FAILED", "Failed to retrieve users")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Users retrieved successfully", response)
}

func (h *UserHandler) SearchUsers(c *gin.Context) {
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

	response, err := h.userClient.SearchUsers(c.Request.Context(), query, limit, offset)
	if err != nil {
		h.handleUserError(c, err, "SEARCH_FAILED", "Failed to search users")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Search completed successfully", response)
}

func (h *UserHandler) GetUserProfile(c *gin.Context) {
	id := c.Param("id")

	response, err := h.userClient.GetUserProfile(c.Request.Context(), id)
	if err != nil {
		h.handleUserError(c, err, "PROFILE_NOT_FOUND", "User profile not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "User profile retrieved successfully", response)
}

func (h *UserHandler) GetStats(c *gin.Context) {
	response, err := h.userClient.GetStats(c.Request.Context())
	if err != nil {
		h.handleUserError(c, err, "STATS_FAILED", "Failed to retrieve user statistics")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "User statistics retrieved successfully", response)
}

func (h *UserHandler) Follow(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		utils.ErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}
	followeeID := c.Param("id")
	if followeeID == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "User ID is required")
		return
	}
	if err := h.userClient.Follow(c.Request.Context(), userID.(string), followeeID); err != nil {
		h.handleUserError(c, err, "FOLLOW_FAILED", "Failed to follow user")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Followed successfully", nil)
}

func (h *UserHandler) Unfollow(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		utils.ErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}
	followeeID := c.Param("id")
	if followeeID == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "User ID is required")
		return
	}
	if err := h.userClient.Unfollow(c.Request.Context(), userID.(string), followeeID); err != nil {
		h.handleUserError(c, err, "UNFOLLOW_FAILED", "Failed to unfollow user")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Unfollowed successfully", nil)
}

func (h *UserHandler) GetFollowers(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "User ID is required")
		return
	}
	limitStr := c.DefaultQuery("limit", "20")
	cursor := c.DefaultQuery("cursor", "")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	response, err := h.userClient.GetFollowers(c.Request.Context(), userID, limit, cursor)
	if err != nil {
		h.handleUserError(c, err, "FOLLOWERS_FAILED", "Failed to retrieve followers")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Followers retrieved successfully", response)
}

func (h *UserHandler) GetFollowing(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "User ID is required")
		return
	}
	limitStr := c.DefaultQuery("limit", "20")
	cursor := c.DefaultQuery("cursor", "")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	response, err := h.userClient.GetFollowing(c.Request.Context(), userID, limit, cursor)
	if err != nil {
		h.handleUserError(c, err, "FOLLOWING_FAILED", "Failed to retrieve following")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Following retrieved successfully", response)
}

func (h *UserHandler) handleUserError(c *gin.Context, err error, code, message string) {
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
		}
	}

	h.logger.Error("User service operation failed: " + err.Error())
	utils.ErrorResponse(c, http.StatusInternalServerError, code, message)
}
