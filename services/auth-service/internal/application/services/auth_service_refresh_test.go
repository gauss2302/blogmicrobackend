package services

import (
	"context"
	"testing"
	"time"

	"auth-service/internal/application/errors"
	"auth-service/internal/application/services/dto"
	"auth-service/internal/config"
	"auth-service/internal/domain/entities"
	"auth-service/internal/domain/repositories"
	"auth-service/pkg/jwt"
	"auth-service/pkg/logger"
)

type refreshTokenRepo struct {
	rotateErr    error
	blacklisted  bool
	blacklistErr error
	deleteErr    error
	deletedUser  string
	storedAccess string
	rotatedOld   string
	rotatedNew   string
	rotateCalls  int
}

func (r *refreshTokenRepo) StoreAuthCode(context.Context, string, *entities.AuthCodePayload, time.Duration) error {
	return nil
}
func (r *refreshTokenRepo) GetAndDeleteAuthCode(context.Context, string) (*entities.AuthCodePayload, error) {
	return nil, nil
}
func (r *refreshTokenRepo) StoreState(context.Context, string, *entities.OAuthState, time.Duration) error {
	return nil
}
func (r *refreshTokenRepo) GetAndDeleteState(context.Context, string) (*entities.OAuthState, error) {
	return nil, nil
}
func (r *refreshTokenRepo) StoreAccessToken(_ context.Context, token string, _ *entities.StoredToken, _ time.Duration) error {
	r.storedAccess = token
	return nil
}
func (r *refreshTokenRepo) StoreRefreshToken(context.Context, string, *entities.StoredToken, time.Duration) error {
	return nil
}
func (r *refreshTokenRepo) GetTokenData(context.Context, string) (*entities.StoredToken, error) {
	return nil, nil
}
func (r *refreshTokenRepo) DeleteToken(context.Context, string) error { return nil }
func (r *refreshTokenRepo) DeleteUserTokens(_ context.Context, userID string) error {
	r.deletedUser = userID
	return r.deleteErr
}
func (r *refreshTokenRepo) RotateRefreshToken(_ context.Context, oldToken, newToken string, _ *entities.StoredToken, _ time.Duration) error {
	r.rotateCalls++
	r.rotatedOld = oldToken
	r.rotatedNew = newToken
	return r.rotateErr
}
func (r *refreshTokenRepo) IsTokenBlacklisted(context.Context, string) (bool, error) {
	return r.blacklisted, r.blacklistErr
}
func (r *refreshTokenRepo) BlacklistToken(context.Context, string, time.Duration) error {
	return nil
}

func newRefreshService(repo repositories.TokenRepository) *AuthService {
	return NewAuthService(repo, nil, nil, config.JWTConfig{
		Secret:          "0123456789abcdef0123456789abcdef",
		AccessTokenTTL:  15,
		RefreshTokenTTL: 168,
		Issuer:          "auth-service",
	}, config.GoogleConfig{}, logger.New("error"))
}

func refreshTokenFor(userID string) string {
	manager := jwt.NewManager("0123456789abcdef0123456789abcdef", "auth-service")
	token, err := manager.GenerateToken(&entities.TokenClaims{
		UserID: userID,
		Email:  "user@example.com",
		Type:   "refresh",
	}, time.Hour)
	if err != nil {
		panic(err)
	}
	return token
}

func TestRefreshTokenRotatesAndReturnsNewPair(t *testing.T) {
	repo := &refreshTokenRepo{}
	svc := newRefreshService(repo)
	old := refreshTokenFor("user-1")

	resp, err := svc.RefreshToken(context.Background(), &dto.RefreshTokenRequest{RefreshToken: old})
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if repo.rotateCalls != 1 || repo.rotatedOld != old {
		t.Fatalf("rotate calls=%d old=%q", repo.rotateCalls, repo.rotatedOld)
	}
	if resp.Tokens.RefreshToken == "" || resp.Tokens.RefreshToken == old {
		t.Fatalf("expected a new refresh token, got %q", resp.Tokens.RefreshToken)
	}
	if repo.rotatedNew != resp.Tokens.RefreshToken {
		t.Fatalf("stored refresh %q, response %q", repo.rotatedNew, resp.Tokens.RefreshToken)
	}
	if repo.storedAccess != resp.Tokens.AccessToken {
		t.Fatalf("stored access %q, response %q", repo.storedAccess, resp.Tokens.AccessToken)
	}
}

func TestRefreshTokenReuseRevokesFamily(t *testing.T) {
	repo := &refreshTokenRepo{rotateErr: repositories.ErrRefreshTokenMissing, blacklisted: true}
	svc := newRefreshService(repo)

	_, err := svc.RefreshToken(context.Background(), &dto.RefreshTokenRequest{RefreshToken: refreshTokenFor("user-1")})
	if err != errors.ErrTokenBlacklisted {
		t.Fatalf("err = %v, want blacklisted", err)
	}
	if repo.deletedUser != "user-1" {
		t.Fatalf("deleted user %q", repo.deletedUser)
	}
}

func TestRefreshTokenStorageFailureDoesNotReportSuccess(t *testing.T) {
	repo := &refreshTokenRepo{rotateErr: context.DeadlineExceeded}
	svc := newRefreshService(repo)

	resp, err := svc.RefreshToken(context.Background(), &dto.RefreshTokenRequest{RefreshToken: refreshTokenFor("user-1")})
	if err != errors.ErrTokenStorage {
		t.Fatalf("err = %v", err)
	}
	if resp != nil {
		t.Fatal("response returned after a failed rotation")
	}
}

func TestNormalizePKCERejectsPlain(t *testing.T) {
	_, _, err := normalizePKCE(entities.OAuthPlatformWeb, &dto.GoogleAuthURLRequest{
		CodeChallenge:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		CodeChallengeMethod: "plain",
	})
	if err != errors.ErrInvalidRequest {
		t.Fatalf("err = %v, want invalid request", err)
	}
}
