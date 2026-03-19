package services

import (
	"context"
	"errors"
	"testing"

	apperrors "user-service/internal/application/errors"
	"user-service/internal/domain/entities"
	"user-service/internal/domain/repositories"
	"user-service/pkg/logger"
)

type mockUserRepo struct {
	getByID func(ctx context.Context, id string) (*entities.User, error)
}

func (m *mockUserRepo) Create(ctx context.Context, user *entities.User) error { return nil }
func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*entities.User, error) {
	if m.getByID != nil {
		return m.getByID(ctx, id)
	}
	return nil, errors.New("not found")
}
func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*entities.User, error) {
	return nil, nil
}
func (m *mockUserRepo) Update(ctx context.Context, user *entities.User) error { return nil }
func (m *mockUserRepo) Delete(ctx context.Context, id string) error            { return nil }
func (m *mockUserRepo) List(ctx context.Context, limit, offset int) ([]*entities.User, error) {
	return nil, nil
}
func (m *mockUserRepo) Search(ctx context.Context, query string, limit, offset int) ([]*entities.User, error) {
	return nil, nil
}
func (m *mockUserRepo) Exists(ctx context.Context, id string) (bool, error) { return false, nil }
func (m *mockUserRepo) GetActiveUsersCount(ctx context.Context) (int64, error) { return 0, nil }

type mockFollowRepo struct {
	createErr error
	deleteErr error
}

func (m *mockFollowRepo) Create(ctx context.Context, followerID, followeeID string) error {
	return m.createErr
}
func (m *mockFollowRepo) Delete(ctx context.Context, followerID, followeeID string) error {
	return m.deleteErr
}
func (m *mockFollowRepo) Exists(ctx context.Context, followerID, followeeID string) (bool, error) {
	return false, nil
}
func (m *mockFollowRepo) GetFollowers(ctx context.Context, userID string, limit int, cursor string) ([]*entities.User, string, error) {
	return nil, "", nil
}
func (m *mockFollowRepo) GetFollowing(ctx context.Context, userID string, limit int, cursor string) ([]*entities.User, string, error) {
	return nil, "", nil
}
func (m *mockFollowRepo) AreFollowed(ctx context.Context, followerID string, followeeIDs []string) ([]string, error) {
	return nil, nil
}

func TestFollow_CannotFollowSelf(t *testing.T) {
	svc := NewUserService(&mockUserRepo{}, &mockFollowRepo{}, logger.New("info"))
	ctx := context.Background()
	err := svc.Follow(ctx, "user1", "user1")
	if err == nil {
		t.Fatal("expected error for self-follow")
	}
	if err != apperrors.ErrCannotFollowSelf {
		t.Errorf("expected ErrCannotFollowSelf, got %v", err)
	}
}

func TestFollow_UserNotFound(t *testing.T) {
	userRepo := &mockUserRepo{
		getByID: func(ctx context.Context, id string) (*entities.User, error) {
			return nil, errors.New("not found")
		},
	}
	svc := NewUserService(userRepo, &mockFollowRepo{}, logger.New("info"))
	ctx := context.Background()
	err := svc.Follow(ctx, "follower", "nonexistent")
	if err == nil {
		t.Fatal("expected error when followee not found")
	}
	if err != apperrors.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestFollow_Success(t *testing.T) {
	userRepo := &mockUserRepo{
		getByID: func(ctx context.Context, id string) (*entities.User, error) {
			return &entities.User{ID: id, Name: "u"}, nil
		},
	}
	svc := NewUserService(userRepo, &mockFollowRepo{}, logger.New("info"))
	ctx := context.Background()
	err := svc.Follow(ctx, "follower", "followee")
	if err != nil {
		t.Fatalf("Follow: %v", err)
	}
}

func TestFollow_Idempotent(t *testing.T) {
	userRepo := &mockUserRepo{
		getByID: func(ctx context.Context, id string) (*entities.User, error) {
			return &entities.User{ID: id}, nil
		},
	}
	// Create succeeds (e.g. ON CONFLICT DO NOTHING); second Follow also succeeds
	followRepo := &mockFollowRepo{}
	svc := NewUserService(userRepo, followRepo, logger.New("info"))
	ctx := context.Background()
	err1 := svc.Follow(ctx, "f", "e")
	err2 := svc.Follow(ctx, "f", "e")
	if err1 != nil {
		t.Fatalf("first Follow: %v", err1)
	}
	if err2 != nil {
		t.Fatalf("second Follow (idempotent): %v", err2)
	}
}

func TestUnfollow_Success(t *testing.T) {
	svc := NewUserService(&mockUserRepo{}, &mockFollowRepo{}, logger.New("info"))
	ctx := context.Background()
	err := svc.Unfollow(ctx, "follower", "followee")
	if err != nil {
		t.Fatalf("Unfollow: %v", err)
	}
}

func TestUnfollow_Idempotent(t *testing.T) {
	svc := NewUserService(&mockUserRepo{}, &mockFollowRepo{}, logger.New("info"))
	ctx := context.Background()
	err1 := svc.Unfollow(ctx, "f", "e")
	err2 := svc.Unfollow(ctx, "f", "e")
	if err1 != nil {
		t.Fatalf("first Unfollow: %v", err1)
	}
	if err2 != nil {
		t.Fatalf("second Unfollow (idempotent): %v", err2)
	}
}

// Ensure mockFollowRepo implements repositories.FollowRepository
var _ repositories.FollowRepository = (*mockFollowRepo)(nil)
var _ repositories.UserRepository = (*mockUserRepo)(nil)
