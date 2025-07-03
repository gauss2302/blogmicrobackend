package repositories

import (
	"context"
	"time"

	"auth-service/internal/domain/entities"
)

type TokenRepository interface {
	// Auth code management (for OAuth flow)
	StoreAuthCode(ctx context.Context, authCode string, userInfo *entities.GoogleUserInfo, ttl time.Duration) error
	GetAndDeleteAuthCode(ctx context.Context, authCode string) (*entities.GoogleUserInfo, error)

	// OAuth state management (CRITICAL for CSRF protection)
	StoreState(ctx context.Context, key, state string, ttl time.Duration) error
	GetAndDeleteState(ctx context.Context, key string) (string, error)

	// Token management
	StoreAccessToken(ctx context.Context, token string, data *entities.StoredToken, ttl time.Duration) error
	StoreRefreshToken(ctx context.Context, token string, data *entities.StoredToken, ttl time.Duration) error
	GetTokenData(ctx context.Context, token string) (*entities.StoredToken, error)
	DeleteToken(ctx context.Context, token string) error
	DeleteUserTokens(ctx context.Context, userID string) error

	// Token rotation (security best practice)
	RotateRefreshToken(ctx context.Context, oldToken, newToken string, data *entities.StoredToken, ttl time.Duration) error

	// Blacklist management
	IsTokenBlacklisted(ctx context.Context, token string) (bool, error)
	BlacklistToken(ctx context.Context, token string, ttl time.Duration) error
}
