package repositories

import (
	"context"
	"notification-service/internal/domain/entities"
)

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
