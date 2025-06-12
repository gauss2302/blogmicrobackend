package postgres

import (
	"database/sql"
)

func RunMigrations(db *sql.DB) error {
	query := `
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

	_, err := db.Exec(query)
	return err
}