package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port     string
	LogLevel string
	Database DatabaseConfig
	RabbitMQ RabbitMQConfig
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
	PrefetchCount  int
	ReconnectDelay int
	MaxRetries     int
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:     getEnv("PORT", "8084"),
		LogLevel: getEnv("LOG_LEVEL", "info"),
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
			PrefetchCount:  getEnvAsInt("RABBITMQ_PREFETCH_COUNT", 10),
			ReconnectDelay: getEnvAsInt("RABBITMQ_RECONNECT_DELAY", 5),
			MaxRetries:     getEnvAsInt("RABBITMQ_MAX_RETRIES", 3),
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
