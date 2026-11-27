package services

import (
	"context"
	"errors"
	"testing"

	"user-service/internal/application/dto"
	apperrors "user-service/internal/application/errors"
	"user-service/internal/domain/entities"
	"user-service/internal/domain/repositories"
	"user-service/pkg/logger"
)

// lookupRepo fails every GetByID and GetByEmail with err.
type lookupRepo struct {
	mockUserRepo
	err error
}

func (r *lookupRepo) GetByID(context.Context, string) (*entities.User, error)    { return nil, r.err }
func (r *lookupRepo) GetByEmail(context.Context, string) (*entities.User, error) { return nil, r.err }

func lookupCalls(svc *UserService) map[string]func() error {
	ctx := context.Background()
	name := "N"
	return map[string]func() error{
		"GetUser":        func() error { _, err := svc.GetUser(ctx, "u1"); return err },
		"GetUserByEmail": func() error { _, err := svc.GetUserByEmail(ctx, "a@example.com"); return err },
		"GetUserProfile": func() error { _, err := svc.GetUserProfile(ctx, "u1"); return err },
		"UpdateUser":     func() error { _, err := svc.UpdateUser(ctx, "u1", &dto.UpdateUserRequest{Name: &name}); return err },
		"Follow":         func() error { return svc.Follow(ctx, "u1", "u2") },
	}
}

func TestDatabaseErrorIsNotNotFound(t *testing.T) {
	svc := NewUserService(&lookupRepo{err: errors.New("connection refused")}, &mockFollowRepo{}, logger.New("fatal"))
	for name, call := range lookupCalls(svc) {
		if err := call(); err != apperrors.ErrServiceUnavailable {
			t.Errorf("%s: err %v, want ErrServiceUnavailable", name, err)
		}
	}
	if _, err := svc.ValidateCredentials(context.Background(), "a@example.com", "pw"); err != apperrors.ErrServiceUnavailable {
		t.Errorf("ValidateCredentials: err %v, want ErrServiceUnavailable, not invalid credentials", err)
	}
	if _, err := svc.CreateUser(context.Background(), &dto.CreateUserRequest{Email: "a@example.com", Name: "A", Password: "correct-horse"}); err != apperrors.ErrServiceUnavailable {
		t.Errorf("CreateUser: err %v, want ErrServiceUnavailable", err)
	}
}

func TestMissingUserIsStillNotFound(t *testing.T) {
	svc := NewUserService(&lookupRepo{err: repositories.ErrUserNotFound}, &mockFollowRepo{}, logger.New("fatal"))
	for name, call := range lookupCalls(svc) {
		if err := call(); err != apperrors.ErrUserNotFound {
			t.Errorf("%s: err %v, want ErrUserNotFound", name, err)
		}
	}
	if _, err := svc.ValidateCredentials(context.Background(), "a@example.com", "pw"); err != apperrors.ErrInvalidCredentials {
		t.Errorf("ValidateCredentials: err %v, want ErrInvalidCredentials", err)
	}
}
