package handlers

import (
	"auth-service/internal/application/errors"
	"auth-service/internal/application/services"
	"auth-service/internal/application/services/dto"
	"auth-service/internal/interfaces/validators"
	"auth-service/pkg/logger"
	"auth-service/pkg/utils"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *services.AuthService
	validator   *validators.AuthValidator
	logger      *logger.Logger
}

func NewAuthHandler(authService *services.AuthService, logger *logger.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		validator:   validators.NewAuthValidator(),
		logger:      logger,
	}
}

// Step 1: Get Google Auth URL
func (h *AuthHandler) GetGoogleAuthURL(c *gin.Context) {
	h.logger.Info("Processing Google auth URL request")
	
	response, err := h.authService.GetGoogleAuthURL(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get Google auth URL: " + err.Error())
		if authErr, ok := err.(*errors.AuthError); ok {
			utils.ErrorResponse(c, authErr)
		} else {
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

	h.logger.Info("Google auth URL generated successfully")
	utils.SuccessResponse(c, http.StatusOK, "Google auth URL generated", response)
}

// Step 2: Handle Google OAuth Callback (redirects user back from Google)
func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	state := c.Query("state")
	code := c.Query("code")
	errorParam := c.Query("error")

	h.logger.Info(fmt.Sprintf("Processing Google callback - state: %s, code present: %t, error: %s", 
		state, code != "", errorParam))

	// Handle OAuth errors from Google
	if errorParam != "" {
		h.logger.Warn("Google OAuth error: " + errorParam)
		frontendURL := h.getFrontendErrorURL("google_oauth_error")
		c.Redirect(http.StatusTemporaryRedirect, frontendURL)
		return
	}

	// Validate required parameters
	if state == "" || code == "" {
		h.logger.Warn("Missing required callback parameters")
		frontendURL := h.getFrontendErrorURL("invalid_callback")
		c.Redirect(http.StatusTemporaryRedirect, frontendURL)
		return
	}

	callbackReq := &dto.GoogleCallbackRequest{
		State: state,
		Code:  code,
	}

	if err := h.validator.ValidateGoogleCallbackRequest(callbackReq); err != nil {
		h.logger.Warn("Google callback validation failed: " + err.Error())
		frontendURL := h.getFrontendErrorURL("validation_failed")
		c.Redirect(http.StatusTemporaryRedirect, frontendURL)
		return
	}

	response, err := h.authService.HandleGoogleCallback(c.Request.Context(), callbackReq)
	if err != nil {
		h.logger.Error("Google callback processing failed: " + err.Error())
		
		// Provide more specific error handling
		if authErr, ok := err.(*errors.AuthError); ok {
			switch authErr.Code {
			case "INVALID_GOOGLE_CODE":
				frontendURL := h.getFrontendErrorURL("invalid_code")
				c.Redirect(http.StatusTemporaryRedirect, frontendURL)
			default:
				frontendURL := h.getFrontendErrorURL("callback_failed")
				c.Redirect(http.StatusTemporaryRedirect, frontendURL)
			}
		} else {
			frontendURL := h.getFrontendErrorURL("callback_failed")
			c.Redirect(http.StatusTemporaryRedirect, frontendURL)
		}
		return
	}

	// Success - redirect to frontend with temporary auth code
	frontendURL := h.getFrontendSuccessURL(response.AuthCode)
	h.logger.Info("Google callback processed successfully, redirecting to frontend")
	c.Redirect(http.StatusTemporaryRedirect, frontendURL)
}

// Step 3: Exchange temporary auth code for JWT tokens
func (h *AuthHandler) ExchangeAuthCode(c *gin.Context) {
	var req dto.ExchangeAuthCodeRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid exchange auth code request: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	if err := h.validator.ValidateExchangeAuthCodeRequest(&req); err != nil {
		h.logger.Warn("Exchange auth code validation failed: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	h.logger.Info("Processing auth code exchange")
	response, err := h.authService.ExchangeAuthCode(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Auth code exchange failed: " + err.Error())
		if authErr, ok := err.(*errors.AuthError); ok {
			utils.ErrorResponse(c, authErr)
		} else {
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

	h.logger.Info("Auth code exchanged successfully")
	utils.SuccessResponse(c, http.StatusOK, "Auth code exchanged successfully", response)
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req dto.RefreshTokenRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid refresh token request: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	if err := h.validator.ValidateRefreshTokenRequest(&req); err != nil {
		h.logger.Warn("Refresh token validation failed: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	response, err := h.authService.RefreshToken(c.Request.Context(), &req)
	if err != nil {
		if authErr, ok := err.(*errors.AuthError); ok {
			utils.ErrorResponse(c, authErr)
		} else {
			h.logger.Error("Unexpected error in token refresh: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Token refreshed successfully", response)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req dto.LogoutRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid logout request: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	if err := h.validator.ValidateLogoutRequest(&req); err != nil {
		h.logger.Warn("Logout validation failed: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	err := h.authService.Logout(c.Request.Context(), &req)
	if err != nil {
		if authErr, ok := err.(*errors.AuthError); ok {
			utils.ErrorResponse(c, authErr)
		} else {
			h.logger.Error("Unexpected error in logout: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Logged out successfully", nil)
}

func (h *AuthHandler) ValidateToken(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token == "" {
		utils.ErrorResponse(c, errors.ErrInvalidAccessToken)
		return
	}

	// Remove "Bearer " prefix
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	response, err := h.authService.ValidateToken(c.Request.Context(), token)
	if err != nil {
		if authErr, ok := err.(*errors.AuthError); ok {
			utils.ErrorResponse(c, authErr)
		} else {
			h.logger.Error("Unexpected error in token validation: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Token is valid", response)
}

func (h *AuthHandler) HealthCheck(c *gin.Context) {
	utils.SuccessResponse(c, http.StatusOK, "Auth service is healthy", gin.H{
		"service": "auth-service",
		"status":  "running",
	})
}

// Helper methods for frontend URL construction
func (h *AuthHandler) getFrontendErrorURL(errorType string) string {
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000" // Default fallback
	}
	return fmt.Sprintf("%s/auth/login?error=%s", frontendURL, errorType)
}

func (h *AuthHandler) getFrontendSuccessURL(authCode string) string {
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000" // Default fallback
	}
	return fmt.Sprintf("%s/auth/callback?auth_code=%s", frontendURL, authCode)
}