package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"api-gateway/internal/clients"
	"api-gateway/pkg/logger"
	"api-gateway/pkg/utils"
)

type AuthHandler struct {
	authClient *clients.AuthClient
	logger     *logger.Logger
}

func NewAuthHandler(authClient *clients.AuthClient, logger *logger.Logger) *AuthHandler {
	return &AuthHandler{
		authClient: authClient,
		logger:     logger,
	}
}

func (h *AuthHandler) GoogleLogin(c *gin.Context) {
	var req map[string]interface{}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid Google login request: " + err.Error())
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format")
		return
	}

	response, err := h.authClient.GoogleLogin(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Google login failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusUnauthorized, "LOGIN_FAILED", "Authentication failed")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Login successful", response)
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req map[string]interface{}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid refresh token request: " + err.Error())
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format")
		return
	}

	response, err := h.authClient.RefreshToken(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Token refresh failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusUnauthorized, "REFRESH_FAILED", "Token refresh failed")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Token refreshed successfully", response)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req map[string]interface{}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid logout request: " + err.Error())
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format")
		return
	}

	token, exists := c.Get("token")
	if !exists {
		utils.ErrorResponse(c, http.StatusUnauthorized, "MISSING_TOKEN", "Authentication required")
		return
	}

	err := h.authClient.Logout(c.Request.Context(), req, token.(string))
	if err != nil {
		h.logger.Error("Logout failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusInternalServerError, "LOGOUT_FAILED", "Logout failed")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Logged out successfully", nil)
}

func (h *AuthHandler) ValidateToken(c *gin.Context) {
	token, exists := c.Get("token")
	if !exists {
		utils.ErrorResponse(c, http.StatusUnauthorized, "MISSING_TOKEN", "Authentication required")
		return
	}

	response, err := h.authClient.ValidateToken(c.Request.Context(), token.(string))
	if err != nil {
		h.logger.Error("Token validation failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusUnauthorized, "VALIDATION_FAILED", "Token validation failed")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Token is valid", response)
}