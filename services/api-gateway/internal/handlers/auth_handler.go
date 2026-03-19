package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"api-gateway/internal/clients"
	"api-gateway/internal/config"
	"api-gateway/internal/models"
	"api-gateway/pkg/logger"
	"api-gateway/pkg/utils"
	authv1 "github.com/nikitashilov/microblog_grpc/proto/auth/v1"
)

const defaultRefreshTokenCookieMaxAge = 7 * 24 * 3600 // 7 days in seconds

type AuthHandler struct {
	authClient *clients.AuthClient
	cfg        *config.Config
	logger     *logger.Logger
}

func NewAuthHandler(authClient *clients.AuthClient, cfg *config.Config, logger *logger.Logger) *AuthHandler {
	return &AuthHandler{
		authClient: authClient,
		cfg:        cfg,
		logger:     logger,
	}
}

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

	h.setRefreshTokenCookieIfEnabled(c, resp.GetTokens())
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

	h.setRefreshTokenCookieIfEnabled(c, resp.GetTokens())
	utils.SuccessResponse(c, http.StatusOK, "Login successful", buildAuthResponse(resp.GetUser(), resp.GetTokens()))
}

func (h *AuthHandler) GetGoogleAuthURL(c *gin.Context) {
	platformRaw := strings.ToLower(strings.TrimSpace(c.DefaultQuery("platform", "web")))
	platform := authv1.OAuthPlatform_OAUTH_PLATFORM_WEB
	switch platformRaw {
	case "", "web":
		platform = authv1.OAuthPlatform_OAUTH_PLATFORM_WEB
	case "mobile":
		platform = authv1.OAuthPlatform_OAUTH_PLATFORM_MOBILE
	default:
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_PLATFORM", "platform must be web or mobile")
		return
	}

	req := &authv1.GetGoogleAuthURLRequest{
		Platform:            platform,
		ClientRedirectUri:   c.Query("redirect_uri"),
		CodeChallenge:       c.Query("code_challenge"),
		CodeChallengeMethod: c.Query("code_challenge_method"),
		ClientState:         c.Query("client_state"),
	}

	resp, err := h.authClient.GetGoogleAuthURL(c.Request.Context(), req)
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.InvalidArgument:
				utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", st.Message())
			case codes.Unauthenticated, codes.PermissionDenied:
				utils.ErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", st.Message())
			default:
				utils.ErrorResponse(c, http.StatusInternalServerError, "AUTH_URL_FAILED", "Failed to get Google auth URL")
			}
			return
		}

		h.logger.Error("Get Google auth URL failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusInternalServerError, "AUTH_URL_FAILED", "Failed to get Google auth URL")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Google auth URL generated", resp)
}

func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	h.logger.Info("Received Google callback with params: " + c.Request.URL.RawQuery)

	if errParam := c.Query("error"); errParam != "" {
		h.logger.Warn("Google OAuth error: " + errParam)
		utils.ErrorResponse(c, http.StatusBadRequest, "GOOGLE_OAUTH_ERROR", errParam)
		return
	}

	stateParam := c.Query("state")
	codeParam := c.Query("code")
	if stateParam == "" || codeParam == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_CALLBACK", "Missing state or code parameter")
		return
	}

	resp, err := h.authClient.HandleGoogleCallback(c.Request.Context(), stateParam, codeParam)
	if err != nil {
		h.logger.Error("Google callback failed: " + err.Error())
		if st, ok := status.FromError(err); ok && st.Code() == codes.Unauthenticated {
			utils.ErrorResponse(c, http.StatusUnauthorized, "INVALID_CALLBACK", st.Message())
			return
		}
		utils.ErrorResponse(c, http.StatusInternalServerError, "CALLBACK_FAILED", "Google callback failed")
		return
	}

	redirectURL, buildErr := buildClientRedirectURL(resp.GetClientRedirectUri(), resp.GetAuthCode(), resp.GetClientState())
	if buildErr != nil {
		h.logger.Error("Failed to build callback redirect URL: " + buildErr.Error())
		utils.ErrorResponse(c, http.StatusInternalServerError, "CALLBACK_FAILED", "Google callback failed")
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

func (h *AuthHandler) ExchangeAuthCode(c *gin.Context) {
	var req struct {
		AuthCode     string `json:"auth_code" binding:"required"`
		CodeVerifier string `json:"code_verifier"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid exchange auth code request: " + err.Error())
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format")
		return
	}

	resp, err := h.authClient.ExchangeAuthCodeWithVerifier(c.Request.Context(), req.AuthCode, req.CodeVerifier)
	if err != nil {
		h.logger.Error("Auth code exchange failed: " + err.Error())
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unauthenticated, codes.PermissionDenied:
				utils.ErrorResponse(c, http.StatusUnauthorized, "EXCHANGE_FAILED", st.Message())
			case codes.InvalidArgument:
				utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", st.Message())
			default:
				utils.ErrorResponse(c, http.StatusInternalServerError, "EXCHANGE_FAILED", "Auth code exchange failed")
			}
			return
		}
		utils.ErrorResponse(c, http.StatusUnauthorized, "EXCHANGE_FAILED", "Auth code exchange failed")
		return
	}

	h.setRefreshTokenCookieIfEnabled(c, resp.GetTokens())
	utils.SuccessResponse(c, http.StatusOK, "Auth code exchanged successfully", toAuthResponse(resp))
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	refreshToken := h.getRefreshTokenFromRequest(c)
	if refreshToken == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "Refresh token required (cookie or JSON body)")
		return
	}

	resp, err := h.authClient.RefreshToken(c.Request.Context(), refreshToken)
	if err != nil {
		h.logger.Error("Token refresh failed: " + err.Error())
		utils.ErrorResponse(c, http.StatusUnauthorized, "REFRESH_FAILED", "Token refresh failed")
		return
	}

	h.setRefreshTokenCookieIfEnabled(c, resp.GetTokens())
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

	h.clearRefreshTokenCookie(c)
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

// getRefreshTokenFromRequest returns refresh token from HttpOnly cookie first, then JSON body.
func (h *AuthHandler) getRefreshTokenFromRequest(c *gin.Context) string {
	if h.cfg != nil && h.cfg.Auth.UseRefreshTokenCookie {
		name := h.cfg.Auth.RefreshTokenCookieName
		if name == "" {
			name = "refresh_token"
		}
		if tok, err := c.Cookie(name); err == nil && tok != "" {
			return tok
		}
	}

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&req); err == nil && req.RefreshToken != "" {
		return req.RefreshToken
	}
	return ""
}

// setRefreshTokenCookieIfEnabled sets HttpOnly, Secure (in prod) cookie for refresh token if enabled.
func (h *AuthHandler) setRefreshTokenCookieIfEnabled(c *gin.Context, tokens *authv1.TokenPair) {
	if h.cfg == nil || !h.cfg.Auth.UseRefreshTokenCookie || tokens == nil {
		return
	}
	name := h.cfg.Auth.RefreshTokenCookieName
	if name == "" {
		name = "refresh_token"
	}

	sameSite := h.getRefreshCookieSameSite()
	secure := h.cfg.Environment == "production" || sameSite == http.SameSiteNoneMode

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    tokens.GetRefreshToken(),
		MaxAge:   defaultRefreshTokenCookieMaxAge,
		Path:     "/",
		Domain:   h.cfg.Auth.CookieDomain,
		Secure:   secure,
		HttpOnly: true,
		SameSite: sameSite,
	})
}

// clearRefreshTokenCookie removes the refresh token cookie on logout.
func (h *AuthHandler) clearRefreshTokenCookie(c *gin.Context) {
	if h.cfg == nil || !h.cfg.Auth.UseRefreshTokenCookie {
		return
	}
	name := h.cfg.Auth.RefreshTokenCookieName
	if name == "" {
		name = "refresh_token"
	}

	sameSite := h.getRefreshCookieSameSite()
	secure := h.cfg.Environment == "production" || sameSite == http.SameSiteNoneMode

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    "",
		MaxAge:   -1,
		Path:     "/",
		Domain:   h.cfg.Auth.CookieDomain,
		Secure:   secure,
		HttpOnly: true,
		SameSite: sameSite,
	})
}

func (h *AuthHandler) getRefreshCookieSameSite() http.SameSite {
	if h.cfg == nil {
		return http.SameSiteLaxMode
	}

	switch strings.ToLower(strings.TrimSpace(h.cfg.Auth.RefreshTokenCookieSameSite)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

func buildClientRedirectURL(rawClientRedirectURI, authCode, clientState string) (string, error) {
	parsed, err := url.Parse(rawClientRedirectURI)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" {
		return "", fmt.Errorf("redirect URI has no scheme")
	}

	query := parsed.Query()
	query.Set("auth_code", authCode)
	if strings.TrimSpace(clientState) != "" {
		query.Set("state", clientState)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
