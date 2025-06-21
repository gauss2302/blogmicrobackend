package dto

import "time"

type NotificationResponse struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"user_id"`
	Type      string                 `json:"type"`
	Title     string                 `json:"title"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Read      bool                   `json:"read"`
	CreatedAt time.Time              `json:"created_at"`
	ReadAt    *time.Time             `json:"read_at,omitempty"`
}

type ListNotificationsRequest struct {
	Limit  int  `form:"limit,default=20" binding:"omitempty,min=1,max=100"`
	Offset int  `form:"offset,default=0" binding:"omitempty,min=0"`
	Unread bool `form:"unread,default=false"`
}

type ListNotificationsResponse struct {
	Notifications []*NotificationResponse `json:"notifications"`
	Limit         int                     `json:"limit"`
	Offset        int                     `json:"offset"`
	Total         int                     `json:"total"`
	UnreadCount   int64                   `json:"unread_count"`
}

type MarkAsReadRequest struct {
	NotificationIDs []string `json:"notification_ids,omitempty"`
	MarkAll         bool     `json:"mark_all,omitempty"`
}

type CreateNotificationRequest struct {
	UserID  string                 `json:"user_id" binding:"required"`
	Type    string                 `json:"type" binding:"required"`
	Title   string                 `json:"title" binding:"required,min=1,max=200"`
	Message string                 `json:"message" binding:"required,min=1,max=1000"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

type NotificationStatsResponse struct {
	TotalNotifications  int64            `json:"total_notifications"`
	UnreadNotifications int64            `json:"unread_notifications"`
	NotificationsByType map[string]int64 `json:"notifications_by_type"`
}
