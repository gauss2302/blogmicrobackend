package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeaders sets security-related HTTP headers (HTTPS/token safety recommendations).
func SecurityHeaders(environment string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		if environment == "production" {
			// Enforce HTTPS; 1 year max-age for HSTS (tune as needed).
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}
		c.Next()
	}
}
