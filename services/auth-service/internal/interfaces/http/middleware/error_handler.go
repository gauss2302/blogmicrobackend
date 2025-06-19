package middleware

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"auth-service/internal/application/errors"
	"auth-service/pkg/logger"
	"auth-service/pkg/utils"
)

func ErrorHandler(logger *logger.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		if err, ok := recovered.(string); ok {
			logger.Error("Panic recovered: " + err)
		} else if err, ok := recovered.(error); ok {
			logger.Error("Panic recovered: " + err.Error())
		} else {
			logger.Error("Unknown panic recovered")
		}

		utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		c.Abort()
	})
}

// ✅ FIXED: Request logger with proper status code formatting
func RequestLogger(logger *logger.Logger) gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		// Fix the status code display
		statusStr := strconv.Itoa(param.StatusCode)

		logMsg := fmt.Sprintf("Request: %s %s | Status: %s | Latency: %s | IP: %s",
			param.Method,
			param.Path,
			statusStr,
			param.Latency.String(),
			param.ClientIP,
		)

		logger.Info(logMsg)
		return ""
	})
}

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
