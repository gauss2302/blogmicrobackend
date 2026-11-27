package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	GRPCPort                 string
	MetricsHTTPPort          string
	Environment              string
	LogLevel                 string
	OpenSearch               OpenSearchConfig
	Kafka                    KafkaConfig
	UserServiceGRPC          string
	UsersIndexName           string
	PostsIndexName           string
	GRPCTLS                  GRPCTLSConfig
	ServiceTransportSecurity string
	EnableGRPCReflection     bool
}

type OpenSearchConfig struct {
	URL      string
	Username string
	Password string
	Enabled  bool
}

type KafkaConfig struct {
	Brokers        []string
	TopicUsers     string
	TopicPosts     string
	DLQTopic       string
	ConsumerGroup  string
	SASLUsername   string
	SASLPassword   string
	RetryBackoffMS int
	Enabled        bool
}

type GRPCTLSConfig struct {
	Enabled           bool
	CAFile            string
	CertFile          string
	KeyFile           string
	RequireClientCert bool
}

func Load() (*Config, error) {
	environment := normalizeEnvironment(getEnv("ENVIRONMENT", "development"))

	cfg := &Config{
		GRPCPort:        getEnv("GRPC_PORT", "50054"),
		MetricsHTTPPort: getEnv("METRICS_HTTP_PORT", "9095"),
		Environment:     environment,
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		UserServiceGRPC: getEnv("USER_SERVICE_GRPC_ADDR", "user-service:50052"),
		UsersIndexName:  getEnv("OPENSEARCH_USERS_INDEX", "users"),
		PostsIndexName:  getEnv("OPENSEARCH_POSTS_INDEX", "posts"),
		OpenSearch: OpenSearchConfig{
			URL:      getEnv("OPENSEARCH_URL", "http://opensearch:9200"),
			Username: os.Getenv("OPENSEARCH_USERNAME"),
			Password: os.Getenv("OPENSEARCH_PASSWORD"),
			Enabled:  getEnv("OPENSEARCH_URL", "") != "",
		},
		Kafka: KafkaConfig{
			Brokers:        getEnvSlice("KAFKA_BROKERS", []string{"kafka:9092"}),
			TopicUsers:     getEnv("KAFKA_TOPIC_USERS", "search.users"),
			TopicPosts:     getEnv("KAFKA_TOPIC_POSTS", "search.posts"),
			DLQTopic:       getEnv("KAFKA_DLQ_TOPIC", "search.dlq"),
			ConsumerGroup:  getEnv("KAFKA_CONSUMER_GROUP", "search-indexer"),
			SASLUsername:   os.Getenv("KAFKA_SASL_USERNAME"),
			SASLPassword:   os.Getenv("KAFKA_SASL_PASSWORD"),
			RetryBackoffMS: getEnvAsInt("KAFKA_RETRY_BACKOFF_MS", 500),
			Enabled:        getEnv("KAFKA_BROKERS", "") != "",
		},
		GRPCTLS: GRPCTLSConfig{
			Enabled:           getEnvAsBool("GRPC_TLS_ENABLED", false),
			CAFile:            getEnv("GRPC_TLS_CA_FILE", ""),
			CertFile:          getEnv("GRPC_TLS_CERT_FILE", ""),
			KeyFile:           getEnv("GRPC_TLS_KEY_FILE", ""),
			RequireClientCert: getEnvAsBool("GRPC_TLS_REQUIRE_CLIENT_CERT", false),
		},
		ServiceTransportSecurity: resolveTransportSecurityMode(getEnv("SERVICE_TRANSPORT_SECURITY", ""), environment, getEnvAsBool("GRPC_TLS_ENABLED", false)),
		EnableGRPCReflection:     getEnvAsBool("GRPC_REFLECTION_ENABLED", environment != "production"),
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
	if err := validateEnvironment(c.Environment); err != nil {
		return err
	}
	if c.GRPCPort == "" {
		return fmt.Errorf("GRPC_PORT is required")
	}
	if c.MetricsHTTPPort == "" {
		return fmt.Errorf("METRICS_HTTP_PORT is required")
	}
	if c.MetricsHTTPPort == c.GRPCPort {
		return fmt.Errorf("METRICS_HTTP_PORT must differ from GRPC_PORT")
	}
	if c.OpenSearch.Enabled && (c.OpenSearch.Username == "" || c.OpenSearch.Password == "") {
		return fmt.Errorf("OPENSEARCH_USERNAME and OPENSEARCH_PASSWORD are required when OpenSearch is enabled")
	}
	if c.Kafka.Enabled && (c.Kafka.SASLUsername == "" || c.Kafka.SASLPassword == "") {
		return fmt.Errorf("KAFKA_SASL_USERNAME and KAFKA_SASL_PASSWORD are required when Kafka is enabled")
	}
	if c.Environment == "production" {
		if err := rejectTemplateSecret("OPENSEARCH_PASSWORD", c.OpenSearch.Password); err != nil {
			return err
		}
		if err := rejectTemplateSecret("KAFKA_SASL_PASSWORD", c.Kafka.SASLPassword); err != nil {
			return err
		}
	}
	if c.Kafka.Enabled && len(c.Kafka.Brokers) == 0 {
		return fmt.Errorf("KAFKA_BROKERS is required when Kafka is enabled")
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
	if err := validateTransportSecurityMode(c.Environment, c.ServiceTransportSecurity, c.GRPCTLS.Enabled); err != nil {
		return err
	}
	if c.Environment == "production" && c.EnableGRPCReflection {
		return fmt.Errorf("GRPC_REFLECTION_ENABLED cannot be true in production")
	}
	return nil
}

func rejectTemplateSecret(name, value string) error {
	folded := strings.ToLower(value)
	if strings.Contains(folded, "replace-with") || strings.Contains(folded, "change-me") ||
		(strings.Contains(folded, "<") && strings.Contains(folded, ">")) {
		return fmt.Errorf("%s is still a template placeholder", name)
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

func resolveTransportSecurityMode(value, environment string, grpcTLSEnabled bool) string {
	mode := strings.ToLower(strings.TrimSpace(value))
	if mode != "" {
		return mode
	}
	if environment == "production" {
		return ""
	}
	if grpcTLSEnabled {
		return "app_mtls"
	}
	return "insecure_dev"
}

func validateTransportSecurityMode(environment, mode string, grpcTLSEnabled bool) error {
	switch mode {
	case "mesh":
		return nil
	case "app_mtls":
		if !grpcTLSEnabled {
			return fmt.Errorf("GRPC_TLS_ENABLED=true is required when SERVICE_TRANSPORT_SECURITY=app_mtls")
		}
		return nil
	case "insecure_dev":
		if environment == "production" {
			return fmt.Errorf("SERVICE_TRANSPORT_SECURITY=insecure_dev is not allowed in production")
		}
		return nil
	case "":
		if environment == "production" {
			return fmt.Errorf("SERVICE_TRANSPORT_SECURITY is required in production")
		}
		return nil
	default:
		return fmt.Errorf("SERVICE_TRANSPORT_SECURITY must be one of mesh, app_mtls, insecure_dev")
	}
}
