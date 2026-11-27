package services

import (
	"context"
	"testing"

	"notification-service/internal/domain/entities"
	"notification-service/internal/domain/repositories"
	"notification-service/pkg/logger"
)

// fakeRepo implements only what a test calls; any other method panics on the
// nil embedded interface.
type fakeRepo struct {
	repositories.NotificationRepository
	byEvent map[string]int
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
