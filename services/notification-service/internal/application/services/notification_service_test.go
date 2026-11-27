package services

import (
	"context"
	"errors"
	"testing"

	apperrors "notification-service/internal/application/errors"

	"notification-service/internal/domain/entities"
	"notification-service/internal/domain/repositories"
	"notification-service/pkg/logger"
)

// fakeRepo implements only what a test calls; any other method panics on the
// nil embedded interface.
type fakeRepo struct {
	repositories.NotificationRepository
	byEvent map[string]int
	stored  map[string]*entities.Notification // by id, for GetByID and Delete
}

func (r *fakeRepo) GetByID(_ context.Context, id string) (*entities.Notification, error) {
	if n, ok := r.stored[id]; ok {
		return n, nil
	}
	return nil, errors.New("record not found in postgres db")
}

func (r *fakeRepo) Delete(_ context.Context, id, userID string) error {
	if n, ok := r.stored[id]; ok && n.UserID == userID {
		delete(r.stored, id)
		return nil
	}
	return repositories.ErrNotificationNotFound
}

func (r *fakeRepo) Create(_ context.Context, n *entities.Notification) error {
	if n.EventID != "" && r.byEvent[n.EventID] > 0 {
		return repositories.ErrDuplicateEvent
	}
	r.byEvent[n.EventID]++
	return nil
}

func TestRedeliveredEventIsAcknowledgedOnce(t *testing.T) {
	repo := &fakeRepo{byEvent: map[string]int{}}
	s := NewNotificationService(repo, logger.New("fatal"))
	body := []byte(`{"post_id":"p1","user_id":"u1","title":"Hello","slug":"hello","published":true}`)

	for name, process := range map[string]func(context.Context, string, []byte) error{
		"created": s.ProcessPostCreatedEvent,
		"updated": s.ProcessPostUpdatedEvent,
		"deleted": s.ProcessPostDeletedEvent,
	} {
		eventID := "post." + name + "-1"
		for delivery := 1; delivery <= 2; delivery++ {
			if err := process(context.Background(), eventID, body); err != nil {
				t.Errorf("%s delivery %d: err = %v, want nil (a duplicate is acked, not retried)", name, delivery, err)
			}
		}
		if got := repo.byEvent[eventID]; got != 1 {
			t.Errorf("%s: stored %d notifications for event %s, want 1", name, got, eventID)
		}
	}
}

// Another user's notification answers exactly like a missing one: 404.
func TestForeignNotificationIsNotFound(t *testing.T) {
	newService := func() *NotificationService {
		repo := &fakeRepo{stored: map[string]*entities.Notification{
			"n1": {ID: "n1", UserID: "owner", Type: entities.NotificationTypePostCreated, Title: "t", Message: "m"},
		}}
		return NewNotificationService(repo, logger.New("fatal"))
	}
	ctx := context.Background()

	if _, err := newService().GetNotification(ctx, "n1", "owner"); err != nil {
		t.Fatalf("owner get: %v", err)
	}
	for _, tt := range []struct{ id, user string }{{"n1", "intruder"}, {"missing", "owner"}} {
		if _, err := newService().GetNotification(ctx, tt.id, tt.user); err != apperrors.ErrNotificationNotFound {
			t.Errorf("get %s as %s: err = %v, want ErrNotificationNotFound", tt.id, tt.user, err)
		}
		if err := newService().DeleteNotification(ctx, tt.id, tt.user); err != apperrors.ErrNotificationNotFound {
			t.Errorf("delete %s as %s: err = %v, want ErrNotificationNotFound", tt.id, tt.user, err)
		}
	}
	if err := newService().DeleteNotification(ctx, "n1", "owner"); err != nil {
		t.Fatalf("owner delete: %v", err)
	}
}
