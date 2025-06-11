package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"api-gateway/internal/clients"
	"api-gateway/pkg/logger"
	"api-gateway/pkg/utils"
)

type UserHandler struct {
	userClient *clients.UserClient
	logger     *logger.Logger
}

func NewUserHandler(userClient *clients.UserClient, logger *logger.Logger) *UserHandler {
	return &UserHandler{
		userClient: userClient,
		logger:     logger,
	}
}

func (h *UserHandler) CreateUser(c *gin.Context) {
	var req map[string]interface{}
	
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

	response, err := h.userClient.CreateUser(c.Request.Context(), req, userID.(string))
	if err != nil {
		h.logger.Error("Create user failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusInternalServerError, "CREATE_FAILED", "Failed to create user")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "User created successfully", response)
}

func (h *UserHandler) GetUser(c *gin.Context) {
	id := c.Param("id")
	
	userID, exists := c.Get("userID")
	if !exists {
		utils.ErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	response, err := h.userClient.GetUser(c.Request.Context(), id, userID.(string))
	if err != nil {
		h.logger.Error("Get user failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "User retrieved successfully", response)
}

func (h *UserHandler) UpdateUser(c *gin.Context) {
	id := c.Param("id")
	
	var req map[string]interface{}
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

	response, err := h.userClient.UpdateUser(c.Request.Context(), id, req, userID.(string))
	if err != nil {
		h.logger.Error("Update user failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusInternalServerError, "UPDATE_FAILED", "Failed to update user")
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

	err := h.userClient.DeleteUser(c.Request.Context(), id, userID.(string))
	if err != nil {
		h.logger.Error("Delete user failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusInternalServerError, "DELETE_FAILED", "Failed to delete user")
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
	if err != nil || offset < 0 {
		offset = 0
	}

	userID, exists := c.Get("userID")
	if !exists {
		utils.ErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	response, err := h.userClient.ListUsers(c.Request.Context(), limit, offset, userID.(string))
	if err != nil {
		h.logger.Error("List users failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusInternalServerError, "LIST_FAILED", "Failed to retrieve users")
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
	if err != nil || offset < 0 {
		offset = 0
	}

	response, err := h.userClient.SearchUsers(c.Request.Context(), query, limit, offset)
	if err != nil {
		h.logger.Error("Search users failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusInternalServerError, "SEARCH_FAILED", "Failed to search users")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Search completed successfully", response)
}

func (h *UserHandler) GetUserProfile(c *gin.Context) {
	id := c.Param("id")

	response, err := h.userClient.GetUserProfile(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("Get user profile failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusNotFound, "PROFILE_NOT_FOUND", "User profile not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "User profile retrieved successfully", response)
}

func (h *UserHandler) GetStats(c *gin.Context) {
	response, err := h.userClient.GetStats(c.Request.Context())
	if err != nil {
		h.logger.Error("Get user stats failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusInternalServerError, "STATS_FAILED", "Failed to retrieve user statistics")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "User statistics retrieved successfully", response)
}