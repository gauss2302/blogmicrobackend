// internal/middleware/cors.go
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"api-gateway/internal/config"
)

func CORS(cfg config.CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.Request.Header.Get("Origin"))
		allowedOrigin := resolveAllowedOrigin(origin, cfg.AllowedOrigins, cfg.AllowCredentials)
		if allowedOrigin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			c.Writer.Header().Add("Vary", "Origin")
		}
		if cfg.AllowCredentials {
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if len(cfg.AllowedHeaders) > 0 {
			c.Writer.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
		}
		if len(cfg.AllowedMethods) > 0 {
			c.Writer.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
		}
		if len(cfg.ExposeHeaders) > 0 {
			c.Writer.Header().Set("Access-Control-Expose-Headers", strings.Join(cfg.ExposeHeaders, ", "))
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func resolveAllowedOrigin(origin string, allowedOrigins []string, allowCredentials bool) string {
	if len(allowedOrigins) == 0 {
		return ""
	}

	if hasWildcard(allowedOrigins) {
		if allowCredentials && origin != "" {
			return origin
		}
		return "*"
	}

	if origin != "" && containsIgnoreCase(allowedOrigins, origin) {
		return origin
	}

	if origin == "" {
		return allowedOrigins[0]
	}

	return ""
}

func hasWildcard(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "*" {
			return true
		}
	}
	return false
}

func containsIgnoreCase(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}
