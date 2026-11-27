package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"notification-service/pkg/logger"
)

func TestCreateNotificationBodyLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewNotificationHandler(nil, logger.New("fatal")) // the service is never reached
	r := gin.New()
	r.POST("/api/v1/notifications", func(c *gin.Context) { c.Set("userID", "u1") }, h.CreateNotification)

	tests := []struct {
		name string
		body string
		want int
	}{
		{"over 64 KiB", `{"type":"post_created","title":"t","message":"` + strings.Repeat("a", maxCreateBodyBytes) + `"}`, http.StatusRequestEntityTooLarge},
		{"malformed JSON", `{"type":`, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			if w.Code != tt.want {
				t.Errorf("status = %d, want %d (body %s)", w.Code, tt.want, w.Body.String())
			}
		})
	}
}
