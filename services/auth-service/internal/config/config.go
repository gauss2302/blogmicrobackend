package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port     string
	LogLevel string
	Redis    RedisConfig
	Google   GoogleConfig
	JWT      JWTConfig
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

func LoadConfig() (*Config, error) {
	cfg := &Config{
		Port:     getEnv("PORT", "8081"),
		LogLevel: getEnv("LOG_LEVEL", "info"),
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
			Issuer:          getEnv("JWT_ISSUER", "auth-service"),
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
