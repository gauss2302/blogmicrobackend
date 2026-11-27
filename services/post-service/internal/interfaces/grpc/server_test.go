package grpc

import (
	"context"
	"testing"

	"post-service/internal/application/services"
	"post-service/internal/domain/repositories"
	"post-service/pkg/logger"

	postv1 "github.com/gauss2302/blogmicrobackend/proto/post/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// unpublishRepo records UnpublishByUserID calls; other methods panic via the nil embed.
type unpublishRepo struct {
	repositories.PostRepository
	calls []string
}

func (r *unpublishRepo) UnpublishByUserID(_ context.Context, userID string) (int64, error) {
	r.calls = append(r.calls, userID)
	return 0, nil
}

func TestUnpublishUserPostsRequiresTheUserAsActor(t *testing.T) {
	log := logger.New("fatal")
	for _, tt := range []struct {
		name, userID, actorID string
		want                  codes.Code
	}{
		{"no user", "", "", codes.InvalidArgument},
		{"no actor", "u1", "", codes.PermissionDenied},
		{"someone else", "u1", "u2", codes.PermissionDenied},
		{"the user", "u1", "u1", codes.OK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &unpublishRepo{}
			srv := NewPostServer(services.NewPostService(repo, nil, log), log)

			_, err := srv.UnpublishUserPosts(context.Background(), &postv1.UnpublishUserPostsRequest{UserId: tt.userID, ActorId: tt.actorID})
			if got := status.Code(err); got != tt.want {
				t.Fatalf("code = %s, want %s (%v)", got, tt.want, err)
			}
			wantCalls := 0
			if tt.want == codes.OK {
				wantCalls = 1
			}
			if len(repo.calls) != wantCalls {
				t.Errorf("repository called %d times, want %d", len(repo.calls), wantCalls)
			}
		})
	}
}
