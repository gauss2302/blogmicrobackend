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
	response, err := h.authService.GetGoogleAuthURL(c.Request.Context())
	if err != nil {
		if authErr, ok := err.(*errors.AuthError); ok {
			utils.ErrorResponse(c, authErr)
		} else {
			h.logger.Error("Unexpected error getting Google auth URL: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Google auth URL generated", response)
}

// Step 2: Handle Google OAuth Callback (redirects user back from Google)
func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	state := c.Query("state")
	code := c.Query("code")
	errorParam := c.Query("error")

	if errorParam != "" {
		h.logger.Warn("Google OAuth error: " + errorParam)
		frontendURL := fmt.Sprintf("%s/auth/login?error=%s", 
			os.Getenv("FRONTEND_URL"), errorParam)
		c.Redirect(http.StatusTemporaryRedirect, frontendURL)
		return
	}

	callbackReq := &dto.GoogleCallbackRequest{
		State: state,
		Code:  code,
	}

	if err := h.validator.ValidateGoogleCallbackRequest(callbackReq); err != nil {
		h.logger.Warn("Google callback validation failed: " + err.Error())
		frontendURL := fmt.Sprintf("%s/auth/login?error=invalid_callback", 
			os.Getenv("FRONTEND_URL"))
		c.Redirect(http.StatusTemporaryRedirect, frontendURL)
		return
	}

	response, err := h.authService.HandleGoogleCallback(c.Request.Context(), callbackReq)
	if err != nil {
		h.logger.Error("Google callback failed: " + err.Error())
		frontendURL := fmt.Sprintf("%s/auth/login?error=callback_failed", 
			os.Getenv("FRONTEND_URL"))
		c.Redirect(http.StatusTemporaryRedirect, frontendURL)
		return
	}

	// Redirect to frontend with temporary auth code
	frontendURL := fmt.Sprintf("%s/auth/callback?auth_code=%s", 
		os.Getenv("FRONTEND_URL"), response.AuthCode)
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

	response, err := h.authService.ExchangeAuthCode(c.Request.Context(), &req)
	if err != nil {
		if authErr, ok := err.(*errors.AuthError); ok {
			utils.ErrorResponse(c, authErr)
		} else {
			h.logger.Error("Unexpected error in auth code exchange: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

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