package services

import (
	"context"
	"net/url"
	"testing"
	"time"

	"auth-service/internal/application/services/dto"
	"auth-service/internal/config"
	"auth-service/internal/domain/entities"
	"auth-service/internal/infrastructure/oauth"
	"auth-service/pkg/logger"
)

type stateRepo struct {
	refreshTokenRepo
	saved *entities.OAuthState
}

func (r *stateRepo) StoreState(_ context.Context, _ string, s *entities.OAuthState, _ time.Duration) error {
	r.saved = s
	return nil
}

func TestGoogleAuthURLKeepsPKCEChallengeFromGoogle(t *testing.T) {
	const challenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	google := config.GoogleConfig{
		ClientID:                  "client-id",
		RedirectURL:               "https://api.example.com/api/v1/auth/google/callback",
		AllowedMobileRedirectURIs: []string{"app://auth"},
	}
	repo := &stateRepo{}
	svc := NewAuthService(repo, oauth.NewGoogleProvider(google), nil, config.JWTConfig{}, google, logger.New("fatal"))

	resp, err := svc.GetGoogleAuthURL(context.Background(), &dto.GoogleAuthURLRequest{
		Platform:            dto.OAuthPlatformMobile,
		ClientRedirectURI:   "app://auth",
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
	})
	if err != nil {
		t.Fatalf("GetGoogleAuthURL: %v", err)
	}
	u, err := url.Parse(resp.AuthURL)
	if err != nil {
		t.Fatalf("parse %q: %v", resp.AuthURL, err)
	}
	q := u.Query()
	if q.Has("code_challenge") || q.Has("code_challenge_method") {
		t.Fatalf("auth URL passes the client's PKCE challenge to Google: %s", resp.AuthURL)
	}
	if q.Get("state") != resp.State {
		t.Fatalf("auth URL state %q, want %q", q.Get("state"), resp.State)
	}
	if repo.saved == nil || repo.saved.CodeChallenge != challenge || repo.saved.CodeChallengeMethod != "S256" {
		t.Fatalf("stored state %+v lacks the challenge for /auth/exchange", repo.saved)
	}
}
