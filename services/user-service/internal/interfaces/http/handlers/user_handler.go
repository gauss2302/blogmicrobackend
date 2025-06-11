package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"user-service/internal/application/dto"
	"user-service/internal/application/errors"
	"user-service/internal/application/services"
	"user-service/internal/interfaces/validators"
	"user-service/pkg/logger"
	"user-service/pkg/utils"
)

type UserHandler struct {
	userService *services.UserService
	validator   *validators.UserValidator
	logger      *logger.Logger
}

func NewUserHandler(userService *services.UserService, logger *logger.Logger) *UserHandler {
	return &UserHandler{
		userService: userService,
		validator:   validators.NewUserValidator(),
		logger:      logger,
	}
}

func (h *UserHandler) CreateUser(c *gin.Context) {
	var req dto.CreateUserRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid create user request: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	if err := h.validator.ValidateCreateUserRequest(&req); err != nil {
		h.logger.Warn("Create user validation failed: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidUserData)
		return
	}

	response, err := h.userService.CreateUser(c.Request.Context(), &req)
	if err != nil {
		if userErr, ok := err.(*errors.UserError); ok {
			utils.ErrorResponse(c, userErr)
		} else {
			h.logger.Error("Unexpected error in create user: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "User created successfully", response)
}

func (h *UserHandler) GetUser(c *gin.Context) {
	id := c.Param("id")
	
	if id == "" {
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	response, err := h.userService.GetUser(c.Request.Context(), id)
	if err != nil {
		if userErr, ok := err.(*errors.UserError); ok {
			utils.ErrorResponse(c, userErr)
		} else {
			h.logger.Error("Unexpected error in get user: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "User retrieved successfully", response)
}

func (h *UserHandler) GetUserProfile(c *gin.Context) {
	id := c.Param("id")
	
	if id == "" {
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	response, err := h.userService.GetUserProfile(c.Request.Context(), id)
	if err != nil {
		if userErr, ok := err.(*errors.UserError); ok {
			utils.ErrorResponse(c, userErr)
		} else {
			h.logger.Error("Unexpected error in get user profile: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "User profile retrieved successfully", response)
}

func (h *UserHandler) UpdateUser(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetHeader("X-User-ID")
	
	// Check if user is updating their own profile
	if id != userID {
		utils.ErrorResponse(c, errors.ErrUnauthorizedAccess)
		return
	}

	var req dto.UpdateUserRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid update user request: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	if err := h.validator.ValidateUpdateUserRequest(&req); err != nil {
		h.logger.Warn("Update user validation failed: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidUserData)
		return
	}

	response, err := h.userService.UpdateUser(c.Request.Context(), id, &req)
	if err != nil {
		if userErr, ok := err.(*errors.UserError); ok {
			utils.ErrorResponse(c, userErr)
		} else {
			h.logger.Error("Unexpected error in update user: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "User updated successfully", response)
}

func (h *UserHandler) DeleteUser(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetHeader("X-User-ID")
	
	// Check if user is deleting their own account
	if id != userID {
		utils.ErrorResponse(c, errors.ErrUnauthorizedAccess)
		return
	}

	err := h.userService.DeleteUser(c.Request.Context(), id)
	if err != nil {
		if userErr, ok := err.(*errors.UserError); ok {
			utils.ErrorResponse(c, userErr)
		} else {
			h.logger.Error("Unexpected error in delete user: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "User deleted successfully", nil)
}

func (h *UserHandler) ListUsers(c *gin.Context) {
	var req dto.ListUsersRequest
	
	if err := c.ShouldBindQuery(&req); err != nil {
		h.logger.Warn("Invalid list users request: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	// Set defaults
	if req.Limit == 0 {
		req.Limit = 20
	}

	response, err := h.userService.ListUsers(c.Request.Context(), &req)
	if err != nil {
		if userErr, ok := err.(*errors.UserError); ok {
			utils.ErrorResponse(c, userErr)
		} else {
			h.logger.Error("Unexpected error in list users: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Users retrieved successfully", response)
}

func (h *UserHandler) SearchUsers(c *gin.Context) {
	var req dto.SearchUsersRequest
	
	if err := c.ShouldBindQuery(&req); err != nil {
		h.logger.Warn("Invalid search users request: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	// Set defaults
	if req.Limit == 0 {
		req.Limit = 20
	}

	if err := h.validator.ValidateSearchUsersRequest(&req); err != nil {
		h.logger.Warn("Search users validation failed: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	response, err := h.userService.SearchUsers(c.Request.Context(), &req)
	if err != nil {
		if userErr, ok := err.(*errors.UserError); ok {
			utils.ErrorResponse(c, userErr)
		} else {
			h.logger.Error("Unexpected error in search users: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "User search completed successfully", response)
}

func (h *UserHandler) GetStats(c *gin.Context) {
	response, err := h.userService.GetStats(c.Request.Context())
	if err != nil {
		if userErr, ok := err.(*errors.UserError); ok {
			utils.ErrorResponse(c, userErr)
		} else {
			h.logger.Error("Unexpected error in get stats: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "User statistics retrieved successfully", response)
}

func (h *UserHandler) HealthCheck(c *gin.Context) {
	utils.SuccessResponse(c, http.StatusOK, "User service is healthy", gin.H{
		"service": "user-service",
		"status":  "running",
	})
}