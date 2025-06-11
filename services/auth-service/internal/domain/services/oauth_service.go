package domainServices

import (
	"auth-service/internal/domain/entities"
	"context"
)

type OAuthProvider interface {
	GetAuthURL(state string) string
	ExchangeCodeForToken(ctx context.Context, code string) (*entities.GoogleUserInfo, error)
	GetUserInfo(ctx context.Context, accessToken string) (*entities.GoogleUserInfo, error)
}