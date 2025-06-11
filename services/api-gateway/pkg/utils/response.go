package utils

import (
	"api-gateway/internal/models"

	"github.com/gin-gonic/gin"
)

func SuccessResponse(c *gin.Context, statusCode int, message string, data interface{}) {
	c.JSON(statusCode, models.APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

func ErrorResponse(c *gin.Context, statusCode int, code, message string) {
	c.JSON(statusCode, models.APIResponse{
		Success: false,
		Message: "Request failed",
		Error: &models.ErrorData{
			Code:    code,
			Message: message,
		},
	})
}