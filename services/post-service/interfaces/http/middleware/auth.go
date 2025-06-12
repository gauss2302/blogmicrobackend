package middleware

import (
	"net/http"

	"post-service/internal/application/errors"
	"post-service/pkg/utils"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from header (set by API Gateway)
		userID := c.GetHeader("X-User-ID")
		if userID == "" {
			utils.ErrorResponse(c, errors.ErrUnauthorizedAccess)
			c.Abort()
			return
		}

		// Set user ID in context for handlers to use
		c.Set("userID", userID)
		c.Next()
	}
}

func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from header (optional)
		userID := c.GetHeader("X-User-ID")
		if userID != "" {
			c.Set("userID", userID)
		}
		c.Next()
	}
}

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-User-ID")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}