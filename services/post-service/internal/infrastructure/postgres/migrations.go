package postgres

import (
	"database/sql"
	"fmt"
)

// migrationLockKey serializes migrations of replicas that start together:
// concurrent CREATE OR REPLACE FUNCTION fails with "tuple concurrently updated".
const migrationLockKey = 7_200_002

// RunMigrations brings the schema up to date in one transaction under one
// advisory lock, so it is safe to run again and from several replicas at once.
// extra is DDL owned by other packages that depends on the posts table
// (search.OutboxDDL); it runs in the same transaction.
func RunMigrations(db *sql.DB, extra ...string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // a no-op after Commit

	if _, err := tx.Exec(`SELECT pg_advisory_xact_lock($1)`, migrationLockKey); err != nil {
		return fmt.Errorf("migration lock: %w", err)
	}
	if _, err := tx.Exec(postsDDL); err != nil {
		return err
	}
	for _, ddl := range extra {
		if _, err := tx.Exec(ddl); err != nil {
			return err
		}
	}
	return tx.Commit()
}

const postsDDL = `
	CREATE TABLE IF NOT EXISTS posts (
		id VARCHAR(255) PRIMARY KEY,
		user_id VARCHAR(255) NOT NULL,
		title VARCHAR(200) NOT NULL,
		content TEXT NOT NULL,
		slug VARCHAR(100) UNIQUE NOT NULL,
		published BOOLEAN DEFAULT false,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_posts_user_id ON posts(user_id);
	CREATE INDEX IF NOT EXISTS idx_posts_slug ON posts(slug);
	CREATE INDEX IF NOT EXISTS idx_posts_published ON posts(published);
	CREATE INDEX IF NOT EXISTS idx_posts_created_at ON posts(created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_posts_search ON posts USING gin(to_tsvector('english', title || ' ' || content));
	
	-- Trigger to automatically update updated_at
	CREATE OR REPLACE FUNCTION update_updated_at_column()
	RETURNS TRIGGER AS $$
	BEGIN
		NEW.updated_at = CURRENT_TIMESTAMP;
		RETURN NEW;
	END;
	$$ language 'plpgsql';

	DROP TRIGGER IF EXISTS update_posts_updated_at ON posts;
	CREATE TRIGGER update_posts_updated_at 
		BEFORE UPDATE ON posts 
		FOR EACH ROW 
		EXECUTE FUNCTION update_updated_at_column();
	`
