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
	"fmt"
	"time"

	"github.com/google/uuid"
)

type AuthService struct {
	tokenRepo     repositories.TokenRepository
	oauthProvider domainServices.OAuthProvider
	jwtManager    *jwt.Manager
	config        config.JWTConfig
	logger        *logger.Logger
}

func NewAuthService(
	tokenRepo repositories.TokenRepository,
	oauthProvider domainServices.OAuthProvider,
	jwtConfig config.JWTConfig,
	logger *logger.Logger,
) *AuthService {
	jwtManager := jwt.NewManager(jwtConfig.Secret, jwtConfig.Issuer)

	return &AuthService{
		tokenRepo:     tokenRepo,
		oauthProvider: oauthProvider,
		config:        jwtConfig,
		jwtManager:    jwtManager,
		logger:        logger,
	}
}

// Main OAuth Flow: Get Google Auth URL
func (s *AuthService) GetGoogleAuthURL(ctx context.Context) (*dto.GoogleAuthURLResponse, error) {
	state := uuid.New().String()

	// Store state in Redis with expiration
	stateKey := fmt.Sprintf("oauth:state:%s", state)
	if err := s.tokenRepo.StoreState(ctx, stateKey, state, 10*time.Minute); err != nil {
		return nil, errors.ErrTokenStorage
	}

	authURL := s.oauthProvider.GetAuthURL(state)

	return &dto.GoogleAuthURLResponse{
		AuthURL: authURL,
		State:   state,
	}, nil
}

// OAuth Callback Handler
func (s *AuthService) HandleGoogleCallback(ctx context.Context, req *dto.GoogleCallbackRequest) (*dto.GoogleCallbackResponse, error) {
	s.logger.Info(fmt.Sprintf("Processing Google callback - state: %s, code length: %d",
		req.State, len(req.Code)))

	// Validate state parameter
	stateKey := fmt.Sprintf("oauth:state:%s", req.State)
	storedState, err := s.tokenRepo.GetAndDeleteState(ctx, stateKey)
	if err != nil || storedState != req.State {
		s.logger.Error("Invalid state parameter - possible CSRF attack")
		return nil, errors.ErrInvalidGoogleCode
	}
	// Exchange code for user info
	s.logger.Debug("Exchanging authorization code for user info")
	userInfo, err := s.oauthProvider.ExchangeCodeForToken(ctx, req.Code)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to exchange Google code: %v", err))
		return nil, errors.ErrInvalidGoogleCode
	}

	s.logger.Info(fmt.Sprintf("Successfully retrieved user info for: %s (verified: %t)",
		userInfo.Email, userInfo.VerifiedEmail))

	// Validate user info
	if !userInfo.IsValid() {
		s.logger.Error("Invalid user info received from Google")
		return nil, errors.ErrInvalidGoogleCode
	}

	// Generate temporary auth code for frontend
	authCode := uuid.New().String()

	// Store auth code in Redis with 5-minute expiration
	authCodeTTL := 5 * time.Minute
	if err := s.tokenRepo.StoreAuthCode(ctx, authCode, userInfo, authCodeTTL); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to store auth code in Redis: %v", err))
		return nil, errors.ErrTokenStorage
	}

	s.logger.Info(fmt.Sprintf("Generated and stored temporary auth code in Redis: %s", authCode))

	return &dto.GoogleCallbackResponse{
		AuthCode: authCode,
	}, nil
}

// ✅ SIMPLIFIED: Exchange temporary auth code for JWT tokens (no user registration)
func (s *AuthService) ExchangeAuthCode(ctx context.Context, req *dto.ExchangeAuthCodeRequest) (*dto.ExchangeAuthCodeResponse, error) {
	s.logger.Info(fmt.Sprintf("Processing auth code exchange for code: %s", req.AuthCode))

	// Get user info from Redis and delete the auth code atomically
	userInfo, err := s.tokenRepo.GetAndDeleteAuthCode(ctx, req.AuthCode)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Invalid or expired auth code: %s - %v", req.AuthCode, err))
		return nil, errors.ErrInvalidGoogleCode
	}

	// Generate token pair - AUTH SERVICE ONLY CREATES TOKENS
	tokenPair, err := s.generateTokenPair(userInfo)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to generate tokens for user %s: %v", userInfo.Email, err))
		return nil, errors.ErrTokenGeneration
	}

	// Store tokens in Redis
	if err := s.storeTokens(ctx, tokenPair, userInfo); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to store tokens for user %s: %v", userInfo.Email, err))
		return nil, errors.ErrTokenStorage
	}

	s.logger.Info(fmt.Sprintf("Successful auth code exchange for user: %s", userInfo.Email))

	return &dto.ExchangeAuthCodeResponse{
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

	// Generate new token pair
	userInfo := &entities.GoogleUserInfo{
		ID:    storedToken.UserID,
		Email: storedToken.Email,
	}

	tokenPair, err := s.generateTokenPair(userInfo)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to generate new tokens: %v", err))
		return nil, errors.ErrTokenGeneration
	}

	// Blacklist old refresh token
	if err := s.tokenRepo.BlacklistToken(ctx, req.RefreshToken, time.Duration(s.config.RefreshTokenTTL)*time.Hour); err != nil {
		s.logger.Warn(fmt.Sprintf("Failed to blacklist old refresh token: %v", err))
	}

	// Store new tokens
	if err := s.storeTokens(ctx, tokenPair, userInfo); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to store new tokens: %v", err))
		return nil, errors.ErrTokenStorage
	}

	s.logger.Info(fmt.Sprintf("Token refreshed for user: %s", storedToken.Email))

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

	// Validate access token
	claims, err := s.jwtManager.ValidateToken(req.AccessToken)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Invalid access token during logout: %v", err))
		return errors.ErrInvalidAccessToken
	}

	// Delete all user tokens
	if err := s.tokenRepo.DeleteUserTokens(ctx, claims.UserID); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to delete user tokens: %v", err))
		return errors.ErrTokenDeletion
	}

	// Blacklist the current token
	ttl := time.Duration(s.config.AccessTokenTTL) * time.Minute
	if err := s.tokenRepo.BlacklistToken(ctx, req.AccessToken, ttl); err != nil {
		s.logger.Warn(fmt.Sprintf("Failed to blacklist access token: %v", err))
	}

	s.logger.Info(fmt.Sprintf("User logged out: %s", claims.Email))
	return nil
}

func (s *AuthService) ValidateToken(ctx context.Context, token string) (*dto.TokenValidationResponse, error) {
	// Check if token is blacklisted
	blacklisted, err := s.tokenRepo.IsTokenBlacklisted(ctx, token)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to check token blacklist: %v", err))
		return nil, errors.ErrTokenValidation
	}
	if blacklisted {
		return nil, errors.ErrTokenBlacklisted
	}

	// Validate JWT
	claims, err := s.jwtManager.ValidateToken(token)
	if err != nil {
		return nil, errors.ErrInvalidAccessToken
	}

	// Ensure it's an access token
	if claims.Type != "access" {
		return nil, errors.ErrInvalidTokenType
	}

	// Check if token exists in Redis (optional additional security check)
	_, err = s.tokenRepo.GetTokenData(ctx, token)
	if err != nil {
		return nil, errors.ErrTokenNotFound
	}

	return &dto.TokenValidationResponse{
		Valid:  true,
		UserID: claims.UserID,
		Email:  claims.Email,
	}, nil
}

func (s *AuthService) generateTokenPair(userInfo *entities.GoogleUserInfo) (*entities.TokenPair, error) {
	accessTokenTTL := time.Duration(s.config.AccessTokenTTL) * time.Minute
	refreshTokenTTL := time.Duration(s.config.RefreshTokenTTL) * time.Hour

	// Generate access token
	accessClaims := &entities.TokenClaims{
		UserID: userInfo.ID,
		Email:  userInfo.Email,
		Type:   "access",
	}
	accessToken, err := s.jwtManager.GenerateToken(accessClaims, accessTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate refresh token
	refreshClaims := &entities.TokenClaims{
		UserID: userInfo.ID,
		Email:  userInfo.Email,
		Type:   "refresh",
	}
	refreshToken, err := s.jwtManager.GenerateToken(refreshClaims, refreshTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	s.logger.Debug(fmt.Sprintf("Generated token pair for user %s", userInfo.Email))

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

	// Store access token
	accessTTL := time.Duration(s.config.AccessTokenTTL) * time.Minute
	if err := s.tokenRepo.StoreAccessToken(ctx, tokenPair.AccessToken, storedToken, accessTTL); err != nil {
		return fmt.Errorf("failed to store access token: %w", err)
	}

	// Store refresh token
	refreshTTL := time.Duration(s.config.RefreshTokenTTL) * time.Hour
	if err := s.tokenRepo.StoreRefreshToken(ctx, tokenPair.RefreshToken, storedToken, refreshTTL); err != nil {
		return fmt.Errorf("failed to store refresh token: %w", err)
	}

	s.logger.Debug(fmt.Sprintf("Stored tokens for user %s", userInfo.Email))
	return nil
}
