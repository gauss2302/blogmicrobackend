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
	JWTSecret             string
	Database              DatabaseConfig
	Redis                 RedisConfig
	RabbitMQ              RabbitMQConfig
	InternalHTTPTrustMode string
	Notification          NotificationConfig
}

type RedisConfig struct {
	URL      string
	Password string
	DB       int
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
	environment := normalizeEnvironment(getEnv("ENVIRONMENT", "development"))

	cfg := &Config{
		Port:        getEnv("PORT", "8084"),
		Environment: environment,
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		Database: DatabaseConfig{
			URL:             os.Getenv("DATABASE_URL"),
			MaxOpenConns:    getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvAsInt("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: getEnvAsInt("DB_CONN_MAX_LIFETIME", 60),
		},
		Redis: RedisConfig{
			URL:      os.Getenv("REDIS_URL"),
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},
		RabbitMQ: RabbitMQConfig{
			URL:            getEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/"),
			ExchangeName:   getEnv("RABBITMQ_EXCHANGE", "blog_events"),
			QueueName:      getEnv("RABBITMQ_QUEUE", "post_notifications"),
			RoutingKey:     getEnv("RABBITMQ_ROUTING_KEY", "post.*"), // created, updated and deleted
			DLXName:        getEnv("RABBITMQ_DLX", "blog_events.dlx"),
			DLQName:        getEnv("RABBITMQ_DLQ", "post_notifications_dlq"),
			DLQRoutingKey:  getEnv("RABBITMQ_DLQ_ROUTING_KEY", "post.failed"),
			PrefetchCount:  getEnvAsInt("RABBITMQ_PREFETCH_COUNT", 10),
			ReconnectDelay: getEnvAsInt("RABBITMQ_RECONNECT_DELAY", 5),
			MaxRetries:     getEnvAsInt("RABBITMQ_MAX_RETRIES", 3),
		},
		InternalHTTPTrustMode: resolveInternalHTTPTrustMode(getEnv("INTERNAL_HTTP_TRUST_MODE", ""), environment),
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
	if err := validateEnvironment(c.Environment); err != nil {
		return err
	}
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
	// The JWT secret must match auth-service's so access tokens can be verified.
	// It is mandatory in every mode except the explicitly insecure local-dev mode,
	// which permits the unauthenticated X-User-ID header fallback instead.
	if c.InternalHTTPTrustMode != "insecure_dev" {
		if len(c.JWTSecret) < 32 {
			return fmt.Errorf("JWT_SECRET must be set and at least 32 characters")
		}
	} else if c.JWTSecret != "" && len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	if c.InternalHTTPTrustMode != "insecure_dev" && c.Redis.URL == "" {
		return fmt.Errorf("REDIS_URL is required to check token revocation")
	}
	if c.Environment == "production" {
		if err := rejectPlaceholderSecrets(
			namedSecret{"JWT_SECRET", c.JWTSecret},
			namedSecret{"DATABASE_URL", c.Database.URL},
			namedSecret{"RABBITMQ_URL", c.RabbitMQ.URL},
			namedSecret{"REDIS_PASSWORD", c.Redis.Password},
		); err != nil {
			return err
		}
	}
	if c.RabbitMQ.MaxRetries < 0 {
		return fmt.Errorf("RABBITMQ_MAX_RETRIES must be >= 0")
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
	if isDevEnvironment(environment) {
		return "insecure_dev"
	}
	// Unknown environment: fail closed. "" still requires a verified token.
	return ""
}

func validateInternalHTTPTrustMode(environment, mode string) error {
	switch mode {
	case "private_network", "disabled":
		return nil
	case "insecure_dev":
		if !isDevEnvironment(environment) {
			return fmt.Errorf("INTERNAL_HTTP_TRUST_MODE=insecure_dev is not allowed outside a development environment")
		}
		return nil
	case "":
		if !isDevEnvironment(environment) {
			return fmt.Errorf("INTERNAL_HTTP_TRUST_MODE is required outside a development environment")
		}
		return nil
	default:
		return fmt.Errorf("INTERNAL_HTTP_TRUST_MODE must be one of private_network, disabled, insecure_dev")
	}
}
