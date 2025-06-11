package utils

import (
	"auth-service/internal/application/errors"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorData  `json:"error,omitempty"`
}

type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func SuccessResponse(c *gin.Context, statusCode int, message string, data interface{}) {
	c.JSON(statusCode, Response{
		Success: true,
		Message: message,
		Data:    data,
	})
}

func ErrorResponse(c *gin.Context, err *errors.AuthError) {
	c.JSON(err.StatusCode, Response{
		Success: false,
		Message: "Request failed",
		Error: &ErrorData{
			Code:    err.Code,
			Message: err.Message,
		},
	})
}