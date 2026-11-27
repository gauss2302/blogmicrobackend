package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"api-gateway/internal/config"
)

// A-71: the public router keeps /health but no longer serves /metrics.
func TestPublicRouterHasNoMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetupRoutes(router, nil, nil, nil, nil, nil, nil, nil, &config.Config{RequestMaxBodyBytes: 1 << 20})

	health := false
	for _, r := range router.Routes() {
		if r.Path == "/metrics" {
			t.Fatalf("public router registers %s /metrics", r.Method)
		}
		health = health || (r.Method == http.MethodGet && r.Path == "/health")
	}
	if !health {
		t.Fatal("GET /health is not registered")
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("GET /metrics = %d, want 404", w.Code)
	}
}
