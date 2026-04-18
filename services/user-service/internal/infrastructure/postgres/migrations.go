package postgres

import (
	"database/sql"
)

func RunMigrations(db *sql.DB) error {
	query := `
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
	CREATE INDEX IF NOT EXISTS idx_users_search ON users USING gin(to_tsvector('simple', COALESCE(name, '') || ' ' || COALESCE(email, '')));
	
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

	if _, err := db.Exec(query); err != nil {
		return err
	}

	// Add password_hash if table already exists without it (idempotent migration)
	alterQuery := `
	ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash VARCHAR(255);
	`
	if _, err := db.Exec(alterQuery); err != nil {
		return err
	}

	// Follows table for follow/subscription graph
	followsQuery := `
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
	_, err := db.Exec(followsQuery)
	return err
}
