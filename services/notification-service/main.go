package main

import (
	"log"
	"notification-service/internal/config"
	postgres "notification-service/internal/infrastructure"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := postgres.NewConntection(cfg.Database)
	if err != nil {
		// TODO logger
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()

	if err := postgres.RunMigrations(db); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	notificationRepo := postgres.NewNotificationRepository(db)
}
