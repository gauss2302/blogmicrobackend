package domainServices

import (
	"auth-service/internal/domain/entities"
	"context"
)

type AuthURLRequest struct {
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
}

type OAuthProvider interface {
	GetAuthURL(req *AuthURLRequest) string
	ExchangeCodeForToken(ctx context.Context, code string) (*entities.GoogleUserInfo, error)
	GetUserInfo(ctx context.Context, accessToken string) (*entities.GoogleUserInfo, error)
}
