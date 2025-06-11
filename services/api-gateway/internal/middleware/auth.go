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
		tokenResp, err := authClient.ValidateToken(c.Request.Context(), tokenString)
		if err != nil {
			utils.ErrorResponse(c, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid or expired token")
			c.Abort()
			return
		}

		if !tokenResp.Valid {
			utils.ErrorResponse(c, http.StatusUnauthorized, "INVALID_TOKEN", "Token validation failed")
			c.Abort()
			return
		}

		// Set user information in context
		c.Set("userID", tokenResp.UserID)
		c.Set("userEmail", tokenResp.Email)
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
		tokenResp, err := authClient.ValidateToken(c.Request.Context(), tokenString)
		if err == nil && tokenResp.Valid {
			c.Set("userID", tokenResp.UserID)
			c.Set("userEmail", tokenResp.Email)
			c.Set("token", tokenString)
		}
		
		c.Next()
	}
}
