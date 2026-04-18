package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	GRPCPort             string
	Environment          string
	LogLevel             string
	OpenSearch           OpenSearchConfig
	Kafka                KafkaConfig
	UserServiceGRPC      string
	UsersIndexName       string
	PostsIndexName       string
	GRPCTLS              GRPCTLSConfig
	EnableGRPCReflection bool
}

type OpenSearchConfig struct {
	URL     string
	Enabled bool
}

type KafkaConfig struct {
	Brokers              []string
	TopicUsers           string
	TopicPosts           string
	DLQTopic             string
	ConsumerGroup        string
	MaxProcessingRetries int
	RetryBackoffMS       int
	Enabled              bool
}

type GRPCTLSConfig struct {
	Enabled           bool
	CAFile            string
	CertFile          string
	KeyFile           string
	RequireClientCert bool
}

func Load() (*Config, error) {
	cfg := &Config{
		GRPCPort:        getEnv("GRPC_PORT", "50054"),
		Environment:     getEnv("ENVIRONMENT", "development"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		UserServiceGRPC: getEnv("USER_SERVICE_GRPC_ADDR", "user-service:50052"),
		UsersIndexName:  getEnv("OPENSEARCH_USERS_INDEX", "users"),
		PostsIndexName:  getEnv("OPENSEARCH_POSTS_INDEX", "posts"),
		OpenSearch: OpenSearchConfig{
			URL:     getEnv("OPENSEARCH_URL", "http://opensearch:9200"),
			Enabled: getEnv("OPENSEARCH_URL", "") != "",
		},
		Kafka: KafkaConfig{
			Brokers:              getEnvSlice("KAFKA_BROKERS", []string{"kafka:9092"}),
			TopicUsers:           getEnv("KAFKA_TOPIC_USERS", "search.users"),
			TopicPosts:           getEnv("KAFKA_TOPIC_POSTS", "search.posts"),
			DLQTopic:             getEnv("KAFKA_DLQ_TOPIC", "search.dlq"),
			ConsumerGroup:        getEnv("KAFKA_CONSUMER_GROUP", "search-indexer"),
			MaxProcessingRetries: getEnvAsInt("KAFKA_MAX_PROCESSING_RETRIES", 3),
			RetryBackoffMS:       getEnvAsInt("KAFKA_RETRY_BACKOFF_MS", 500),
			Enabled:              getEnv("KAFKA_BROKERS", "") != "",
		},
		GRPCTLS: GRPCTLSConfig{
			Enabled:           getEnvAsBool("GRPC_TLS_ENABLED", false),
			CAFile:            getEnv("GRPC_TLS_CA_FILE", ""),
			CertFile:          getEnv("GRPC_TLS_CERT_FILE", ""),
			KeyFile:           getEnv("GRPC_TLS_KEY_FILE", ""),
			RequireClientCert: getEnvAsBool("GRPC_TLS_REQUIRE_CLIENT_CERT", false),
		},
		EnableGRPCReflection: getEnvAsBool("GRPC_REFLECTION_ENABLED", getEnv("ENVIRONMENT", "development") != "production"),
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

func getEnvSlice(key string, defaultVal []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultVal
}

func (c *Config) validate() error {
	if c.GRPCPort == "" {
		return fmt.Errorf("GRPC_PORT is required")
	}
	if c.OpenSearch.Enabled && c.OpenSearch.URL == "" {
		return fmt.Errorf("OPENSEARCH_URL is required when OpenSearch is enabled")
	}
	if c.Kafka.Enabled && len(c.Kafka.Brokers) == 0 {
		return fmt.Errorf("KAFKA_BROKERS is required when Kafka is enabled")
	}
	if c.Kafka.MaxProcessingRetries < 1 {
		return fmt.Errorf("KAFKA_MAX_PROCESSING_RETRIES must be >= 1")
	}
	if c.Kafka.RetryBackoffMS < 0 {
		return fmt.Errorf("KAFKA_RETRY_BACKOFF_MS must be >= 0")
	}
	if c.GRPCTLS.Enabled {
		if c.GRPCTLS.CAFile == "" {
			return fmt.Errorf("GRPC_TLS_CA_FILE is required when GRPC_TLS_ENABLED=true")
		}
		if c.GRPCTLS.CertFile == "" || c.GRPCTLS.KeyFile == "" {
			return fmt.Errorf("GRPC_TLS_CERT_FILE and GRPC_TLS_KEY_FILE are required when GRPC_TLS_ENABLED=true")
		}
	}
	if (c.GRPCTLS.CertFile == "") != (c.GRPCTLS.KeyFile == "") {
		return fmt.Errorf("GRPC_TLS_CERT_FILE and GRPC_TLS_KEY_FILE must be set together")
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

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
