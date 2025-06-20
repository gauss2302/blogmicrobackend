package postgres

import "database/sql"

func RunMigrations(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS notifications (
		id VARCHAR(255) PRIMARY KEY,
		user_id VARCHAR(255) NOT NULL,
		type VARCHAR(50) NOT NULL,
		title VARCHAR(200) NOT NULL,
		message VARCHAR(1000) NOT NULL,
		data JSONB,
		read BOOLEAN DEFAULT false,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		read_at TIMESTAMP NULL
	);

		CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id);
	CREATE INDEX IF NOT EXISTS idx_notifications_user_read ON notifications(user_id, read);
	CREATE INDEX IF NOT EXISTS idx_notifications_type ON notifications(type);
	CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_notifications_unread ON notifications(user_id, read, created_at DESC) WHERE read = false;

	-- Gin index for JSONB data field for fast queries on notification data
	CREATE INDEX IF NOT EXISTS idx_notifications_data_gin ON notifications USING gin(data);

	`

	_, err := db.Exec(query)
	return err
}
