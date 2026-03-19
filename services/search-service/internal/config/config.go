package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	GRPCPort       string
	LogLevel       string
	OpenSearch     OpenSearchConfig
	Kafka          KafkaConfig
	UserServiceGRPC string
	UsersIndexName string
	PostsIndexName string
}

type OpenSearchConfig struct {
	URL    string
	Enabled bool
}

type KafkaConfig struct {
	Brokers       []string
	TopicUsers    string
	TopicPosts    string
	ConsumerGroup string
	Enabled       bool
}

func Load() (*Config, error) {
	cfg := &Config{
		GRPCPort:        getEnv("GRPC_PORT", "50054"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		UserServiceGRPC: getEnv("USER_SERVICE_GRPC_ADDR", "user-service:50052"),
		UsersIndexName:  getEnv("OPENSEARCH_USERS_INDEX", "users"),
		PostsIndexName:  getEnv("OPENSEARCH_POSTS_INDEX", "posts"),
		OpenSearch: OpenSearchConfig{
			URL:     getEnv("OPENSEARCH_URL", "http://opensearch:9200"),
			Enabled: getEnv("OPENSEARCH_URL", "") != "",
		},
		Kafka: KafkaConfig{
			Brokers:       getEnvSlice("KAFKA_BROKERS", []string{"kafka:9092"}),
			TopicUsers:    getEnv("KAFKA_TOPIC_USERS", "search.users"),
			TopicPosts:    getEnv("KAFKA_TOPIC_POSTS", "search.posts"),
			ConsumerGroup: getEnv("KAFKA_CONSUMER_GROUP", "search-indexer"),
			Enabled:       getEnv("KAFKA_BROKERS", "") != "",
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
