package postgres

import (
	"database/sql"
	"os"
	"testing"
)

// TestRunMigrations runs the migrations against a real Postgres, twice, to
// catch SQL that Go compiles but Postgres rejects. TEST_DATABASE_URL must point
// at a disposable database: the test creates tables and never cleans up.
func TestRunMigrations(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}

	for i := 1; i <= 2; i++ {
		if err := RunMigrations(db); err != nil {
			t.Fatalf("RunMigrations call %d: %v", i, err)
		}
	}

	exists := func(name string) bool {
		var ok bool
		if err := db.QueryRow(`SELECT to_regclass($1) IS NOT NULL`, name).Scan(&ok); err != nil {
			t.Fatalf("to_regclass(%s): %v", name, err)
		}
		return ok
	}
	for _, name := range []string{"users", "follows", "idx_users_search_name"} {
		if !exists(name) {
			t.Errorf("%s missing after migrations", name)
		}
	}
	if exists("idx_users_search") {
		t.Error("idx_users_search should be dropped")
	}
}
