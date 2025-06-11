package middleware

import (
	"fmt"

	"api-gateway/pkg/logger"

	"github.com/gin-gonic/gin"
)

func RequestLogger(logger *logger.Logger) gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		log := fmt.Sprintf("[%s] %s %s %d %s %s %s",
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			param.Method,
			param.Path,
			param.StatusCode,
			param.Latency,
			param.ClientIP,
			param.ErrorMessage,
		)
		
		logger.Info(log)
		return ""
	})
}