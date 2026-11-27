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
// extra is DDL owned by other packages that depends on the users table
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
	for _, ddl := range append([]string{usersDDL, passwordHashDDL, searchIndexDDL, followsDDL}, extra...) {
		if _, err := tx.Exec(ddl); err != nil {
			return err
		}
	}
	return tx.Commit()
}

const usersDDL = `
	CREATE TABLE IF NOT EXISTS users (
		id VARCHAR(255) PRIMARY KEY,
		email VARCHAR(255) UNIQUE NOT NULL,
		name VARCHAR(100) NOT NULL,
		picture VARCHAR(500),
		password_hash VARCHAR(255),
		bio VARCHAR(500),
		location VARCHAR(100),
		website VARCHAR(255),
		is_active BOOLEAN DEFAULT true,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
	CREATE INDEX IF NOT EXISTS idx_users_name ON users(name);
	CREATE INDEX IF NOT EXISTS idx_users_is_active ON users(is_active);
	CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at DESC);
	-- Name only. Email used to be part of this expression, which let any
	-- authenticated caller enumerate accounts by address. The follow-up
	-- statement below replaces an index created by an older build.
	
	-- Trigger to automatically update updated_at
	CREATE OR REPLACE FUNCTION update_updated_at_column()
	RETURNS TRIGGER AS $$
	BEGIN
		NEW.updated_at = CURRENT_TIMESTAMP;
		RETURN NEW;
	END;
	$$ language 'plpgsql';

	DROP TRIGGER IF EXISTS update_users_updated_at ON users;
	CREATE TRIGGER update_users_updated_at 
		BEFORE UPDATE ON users 
		FOR EACH ROW 
		EXECUTE PROCEDURE update_updated_at_column();
	`

// passwordHashDDL adds password_hash if the table already exists without it.
const passwordHashDDL = `
	ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash VARCHAR(255);
	`

// searchIndexDDL drops the email-matching index (idx_users_search) and installs
// the name-only one. CREATE INDEX IF NOT EXISTS would keep the old expression.
const searchIndexDDL = `
	DROP INDEX IF EXISTS idx_users_search;
	CREATE INDEX IF NOT EXISTS idx_users_search_name ON users USING gin(to_tsvector('simple', name));
	`

// followsDDL is the follow/subscription graph.
const followsDDL = `
	CREATE TABLE IF NOT EXISTS follows (
		follower_id VARCHAR(255) NOT NULL,
		followee_id VARCHAR(255) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (follower_id, followee_id),
		CHECK (follower_id != followee_id),
		FOREIGN KEY (follower_id) REFERENCES users(id) ON DELETE CASCADE,
		FOREIGN KEY (followee_id) REFERENCES users(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_follows_followee_id ON follows(followee_id);
	CREATE INDEX IF NOT EXISTS idx_follows_follower_id ON follows(follower_id);
	`
