package postgres

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"

	"user-service/internal/domain/entities"
	"user-service/internal/domain/repositories"
)

// openTestDB connects to TEST_DATABASE_URL (a disposable database) and runs
// the migrations. Tests skip without it.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return db
}

func newTestUser(email string) *entities.User {
	return &entities.User{ID: uuid.New().String(), Email: email, Name: "Test", IsActive: true}
}

func TestUserRepositoryCreate_DuplicateEmail(t *testing.T) {
	repo := NewUserRepository(openTestDB(t))
	ctx := context.Background()
	email := uuid.New().String() + "@example.com"

	if err := repo.Create(ctx, newTestUser(email)); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	err := repo.Create(ctx, newTestUser(email))
	if !errors.Is(err, repositories.ErrUserAlreadyExists) {
		t.Fatalf("second Create err = %v, want ErrUserAlreadyExists", err)
	}
}

func TestUserRepositoryCreate_DuplicateID(t *testing.T) {
	repo := NewUserRepository(openTestDB(t))
	ctx := context.Background()
	first := newTestUser(uuid.New().String() + "@example.com")
	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	second := newTestUser(uuid.New().String() + "@example.com")
	second.ID = first.ID
	if err := repo.Create(ctx, second); !errors.Is(err, repositories.ErrUserAlreadyExists) {
		t.Fatalf("second Create err = %v, want ErrUserAlreadyExists", err)
	}
}

// Delete is soft, so the email stays taken (freeing it is T-14). Until then a
// new signup on that address must be a conflict, not a generic failure.
func TestUserRepositoryCreate_EmailOfDeletedAccount(t *testing.T) {
	repo := NewUserRepository(openTestDB(t))
	ctx := context.Background()
	email := uuid.New().String() + "@example.com"
	first := newTestUser(email)
	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	if err := repo.Delete(ctx, first.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := repo.Create(ctx, newTestUser(email)); !errors.Is(err, repositories.ErrUserAlreadyExists) {
		t.Fatalf("Create after delete err = %v, want ErrUserAlreadyExists", err)
	}
}
