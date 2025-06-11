package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"api-gateway/pkg/utils"

	"github.com/gin-gonic/gin"
)

func RequestValidator() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "PATCH" {
			if c.GetHeader("Content-Type") == "application/json" {
				body, err := io.ReadAll(c.Request.Body)
				if err != nil {
					utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Invalid request body")
					c.Abort()
					return
				}

				// Validate JSON format
				var jsonData interface{}
				if err := json.Unmarshal(body, &jsonData); err != nil {
					utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON format")
					c.Abort()
					return
				}

				// Restore the body for downstream handlers
				c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
			}
		}
		c.Next()
	}
}