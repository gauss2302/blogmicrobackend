package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

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
	if _, err := repo.Update(ctx, other.ID, func(p *entities.Post) error { p.Slug = slug; return nil }); !errors.Is(err, repositories.ErrSlugTaken) {
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
		posts, _, err := repo.GetByUserID(ctx, author, 20, 0, tt.includeDrafts)
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

	if posts, _, _ := repo.GetByUserID(ctx, author, 20, 0, false); len(posts) != 0 {
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

// TestRepositoryUpdateConcurrent checks against a real Postgres that two
// updates of different fields at once both survive, that updated_at comes back
// from the database, and that another user's draft slug is a conflict.
func TestRepositoryUpdateConcurrent(t *testing.T) {
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
	p := &entities.Post{ID: uuid.NewString(), UserID: "a58", Title: "Old title", Content: "Old content", Slug: "a58-" + uuid.NewString()[:8], Published: true}
	if err := repo.Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Delete(ctx, p.ID) })
	before, _ := repo.GetByID(ctx, p.ID)

	// The first update holds the row while the second one starts.
	inFirst, release := make(chan struct{}), make(chan struct{})
	first := make(chan error)
	go func() {
		_, err := repo.Update(ctx, p.ID, func(p *entities.Post) error {
			close(inFirst)
			<-release
			p.Title = "New title"
			return nil
		})
		first <- err
	}()
	<-inFirst
	second := make(chan error)
	var secondResult *entities.Post
	go func() {
		var err error
		secondResult, err = repo.Update(ctx, p.ID, func(p *entities.Post) error {
			p.Content = "New content"
			return nil
		})
		second <- err
	}()
	time.Sleep(200 * time.Millisecond) // let the second update reach the row lock
	close(release)
	if err := <-first; err != nil {
		t.Fatalf("first update: %v", err)
	}
	if err := <-second; err != nil {
		t.Fatalf("second update: %v", err)
	}

	got, _ := repo.GetByID(ctx, p.ID)
	if got.Title != "New title" || got.Content != "New content" {
		t.Errorf("after both updates: title %q, content %q; want both changes", got.Title, got.Content)
	}
	if !secondResult.UpdatedAt.Equal(got.UpdatedAt) || got.UpdatedAt.Equal(before.UpdatedAt) {
		t.Errorf("returned updated_at %s, stored %s, before %s", secondResult.UpdatedAt, got.UpdatedAt, before.UpdatedAt)
	}

	draft := &entities.Post{ID: uuid.NewString(), UserID: "a58-other", Title: "t", Content: "c", Slug: "a58-draft-" + uuid.NewString()[:8]}
	if err := repo.Create(ctx, draft); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Delete(ctx, draft.ID) })
	if _, err := repo.Update(ctx, p.ID, func(p *entities.Post) error { p.Slug = draft.Slug; return nil }); !errors.Is(err, repositories.ErrSlugTaken) {
		t.Errorf("another user's draft slug: err = %v, want ErrSlugTaken", err)
	}
	if _, err := repo.Update(ctx, "missing", func(*entities.Post) error { return nil }); !errors.Is(err, repositories.ErrPostNotFound) {
		t.Errorf("missing post: err = %v, want ErrPostNotFound", err)
	}
}

// TestRepositoryTotalsAndNotFound checks against a real Postgres that the list
// methods report every match, also past the last page, and that only a
// missing row is ErrPostNotFound.
func TestRepositoryTotalsAndNotFound(t *testing.T) {
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
	author, word := "a56-"+uuid.NewString()[:8], "zebra"+strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	for i := 0; i < 4; i++ {
		p := &entities.Post{ID: uuid.NewString(), UserID: author, Title: "t", Content: "about " + word, Slug: "a56-" + uuid.NewString()[:8], Published: i < 3}
		if err := repo.Create(ctx, p); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = repo.Delete(ctx, p.ID) })
	}

	for _, tt := range []struct {
		name              string
		list              func(limit, offset int) ([]*entities.Post, int, error)
		wantTotal, offset int
	}{
		{"user published", func(l, o int) ([]*entities.Post, int, error) { return repo.GetByUserID(ctx, author, l, o, false) }, 3, 0},
		{"user with drafts", func(l, o int) ([]*entities.Post, int, error) { return repo.GetByUserID(ctx, author, l, o, true) }, 4, 0},
		{"user past the end", func(l, o int) ([]*entities.Post, int, error) { return repo.GetByUserID(ctx, author, l, o, false) }, 3, 10},
		{"search published", func(l, o int) ([]*entities.Post, int, error) { return repo.Search(ctx, word, l, o, true) }, 3, 0},
	} {
		posts, total, err := tt.list(2, tt.offset)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		wantPage := min(2, max(tt.wantTotal-tt.offset, 0))
		if total != tt.wantTotal || len(posts) != wantPage {
			t.Errorf("%s: page %d, total %d; want %d and %d", tt.name, len(posts), total, wantPage, tt.wantTotal)
		}
	}

	var published int
	if err := db.QueryRow(`SELECT count(*) FROM posts WHERE published`).Scan(&published); err != nil {
		t.Fatal(err)
	}
	if posts, total, err := repo.List(ctx, 1, 0, true); err != nil || total != published || len(posts) != 1 {
		t.Errorf("List: page %d, total %d, err %v; want 1, %d, nil", len(posts), total, err, published)
	}

	if _, err := repo.GetByID(ctx, "missing"); !errors.Is(err, repositories.ErrPostNotFound) {
		t.Errorf("GetByID missing: err = %v, want ErrPostNotFound", err)
	}
	if err := repo.Delete(ctx, "missing"); !errors.Is(err, repositories.ErrPostNotFound) {
		t.Errorf("Delete missing: err = %v, want ErrPostNotFound", err)
	}
	closed, _ := sql.Open("postgres", url)
	closed.Close()
	if _, err := NewPostRepository(closed).GetByID(ctx, "any"); err == nil || errors.Is(err, repositories.ErrPostNotFound) {
		t.Errorf("GetByID on a dead database: err = %v, want a failure that is not ErrPostNotFound", err)
	}
}
