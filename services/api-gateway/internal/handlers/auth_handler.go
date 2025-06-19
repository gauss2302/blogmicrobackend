package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"api-gateway/internal/clients"
	"api-gateway/pkg/logger"
	"api-gateway/pkg/utils"
)

type AuthHandler struct {
	authClient *clients.AuthClient
	userClient *clients.UserClient
	logger     *logger.Logger
}

func NewAuthHandler(authClient *clients.AuthClient, userClient *clients.UserClient, logger *logger.Logger) *AuthHandler {
	return &AuthHandler{
		authClient: authClient,
		userClient: userClient,
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
	h.logger.Info("Received Google callback with params: " + c.Request.URL.RawQuery)

	// Proxy request to auth service
	response, err := h.authClient.GoogleCallback(c.Request.Context(), c.Request.URL.Query())
	if err != nil {
		h.logger.Error("Google callback failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusInternalServerError, "CALLBACK_FAILED", "Google callback failed")
		return
	}

	h.logger.Info("Auth service response received")

	// Check if auth service returned a redirect URL
	if redirectURL, ok := response["redirect_url"].(string); ok {
		h.logger.Info("Redirecting browser to: " + redirectURL)
		c.Redirect(http.StatusTemporaryRedirect, redirectURL)
		return
	}

	// If no redirect URL, check for auth_code and redirect to frontend
	if authCode, ok := response["auth_code"].(string); ok {
		frontendURL := fmt.Sprintf("http://localhost:3000/auth/callback?auth_code=%s", authCode)
		h.logger.Info("Redirecting browser to frontend: " + frontendURL)
		c.Redirect(http.StatusTemporaryRedirect, frontendURL)
		return
	}

	// If neither redirect_url nor auth_code, return error
	h.logger.Error("No redirect URL or auth code in response")
	utils.ErrorResponse(c, http.StatusInternalServerError, "CALLBACK_FAILED", "Invalid callback response")
}

func (h *AuthHandler) ExchangeAuthCode(c *gin.Context) {
	var req map[string]interface{}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid exchange auth code request: " + err.Error())
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format")
		return
	}

	h.logger.Info("Processing auth code exchange in API Gateway")

	// Step 1: Exchange auth code with auth service
	authResponse, err := h.authClient.ExchangeAuthCode(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Auth code exchange failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusUnauthorized, "EXCHANGE_FAILED", "Auth code exchange failed")
		return
	}

	h.logger.Info("Auth service returned successful response")

	// Step 2: Extract user info from auth response
	userInfo, ok := authResponse["user"].(map[string]interface{})
	if !ok {
		h.logger.Error("Invalid user info in auth response")
		utils.ErrorResponse(c, http.StatusInternalServerError, "INVALID_RESPONSE", "Invalid auth response format")
		return
	}

	// Step 3: Register user in user service (async, non-blocking)
	go h.registerUserAsync(userInfo)

	// Step 4: Return auth response immediately (don't wait for user registration)
	h.logger.Info("Auth code exchanged successfully, user registration initiated")
	utils.SuccessResponse(c, http.StatusOK, "Auth code exchanged successfully", authResponse)
}

func (h *AuthHandler) registerUserAsync(userInfo map[string]interface{}) {
	h.logger.Info(fmt.Sprintf("Registering user asynchronously: %v", userInfo["email"]))

	// Create user registration request
	userReq := map[string]interface{}{
		"id":      userInfo["id"],
		"email":   userInfo["email"],
		"name":    userInfo["name"],
		"picture": userInfo["picture"],
	}

	// Create a background context for the async operation
	ctx := context.Background()

	// Try to create user (with retries)
	for attempts := 0; attempts < 3; attempts++ {
		_, err := h.userClient.CreateUser(ctx, userReq, userInfo["id"].(string))
		if err == nil {
			h.logger.Info(fmt.Sprintf("User registered successfully: %v", userInfo["email"]))
			return
		}

		// If user already exists, that's fine
		if isConflictError(err) {
			h.logger.Info(fmt.Sprintf("User already exists: %v", userInfo["email"]))
			return
		}

		h.logger.Warn(fmt.Sprintf("User registration attempt %d failed: %v", attempts+1, err))

		// Wait before retry
		if attempts < 2 {
			time.Sleep(time.Duration(attempts+1) * time.Second)
		}
	}

	h.logger.Error(fmt.Sprintf("Failed to register user after 3 attempts: %v", userInfo["email"]))
}

func (h *AuthHandler) ExchangeAuthCodeSync(c *gin.Context) {
	var req map[string]interface{}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid exchange auth code request: " + err.Error())
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format")
		return
	}

	h.logger.Info("Processing synchronous auth code exchange in API Gateway")

	// Step 1: Exchange auth code with auth service
	authResponse, err := h.authClient.ExchangeAuthCode(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Auth code exchange failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusUnauthorized, "EXCHANGE_FAILED", "Auth code exchange failed")
		return
	}

	// Step 2: Extract user info from auth response
	userInfo, ok := authResponse["user"].(map[string]interface{})
	if !ok {
		h.logger.Error("Invalid user info in auth response")
		utils.ErrorResponse(c, http.StatusInternalServerError, "INVALID_RESPONSE", "Invalid auth response format")
		return
	}

	// Step 3: Register user synchronously
	userReq := map[string]interface{}{
		"id":      userInfo["id"],
		"email":   userInfo["email"],
		"name":    userInfo["name"],
		"picture": userInfo["picture"],
	}

	_, userErr := h.userClient.CreateUser(c.Request.Context(), userReq, userInfo["id"].(string))
	if userErr != nil && !isConflictError(userErr) {
		h.logger.Warn(fmt.Sprintf("User registration failed (continuing): %v", userErr))
		// Continue with auth even if user registration fails
	} else if userErr == nil {
		h.logger.Info(fmt.Sprintf("User registered successfully: %v", userInfo["email"]))
	} else {
		h.logger.Info(fmt.Sprintf("User already exists: %v", userInfo["email"]))
	}

	// Step 4: Return successful auth response
	utils.SuccessResponse(c, http.StatusOK, "Auth code exchanged successfully", authResponse)
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

// Helper function to check if error is a conflict (user already exists)
func isConflictError(err error) bool {
	// Check if error message contains conflict indicators
	errMsg := err.Error()
	return strings.Contains(errMsg, "already exists") ||
		strings.Contains(errMsg, "conflict") ||
		strings.Contains(errMsg, "409")
}
