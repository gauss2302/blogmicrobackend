package grpc

import (
	"context"
	"testing"
	"time"

	"post-service/internal/application/services"
	"post-service/internal/domain/repositories"
	"post-service/internal/infrastructure/search"
	"post-service/pkg/logger"

	postv1 "github.com/gauss2302/blogmicrobackend/proto/post/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// unpublishRepo records UnpublishByUserID calls; other methods panic via the nil embed.
type unpublishRepo struct {
	repositories.PostRepository
	calls []string
	ids   []string
}

func (r *unpublishRepo) UnpublishByUserID(_ context.Context, userID string) ([]string, error) {
	r.calls = append(r.calls, userID)
	return r.ids, nil
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
			srv := NewPostServer(services.NewPostService(repo, nil, nil, log), log)

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

func TestUnpublishUserPostsSearchDownIsUnavailable(t *testing.T) {
	log := logger.New("fatal")
	indexer := search.NewIndexer([]string{"127.0.0.1:1"}, "search.posts", "", "", log) // nothing listens there
	t.Cleanup(func() { _ = indexer.Close() })
	repo := &unpublishRepo{ids: []string{"p1"}}
	srv := NewPostServer(services.NewPostService(repo, nil, indexer, log), log)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := srv.UnpublishUserPosts(ctx, &postv1.UnpublishUserPostsRequest{UserId: "u1", ActorId: "u1"})
	if got := status.Code(err); got != codes.Unavailable {
		t.Fatalf("code = %s, want Unavailable (%v)", got, err)
	}
	if len(repo.calls) != 1 {
		t.Errorf("repository called %d times, want 1: posts are hidden before search is told", len(repo.calls))
	}
}
