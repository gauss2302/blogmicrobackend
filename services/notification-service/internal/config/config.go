package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                  string
	Environment           string
	LogLevel              string
	Database              DatabaseConfig
	RabbitMQ              RabbitMQConfig
	InternalHTTPTrustMode string
	Notification          NotificationConfig
}

type DatabaseConfig struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int
}

type RabbitMQConfig struct {
	URL            string
	ExchangeName   string
	QueueName      string
	RoutingKey     string
	DLXName        string
	DLQName        string
	DLQRoutingKey  string
	PrefetchCount  int
	ReconnectDelay int
	MaxRetries     int
}

type NotificationConfig struct {
	CleanupDays int
	BatchSize   int
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:        getEnv("PORT", "8084"),
		Environment: getEnv("ENVIRONMENT", "development"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		Database: DatabaseConfig{
			URL:             os.Getenv("DATABASE_URL"),
			MaxOpenConns:    getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvAsInt("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: getEnvAsInt("DB_CONN_MAX_LIFETIME", 60),
		},
		RabbitMQ: RabbitMQConfig{
			URL:            getEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/"),
			ExchangeName:   getEnv("RABBITMQ_EXCHANGE", "blog_events"),
			QueueName:      getEnv("RABBITMQ_QUEUE", "post_notifications"),
			RoutingKey:     getEnv("RABBITMQ_ROUTING_KEY", "post.created"),
			DLXName:        getEnv("RABBITMQ_DLX", "blog_events.dlx"),
			DLQName:        getEnv("RABBITMQ_DLQ", "post_notifications_dlq"),
			DLQRoutingKey:  getEnv("RABBITMQ_DLQ_ROUTING_KEY", "post.failed"),
			PrefetchCount:  getEnvAsInt("RABBITMQ_PREFETCH_COUNT", 10),
			ReconnectDelay: getEnvAsInt("RABBITMQ_RECONNECT_DELAY", 5),
			MaxRetries:     getEnvAsInt("RABBITMQ_MAX_RETRIES", 3),
		},
		InternalHTTPTrustMode: resolveInternalHTTPTrustMode(getEnv("INTERNAL_HTTP_TRUST_MODE", ""), getEnv("ENVIRONMENT", "development")),
		Notification: NotificationConfig{
			CleanupDays: getEnvAsInt("NOTIFICATION_CLEANUP_DAYS", 30),
			BatchSize:   getEnvAsInt("NOTIFICATION_BATCH_SIZE", 100),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil

}

func (c *Config) validate() error {
	if c.Database.URL == "" {
		return fmt.Errorf("DATABASE_URL is missing")
	}

	if c.RabbitMQ.URL == "" {
		return fmt.Errorf("RABBITMQ_URL is missing")
	}
	if c.RabbitMQ.DLXName == "" {
		return fmt.Errorf("RABBITMQ_DLX is missing")
	}
	if c.RabbitMQ.DLQName == "" {
		return fmt.Errorf("RABBITMQ_DLQ is missing")
	}
	if c.RabbitMQ.DLQRoutingKey == "" {
		return fmt.Errorf("RABBITMQ_DLQ_ROUTING_KEY is missing")
	}
	if err := validateInternalHTTPTrustMode(c.Environment, c.InternalHTTPTrustMode); err != nil {
		return err
	}
	if c.Notification.CleanupDays <= 0 {
		return fmt.Errorf("NOTIFICATION_CLEANUP_DAYS must be greater than 0")
	}
	if c.Notification.BatchSize <= 0 {
		return fmt.Errorf("NOTIFICATION_BATCH_SIZE must be greater than 0")
	}

	return nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultVal
}

func resolveInternalHTTPTrustMode(value, environment string) string {
	mode := strings.ToLower(strings.TrimSpace(value))
	if mode != "" {
		return mode
	}
	if environment == "production" {
		return ""
	}
	return "insecure_dev"
}

func validateInternalHTTPTrustMode(environment, mode string) error {
	switch mode {
	case "private_network", "disabled":
		return nil
	case "insecure_dev":
		if environment == "production" {
			return fmt.Errorf("INTERNAL_HTTP_TRUST_MODE=insecure_dev is not allowed in production")
		}
		return nil
	case "":
		if environment == "production" {
			return fmt.Errorf("INTERNAL_HTTP_TRUST_MODE is required in production")
		}
		return nil
	default:
		return fmt.Errorf("INTERNAL_HTTP_TRUST_MODE must be one of private_network, disabled, insecure_dev")
	}
}
