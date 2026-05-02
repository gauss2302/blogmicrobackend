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
	Server                   ServerConfig
	Redis                    RedisConfig
	Google                   GoogleConfig
	JWT                      JWTConfig
	Services                 ServicesConfig
	GRPCTLS                  GRPCTLSConfig
	ServiceTransportSecurity string
	InternalHTTPTrustMode    string
	EnableGRPCReflection     bool
}

type ServicesConfig struct {
	UserGRPCAddr string
}

type GRPCTLSConfig struct {
	Enabled           bool
	CAFile            string
	CertFile          string
	KeyFile           string
	RequireClientCert bool
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

type GoogleConfig struct {
	ClientID                  string
	ClientSecret              string
	RedirectURL               string
	DefaultWebRedirectURI     string
	AllowedWebRedirectURIs    []string
	AllowedMobileRedirectURIs []string
	AllowedDomains            []string
}

type JWTConfig struct {
	Secret          string
	AccessTokenTTL  int // minutes
	RefreshTokenTTL int // hours
	Issuer          string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:        getEnv("PORT", "8081"),
		GRPCPort:    getEnv("GRPC_PORT", "50051"),
		Environment: getEnv("ENVIRONMENT", "development"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		Server: ServerConfig{
			ReadTimeout:  getEnvAsInt("SERVER_READ_TIMEOUT", 10),
			WriteTimeout: getEnvAsInt("SERVER_WRITE_TIMEOUT", 10),
			IdleTimeout:  getEnvAsInt("SERVER_IDLE_TIMEOUT", 60),
		},
		Redis: RedisConfig{
			URL:      getEnv("REDIS_URL", "redis:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},
		Google: GoogleConfig{
			ClientID:                  os.Getenv("GOOGLE_CLIENT_ID"),
			ClientSecret:              os.Getenv("GOOGLE_CLIENT_SECRET"),
			RedirectURL:               os.Getenv("GOOGLE_REDIRECT_URL"),
			DefaultWebRedirectURI:     getEnv("GOOGLE_DEFAULT_WEB_REDIRECT_URI", getEnv("FRONTEND_URL", "http://localhost:3000")+"/auth/callback"),
			AllowedWebRedirectURIs:    parseCSV(getEnv("GOOGLE_ALLOWED_WEB_REDIRECT_URIS", "")),
			AllowedMobileRedirectURIs: parseCSV(getEnv("GOOGLE_ALLOWED_MOBILE_REDIRECT_URIS", "")),
			AllowedDomains:            parseCSV(getEnv("GOOGLE_ALLOWED_DOMAINS", "")),
		},
		JWT: JWTConfig{
			Secret:          os.Getenv("JWT_SECRET"),
			AccessTokenTTL:  getEnvAsInt("JWT_ACCESS_TTL", 15),   // 15 minutes
			RefreshTokenTTL: getEnvAsInt("JWT_REFRESH_TTL", 168), // 7 days
			Issuer:          getEnv("JWT_ISSUER", "auth-service"),
		},
		Services: ServicesConfig{
			UserGRPCAddr: getEnv("USER_SERVICE_GRPC_ADDR", "localhost:50052"),
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

func (c *Config) validate() error {
	if c.Google.ClientID == "" {
		return fmt.Errorf("GOOGLE_CLIENT_ID is required")
	}
	if c.Google.ClientSecret == "" {
		return fmt.Errorf("GOOGLE_CLIENT_SECRET is required")
	}
	if c.Google.RedirectURL == "" {
		return fmt.Errorf("GOOGLE_REDIRECT_URL is required")
	}
	if c.Google.DefaultWebRedirectURI == "" {
		return fmt.Errorf("GOOGLE_DEFAULT_WEB_REDIRECT_URI is required")
	}
	if len(c.Google.AllowedWebRedirectURIs) == 0 {
		c.Google.AllowedWebRedirectURIs = []string{c.Google.DefaultWebRedirectURI}
	}
	if c.JWT.Secret == "" || len(c.JWT.Secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	if c.Environment == "production" && strings.TrimSpace(c.Redis.Password) == "" {
		return fmt.Errorf("REDIS_PASSWORD is required in production")
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

	//// Validate redirect URL
	//if !isValidRedirectURL(c.Google.RedirectURL) {
	//	return fmt.Errorf("invalid redirect URL")
	//}

	return nil
}

//func isValidRedirectURL(url string) bool {
//	allowedHosts := []string{"localhost", "your-domain.com"}
//
//	return true
//}

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

func parseCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			result = append(result, item)
		}
	}

	return result
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
