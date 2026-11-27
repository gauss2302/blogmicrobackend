package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strings"
	"unicode/utf8"

	"api-gateway/pkg/utils"

	"github.com/gin-gonic/gin"
)

// RequestValidator caps every request body at maxBodyBytes and requires
// POST/PUT/PATCH bodies to be valid application/json. Handlers bind with
// ShouldBindJSON, which ignores Content-Type, so the type is enforced here
// before the body is read (this also blocks text/plain form login-CSRF).
//
// Every request, any method: the path, the query and the strings of a JSON
// body must be valid UTF-8 without NUL. Postgres rejects NUL in text and a
// proto3 string cannot carry invalid UTF-8, so either would otherwise surface
// as a 5xx from the service behind (A-80).
func RequestValidator(maxBodyBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cleanText(c.Request.URL.Path) || !cleanQuery(c.Request.URL.Query()) {
			utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_INPUT", "Request contains NUL or invalid UTF-8")
			c.Abort()
			return
		}

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

		// encoding/json would quietly turn invalid UTF-8 into U+FFFD.
		var doc any
		if !utf8.Valid(body) || json.Unmarshal(body, &doc) != nil {
			utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON format")
			c.Abort()
			return
		}
		if !cleanJSON(doc) {
			utils.ErrorResponse(c, http.StatusBadRequest, "INVALID_INPUT", "Request contains NUL or invalid UTF-8")
			c.Abort()
			return
		}

		// Restore the body for downstream handlers
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Next()
	}
}

func cleanText(s string) bool {
	return utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}

func cleanQuery(q map[string][]string) bool {
	for k, vs := range q {
		if !cleanText(k) {
			return false
		}
		for _, v := range vs {
			if !cleanText(v) {
				return false
			}
		}
	}
	return true
}

// cleanJSON reports whether no string in a decoded JSON value, keys included,
// contains NUL (a \u0000 escape). UTF-8 is checked on the raw body.
func cleanJSON(v any) bool {
	switch v := v.(type) {
	case string:
		return !strings.ContainsRune(v, 0)
	case []any:
		for _, e := range v {
			if !cleanJSON(e) {
				return false
			}
		}
	case map[string]any:
		for k, e := range v {
			if strings.ContainsRune(k, 0) || !cleanJSON(e) {
				return false
			}
		}
	}
	return true
}
