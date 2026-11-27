package services

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"auth-service/internal/application/errors"
	"auth-service/internal/application/services/dto"
	"auth-service/internal/config"
	"auth-service/internal/domain/entities"
	"auth-service/internal/domain/repositories"
	"auth-service/pkg/logger"
)

func TestGoogleAllowedDomainsChecksHostedDomain(t *testing.T) {
	cases := []struct {
		name, email, hd string
		allowed         bool
	}{
		{"personal account with a company address", "x@company.com", "", false},
		{"workspace account of the company", "x@company.com", "Company.com", true},
		{"workspace account of another domain", "x@company.com", "other.com", false},
	}
	for _, tc := range cases {
		repo, users := &callbackRepo{}, &usersStub{}
		google := googleStub{user: entities.GoogleUserInfo{ID: "sub-1", Email: tc.email, VerifiedEmail: true, Hd: tc.hd}}
		svc := NewAuthService(repo, google, users, config.JWTConfig{Secret: "0123456789abcdef0123456789abcdef"},
			config.GoogleConfig{AllowedDomains: []string{"company.com"}}, logger.New("fatal"))

		_, err := svc.HandleGoogleCallback(context.Background(), &dto.GoogleCallbackRequest{State: "s", Code: "c"})
		if tc.allowed && (err != nil || repo.authCodes != 1) {
			t.Errorf("%s: err %v auth codes %d, want accepted", tc.name, err, repo.authCodes)
		}
		// Rejected before ensureUserExists: no account is registered either.
		if !tc.allowed && (err != errors.ErrInvalidGoogleCode || users.creates != 0 || repo.authCodes != 0) {
			t.Errorf("%s: err %v creates %d auth codes %d, want rejected before registration", tc.name, err, users.creates, repo.authCodes)
		}
	}
}

// loginUsers accepts one password and counts credential checks.
type loginUsers struct {
	usersStub
	checks int
	err    error
}

func (u *loginUsers) ValidateCredentials(_ context.Context, email, password string) (UserInfoResult, error) {
	u.checks++
	if u.err != nil {
		return nil, u.err
	}
	if password != "right" {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}
	return userResult{id: "u1", email: email}, nil
}

func newLoginService(repo repositories.TokenRepository, users *loginUsers) *AuthService {
	return NewAuthService(repo, nil, users, config.JWTConfig{
		Secret: "0123456789abcdef0123456789abcdef", AccessTokenTTL: 15, RefreshTokenTTL: 168, Issuer: "auth-service",
	}, config.GoogleConfig{}, logger.New("fatal"))
}

func TestLoginLocksTheAccountAfterTenFailures(t *testing.T) {
	repo, users := &refreshTokenRepo{}, &loginUsers{}
	svc := newLoginService(repo, users)
	ctx := context.Background()

	for i := 0; i < maxLoginFailures; i++ {
		if _, err := svc.Login(ctx, "a@example.com", "wrong"); err != errors.ErrInvalidCredentials {
			t.Fatalf("attempt %d: err %v, want ErrInvalidCredentials", i+1, err)
		}
	}
	// Locked: even the right password is not checked until the window ends.
	if _, err := svc.Login(ctx, "a@example.com", "right"); err != errors.ErrTooManyLoginAttempts {
		t.Fatalf("after %d failures: err %v, want ErrTooManyLoginAttempts", maxLoginFailures, err)
	}
	if users.checks != maxLoginFailures || repo.recordedFails != maxLoginFailures {
		t.Fatalf("credential checks %d, recorded failures %d; want %d each", users.checks, repo.recordedFails, maxLoginFailures)
	}
}

func TestLoginSuccessClearsFailures(t *testing.T) {
	repo := &refreshTokenRepo{failures: 3}
	if _, err := newLoginService(repo, &loginUsers{}).Login(context.Background(), "a@example.com", "right"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if repo.cleared != 1 || repo.failures != 0 {
		t.Fatalf("cleared %d, failures %d; want the counter reset", repo.cleared, repo.failures)
	}
}

func TestLoginFailsClosedWhenTheCounterIsUnavailable(t *testing.T) {
	users := &loginUsers{}
	repo := &refreshTokenRepo{failuresErr: stderrors.New("redis down")}
	if _, err := newLoginService(repo, users).Login(context.Background(), "a@example.com", "right"); err != errors.ErrServiceUnavailable || users.checks != 0 {
		t.Fatalf("err %v, checks %d; want ErrServiceUnavailable before any credential check", err, users.checks)
	}
}

func TestValidateTokenStoreFailureIsUnavailable(t *testing.T) {
	access := tokenFor(t, "u1", "access", time.Hour)
	cases := map[string]struct {
		repo *refreshTokenRepo
		want error
	}{
		"blacklist check fails": {&refreshTokenRepo{blacklistErr: stderrors.New("redis down")}, errors.ErrServiceUnavailable},
		"store read fails":      {&refreshTokenRepo{tokenDataErr: stderrors.New("redis down")}, errors.ErrServiceUnavailable},
		"token not in store":    {&refreshTokenRepo{tokenDataErr: repositories.ErrTokenNotFound}, errors.ErrTokenNotFound},
	}
	for name, tc := range cases {
		if _, err := newRefreshService(tc.repo).ValidateToken(context.Background(), access); err != tc.want {
			t.Errorf("%s: err %v, want %v", name, err, tc.want)
		}
	}
}
