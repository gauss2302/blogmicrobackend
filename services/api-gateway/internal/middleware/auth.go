// internal/middleware/auth.go
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"api-gateway/internal/clients"
	"api-gateway/pkg/utils"
)

func AuthMiddleware(authClient *clients.AuthClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			utils.ErrorResponse(c, http.StatusUnauthorized, "MISSING_TOKEN", "Authorization header required")
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			utils.ErrorResponse(c, http.StatusUnauthorized, "INVALID_TOKEN_FORMAT", "Invalid authorization header format")
			c.Abort()
			return
		}

		// Validate token with Auth Service
		resp, err := authClient.ValidateToken(c.Request.Context(), tokenString)
		if err != nil {
			statusCode := http.StatusUnauthorized
			if !clients.IsUnauthenticatedError(err) {
				statusCode = http.StatusInternalServerError
			}
			utils.ErrorResponse(c, statusCode, "INVALID_TOKEN", "Token validation failed")
			c.Abort()
			return
		}

		if !resp.GetValid() {
			utils.ErrorResponse(c, http.StatusUnauthorized, "INVALID_TOKEN", "Token validation failed")
			c.Abort()
			return
		}

		// Set user information in context
		c.Set("userID", resp.GetUserId())
		c.Set("userEmail", resp.GetEmail())
		c.Set("token", tokenString)
		c.Next()
	}
}

func OptionalAuthMiddleware(authClient *clients.AuthClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.Next()
			return
		}

		// Try to validate token, but don't fail if invalid
		resp, err := authClient.ValidateToken(c.Request.Context(), tokenString)
		if err == nil && resp.GetValid() {
			c.Set("userID", resp.GetUserId())
			c.Set("userEmail", resp.GetEmail())
			c.Set("token", tokenString)
		}

		c.Next()
	}
}
