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
	
	// Store temporary auth codes (in production, use Redis with expiration)
	tempCodes map[string]*entities.GoogleUserInfo
}

func NewAuthService(
	tokenRepo repositories.TokenRepository, 
	oauthProvider domainServices.OAuthProvider, 
	jwtConfig config.JWTConfig, 
	logger *logger.Logger) *AuthService {

	jwtManager := jwt.NewManager(jwtConfig.Secret)

	return &AuthService{
		tokenRepo:     tokenRepo,
		oauthProvider: oauthProvider,
		config:        jwtConfig,
		jwtManager:    jwtManager,
		logger:        logger,
		tempCodes:     make(map[string]*entities.GoogleUserInfo),
	}
}

// Main OAuth Flow: Get Google Auth URL
func (s *AuthService) GetGoogleAuthURL(ctx context.Context) (*dto.GoogleAuthURLResponse, error) {
	state := uuid.New().String()
	authURL := s.oauthProvider.GetAuthURL(state)
	
	s.logger.Info(fmt.Sprintf("Generated Google auth URL with state: %s", state))
	
	return &dto.GoogleAuthURLResponse{
		AuthURL: authURL,
		State:   state,
	}, nil
}

// OAuth Callback Handler
func (s *AuthService) HandleGoogleCallback(ctx context.Context, req *dto.GoogleCallbackRequest) (*dto.GoogleCallbackResponse, error) {
	s.logger.Info(fmt.Sprintf("Processing Google callback with state: %s", req.State))
	
	// Exchange code for user info
	userInfo, err := s.oauthProvider.ExchangeCodeForToken(ctx, req.Code)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to exchange Google code: %v", err))
		return nil, errors.ErrInvalidGoogleCode
	}

	// Generate temporary auth code for frontend
	authCode := uuid.New().String()
	s.tempCodes[authCode] = userInfo
	
	// In production, store this in Redis with expiration
	go func() {
		time.Sleep(5 * time.Minute) // Clean up after 5 minutes
		delete(s.tempCodes, authCode)
	}()

	return &dto.GoogleCallbackResponse{
		AuthCode: authCode,
	}, nil
}

// Exchange temporary auth code for JWT tokens
func (s *AuthService) ExchangeAuthCode(ctx context.Context, req *dto.ExchangeAuthCodeRequest) (*dto.ExchangeAuthCodeResponse, error) {
	s.logger.Info("Processing auth code exchange")

	// Get user info from temporary storage
	userInfo, exists := s.tempCodes[req.AuthCode]
	if !exists {
		s.logger.Error("Invalid or expired auth code")
		return nil, errors.ErrInvalidGoogleCode
	}

	// Clean up used code
	delete(s.tempCodes, req.AuthCode)

	// Generate token pair
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

// Legacy endpoint - keep for backward compatibility if needed
func (s *AuthService) GoogleLogin(ctx context.Context, req *dto.GoogleLoginRequest) (*dto.AuthResponse, error) {
	s.logger.Info(fmt.Sprintf("Processing legacy Google login for code: %s", req.Code[:10]+"..."))
	
	// Exchange code for info
	userInfo, err := s.oauthProvider.ExchangeCodeForToken(ctx, req.Code)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to exchange Google code: %v", err))
		return nil, errors.ErrInvalidGoogleCode
	}

	// Generate Token Pairs
	tokenPair, err := s.generateTokenPair(userInfo)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to generate tokens for user %s: %v", userInfo.Email, err))
		return nil, errors.ErrTokenGeneration
	}

	// Store Tokens in Redis
	if err := s.storeTokens(ctx, tokenPair, userInfo); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to store tokens for user %s: %v", userInfo.Email, err))
		return nil, errors.ErrTokenStorage
	}

	s.logger.Info(fmt.Sprintf("Successful legacy login for user: %s", userInfo.Email))

	return &dto.AuthResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		TokenType:    tokenPair.TokenType,
		ExpiresIn:    tokenPair.ExpiresIn,
		User: &dto.UserInfo{
			ID:      userInfo.ID,
			Email:   userInfo.Email,
			Name:    userInfo.Name,
			Picture: userInfo.Picture,
		},
	}, nil	
}

func (s *AuthService) RefreshToken(ctx context.Context, req *dto.RefreshTokenRequest) (*dto.AuthResponse, error) {
	s.logger.Info("Processing token refresh")

	// Validate refresh token
	claims, err := s.jwtManager.ValidateToken(req.RefreshToken)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Invalid refresh token: %v", err))
		return nil, errors.ErrInvalidRefreshToken
	}

	if claims.Type != "refresh" {
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

	return &dto.AuthResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		TokenType:    tokenPair.TokenType,
		ExpiresIn:    tokenPair.ExpiresIn,
		User: &dto.UserInfo{
			ID:    storedToken.UserID,
			Email: storedToken.Email,
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
		return nil, err
	}

	// Generate refresh token
	refreshClaims := &entities.TokenClaims{
		UserID: userInfo.ID,
		Email:  userInfo.Email,
		Type:   "refresh",
	}
	refreshToken, err := s.jwtManager.GenerateToken(refreshClaims, refreshTokenTTL)
	if err != nil {
		return nil, err
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

	// Store access token
	accessTTL := time.Duration(s.config.AccessTokenTTL) * time.Minute
	if err := s.tokenRepo.StoreAccessToken(ctx, tokenPair.AccessToken, storedToken, accessTTL); err != nil {
		return err
	}

	// Store refresh token
	refreshTTL := time.Duration(s.config.RefreshTokenTTL) * time.Hour
	return s.tokenRepo.StoreRefreshToken(ctx, tokenPair.RefreshToken, storedToken, refreshTTL)
}