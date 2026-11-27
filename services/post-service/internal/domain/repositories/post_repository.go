package repositories

import (
	"context"
	"errors"
	"post-service/internal/domain/entities"
)

// ErrSlugTaken is returned by Create and Update when another post already uses the slug.
var ErrSlugTaken = errors.New("slug already taken")

// ErrPostNotFound is returned when no post has the id.
var ErrPostNotFound = errors.New("post not found")

type PostRepository interface {
	Create(ctx context.Context, post *entities.Post) error
	GetByID(ctx context.Context, id string) (*entities.Post, error)
	GetBySlug(ctx context.Context, slug string) (*entities.Post, error)
	// GetByUserID lists the user's posts, newest first; drafts only when includeDrafts.
	GetByUserID(ctx context.Context, userID string, limit, offset int, includeDrafts bool) ([]*entities.Post, error)
	// Update locks the post (SELECT ... FOR UPDATE), lets apply change it, and
	// writes it back in the same transaction, returning it with updated_at from
	// the database. An error from apply aborts the update and is returned as is.
	Update(ctx context.Context, id string, apply func(post *entities.Post) error) (*entities.Post, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, limit, offset int, publishedOnly bool) ([]*entities.Post, error)
	Search(ctx context.Context, query string, limit, offset int, publishedOnly bool) ([]*entities.Post, error)
	Exists(ctx context.Context, id string) (bool, error)
	GetPublishedCount(ctx context.Context) (int64, error)
	GetUserPostsCount(ctx context.Context, userID string) (int64, error)
	// UnpublishByUserID sets published = false on every published post of the
	// user and returns how many it changed.
	UnpublishByUserID(ctx context.Context, userID string) (int64, error)
}
