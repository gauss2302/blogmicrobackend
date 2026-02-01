package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port     string
	GRPCPort string
	LogLevel string
	Server   ServerConfig
	Redis    RedisConfig
	Google   GoogleConfig
	JWT      JWTConfig
	Services ServicesConfig
}

type ServicesConfig struct {
	UserGRPCAddr string
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
	ClientID       string
	ClientSecret   string
	RedirectURL    string
	AllowedDomains []string
}

type JWTConfig struct {
	Secret          string
	AccessTokenTTL  int // minutes
	RefreshTokenTTL int // hours
	Issuer          string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:     getEnv("PORT", "8081"),
		GRPCPort: getEnv("GRPC_PORT", "50051"),
		LogLevel: getEnv("LOG_LEVEL", "info"),
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
			ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
			ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
			RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		},
		JWT: JWTConfig{
			Secret:          os.Getenv("JWT_SECRET"),
			AccessTokenTTL:  getEnvAsInt("JWT_ACCESS_TTL", 15),   // 15 minutes
			RefreshTokenTTL: getEnvAsInt("JWT_REFRESH_TTL", 168), // 7 days
		},
		Services: ServicesConfig{
			UserGRPCAddr: getEnv("USER_SERVICE_GRPC_ADDR", "localhost:50052"),
		},
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
	if c.JWT.Secret == "" || len(c.JWT.Secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters")
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
