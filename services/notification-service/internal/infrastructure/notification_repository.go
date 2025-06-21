package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/lib/pq"
	"notification-service/internal/domain/entities"
	"time"
)

type NotificationRepository struct {
	db *sql.DB
}

func NewNotificationRepository(db *sql.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) Create(ctx context.Context, notification *entities.Notification) error {
	query := `
		INSERT INTO notifications (id, user_id, type, title, message, data, read, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	dataJSON, err := json.Marshal(notification.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal notif data: %w", err)
	}

	now := time.Now()
	_, err = r.db.ExecContext(
		ctx, query, notification.ID, notification.UserID, notification.Type,
		notification.Title, notification.Message, dataJSON, notification.Read, now)

	if err != nil {
		return fmt.Errorf("failed to create notif: %w", err)
	}

	notification.CreatedAt = now
	return nil

}

func (r *NotificationRepository) GetByID(ctx context.Context, id string) (*entities.Notification, error) {
	query := `
		SELECT id, user_id, type, title, message, data, read, created_at, read_at
		FROM notifications 
		WHERE id = $1
	`

	notification := &entities.Notification{}
	var dataJSON []byte
	var readAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&notification.ID, &notification.UserID, &notification.Type, &notification.Title, &notification.Message, &dataJSON, &notification.Read, &notification.CreatedAt, &readAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("record not found in postgres db")
		}
		return nil, fmt.Errorf("failed to get notif: %w", err)
	}

	if len(dataJSON) > 0 {
		if err := json.Unmarshal(dataJSON, &notification.Data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal notif data: %w", err)
		}
	}

	if readAt.Valid {
		notification.ReadAt = &readAt.Time
	}
	return notification, nil
}

func (r *NotificationRepository) GetByUserID(ctx context.Context, userID string, limit, offset int) ([]*entities.Notification, error) {
	query := `
		SELECT id, user_id, type, title, message, data, read, created_at, read_at
		FROM notifications 
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get user notif: %w", err)
	}

	defer rows.Close()

	return r.scanNotifications(rows)

}

func (r *NotificationRepository) GetUnreadByUserID(ctx context.Context, userID string, limit, offset int) ([]*entities.Notification, error) {
	query := `
	SELECT id, user_id, type, title, message, data, read, created_at, read_at
	FROM notifications
	WHERE user_id = $1 AND read = false
	ORDER BY created_at DESC
	LIMIT $2 OFFSET $3
		`

	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get unread notif: %w", err)
	}

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			_ = fmt.Errorf("failed to close sql conn: %w", err)
		}
	}(rows)
	return r.scanNotifications(rows)
}

func (r *NotificationRepository) MarkAsRead(ctx context.Context, id, userID string) error {
	query := `
		UPDATE notifications 
		SET read = true, read_at = $3
		WHERE id = $1 AND user_id = $2 AND read = false
	`

	result, err := r.db.ExecContext(ctx, query, id, userID, time.Now())

	if err != nil {
		return fmt.Errorf("failed to mark notif as read: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("notif not found or already read")
	}

	return nil
}

func (r *NotificationRepository) MakeAllAsRead(ctx context.Context, userID string) error {
	query := `
	UPDATE notifications
	SET read = true, read_at = $2
	WHERE user_id = $1 AND read = false
	`

	_, err := r.db.ExecContext(ctx, query, userID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to mark all notif as read: %w", err)
	}

	return err
}

func (r *NotificationRepository) Delete(ctx context.Context, id, userID string) error {
	query := `DELETE FROM notifications WHERE id = $1 AND user_id = $2`

	result, err := r.db.ExecContext(ctx, query, id, userID)

	if err != nil {
		return fmt.Errorf("failed to delete notif: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *NotificationRepository) GetUnreadCount(ctx context.Context, userID string) (int64, error) {
	query := `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read = false`

	var count int64
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}
	return count, nil
}

func (r *NotificationRepository) List(ctx context.Context, limit, offset int) ([]*entities.Notification, error) {
	query := `
		SELECT id, user_id, type, title, message, data, read, created_at, read_at
		FROM notifications 
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list notifs: %w", err)
	}

	defer rows.Close()

	return r.scanNotifications(rows)
}

func (r *NotificationRepository) DeleteOld(ctx context.Context, olderThan int) error {
	query := `DELETE FROM notifications WHERE created_at < $1`

	cutoffDate := time.Now().AddDate(0, 0, -olderThan)

	result, err := r.db.ExecContext(ctx, query, cutoffDate)
	if err != nil {
		return fmt.Errorf("failed to delete old notifications: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected > 0 {
		fmt.Printf("Deleted %d old notifications\n", rowsAffected)
	}

	return nil
}

// Helper
func (r *NotificationRepository) scanNotifications(rows *sql.Rows) ([]*entities.Notification, error) {
	var notifications []*entities.Notification

	for rows.Next() {
		notification := &entities.Notification{}
		var dataJSON []byte
		var readAt sql.NullTime

		err := rows.Scan(
			&notification.ID, &notification.UserID, &notification.Type,
			&notification.Title, &notification.Message, &dataJSON,
			&notification.Read, &notification.CreatedAt, &readAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan notification: %w", err)
		}

		// Parse JSON data
		if len(dataJSON) > 0 {
			if err := json.Unmarshal(dataJSON, &notification.Data); err != nil {
				return nil, fmt.Errorf("failed to unmarshal notification data: %w", err)
			}
		}

		if readAt.Valid {
			notification.ReadAt = &readAt.Time
		}

		notifications = append(notifications, notification)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}

	return notifications, nil
}
