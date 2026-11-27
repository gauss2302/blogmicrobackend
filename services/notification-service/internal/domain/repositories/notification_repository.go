package repositories

import (
	"context"
	"errors"
	"notification-service/internal/domain/entities"
)

// ErrDuplicateEvent is returned by Create when a notification for the same
// event ID already exists: the event was delivered again.
var ErrDuplicateEvent = errors.New("notification for this event already exists")

// ErrNotificationNotFound is returned when no notification matches, including
// one that exists but belongs to another user.
var ErrNotificationNotFound = errors.New("notification not found")

type NotificationRepository interface {
	Create(ctx context.Context, notification *entities.Notification) error
	GetByID(ctx context.Context, id string) (*entities.Notification, error)
	GetByUserID(ctx context.Context, userID string, limit, offset int) ([]*entities.Notification, error)
	GetUnreadByUserID(ctx context.Context, userID string, limit, offset int) ([]*entities.Notification, error)
	MarkAsRead(ctx context.Context, id string, userID string) error
	MakeAllAsRead(ctx context.Context, userID string) error
	Delete(ctx context.Context, id string, userID string) error
	GetUnreadCount(ctx context.Context, userID string) (int64, error)
	List(ctx context.Context, limit, offset int) ([]*entities.Notification, error)
	DeleteOld(ctx context.Context, olderThan int) error
}
