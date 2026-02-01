package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"api-gateway/internal/clients"
	"api-gateway/internal/models"
	"api-gateway/pkg/logger"
	"api-gateway/pkg/utils"
	authv1 "github.com/nikitashilov/microblog_grpc/proto/auth/v1"
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

func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=8"`
		Name     string `json:"name" binding:"required,min=1,max=100"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid register request: " + err.Error())
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format")
		return
	}

	resp, err := h.authClient.Register(c.Request.Context(), req.Email, req.Password, req.Name)
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.AlreadyExists {
			utils.ErrorResponse(c, http.StatusConflict, "USER_ALREADY_EXISTS", "User with this email already exists")
			return
		}
		h.logger.Error("Register failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusInternalServerError, "REGISTER_FAILED", "Registration failed")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "User registered successfully", buildAuthResponse(resp.GetUser(), resp.GetTokens()))
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid login request: " + err.Error())
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format")
		return
	}

	resp, err := h.authClient.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.Unauthenticated {
			utils.ErrorResponse(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password")
			return
		}
		h.logger.Error("Login failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusInternalServerError, "LOGIN_FAILED", "Login failed")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Login successful", buildAuthResponse(resp.GetUser(), resp.GetTokens()))
}

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

	if errParam := c.Query("error"); errParam != "" {
		h.logger.Warn("Google OAuth error: " + errParam)
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("http://localhost:3000/auth/login?error=%s", errParam))
		return
	}

	state := c.Query("state")
	code := c.Query("code")
	if state == "" || code == "" {
		h.logger.Warn("Missing required callback parameters")
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_CALLBACK", "Missing state or code parameter")
		return
	}

	resp, err := h.authClient.HandleGoogleCallback(c.Request.Context(), state, code)
	if err != nil {
		h.logger.Error("Google callback failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusInternalServerError, "CALLBACK_FAILED", "Google callback failed")
		return
	}

	frontendURL := fmt.Sprintf("http://localhost:3000/auth/callback?auth_code=%s", resp.GetAuthCode())
	h.logger.Info("Redirecting browser to frontend: " + frontendURL)
	c.Redirect(http.StatusTemporaryRedirect, frontendURL)
}

func (h *AuthHandler) ExchangeAuthCode(c *gin.Context) {
	var req struct {
		AuthCode string `json:"auth_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid exchange auth code request: " + err.Error())
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format")
		return
	}

	h.logger.Info("Processing auth code exchange in API Gateway")

	resp, err := h.authClient.ExchangeAuthCode(c.Request.Context(), req.AuthCode)
	if err != nil {
		h.logger.Error("Auth code exchange failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusUnauthorized, "EXCHANGE_FAILED", "Auth code exchange failed")
		return
	}

	userInfo := resp.GetUser()
	if userInfo == nil {
		h.logger.Error("Invalid user info in auth response")
		utils.ErrorResponse(c, http.StatusInternalServerError, "INVALID_RESPONSE", "Invalid auth response format")
		return
	}

	// Synchronous user creation: ensure user record exists before returning tokens (plan: prefer sync create).
	h.registerUserSync(c.Request.Context(), userInfo)

	h.logger.Info("Auth code exchanged successfully")
	utils.SuccessResponse(c, http.StatusOK, "Auth code exchanged successfully", toAuthResponse(resp))
}

// registerUserSync creates the user in user-service synchronously (retries on conflict/transient errors).
func (h *AuthHandler) registerUserSync(ctx context.Context, userInfo *authv1.UserInfo) {
	if userInfo == nil {
		return
	}

	userReq := &clients.CreateUserInput{
		ID:      userInfo.GetId(),
		Email:   userInfo.GetEmail(),
		Name:    userInfo.GetName(),
		Picture: userInfo.GetPicture(),
	}
	if userReq.Name == "" {
		userReq.Name = userInfo.GetEmail()
	}

	for attempts := 0; attempts < 3; attempts++ {
		_, err := h.userClient.CreateUser(ctx, userReq)
		if err == nil {
			h.logger.Info(fmt.Sprintf("User registered: %v", userInfo.GetEmail()))
			return
		}
		if isConflictError(err) {
			h.logger.Info(fmt.Sprintf("User already exists: %v", userInfo.GetEmail()))
			return
		}
		h.logger.Warn(fmt.Sprintf("User registration attempt %d failed: %v", attempts+1, err))
		if attempts < 2 {
			time.Sleep(time.Duration(attempts+1) * time.Second)
		}
	}
	h.logger.Error(fmt.Sprintf("Failed to register user after 3 attempts: %v", userInfo.GetEmail()))
}

func (h *AuthHandler) registerUserAsync(userInfo *authv1.UserInfo) {
	if userInfo == nil {
		return
	}
	ctx := context.Background()
	// Reuse sync logic in background for any code paths that still call async.
	h.registerUserSync(ctx, userInfo)
}

func (h *AuthHandler) ExchangeAuthCodeSync(c *gin.Context) {
	var req struct {
		AuthCode string `json:"auth_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid exchange auth code request: " + err.Error())
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format")
		return
	}

	h.logger.Info("Processing synchronous auth code exchange in API Gateway")

	resp, err := h.authClient.ExchangeAuthCode(c.Request.Context(), req.AuthCode)
	if err != nil {
		h.logger.Error("Auth code exchange failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusUnauthorized, "EXCHANGE_FAILED", "Auth code exchange failed")
		return
	}

	userInfo := resp.GetUser()
	if userInfo == nil {
		h.logger.Error("Invalid user info in auth response")
		utils.ErrorResponse(c, http.StatusInternalServerError, "INVALID_RESPONSE", "Invalid auth response format")
		return
	}

	// Step 3: Register user synchronously
	userReq := &clients.CreateUserInput{
		ID:      userInfo.GetId(),
		Email:   userInfo.GetEmail(),
		Name:    userInfo.GetName(),
		Picture: userInfo.GetPicture(),
	}

	if userReq.Name == "" {
		userReq.Name = userInfo.GetEmail()
	}

	_, userErr := h.userClient.CreateUser(c.Request.Context(), userReq)
	if userErr != nil && !isConflictError(userErr) {
		h.logger.Warn(fmt.Sprintf("User registration failed (continuing): %v", userErr))
		// Continue with auth even if user registration fails
	} else if userErr == nil {
		h.logger.Info(fmt.Sprintf("User registered successfully: %v", userInfo.GetEmail()))
	} else {
		h.logger.Info(fmt.Sprintf("User already exists: %v", userInfo.GetEmail()))
	}

	utils.SuccessResponse(c, http.StatusOK, "Auth code exchanged successfully", toAuthResponse(resp))
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid refresh token request: " + err.Error())
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format")
		return
	}

	resp, err := h.authClient.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		h.logger.Error("Token refresh failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusUnauthorized, "REFRESH_FAILED", "Token refresh failed")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Token refreshed successfully", toAuthResponseFromRefresh(resp))
}

func (h *AuthHandler) Logout(c *gin.Context) {
	token, exists := c.Get("token")
	if !exists {
		utils.ErrorResponse(c, http.StatusUnauthorized, "MISSING_TOKEN", "Authentication required")
		return
	}

	if err := h.authClient.Logout(c.Request.Context(), token.(string)); err != nil {
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

	resp, err := h.authClient.ValidateToken(c.Request.Context(), token.(string))
	if err != nil {
		h.logger.Error("Token validation failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusUnauthorized, "VALIDATION_FAILED", "Token validation failed")
		return
	}

	if !resp.GetValid() {
		h.logger.Warn("Token validation returned invalid status")
		utils.ErrorResponse(c, http.StatusUnauthorized, "INVALID_TOKEN", "Token validation failed")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Token is valid", toTokenValidationResponse(resp))
}

func toAuthResponse(resp *authv1.ExchangeAuthCodeResponse) *models.AuthResponse {
	if resp == nil {
		return nil
	}
	return buildAuthResponse(resp.GetUser(), resp.GetTokens())
}

func toAuthResponseFromRefresh(resp *authv1.RefreshTokenResponse) *models.AuthResponse {
	if resp == nil {
		return nil
	}
	return buildAuthResponse(resp.GetUser(), resp.GetTokens())
}

func buildAuthResponse(user *authv1.UserInfo, tokens *authv1.TokenPair) *models.AuthResponse {
	if user == nil || tokens == nil {
		return nil
	}

	return &models.AuthResponse{
		AccessToken:  tokens.GetAccessToken(),
		RefreshToken: tokens.GetRefreshToken(),
		TokenType:    tokens.GetTokenType(),
		ExpiresIn:    int(tokens.GetExpiresIn()),
		User: &models.UserInfo{
			ID:      user.GetId(),
			Email:   user.GetEmail(),
			Name:    user.GetName(),
			Picture: user.GetPicture(),
		},
	}
}

func toTokenValidationResponse(resp *authv1.ValidateTokenResponse) *models.TokenValidationResponse {
	if resp == nil {
		return nil
	}

	return &models.TokenValidationResponse{
		Valid:  resp.GetValid(),
		UserID: resp.GetUserId(),
		Email:  resp.GetEmail(),
	}
}

// Helper function to check if error is a conflict (user already exists)
func isConflictError(err error) bool {
	if err == nil {
		return false
	}

	if st, ok := status.FromError(err); ok {
		return st.Code() == codes.AlreadyExists
	}

	errMsg := err.Error()
	return strings.Contains(errMsg, "already exists") || strings.Contains(errMsg, "conflict") || strings.Contains(errMsg, "409")
}
