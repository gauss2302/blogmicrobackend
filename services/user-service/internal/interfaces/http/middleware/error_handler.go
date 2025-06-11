package middleware

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"user-service/internal/application/errors"
	"user-service/pkg/logger"
	"user-service/pkg/utils"
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