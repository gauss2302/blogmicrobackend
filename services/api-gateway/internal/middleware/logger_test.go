package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"api-gateway/pkg/logger"
)

func TestRequestLoggerOmitsQueryString(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	l := logger.New("info")
	l.SetOutput(&buf)

	r := gin.New()
	r.Use(RequestLogger(l))
	r.GET("/api/v1/auth/google/callback", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback?code=secret&state=secret", nil))

	line := buf.String()
	if !strings.Contains(line, "/api/v1/auth/google/callback") {
		t.Fatalf("path missing from log line %q", line)
	}
	if strings.Contains(line, "secret") {
		t.Fatalf("query string leaked into log line %q", line)
	}
}
