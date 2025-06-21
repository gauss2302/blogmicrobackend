package validators

import (
	"fmt"
	"notification-service/internal/application/dto"
	"strings"
)

type NotificationValidator struct {
}

func NewNotificationValidator() *NotificationValidator {
	return &NotificationValidator{}
}

func (v *NotificationValidator) ValidateCreateNotificationRequest(req *dto.CreateNotificationRequest) error {
	if strings.TrimSpace(req.UserID) == "" {
		return fmt.Errorf("user id is required")
	}
	if strings.TrimSpace(req.Type) == "" {
		return fmt.Errorf("notif type is required")
	}

	validTypes := map[string]bool{
		"post_created":  true,
		"post_updated":  true,
		"post_deleted":  true,
		"user_followed": true,
		"comment_added": true,
		"system_alert":  true}

	if !validTypes[req.Type] {
		return fmt.Errorf("invalid notif type: %s", req.Type)
	}

	if strings.TrimSpace(req.Title) == "" {
		return fmt.Errorf("title is required")
	}

	if len(req.Title) > 200 {
		return fmt.Errorf("title must be less than 200 characters")
	}

	if strings.TrimSpace(req.Message) == "" {
		return fmt.Errorf("message is required")
	}

	if len(req.Message) > 1000 {
		return fmt.Errorf("message must be less than 1000 characters")
	}

	return nil

}

func (v *NotificationValidator) ValidateMarkAsReadRequest(req *dto.MarkAsReadRequest) error {
	if !req.MarkAll && len(req.NotificationIDs) == 0 {
		return fmt.Errorf("either mark_all must be true or notification_ids must be provided")
	}

	if req.MarkAll && len(req.NotificationIDs) > 0 {
		return fmt.Errorf("cannot specify both mark_all and notification_ids")
	}

	for _, id := range req.NotificationIDs {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("notif id cannot be empty")
		}
	}

	return nil
}
