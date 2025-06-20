package entities

import (
	"fmt"
	"strings"
	"time"
)

type NotificationType string

const (
	NotificationTypePostCreated NotificationType = "post_created"
	NotificationTypePostUpdated NotificationType = "post_updated"
	NotificationTypePostDeleted NotificationType = "post_deleted"
)

type Notification struct {
	ID        string                 `json:"id" db:"id"`
	UserID    string                 `json:"user_id" db:"user_id"`
	Type      NotificationType       `json:"type" db:"type"`
	Title     string                 `json:"title" db:"title"`
	Message   string                 `json:"message" db:"message"`
	Data      map[string]interface{} `json:"data,omitempty" db:"data"`
	Read      bool                   `json:"read" db:"read"`
	CreatedAt time.Time              `json:"created_at" db:"created_at"`
	ReadAt    *time.Time             `json:"read_at,omitempty" db:"read_at"`
}

type PostCreatedEvent struct {
	PostID    string    `json:"post_id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	Slug      string    `json:"slug"`
	Published bool      `json:"published"`
	CreatedAt time.Time `json:"created_at"`
}

type PostUpdatedEvent struct {
	PostID    string    `json:"post_id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	Slug      string    `json:"slug"`
	Published bool      `json:"published"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PostDeletedEvent struct {
	PostID    string    `json:"post_id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	DeletedAt time.Time `json:"deleted_at"`
}

func (n *Notification) IsValid() error {
	if strings.TrimSpace(n.ID) == "" {
		return fmt.Errorf("notification ID is required")
	}

	if strings.TrimSpace(n.UserID) == "" {
		return fmt.Errorf("user ID is required")
	}

	if strings.TrimSpace(string(n.Type)) == "" {
		return fmt.Errorf("notification type is required")
	}

	if strings.TrimSpace(n.Title) == "" {
		return fmt.Errorf("title is required")
	}

	if len(n.Title) > 200 {
		return fmt.Errorf("title must be less than 200 characters")
	}

	if strings.TrimSpace(n.Message) == "" {
		return fmt.Errorf("message is required")
	}

	if len(n.Message) > 1000 {
		return fmt.Errorf("message must be less than 1000 characters")
	}

	return nil
}

func (n *Notification) Sanitize() {
	n.Title = strings.TrimSpace(n.Title)
	n.Message = strings.TrimSpace(n.Message)
}

func (n *Notification) MarkAsRead() {
	n.Read = true
	now := time.Now()
	n.ReadAt = &now
}

func (e *PostCreatedEvent) ToNotification(userID string) *Notification {
	title := "New Post Published"

	message := fmt.Sprintf("A new post %s has been published", e.Title)

	if !e.Published {
		title = "New Post Created"
		message = fmt.Sprintf("A new post '%s' has been created", e.Title)
	}

	return &Notification{
		UserID:  userID,
		Type:    NotificationTypePostCreated,
		Title:   title,
		Message: message,
		Data: map[string]interface{}{
			"post_id":   e.PostID,
			"post_slug": e.Slug,
			"author_id": e.UserID,
		},
		Read: false,
	}
}

func (e *PostUpdatedEvent) ToNotification(userID string) *Notification {
	title := "Post is Updated"

	message := fmt.Sprintf("A new post %s was updated", e.Title)

	return &Notification{
		UserID:  userID,
		Type:    NotificationTypePostUpdated,
		Title:   title,
		Message: message,
		Data: map[string]interface{}{
			"post_id":   e.PostID,
			"post_slug": e.Slug,
			"author_id": e.UserID,
		},
		Read: false,
	}
}

func (e *PostDeletedEvent) ToNotification(userID string) *Notification {
	title := "Post Deleted"
	message := fmt.Sprintf("The post %s was deleted", e.Title)

	return &Notification{
		UserID:  userID,
		Type:    NotificationTypePostDeleted,
		Title:   title,
		Message: message,
		Data: map[string]interface{}{
			"post_id":   e.PostID,
			"author_id": e.UserID,
		},
		Read: false,
	}
}
