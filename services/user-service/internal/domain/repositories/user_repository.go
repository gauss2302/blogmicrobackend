package repositories

import (
	"context"
	"errors"

	"user-service/internal/domain/entities"
)

// ErrUserAlreadyExists is returned by Create when the id or email is taken,
// including by a soft-deleted account.
var ErrUserAlreadyExists = errors.New("user already exists")

type UserRepository interface {
	Create(ctx context.Context, user *entities.User) error
	GetByID(ctx context.Context, id string) (*entities.User, error)
	GetByEmail(ctx context.Context, email string) (*entities.User, error)
	Update(ctx context.Context, user *entities.User) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, limit, offset int) ([]*entities.User, error)
	Search(ctx context.Context, query string, limit, offset int) ([]*entities.User, error)
	Exists(ctx context.Context, id string) (bool, error)
	GetActiveUsersCount(ctx context.Context) (int64, error)
}
