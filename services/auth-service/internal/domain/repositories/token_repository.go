package repositories

import (
	"context"
	"time"

	"auth-service/internal/domain/entities"
)

type TokenRepository interface {
	StoreAccessToken(ctx context.Context, token string, data *entities.StoredToken, ttl time.Duration) error
	StoreRefreshToken(ctx context.Context, token string, data *entities.StoredToken, ttl time.Duration) error
	GetTokenData(ctx context.Context, token string) (*entities.StoredToken, error)
	DeleteToken(ctx context.Context, token string) error
	DeleteUserTokens(ctx context.Context, userID string) error
	IsTokenBlacklisted(ctx context.Context, token string) (bool, error)
	BlacklistToken(ctx context.Context, token string, ttl time.Duration) error
}