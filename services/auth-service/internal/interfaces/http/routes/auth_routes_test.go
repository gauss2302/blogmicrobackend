package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"auth-service/pkg/metrics"
)

func TestOnlyHealthAndMetricsAreServed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	metrics.Init()
	router := gin.New()
	SetupAuthRoutes(router)

	do := func(method, path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader("{}")))
		return rec
	}

	if rec := do(http.MethodGet, "/health"); rec.Code != http.StatusOK {
		t.Fatalf("GET /health: status %d", rec.Code)
	}
	rec := do(http.MethodGet, "/metrics")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "microblog_http_requests_total") {
		t.Fatalf("GET /metrics: status %d, body lacks HTTP request metrics", rec.Code)
	}

	for _, r := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/auth/google"},
		{http.MethodGet, "/api/v1/auth/google/callback"},
		{http.MethodPost, "/api/v1/auth/exchange"},
		{http.MethodPost, "/api/v1/auth/refresh"},
		{http.MethodPost, "/api/v1/auth/logout"},
		{http.MethodGet, "/api/v1/auth/validate"},
	} {
		if rec := do(r.method, r.path); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s: status %d, want 404", r.method, r.path, rec.Code)
		}
	}
}
