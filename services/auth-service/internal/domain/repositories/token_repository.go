package repositories

import (
	"context"
	"errors"
	"time"

	"auth-service/internal/domain/entities"
)

// ErrRefreshTokenMissing means the refresh-token record was not in Redis.
// A blacklisted token is the reuse case: the caller must revoke the family.
var ErrRefreshTokenMissing = errors.New("refresh token is not in the store")

// ErrTokenNotFound means the token is not in the store. Any other error from
// GetTokenData is the store failing, not the token.
var ErrTokenNotFound = errors.New("token is not in the store")

type TokenRepository interface {
	// Auth code management (for OAuth flow)
	StoreAuthCode(ctx context.Context, authCode string, payload *entities.AuthCodePayload, ttl time.Duration) error
	GetAndDeleteAuthCode(ctx context.Context, authCode string) (*entities.AuthCodePayload, error)

	// OAuth state management
	StoreState(ctx context.Context, state string, payload *entities.OAuthState, ttl time.Duration) error
	GetAndDeleteState(ctx context.Context, state string) (*entities.OAuthState, error)

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

	// Failed password sign-ins per email, counted over a fixed window that
	// starts with the first failure.
	LoginFailures(ctx context.Context, email string) (int64, error)
	RecordLoginFailure(ctx context.Context, email string, window time.Duration) error
	ClearLoginFailures(ctx context.Context, email string) error
}
