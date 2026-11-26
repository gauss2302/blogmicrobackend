package services

import (
	"context"
	"errors"
	"sync"
	"testing"

	"user-service/internal/application/dto"
	apperrors "user-service/internal/application/errors"
	"user-service/internal/domain/entities"
	"user-service/pkg/logger"
)

// memUserRepo matches email exactly, like WHERE email = $1 in Postgres.
type memUserRepo struct {
	mockUserRepo
	mu        sync.Mutex
	byEmail   map[string]*entities.User
	createErr error
}

func newMemUserRepo() *memUserRepo {
	return &memUserRepo{byEmail: map[string]*entities.User{}}
}

func (m *memUserRepo) Create(ctx context.Context, user *entities.User) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byEmail[user.Email]; ok {
		return errors.New("duplicate key")
	}
	u := *user
	m.byEmail[user.Email] = &u
	return nil
}

func (m *memUserRepo) GetByEmail(ctx context.Context, email string) (*entities.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u, ok := m.byEmail[email]; ok {
		c := *u
		return &c, nil
	}
	return nil, errors.New("user not found")
}

func registerAlice(t *testing.T) *UserService {
	t.Helper()
	svc := NewUserService(newMemUserRepo(), &mockFollowRepo{}, logger.New("error"))
	_, err := svc.CreateUser(context.Background(), &dto.CreateUserRequest{
		Email: "Alice@Example.com", Name: "Alice", Password: "correct-horse",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return svc
}

func TestValidateCredentials_SameMixedCaseEmail(t *testing.T) {
	svc := registerAlice(t)
	if _, err := svc.ValidateCredentials(context.Background(), "Alice@Example.com", "correct-horse"); err != nil {
		t.Fatalf("ValidateCredentials: %v", err)
	}
}

func TestCreateUser_OtherCaseIsDuplicate(t *testing.T) {
	svc := registerAlice(t)
	_, err := svc.CreateUser(context.Background(), &dto.CreateUserRequest{
		Email: " ALICE@example.COM ", Name: "Alice 2", Password: "correct-horse",
	})
	if err != apperrors.ErrUserAlreadyExists {
		t.Fatalf("err = %v, want ErrUserAlreadyExists", err)
	}
}

func TestGetUserByEmail_MixedCase(t *testing.T) {
	svc := registerAlice(t)
	got, err := svc.GetUserByEmail(context.Background(), "aLiCe@EXAMPLE.com")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if got.Email != "alice@example.com" {
		t.Errorf("Email = %q, want alice@example.com", got.Email)
	}
}
