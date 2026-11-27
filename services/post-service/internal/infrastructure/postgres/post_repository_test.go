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
