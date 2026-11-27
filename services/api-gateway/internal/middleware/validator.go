package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net/http"

	"api-gateway/pkg/utils"

	"github.com/gin-gonic/gin"
)

// RequestValidator caps every request body at maxBodyBytes and requires
// POST/PUT/PATCH bodies to be valid application/json. Handlers bind with
// ShouldBindJSON, which ignores Content-Type, so the type is enforced here
// before the body is read (this also blocks text/plain form login-CSRF).
func RequestValidator(maxBodyBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		// net/http sets ContentLength to 0 only when the request carries no
		// body; chunked or unknown-length bodies are -1.
		if c.Request.ContentLength == 0 {
			c.Next()
			return
		}

		if maxBodyBytes > 0 {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes)
		}

		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
		default:
			c.Next()
			return
		}

		if mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type")); err != nil || mediaType != "application/json" {
			utils.ErrorResponse(c, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json")
			c.Abort()
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			if _, ok := err.(*http.MaxBytesError); ok {
				utils.ErrorResponse(c, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body is too large")
				c.Abort()
				return
			}
			utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Invalid request body")
			c.Abort()
			return
		}

		if !json.Valid(body) {
			utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON format")
			c.Abort()
			return
		}

		// Restore the body for downstream handlers
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Next()
	}
}
