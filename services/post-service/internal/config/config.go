package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port string
	LogLevel string
	Database DatabaseConfig
}

type DatabaseConfig struct {
	URL string
	MaxOpenConns int
	MaxIdleConns int
	ConnMaxLifetime int
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:     getEnv("PORT", "8083"),
		LogLevel: getEnv("LOG_LEVEL", "info"),
		Database: DatabaseConfig{
			URL:             os.Getenv("DATABASE_URL"),
			MaxOpenConns:    getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvAsInt("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: getEnvAsInt("DB_CONN_MAX_LIFETIME", 60),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}


func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func (c *Config) validate() error {
	if c.Database.URL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	return nil
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}