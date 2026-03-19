package services

import (
	"auth-service/internal/application/errors"
	"auth-service/internal/application/services/dto"
	"auth-service/internal/config"
	"auth-service/internal/domain/entities"
	"auth-service/internal/domain/repositories"
	domainServices "auth-service/internal/domain/services"
	"auth-service/pkg/jwt"
	"auth-service/pkg/logger"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UserInfoResult is returned by user-service CreateUser/GetUserByEmail/ValidateCredentials.
type UserInfoResult interface {
	GetId() string
	GetEmail() string
	GetName() string
	GetPicture() string
}

// UserServiceClient is used by auth-service for user lifecycle operations.
type UserServiceClient interface {
	CreateUser(ctx context.Context, id, email, name, picture, password string) (UserInfoResult, error)
	GetUserByEmail(ctx context.Context, email string) (UserInfoResult, error)
	ValidateCredentials(ctx context.Context, email, password string) (UserInfoResult, error)
}

type AuthService struct {
	tokenRepo     repositories.TokenRepository
	oauthProvider domainServices.OAuthProvider
	userClient    UserServiceClient
	jwtManager    *jwt.Manager
	jwtConfig     config.JWTConfig
	googleConfig  config.GoogleConfig
	logger        *logger.Logger
}

func NewAuthService(
	tokenRepo repositories.TokenRepository,
	oauthProvider domainServices.OAuthProvider,
	userClient UserServiceClient,
	jwtConfig config.JWTConfig,
	googleConfig config.GoogleConfig,
	logger *logger.Logger,
) *AuthService {
	jwtManager := jwt.NewManager(jwtConfig.Secret, jwtConfig.Issuer)

	return &AuthService{
		tokenRepo:     tokenRepo,
		oauthProvider: oauthProvider,
		userClient:    userClient,
		jwtConfig:     jwtConfig,
		googleConfig:  googleConfig,
		jwtManager:    jwtManager,
		logger:        logger,
	}
}

// Main OAuth Flow: Get Google Auth URL.
func (s *AuthService) GetGoogleAuthURL(ctx context.Context, req *dto.GoogleAuthURLRequest) (*dto.GoogleAuthURLResponse, error) {
	platform, err := normalizeOAuthPlatform(req)
	if err != nil {
		return nil, err
	}

	clientRedirectURI, err := s.resolveClientRedirectURI(platform, req)
	if err != nil {
		return nil, err
	}

	codeChallenge, challengeMethod, err := normalizePKCE(platform, req)
	if err != nil {
		return nil, err
	}

	clientState := ""
	if req != nil {
		clientState = strings.TrimSpace(req.ClientState)
	}

	state, err := generateSecureToken(32)
	if err != nil {
		s.logger.Error("Failed to generate secure oauth state: " + err.Error())
		return nil, errors.ErrServiceUnavailable
	}

	statePayload := &entities.OAuthState{
		State:               state,
		Platform:            platform,
		ClientRedirectURI:   clientRedirectURI,
		ClientState:         clientState,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: challengeMethod,
	}

	if err := s.tokenRepo.StoreState(ctx, state, statePayload, 10*time.Minute); err != nil {
		s.logger.Error("Failed to store oauth state: " + err.Error())
		return nil, errors.ErrTokenStorage
	}

	authURL := s.oauthProvider.GetAuthURL(&domainServices.AuthURLRequest{
		State:               state,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: challengeMethod,
	})

	return &dto.GoogleAuthURLResponse{
		AuthURL: authURL,
		State:   state,
	}, nil
}

// OAuth Callback Handler.
func (s *AuthService) HandleGoogleCallback(ctx context.Context, req *dto.GoogleCallbackRequest) (*dto.GoogleCallbackResponse, error) {
	s.logger.Info(fmt.Sprintf("Processing Google callback - state: %s, code length: %d", req.State, len(req.Code)))

	storedState, err := s.tokenRepo.GetAndDeleteState(ctx, req.State)
	if err != nil || storedState == nil || storedState.State != req.State {
		s.logger.Warn("Invalid or expired OAuth state")
		return nil, errors.ErrInvalidOAuthState
	}

	userInfo, err := s.oauthProvider.ExchangeCodeForToken(ctx, req.Code)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to exchange Google code: %v", err))
		return nil, errors.ErrInvalidGoogleCode
	}
	if !userInfo.IsValid() {
		s.logger.Error("Invalid user info received from Google")
		return nil, errors.ErrInvalidGoogleCode
	}
	if !s.isAllowedEmailDomain(userInfo.Email) {
		s.logger.Warn("Google account rejected by domain allowlist: " + userInfo.Email)
		return nil, errors.ErrInvalidGoogleCode
	}

	canonicalUser, err := s.ensureUserExists(ctx, userInfo)
	if err != nil {
		return nil, err
	}

	authCode, err := generateSecureToken(32)
	if err != nil {
		s.logger.Error("Failed to generate temporary auth code: " + err.Error())
		return nil, errors.ErrServiceUnavailable
	}

	authPayload := &entities.AuthCodePayload{
		User:                canonicalUser,
		Platform:            storedState.Platform,
		ClientRedirectURI:   storedState.ClientRedirectURI,
		ClientState:         storedState.ClientState,
		CodeChallenge:       storedState.CodeChallenge,
		CodeChallengeMethod: storedState.CodeChallengeMethod,
	}
	if err := s.tokenRepo.StoreAuthCode(ctx, authCode, authPayload, 5*time.Minute); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to store auth code in Redis: %v", err))
		return nil, errors.ErrTokenStorage
	}

	return &dto.GoogleCallbackResponse{
		AuthCode:          authCode,
		ClientRedirectURI: storedState.ClientRedirectURI,
		ClientState:       storedState.ClientState,
		Platform:          toDTOPlatform(storedState.Platform),
	}, nil
}

// Exchange temporary auth code for JWT tokens.
func (s *AuthService) ExchangeAuthCode(ctx context.Context, req *dto.ExchangeAuthCodeRequest) (*dto.ExchangeAuthCodeResponse, error) {
	s.logger.Info("Processing auth code exchange")

	authPayload, err := s.tokenRepo.GetAndDeleteAuthCode(ctx, req.AuthCode)
	if err != nil || authPayload == nil || authPayload.User == nil {
		s.logger.Warn(fmt.Sprintf("Invalid or expired auth code: %s", req.AuthCode))
		return nil, errors.ErrInvalidGoogleCode
	}

	if err := verifyPKCE(req.CodeVerifier, authPayload.CodeChallenge, authPayload.CodeChallengeMethod); err != nil {
		return nil, err
	}

	tokenPair, err := s.generateTokenPair(authPayload.User)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to generate tokens for user %s: %v", authPayload.User.Email, err))
		return nil, errors.ErrTokenGeneration
	}

	if err := s.storeTokens(ctx, tokenPair, authPayload.User); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to store tokens for user %s: %v", authPayload.User.Email, err))
		return nil, errors.ErrTokenStorage
	}

	return &dto.ExchangeAuthCodeResponse{
		User: &dto.UserInfo{
			ID:      authPayload.User.ID,
			Email:   authPayload.User.Email,
			Name:    authPayload.User.Name,
			Picture: authPayload.User.Picture,
		},
		Tokens: &dto.TokenPair{
			AccessToken:  tokenPair.AccessToken,
			RefreshToken: tokenPair.RefreshToken,
			TokenType:    tokenPair.TokenType,
			ExpiresIn:    tokenPair.ExpiresIn,
		},
	}, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, req *dto.RefreshTokenRequest) (*dto.RefreshTokenResponse, error) {
	s.logger.Info("Processing token refresh")

	// Validate refresh token
	claims, err := s.jwtManager.ValidateToken(req.RefreshToken)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Invalid refresh token: %v", err))
		return nil, errors.ErrInvalidRefreshToken
	}

	if claims.Type != "refresh" {
		s.logger.Error("Invalid token type for refresh")
		return nil, errors.ErrInvalidTokenType
	}

	// Check if token exists in Redis
	storedToken, err := s.tokenRepo.GetTokenData(ctx, req.RefreshToken)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Refresh token not found in store: %v", err))
		return nil, errors.ErrTokenNotFound
	}

	// Check if token is blacklisted
	blacklisted, err := s.tokenRepo.IsTokenBlacklisted(ctx, req.RefreshToken)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to check token blacklist: %v", err))
		return nil, errors.ErrTokenValidation
	}
	if blacklisted {
		s.logger.Warn("Attempted to use blacklisted refresh token")
		return nil, errors.ErrTokenBlacklisted
	}

	userInfo := &entities.GoogleUserInfo{
		ID:    storedToken.UserID,
		Email: storedToken.Email,
	}

	tokenPair, err := s.generateTokenPair(userInfo)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to generate new tokens: %v", err))
		return nil, errors.ErrTokenGeneration
	}

	if err := s.tokenRepo.BlacklistToken(ctx, req.RefreshToken, time.Duration(s.jwtConfig.RefreshTokenTTL)*time.Hour); err != nil {
		s.logger.Warn(fmt.Sprintf("Failed to blacklist old refresh token: %v", err))
	}

	if err := s.storeTokens(ctx, tokenPair, userInfo); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to store new tokens: %v", err))
		return nil, errors.ErrTokenStorage
	}

	return &dto.RefreshTokenResponse{
		User: &dto.UserInfo{
			ID:    storedToken.UserID,
			Email: storedToken.Email,
		},
		Tokens: &dto.TokenPair{
			AccessToken:  tokenPair.AccessToken,
			RefreshToken: tokenPair.RefreshToken,
			TokenType:    tokenPair.TokenType,
			ExpiresIn:    tokenPair.ExpiresIn,
		},
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, req *dto.LogoutRequest) error {
	s.logger.Info("Processing logout")

	claims, err := s.jwtManager.ValidateToken(req.AccessToken)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Invalid access token during logout: %v", err))
		return errors.ErrInvalidAccessToken
	}

	if err := s.tokenRepo.DeleteUserTokens(ctx, claims.UserID); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to delete user tokens: %v", err))
		return errors.ErrTokenDeletion
	}

	ttl := time.Duration(s.jwtConfig.AccessTokenTTL) * time.Minute
	if err := s.tokenRepo.BlacklistToken(ctx, req.AccessToken, ttl); err != nil {
		s.logger.Warn(fmt.Sprintf("Failed to blacklist access token: %v", err))
	}

	return nil
}

// Register creates a user in user-service (email/password) and returns JWT tokens.
func (s *AuthService) Register(ctx context.Context, email, password, name string) (*dto.RegisterResponse, error) {
	s.logger.Info(fmt.Sprintf("Registering user with email: %s", email))

	userResp, err := s.userClient.CreateUser(ctx, "", email, name, "", password)
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.AlreadyExists {
			return nil, errors.ErrUserAlreadyExists
		}
		s.logger.Error(fmt.Sprintf("User service CreateUser failed: %v", err))
		return nil, errors.ErrServiceUnavailable
	}

	userInfo := &entities.GoogleUserInfo{
		ID:            userResp.GetId(),
		Email:         userResp.GetEmail(),
		Name:          userResp.GetName(),
		Picture:       userResp.GetPicture(),
		VerifiedEmail: true,
	}

	tokenPair, err := s.generateTokenPair(userInfo)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to generate tokens for user %s: %v", userInfo.Email, err))
		return nil, errors.ErrTokenGeneration
	}

	if err := s.storeTokens(ctx, tokenPair, userInfo); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to store tokens for user %s: %v", userInfo.Email, err))
		return nil, errors.ErrTokenStorage
	}

	return &dto.RegisterResponse{
		User: &dto.UserInfo{
			ID:      userInfo.ID,
			Email:   userInfo.Email,
			Name:    userInfo.Name,
			Picture: userInfo.Picture,
		},
		Tokens: &dto.TokenPair{
			AccessToken:  tokenPair.AccessToken,
			RefreshToken: tokenPair.RefreshToken,
			TokenType:    tokenPair.TokenType,
			ExpiresIn:    tokenPair.ExpiresIn,
		},
	}, nil
}

// Login validates credentials with user-service and returns JWT tokens.
func (s *AuthService) Login(ctx context.Context, email, password string) (*dto.LoginResponse, error) {
	s.logger.Info(fmt.Sprintf("Login attempt for email: %s", email))

	userResp, err := s.userClient.ValidateCredentials(ctx, email, password)
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.Unauthenticated {
			return nil, errors.ErrInvalidCredentials
		}
		s.logger.Error(fmt.Sprintf("User service ValidateCredentials failed: %v", err))
		return nil, errors.ErrServiceUnavailable
	}

	userInfo := &entities.GoogleUserInfo{
		ID:            userResp.GetId(),
		Email:         userResp.GetEmail(),
		Name:          userResp.GetName(),
		Picture:       userResp.GetPicture(),
		VerifiedEmail: true,
	}

	tokenPair, err := s.generateTokenPair(userInfo)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to generate tokens for user %s: %v", userInfo.Email, err))
		return nil, errors.ErrTokenGeneration
	}

	if err := s.storeTokens(ctx, tokenPair, userInfo); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to store tokens for user %s: %v", userInfo.Email, err))
		return nil, errors.ErrTokenStorage
	}

	return &dto.LoginResponse{
		User: &dto.UserInfo{
			ID:      userInfo.ID,
			Email:   userInfo.Email,
			Name:    userInfo.Name,
			Picture: userInfo.Picture,
		},
		Tokens: &dto.TokenPair{
			AccessToken:  tokenPair.AccessToken,
			RefreshToken: tokenPair.RefreshToken,
			TokenType:    tokenPair.TokenType,
			ExpiresIn:    tokenPair.ExpiresIn,
		},
	}, nil
}

func (s *AuthService) ValidateToken(ctx context.Context, token string) (*dto.TokenValidationResponse, error) {
	blacklisted, err := s.tokenRepo.IsTokenBlacklisted(ctx, token)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to check token blacklist: %v", err))
		return nil, errors.ErrTokenValidation
	}
	if blacklisted {
		return nil, errors.ErrTokenBlacklisted
	}

	claims, err := s.jwtManager.ValidateToken(token)
	if err != nil {
		return nil, errors.ErrInvalidAccessToken
	}
	if claims.Type != "access" {
		return nil, errors.ErrInvalidTokenType
	}

	if _, err := s.tokenRepo.GetTokenData(ctx, token); err != nil {
		return nil, errors.ErrTokenNotFound
	}

	return &dto.TokenValidationResponse{
		Valid:  true,
		UserID: claims.UserID,
		Email:  claims.Email,
	}, nil
}

func (s *AuthService) generateTokenPair(userInfo *entities.GoogleUserInfo) (*entities.TokenPair, error) {
	accessTokenTTL := time.Duration(s.jwtConfig.AccessTokenTTL) * time.Minute
	refreshTokenTTL := time.Duration(s.jwtConfig.RefreshTokenTTL) * time.Hour

	accessClaims := &entities.TokenClaims{
		UserID: userInfo.ID,
		Email:  userInfo.Email,
		Type:   "access",
	}
	accessToken, err := s.jwtManager.GenerateToken(accessClaims, accessTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshClaims := &entities.TokenClaims{
		UserID: userInfo.ID,
		Email:  userInfo.Email,
		Type:   "refresh",
	}
	refreshToken, err := s.jwtManager.GenerateToken(refreshClaims, refreshTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &entities.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(accessTokenTTL.Seconds()),
		ExpiresAt:    time.Now().Add(accessTokenTTL),
	}, nil
}

func (s *AuthService) storeTokens(ctx context.Context, tokenPair *entities.TokenPair, userInfo *entities.GoogleUserInfo) error {
	now := time.Now()
	storedToken := &entities.StoredToken{
		UserID:    userInfo.ID,
		Email:     userInfo.Email,
		CreatedAt: now,
		ExpiresAt: tokenPair.ExpiresAt,
	}

	accessTTL := time.Duration(s.jwtConfig.AccessTokenTTL) * time.Minute
	if err := s.tokenRepo.StoreAccessToken(ctx, tokenPair.AccessToken, storedToken, accessTTL); err != nil {
		return fmt.Errorf("failed to store access token: %w", err)
	}

	refreshTTL := time.Duration(s.jwtConfig.RefreshTokenTTL) * time.Hour
	if err := s.tokenRepo.StoreRefreshToken(ctx, tokenPair.RefreshToken, storedToken, refreshTTL); err != nil {
		return fmt.Errorf("failed to store refresh token: %w", err)
	}

	return nil
}

func (s *AuthService) ensureUserExists(ctx context.Context, googleUser *entities.GoogleUserInfo) (*entities.GoogleUserInfo, error) {
	createResp, err := s.userClient.CreateUser(ctx, googleUser.ID, googleUser.Email, googleUser.Name, googleUser.Picture, "")
	if err == nil {
		return mergeUserInfo(createResp, googleUser), nil
	}

	if st, ok := status.FromError(err); ok && st.Code() == codes.AlreadyExists {
		existingUser, getErr := s.userClient.GetUserByEmail(ctx, googleUser.Email)
		if getErr != nil {
			s.logger.Error(fmt.Sprintf("User exists but fetch by email failed: %v", getErr))
			return nil, errors.ErrServiceUnavailable
		}
		return mergeUserInfo(existingUser, googleUser), nil
	}

	s.logger.Error(fmt.Sprintf("Create user for oauth failed: %v", err))
	return nil, errors.ErrServiceUnavailable
}

func mergeUserInfo(result UserInfoResult, fallback *entities.GoogleUserInfo) *entities.GoogleUserInfo {
	if result == nil {
		return fallback
	}

	user := &entities.GoogleUserInfo{
		ID:            result.GetId(),
		Email:         result.GetEmail(),
		Name:          result.GetName(),
		Picture:       result.GetPicture(),
		VerifiedEmail: true,
	}

	if user.Name == "" && fallback != nil {
		user.Name = fallback.Name
	}
	if user.Picture == "" && fallback != nil {
		user.Picture = fallback.Picture
	}

	return user
}

func normalizeOAuthPlatform(req *dto.GoogleAuthURLRequest) (entities.OAuthPlatform, error) {
	if req == nil {
		return entities.OAuthPlatformWeb, nil
	}

	platform := strings.ToLower(strings.TrimSpace(string(req.Platform)))
	switch platform {
	case "", string(dto.OAuthPlatformWeb):
		return entities.OAuthPlatformWeb, nil
	case string(dto.OAuthPlatformMobile):
		return entities.OAuthPlatformMobile, nil
	default:
		return "", errors.ErrInvalidRequest
	}
}

func (s *AuthService) resolveClientRedirectURI(platform entities.OAuthPlatform, req *dto.GoogleAuthURLRequest) (string, error) {
	redirectURI := ""
	if req != nil {
		redirectURI = strings.TrimSpace(req.ClientRedirectURI)
	}

	if redirectURI == "" && platform == entities.OAuthPlatformWeb {
		redirectURI = s.googleConfig.DefaultWebRedirectURI
	}
	if redirectURI == "" {
		return "", errors.ErrInvalidRedirectURI
	}

	if _, err := url.ParseRequestURI(redirectURI); err != nil {
		return "", errors.ErrInvalidRedirectURI
	}

	var allowed []string
	if platform == entities.OAuthPlatformMobile {
		allowed = s.googleConfig.AllowedMobileRedirectURIs
	} else {
		allowed = s.googleConfig.AllowedWebRedirectURIs
	}

	if len(allowed) == 0 {
		return "", errors.ErrInvalidRedirectURI
	}
	if !isRedirectURIAllowed(redirectURI, allowed) {
		return "", errors.ErrInvalidRedirectURI
	}

	return redirectURI, nil
}

func normalizePKCE(platform entities.OAuthPlatform, req *dto.GoogleAuthURLRequest) (string, string, error) {
	if req == nil {
		if platform == entities.OAuthPlatformMobile {
			return "", "", errors.ErrPKCERequired
		}
		return "", "", nil
	}

	codeChallenge := strings.TrimSpace(req.CodeChallenge)
	challengeMethod := strings.ToUpper(strings.TrimSpace(req.CodeChallengeMethod))

	if codeChallenge == "" {
		if platform == entities.OAuthPlatformMobile {
			return "", "", errors.ErrPKCERequired
		}
		return "", "", nil
	}
	if !isValidPKCEValue(codeChallenge) {
		return "", "", errors.ErrInvalidRequest
	}

	if challengeMethod == "" {
		challengeMethod = "S256"
	}
	switch challengeMethod {
	case "S256":
		return codeChallenge, "S256", nil
	case "PLAIN":
		return codeChallenge, "plain", nil
	default:
		return "", "", errors.ErrInvalidRequest
	}
}

func verifyPKCE(codeVerifier, codeChallenge, codeChallengeMethod string) error {
	if strings.TrimSpace(codeChallenge) == "" {
		return nil
	}

	verifier := strings.TrimSpace(codeVerifier)
	if verifier == "" {
		return errors.ErrPKCERequired
	}
	if !isValidPKCEValue(verifier) {
		return errors.ErrInvalidCodeVerifier
	}

	method := strings.ToUpper(strings.TrimSpace(codeChallengeMethod))
	if method == "" {
		method = "S256"
	}

	switch method {
	case "S256":
		hash := sha256.Sum256([]byte(verifier))
		expected := base64.RawURLEncoding.EncodeToString(hash[:])
		if subtle.ConstantTimeCompare([]byte(expected), []byte(codeChallenge)) != 1 {
			return errors.ErrInvalidCodeVerifier
		}
	case "PLAIN":
		if subtle.ConstantTimeCompare([]byte(verifier), []byte(codeChallenge)) != 1 {
			return errors.ErrInvalidCodeVerifier
		}
	default:
		return errors.ErrInvalidCodeVerifier
	}

	return nil
}

func generateSecureToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func isValidPKCEValue(value string) bool {
	if len(value) < 43 || len(value) > 128 {
		return false
	}

	for _, ch := range value {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			continue
		}
		if ch == '-' || ch == '.' || ch == '_' || ch == '~' {
			continue
		}
		return false
	}

	return true
}

func isRedirectURIAllowed(candidate string, allowList []string) bool {
	normalizedCandidate := normalizeRedirectURI(candidate)
	for _, allowed := range allowList {
		if normalizedCandidate == normalizeRedirectURI(allowed) {
			return true
		}
	}
	return false
}

func normalizeRedirectURI(rawURI string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURI))
	if err != nil {
		return strings.TrimSpace(rawURI)
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	if parsed.Path != "/" {
		parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	}
	parsed.Fragment = ""
	return parsed.String()
}

func (s *AuthService) isAllowedEmailDomain(email string) bool {
	if len(s.googleConfig.AllowedDomains) == 0 {
		return true
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}

	domain := strings.ToLower(parts[1])
	for _, allowed := range s.googleConfig.AllowedDomains {
		if strings.EqualFold(strings.TrimSpace(allowed), domain) {
			return true
		}
	}
	return false
}

func toDTOPlatform(platform entities.OAuthPlatform) dto.OAuthPlatform {
	switch platform {
	case entities.OAuthPlatformMobile:
		return dto.OAuthPlatformMobile
	default:
		return dto.OAuthPlatformWeb
	}
}
