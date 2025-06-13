// Fix 5: Update services/api-gateway/internal/handlers/auth_handler.go
// Remove GoogleLogin method, keep only modern OAuth flow methods

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

// Modern OAuth2 Flow Handlers

func (h *AuthHandler) GetGoogleAuthURL(c *gin.Context) {
	// Proxy request to auth service
	response, err := h.authClient.GetGoogleAuthURL(c.Request.Context())
	if err != nil {
		h.logger.Error("Get Google auth URL failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusInternalServerError, "AUTH_URL_FAILED", "Failed to get Google auth URL")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Google auth URL generated", response)
}

func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	// Proxy request to auth service
	response, err := h.authClient.GoogleCallback(c.Request.Context(), c.Request.URL.Query())
	if err != nil {
		h.logger.Error("Google callback failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusInternalServerError, "CALLBACK_FAILED", "Google callback failed")
		return
	}

	// Redirect to frontend or return response
	if redirectURL, ok := response["redirect_url"].(string); ok {
		c.Redirect(http.StatusTemporaryRedirect, redirectURL)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Google callback processed", response)
}

func (h *AuthHandler) ExchangeAuthCode(c *gin.Context) {
	var req map[string]interface{}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid exchange auth code request: " + err.Error())
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format")
		return
	}

	response, err := h.authClient.ExchangeAuthCode(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Auth code exchange failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusUnauthorized, "EXCHANGE_FAILED", "Auth code exchange failed")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Auth code exchanged successfully", response)
}

// Token Management Handlers

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