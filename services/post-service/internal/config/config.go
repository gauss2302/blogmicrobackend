package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                     string
	GRPCPort                 string
	Environment              string
	LogLevel                 string
	JWTSecret                string
	Database                 DatabaseConfig
	Redis                    RedisConfig
	RabbitMQ                 RabbitMQConfig
	GRPCTLS                  GRPCTLSConfig
	Kafka                    KafkaConfig
	ServiceTransportSecurity string
	InternalHTTPTrustMode    string
	EnableGRPCReflection     bool
}

// KafkaConfig configures publishing post change events for search indexing.
type KafkaConfig struct {
	Brokers      []string
	TopicPosts   string
	SASLUsername string
	SASLPassword string
	Enabled      bool // true when KAFKA_BROKERS is provided
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
	URL          string
	ExchangeName string
	Enabled      bool
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
		Port:        getEnv("PORT", "8083"),
		GRPCPort:    getEnv("GRPC_PORT", "50053"),
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
			URL:          getEnv("RABBITMQ_URL", ""),
			ExchangeName: getEnv("RABBITMQ_EXCHANGE", "blog_events"),
			Enabled:      getEnv("RABBITMQ_URL", "") != "", // Enabled if URL is provided
		},
		GRPCTLS: GRPCTLSConfig{
			Enabled:           getEnvAsBool("GRPC_TLS_ENABLED", false),
			CAFile:            getEnv("GRPC_TLS_CA_FILE", ""),
			CertFile:          getEnv("GRPC_TLS_CERT_FILE", ""),
			KeyFile:           getEnv("GRPC_TLS_KEY_FILE", ""),
			RequireClientCert: getEnvAsBool("GRPC_TLS_REQUIRE_CLIENT_CERT", false),
		},
		Kafka: KafkaConfig{
			Brokers:      parseCSVEnv("KAFKA_BROKERS"),
			TopicPosts:   getEnv("KAFKA_TOPIC_POSTS", "search.posts"),
			SASLUsername: os.Getenv("KAFKA_SASL_USERNAME"),
			SASLPassword: os.Getenv("KAFKA_SASL_PASSWORD"),
			Enabled:      getEnv("KAFKA_BROKERS", "") != "",
		},
		ServiceTransportSecurity: resolveTransportSecurityMode(getEnv("SERVICE_TRANSPORT_SECURITY", ""), environment, getEnvAsBool("GRPC_TLS_ENABLED", false)),
		InternalHTTPTrustMode:    resolveInternalHTTPTrustMode(getEnv("INTERNAL_HTTP_TRUST_MODE", ""), environment),
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

func (c *Config) validate() error {
	if err := validateEnvironment(c.Environment); err != nil {
		return err
	}
	if c.Database.URL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.GRPCPort == "" {
		return fmt.Errorf("GRPC_PORT is required")
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
	if err := validateInternalHTTPTrustMode(c.Environment, c.InternalHTTPTrustMode); err != nil {
		return err
	}
	// The JWT secret must match auth-service's so access tokens presented to the
	// HTTP API can be verified. It is mandatory in every mode except the
	// explicitly insecure local-dev mode, which permits the unauthenticated
	// X-User-ID header fallback instead.
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
	if c.Kafka.Enabled && (c.Kafka.SASLUsername == "" || c.Kafka.SASLPassword == "") {
		return fmt.Errorf("KAFKA_SASL_USERNAME and KAFKA_SASL_PASSWORD are required when Kafka is enabled")
	}
	if c.Environment == "production" && c.EnableGRPCReflection {
		return fmt.Errorf("GRPC_REFLECTION_ENABLED cannot be true in production")
	}
	if c.Environment == "production" {
		if err := rejectPlaceholderSecrets(
			namedSecret{"JWT_SECRET", c.JWTSecret},
			namedSecret{"DATABASE_URL", c.Database.URL},
			namedSecret{"RABBITMQ_URL", c.RabbitMQ.URL},
			namedSecret{"REDIS_PASSWORD", c.Redis.Password},
			namedSecret{"KAFKA_SASL_PASSWORD", c.Kafka.SASLPassword},
		); err != nil {
			return err
		}
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

func parseCSVEnv(key string) []string {
	value := os.Getenv(key)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func resolveTransportSecurityMode(value, environment string, grpcTLSEnabled bool) string {
	mode := strings.ToLower(strings.TrimSpace(value))
	if mode != "" {
		return mode
	}
	if !isDevEnvironment(environment) {
		// Unknown environment: fail closed rather than implying plaintext.
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
		if !isDevEnvironment(environment) {
			return fmt.Errorf("SERVICE_TRANSPORT_SECURITY=insecure_dev is not allowed outside a development environment")
		}
		return nil
	case "":
		if !isDevEnvironment(environment) {
			return fmt.Errorf("SERVICE_TRANSPORT_SECURITY is required outside a development environment")
		}
		return nil
	default:
		return fmt.Errorf("SERVICE_TRANSPORT_SECURITY must be one of mesh, app_mtls, insecure_dev")
	}
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
