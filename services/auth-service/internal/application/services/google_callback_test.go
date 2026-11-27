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
	domainServices "auth-service/internal/domain/services"
	"auth-service/pkg/logger"
)

// callbackRepo serves a valid OAuth state and counts every write that would
// let the caller obtain tokens.
type callbackRepo struct {
	refreshTokenRepo
	authCodes, tokenWrites int
}

func (r *callbackRepo) GetAndDeleteState(_ context.Context, state string) (*entities.OAuthState, error) {
	return &entities.OAuthState{State: state, Platform: entities.OAuthPlatformWeb, ClientRedirectURI: "https://app.example.com/cb"}, nil
}
func (r *callbackRepo) StoreAuthCode(context.Context, string, *entities.AuthCodePayload, time.Duration) error {
	r.authCodes++
	return nil
}
func (r *callbackRepo) StoreAccessToken(context.Context, string, *entities.StoredToken, time.Duration) error {
	r.tokenWrites++
	return nil
}
func (r *callbackRepo) StoreRefreshToken(context.Context, string, *entities.StoredToken, time.Duration) error {
	r.tokenWrites++
	return nil
}

type googleStub struct{ user entities.GoogleUserInfo }

func (g googleStub) GetAuthURL(*domainServices.AuthURLRequest) string { return "" }
func (g googleStub) ExchangeCodeForToken(context.Context, string) (*entities.GoogleUserInfo, error) {
	u := g.user
	return &u, nil
}
func (g googleStub) GetUserInfo(context.Context, string) (*entities.GoogleUserInfo, error) {
	return nil, stderrors.New("unused")
}

type userResult struct{ id, email string }

func (u userResult) GetId() string      { return u.id }
func (u userResult) GetEmail() string   { return u.email }
func (u userResult) GetName() string    { return "" }
func (u userResult) GetPicture() string { return "" }

// usersStub holds at most one existing user, keyed by email.
type usersStub struct{ existing *userResult }

func (u *usersStub) CreateUser(_ context.Context, id, email, _, _, _ string) (UserInfoResult, error) {
	if u.existing != nil && u.existing.email == email {
		return nil, status.Error(codes.AlreadyExists, "user already exists")
	}
	return userResult{id: id, email: email}, nil
}
func (u *usersStub) GetUserByEmail(_ context.Context, email string) (UserInfoResult, error) {
	if u.existing != nil && u.existing.email == email {
		return *u.existing, nil
	}
	return nil, status.Error(codes.NotFound, "not found")
}
func (u *usersStub) ValidateCredentials(context.Context, string, string) (UserInfoResult, error) {
	return nil, stderrors.New("unused")
}

func runGoogleCallback(existing *userResult) (*dto.GoogleCallbackResponse, *callbackRepo, error) {
	repo := &callbackRepo{}
	google := googleStub{user: entities.GoogleUserInfo{ID: "google-sub-1", Email: "victim@gmail.com", VerifiedEmail: true}}
	svc := NewAuthService(repo, google, &usersStub{existing: existing}, config.JWTConfig{
		Secret: "0123456789abcdef0123456789abcdef", AccessTokenTTL: 15, RefreshTokenTTL: 168,
	}, config.GoogleConfig{}, logger.New("fatal"))
	resp, err := svc.HandleGoogleCallback(context.Background(), &dto.GoogleCallbackRequest{State: "s", Code: "c"})
	return resp, repo, err
}

func TestGoogleCallbackRefusesAccountOwnedByAnotherID(t *testing.T) {
	_, repo, err := runGoogleCallback(&userResult{id: "attacker-uuid", email: "victim@gmail.com"})
	if err != errors.ErrAccountExistsUsePassword {
		t.Fatalf("err = %v, want ErrAccountExistsUsePassword", err)
	}
	if repo.authCodes != 0 || repo.tokenWrites != 0 {
		t.Fatalf("auth codes=%d token writes=%d, want none", repo.authCodes, repo.tokenWrites)
	}
}

func TestGoogleCallbackLinksSameGoogleID(t *testing.T) {
	resp, repo, err := runGoogleCallback(&userResult{id: "google-sub-1", email: "victim@gmail.com"})
	if err != nil || resp.AuthCode == "" || repo.authCodes != 1 {
		t.Fatalf("err=%v resp=%+v auth codes=%d", err, resp, repo.authCodes)
	}
}

func TestGoogleCallbackCreatesNewUser(t *testing.T) {
	resp, repo, err := runGoogleCallback(nil)
	if err != nil || resp.AuthCode == "" || repo.authCodes != 1 {
		t.Fatalf("err=%v resp=%+v auth codes=%d", err, resp, repo.authCodes)
	}
}
