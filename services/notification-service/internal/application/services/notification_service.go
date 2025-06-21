package services

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"notification-service/internal/application/dto"
	"notification-service/internal/application/errors"
	"notification-service/internal/domain/entities"
	"notification-service/internal/domain/repositories"
	"notification-service/pkg/logger"
)

type NotificationService struct {
	notificationRepo repositories.NotificationRepository
	logger           *logger.Logger
}

func NewNotificationService(notificationRepo repositories.NotificationRepository, logger *logger.Logger) *NotificationService {
	return &NotificationService{
		notificationRepo: notificationRepo,
		logger:           logger,
	}
}

func (s *NotificationService) CreateNotification(ctx context.Context, req *dto.CreateNotificationRequest) (*dto.NotificationResponse, error) {
	s.logger.Info(fmt.Sprintf("creating notif for user: %s", req.UserID))

	notification := &entities.Notification{
		ID:      uuid.New().String(),
		UserID:  req.UserID,
		Type:    entities.NotificationType(req.Type),
		Title:   req.Title,
		Message: req.Message,
		Data:    req.Data,
		Read:    false,
	}

	notification.Sanitize()
	if err := notification.IsValid(); err != nil {
		s.logger.Warn(fmt.Sprintf("notif validation failed: %s", err))
		return nil, errors.ErrInvalidNotificationData
	}

	if err := s.notificationRepo.Create(ctx, notification); err != nil {
		s.logger.Error(fmt.Sprintf("failed to create notif: %v", err))
		return nil, errors.ErrNotificationCreationFailed
	}

	s.logger.Info(fmt.Sprintf("notif created successfully: %s", notification.ID))

	return &dto.NotificationResponse{
		ID:        notification.ID,
		UserID:    notification.UserID,
		Type:      string(notification.Type),
		Title:     notification.Title,
		Message:   notification.Message,
		Data:      notification.Data,
		Read:      notification.Read,
		CreatedAt: notification.CreatedAt,
		ReadAt:    notification.ReadAt,
	}, nil
}

func (s *NotificationService) GetNotification(ctx context.Context, id string, userID string) (*dto.NotificationResponse, error) {
	s.logger.Info(fmt.Sprintf("getting notif: %s for user: %s", id, userID))

	notification, err := s.notificationRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.Warn(fmt.Sprintf("notif not found: %s", id))
		return nil, errors.ErrNotificationNotFound
	}

	if notification.UserID != userID {
		return nil, errors.ErrUnauthorizedAccess
	}

	return &dto.NotificationResponse{
		ID:        notification.ID,
		UserID:    notification.UserID,
		Type:      string(notification.Type),
		Title:     notification.Title,
		Message:   notification.Message,
		Data:      notification.Data,
		Read:      notification.Read,
		CreatedAt: notification.CreatedAt,
		ReadAt:    notification.ReadAt,
	}, nil
}

func (s *NotificationService) ListNotifications(ctx context.Context, userID string, req *dto.ListNotificationsRequest) (*dto.ListNotificationsResponse, error) {
	s.logger.Info(fmt.Sprintf("listing notif for user: %s, limit=%d, offset=%d, unread=%t",
		userID, req.Limit, req.Offset, req.Unread))

	var notifications []*entities.Notification
	var err error

	if req.Unread {
		notifications, err = s.notificationRepo.GetUnreadByUserID(ctx, userID, req.Limit, req.Offset)
	} else {
		notifications, err = s.notificationRepo.GetByUserID(ctx, userID, req.Limit, req.Offset)
	}

	if err != nil {
		s.logger.Error(fmt.Sprintf("failed to list notif: %v", err))
		return nil, errors.ErrNotificationListFailed
	}

	unreadCount, err := s.notificationRepo.GetUnreadCount(ctx, userID)
	if err != nil {
		s.logger.Error(fmt.Sprintf("failed to get unread count: %v", err))
		unreadCount = 0 // Continue with 0 instead of failing
	}

	var notificationResponses []*dto.NotificationResponse
	for _, notification := range notifications {
		notificationResponses = append(notificationResponses, &dto.NotificationResponse{
			ID:        notification.ID,
			UserID:    notification.UserID,
			Type:      string(notification.Type),
			Title:     notification.Title,
			Message:   notification.Message,
			Data:      notification.Data,
			Read:      notification.Read,
			CreatedAt: notification.CreatedAt,
			ReadAt:    notification.ReadAt,
		})
	}

	return &dto.ListNotificationsResponse{
		Notifications: notificationResponses,
		Limit:         req.Limit,
		Offset:        req.Offset,
		Total:         len(notificationResponses),
		UnreadCount:   int64(unreadCount),
	}, nil
}

func (s *NotificationService) MarkAsRead(ctx context.Context, userID string, req *dto.MarkAsReadRequest) error {
	s.logger.Info(fmt.Sprintf("Marking notifications as read for user: %s", userID))

	if req.MarkAll {
		if err := s.notificationRepo.MakeAllAsRead(ctx, userID); err != nil {
			s.logger.Error(fmt.Sprintf("Failed to mark all notifications as read: %v", err))
			return errors.ErrNotificationUpdateFailed
		}
		s.logger.Info(fmt.Sprintf("All notifications marked as read for user: %s", userID))
		return nil
	}

	// Mark specific notifications as read
	for _, notificationID := range req.NotificationIDs {
		if err := s.notificationRepo.MarkAsRead(ctx, notificationID, userID); err != nil {
			s.logger.Error(fmt.Sprintf("Failed to mark notification %s as read: %v", notificationID, err))
			// Continue with other notifications instead of failing completely
		}
	}

	s.logger.Info(fmt.Sprintf("Notifications marked as read for user: %s", userID))
	return nil
}

func (s *NotificationService) DeleteNotification(ctx context.Context, id string, userID string) error {
	s.logger.Info(fmt.Sprintf("Deleting notification: %s for user: %s", id, userID))

	if err := s.notificationRepo.Delete(ctx, id, userID); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to delete notification: %v", err))
		return errors.ErrNotificationDeletionFailed
	}

	s.logger.Info(fmt.Sprintf("Notification deleted successfully: %s", id))
	return nil
}

func (s *NotificationService) GetUnreadCount(ctx context.Context, userID string) (int64, error) {
	count, err := s.notificationRepo.GetUnreadCount(ctx, userID)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to get unread count: %v", err))
		return 0, errors.ErrNotificationListFailed
	}

	return count, nil
}

func (s *NotificationService) ProcessPostCreatedEvent(ctx context.Context, eventData []byte) error {
	var event entities.PostCreatedEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal post created event: %w", err)
	}

	s.logger.Info(fmt.Sprintf("Processing post created event: %s by user %s", event.PostID, event.UserID))

	// Create notification for post author (optional - they might not want to be notified about their own posts)
	// In a real system, you might want to notify followers instead

	// For demo purposes, we'll create a notification for the author
	notification := event.ToNotification(event.UserID)
	notification.ID = uuid.New().String()

	// Validate and save
	notification.Sanitize()
	if err := notification.IsValid(); err != nil {
		return fmt.Errorf("invalid notification from event: %w", err)
	}

	if err := s.notificationRepo.Create(ctx, notification); err != nil {
		return fmt.Errorf("failed to create notification from event: %w", err)
	}

	s.logger.Info(fmt.Sprintf("Created notification %s for post created event", notification.ID))
	return nil
}

func (s *NotificationService) ProcessPostUpdatedEvent(ctx context.Context, eventData []byte) error {
	var event entities.PostUpdatedEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal post updated event: %w", err)
	}

	s.logger.Info(fmt.Sprintf("Processing post updated event: %s by user %s", event.PostID, event.UserID))

	// Create notification for post author
	notification := event.ToNotification(event.UserID)
	notification.ID = uuid.New().String()

	// Validate and save
	notification.Sanitize()
	if err := notification.IsValid(); err != nil {
		return fmt.Errorf("invalid notification from event: %w", err)
	}

	if err := s.notificationRepo.Create(ctx, notification); err != nil {
		return fmt.Errorf("failed to create notification from event: %w", err)
	}

	s.logger.Info(fmt.Sprintf("Created notification %s for post updated event", notification.ID))
	return nil
}

func (s *NotificationService) ProcessPostDeletedEvent(ctx context.Context, eventData []byte) error {
	var event entities.PostDeletedEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal post deleted event: %w", err)
	}

	s.logger.Info(fmt.Sprintf("Processing post deleted event: %s by user %s", event.PostID, event.UserID))

	// Create notification for post author
	notification := event.ToNotification(event.UserID)
	notification.ID = uuid.New().String()

	// Validate and save
	notification.Sanitize()
	if err := notification.IsValid(); err != nil {
		return fmt.Errorf("invalid notification from event: %w", err)
	}

	if err := s.notificationRepo.Create(ctx, notification); err != nil {
		return fmt.Errorf("failed to create notification from event: %w", err)
	}

	s.logger.Info(fmt.Sprintf("Created notification %s for post deleted event", notification.ID))
	return nil
}

func (s *NotificationService) CleanupOldNotifications(ctx context.Context, olderThanDays int) error {
	s.logger.Info(fmt.Sprintf("Cleaning up notifications older than %d days", olderThanDays))

	if err := s.notificationRepo.DeleteOld(ctx, olderThanDays); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to cleanup old notifications: %v", err))
		return errors.ErrNotificationDeletionFailed
	}

	return nil
}
