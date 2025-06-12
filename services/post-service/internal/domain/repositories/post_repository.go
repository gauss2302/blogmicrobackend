package repositories

import (
	"context"
	"post-service/internal/domain/entities"
)

type PostRepository interface {
	Create(ctx context.Context, post *entities.Post) error
	GetByID(ctx context.Context, id string) (*entities.Post, error)
	GetBySlug(ctx context.Context, slug string) (*entities.Post, error)
	GetByUserID(ctx context.Context, userID string, limit, offset int) ([]*entities.Post, error)
	Update(ctx context.Context, post *entities.Post) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, limit, offset int, publishedOnly bool) ([]*entities.Post, error)
	Search(ctx context.Context, query string, limit, offset int, publishedOnly bool) ([]*entities.Post, error)
	Exists(ctx context.Context, id string) (bool, error)
	ExistsBySlug(ctx context.Context, slug string) (bool, error)
	GetPublishedCount(ctx context.Context) (int64, error)
	GetUserPostsCount(ctx context.Context, userID string) (int64, error)
}