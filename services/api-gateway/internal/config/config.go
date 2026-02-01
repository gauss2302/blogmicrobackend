package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port        string
	Environment string
	LogLevel    string
	Server      ServerConfig
	Redis       RedisConfig
	Services    ServicesConfig
	RateLimit   RateLimitConfig
}

type ServerConfig struct {
	ReadTimeout  int
	WriteTimeout int
	IdleTimeout  int
}

type RedisConfig struct {
	URL      string
	Password string
	DB       int
}

type ServicesConfig struct {
	AuthURL         string
	AuthGRPCAddr    string
	UserURL         string
	UserGRPCAddr    string
	PostGRPCAddr    string
	NotificationURL string
}

type RateLimitConfig struct {
	RequestsPerMinute int
	BurstSize         int
	Enabled           bool
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:        getEnv("PORT", "8080"),
		Environment: getEnv("ENVIRONMENT", "development"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		Server: ServerConfig{
			ReadTimeout:  getEnvAsInt("SERVER_READ_TIMEOUT", 30),
			WriteTimeout: getEnvAsInt("SERVER_WRITE_TIMEOUT", 30),
			IdleTimeout:  getEnvAsInt("SERVER_IDLE_TIMEOUT", 60),
		},
		Redis: RedisConfig{
			URL:      getEnv("REDIS_URL", "redis:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},
		Services: ServicesConfig{
			AuthURL:         getEnv("AUTH_SERVICE_URL", "http://localhost:8081"),
			AuthGRPCAddr:    getEnv("AUTH_SERVICE_GRPC_ADDR", "localhost:50051"),
			UserURL:         getEnv("USER_SERVICE_URL", "http://localhost:8082"),
			UserGRPCAddr:    getEnv("USER_SERVICE_GRPC_ADDR", "localhost:50052"),
			PostGRPCAddr:    getEnv("POST_SERVICE_GRPC_ADDR", "localhost:50053"),
			NotificationURL: getEnv("NOTIFICATION_SERVICE_URL", "http://localhost:8084"),
		},
		RateLimit: RateLimitConfig{
			RequestsPerMinute: getEnvAsInt("RATE_LIMIT_RPM", 100),
			BurstSize:         getEnvAsInt("RATE_LIMIT_BURST", 20),
			Enabled:           getEnvAsBool("RATE_LIMIT_ENABLED", true),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.Services.AuthURL == "" {
		return fmt.Errorf("AUTH_SERVICE_URL is required")
	}
	if c.Services.AuthGRPCAddr == "" {
		return fmt.Errorf("AUTH_SERVICE_GRPC_ADDR is required")
	}
	if c.Services.UserGRPCAddr == "" {
		return fmt.Errorf("USER_SERVICE_GRPC_ADDR is required")
	}
	if c.Services.UserURL == "" {
		return fmt.Errorf("USER_SERVICE_URL is required")
	}
	if c.Services.PostGRPCAddr == "" {
		return fmt.Errorf("POST_SERVICE_GRPC_ADDR is required")
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
