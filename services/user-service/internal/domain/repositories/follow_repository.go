package repositories

import (
	"context"
	"user-service/internal/domain/entities"
)

type FollowRepository interface {
	Create(ctx context.Context, followerID, followeeID string) error
	Delete(ctx context.Context, followerID, followeeID string) error
	Exists(ctx context.Context, followerID, followeeID string) (bool, error)
	GetFollowers(ctx context.Context, userID string, limit int, cursor string) ([]*entities.User, string, error)
	GetFollowing(ctx context.Context, userID string, limit int, cursor string) ([]*entities.User, string, error)
	AreFollowed(ctx context.Context, followerID string, followeeIDs []string) ([]string, error)
}
