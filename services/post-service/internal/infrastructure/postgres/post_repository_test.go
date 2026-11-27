package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"

	"post-service/internal/domain/entities"
	"post-service/internal/domain/repositories"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// TestRepositorySlugConflict checks against a real Postgres that the migration's
// unique index surfaces as ErrSlugTaken from Create and Update.
func TestRepositorySlugConflict(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := RunMigrations(db); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	ctx := context.Background()
	repo := NewPostRepository(db)
	newPost := func(slug string) *entities.Post {
		p := &entities.Post{ID: uuid.NewString(), UserID: "u", Title: "t", Content: "c", Slug: slug}
		t.Cleanup(func() { _ = repo.Delete(ctx, p.ID) })
		return p
	}
	slug := "t7-" + uuid.NewString()[:8]

	if err := repo.Create(ctx, newPost(slug)); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	if err := repo.Create(ctx, newPost(slug)); !errors.Is(err, repositories.ErrSlugTaken) {
		t.Errorf("Create with taken slug: err = %v, want ErrSlugTaken", err)
	}

	other := newPost(slug + "-other")
	if err := repo.Create(ctx, other); err != nil {
		t.Fatalf("Create other: %v", err)
	}
	other.Slug = slug
	if err := repo.Update(ctx, other); !errors.Is(err, repositories.ErrSlugTaken) {
		t.Errorf("Update to taken slug: err = %v, want ErrSlugTaken", err)
	}
}

func TestIsSlugConflict(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"slug unique violation", &pq.Error{Code: "23505", Constraint: "posts_slug_key"}, true},
		{"wrapped slug violation", fmt.Errorf("exec: %w", &pq.Error{Code: "23505", Constraint: "posts_slug_key"}), true},
		{"primary key violation", &pq.Error{Code: "23505", Constraint: "posts_pkey"}, false},
		{"other pq error", &pq.Error{Code: "23502", Constraint: "posts_slug_key"}, false},
		{"plain error", errors.New("duplicate key value violates unique constraint \"posts_slug_key\""), false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSlugConflict(tt.err); got != tt.want {
				t.Errorf("isSlugConflict(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// TestRepositoryGetByUserIDDrafts checks against a real Postgres that drafts
// are listed only with includeDrafts.
func TestRepositoryGetByUserIDDrafts(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := RunMigrations(db); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	ctx := context.Background()
	repo := NewPostRepository(db)
	author := "a16-" + uuid.NewString()[:8]
	for _, published := range []bool{true, false} {
		p := &entities.Post{ID: uuid.NewString(), UserID: author, Title: "t", Content: "c", Slug: "a16-" + uuid.NewString()[:8], Published: published}
		if err := repo.Create(ctx, p); err != nil {
			t.Fatalf("Create: %v", err)
		}
		t.Cleanup(func() { _ = repo.Delete(ctx, p.ID) })
	}

	for _, tt := range []struct {
		includeDrafts bool
		want          int
	}{{true, 2}, {false, 1}} {
		posts, err := repo.GetByUserID(ctx, author, 20, 0, tt.includeDrafts)
		if err != nil {
			t.Fatalf("GetByUserID(includeDrafts=%t): %v", tt.includeDrafts, err)
		}
		if len(posts) != tt.want {
			t.Errorf("includeDrafts=%t: got %d posts, want %d", tt.includeDrafts, len(posts), tt.want)
		}
		for _, p := range posts {
			if !p.Published && !tt.includeDrafts {
				t.Errorf("includeDrafts=false returned draft %s", p.ID)
			}
		}
	}
}

// TestRepositoryUnpublishByUserID checks against a real Postgres that every
// post of the user is hidden, others are untouched, drafts are not rewritten,
// and a retry writes no row again.
func TestRepositoryUnpublishByUserID(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := RunMigrations(db); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	ctx := context.Background()
	repo := NewPostRepository(db)
	author, other := "a13-"+uuid.NewString()[:8], "a13-"+uuid.NewString()[:8]
	create := func(userID string, published bool) *entities.Post {
		p := &entities.Post{ID: uuid.NewString(), UserID: userID, Title: "t", Content: "c", Slug: "a13-" + uuid.NewString()[:8], Published: published}
		if err := repo.Create(ctx, p); err != nil {
			t.Fatalf("Create: %v", err)
		}
		t.Cleanup(func() { _ = repo.Delete(ctx, p.ID) })
		return p
	}
	live, draft := create(author, true), create(author, false)
	othersPost := create(other, true)
	draftBefore, _ := repo.GetByID(ctx, draft.ID)

	var liveAfterFirst *entities.Post
	for attempt, want := range []int64{1, 0} { // only the published post, then nothing
		n, err := repo.UnpublishByUserID(ctx, author)
		if err != nil {
			t.Fatalf("attempt %d: %v", attempt+1, err)
		}
		if n != want {
			t.Errorf("attempt %d: changed %d posts, want %d", attempt+1, n, want)
		}
		got, _ := repo.GetByID(ctx, live.ID)
		if attempt == 0 {
			liveAfterFirst = got
		} else if !got.UpdatedAt.Equal(liveAfterFirst.UpdatedAt) {
			t.Errorf("retry rewrote the hidden post: updated_at %s -> %s", liveAfterFirst.UpdatedAt, got.UpdatedAt)
		}
	}

	if posts, _ := repo.GetByUserID(ctx, author, 20, 0, false); len(posts) != 0 {
		t.Errorf("author still has %d published posts", len(posts))
	}
	if got, _ := repo.GetByID(ctx, othersPost.ID); !got.Published {
		t.Error("another user's post was unpublished")
	}
	if got, _ := repo.GetByID(ctx, draft.ID); !got.UpdatedAt.Equal(draftBefore.UpdatedAt) {
		t.Errorf("draft updated_at moved: %s -> %s", draftBefore.UpdatedAt, got.UpdatedAt)
	}
	if n, err := repo.UnpublishByUserID(ctx, "a13-nobody"); err != nil || n != 0 {
		t.Errorf("user without posts: n = %d, err = %v; want 0, nil", n, err)
	}
}
