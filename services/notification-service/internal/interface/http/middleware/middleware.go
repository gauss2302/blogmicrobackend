package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"notification-service/internal/application/errors"
	"notification-service/pkg/logger"
	"notification-service/pkg/utils"
	"strconv"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-ID")
		if userID == "" {
			utils.ErrorResponse(c, errors.ErrUnauthorizedAccess)
			c.Abort()
			return
		}

		// Set user id in ctx for handlers
		c.Set("userID", userID)
		c.Next()
	}
}

func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
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

func ErrorHandler(logger *logger.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		if err, ok := recovered.(string); ok {
			logger.Error("panic recovered: " + err)
		} else if err, ok := recovered.(error); ok {
			logger.Error("panic recovered: " + err.Error())
		} else {
			logger.Error("unknown panic recovered")
		}

		utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		c.Abort()
	})
}

func RequestLogger(logger *logger.Logger) gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		logger.Info(
			"Request: " + param.Method + " " + param.Path +
				" | Status: " + strconv.Itoa(param.StatusCode) +
				" | Latency: " + param.Latency.String(),
		)
		return ""
	})
}
