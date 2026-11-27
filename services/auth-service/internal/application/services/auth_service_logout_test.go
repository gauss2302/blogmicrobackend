package services

import (
	"context"
	"testing"
	"time"

	"auth-service/internal/application/errors"
	"auth-service/internal/application/services/dto"
	"auth-service/internal/domain/entities"
	"auth-service/pkg/jwt"
)

func tokenFor(t *testing.T, userID, tokenType string, ttl time.Duration) string {
	t.Helper()
	token, err := jwt.NewManager("0123456789abcdef0123456789abcdef", "auth-service").
		GenerateToken(&entities.TokenClaims{UserID: userID, Type: tokenType}, ttl)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestLogoutWithExpiredAccessUsesRefreshToken(t *testing.T) {
	repo := &refreshTokenRepo{}
	err := newRefreshService(repo).Logout(context.Background(), &dto.LogoutRequest{
		AccessToken:  tokenFor(t, "user-1", "access", -time.Minute),
		RefreshToken: tokenFor(t, "user-1", "refresh", time.Hour),
	})
	if err != nil || repo.deletedUser != "user-1" {
		t.Fatalf("err %v, revoked user %q; want nil and user-1", err, repo.deletedUser)
	}
}

func TestLogoutWithRefreshTokenOnly(t *testing.T) {
	repo := &refreshTokenRepo{}
	err := newRefreshService(repo).Logout(context.Background(), &dto.LogoutRequest{RefreshToken: tokenFor(t, "user-2", "refresh", time.Hour)})
	if err != nil || repo.deletedUser != "user-2" {
		t.Fatalf("err %v, revoked user %q; want nil and user-2", err, repo.deletedUser)
	}
}

func TestLogoutRejectsWithoutAValidToken(t *testing.T) {
	cases := map[string]*dto.LogoutRequest{
		"none":              {},
		"garbage":           {AccessToken: "x", RefreshToken: "y"},
		"both expired":      {AccessToken: tokenFor(t, "u", "access", -time.Minute), RefreshToken: tokenFor(t, "u", "refresh", -time.Minute)},
		"refresh as access": {AccessToken: tokenFor(t, "u", "refresh", time.Hour)},
		"access as refresh": {RefreshToken: tokenFor(t, "u", "access", time.Hour)},
	}
	for name, req := range cases {
		repo := &refreshTokenRepo{}
		if err := newRefreshService(repo).Logout(context.Background(), req); err != errors.ErrInvalidAccessToken || repo.deletedUser != "" {
			t.Errorf("%s: err %v, revoked %q; want ErrInvalidAccessToken and nothing revoked", name, err, repo.deletedUser)
		}
	}
}
