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
	Database                 DatabaseConfig
	RabbitMQ                 RabbitMQConfig
	GRPCTLS                  GRPCTLSConfig
	ServiceTransportSecurity string
	InternalHTTPTrustMode    string
	EnableGRPCReflection     bool
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
	cfg := &Config{
		Port:        getEnv("PORT", "8083"),
		GRPCPort:    getEnv("GRPC_PORT", "50053"),
		Environment: getEnv("ENVIRONMENT", "development"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		Database: DatabaseConfig{
			URL:             os.Getenv("DATABASE_URL"),
			MaxOpenConns:    getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvAsInt("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: getEnvAsInt("DB_CONN_MAX_LIFETIME", 60),
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
		ServiceTransportSecurity: resolveTransportSecurityMode(getEnv("SERVICE_TRANSPORT_SECURITY", ""), getEnv("ENVIRONMENT", "development"), getEnvAsBool("GRPC_TLS_ENABLED", false)),
		InternalHTTPTrustMode:    resolveInternalHTTPTrustMode(getEnv("INTERNAL_HTTP_TRUST_MODE", ""), getEnv("ENVIRONMENT", "development")),
		EnableGRPCReflection:     getEnvAsBool("GRPC_REFLECTION_ENABLED", getEnv("ENVIRONMENT", "development") != "production"),
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
	if c.Environment == "production" && c.EnableGRPCReflection {
		return fmt.Errorf("GRPC_REFLECTION_ENABLED cannot be true in production")
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
