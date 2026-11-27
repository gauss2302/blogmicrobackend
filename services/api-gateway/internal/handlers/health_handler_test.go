package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type healthBody struct {
	Data struct {
		Status   string            `json:"status"`
		Services map[string]string `json:"services"`
	} `json:"data"`
}

func serveHealth(t *testing.T, h *HealthHandler) (int, healthBody) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", h.HealthCheck)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	var body healthBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", w.Body.String(), err)
	}
	return w.Code, body
}

func ok(context.Context) error   { return nil }
func down(context.Context) error { return errors.New("down") }

func fourChecks(auth, user, post, notification func(context.Context) error) []healthCheck {
	return []healthCheck{
		{name: "auth-service", critical: true, check: auth},
		{name: "user-service", critical: true, check: user},
		{name: "post-service", critical: true, check: post},
		{name: "notification-service", check: notification},
	}
}

func TestHealthStatuses(t *testing.T) {
	cases := []struct {
		name       string
		checks     []healthCheck
		wantCode   int
		wantStatus string
		wantDown   string
	}{
		{"all up", fourChecks(ok, ok, ok, ok), http.StatusOK, "healthy", ""},
		{"notification down", fourChecks(ok, ok, ok, down), http.StatusOK, "degraded", "notification-service"},
		{"post down", fourChecks(ok, ok, down, ok), http.StatusServiceUnavailable, "unhealthy", "post-service"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, body := serveHealth(t, newHealthHandler(tc.checks, testLogger))
			if code != tc.wantCode || body.Data.Status != tc.wantStatus {
				t.Fatalf("got %d %q, want %d %q", code, body.Data.Status, tc.wantCode, tc.wantStatus)
			}
			for name, state := range body.Data.Services {
				want := "healthy"
				if name == tc.wantDown {
					want = "unhealthy"
				}
				if state != want {
					t.Fatalf("%s = %q, want %q (%v)", name, state, want, body.Data.Services)
				}
			}
		})
	}
}

func TestHealthHungCheckDoesNotOutlastDeadline(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	// Ignores its context on purpose: the handler must not rely on checks
	// honouring the deadline.
	hung := func(context.Context) error { <-release; return nil }

	h := newHealthHandler(fourChecks(ok, hung, ok, ok), testLogger)
	h.deadline = 100 * time.Millisecond

	start := time.Now()
	code, body := serveHealth(t, h)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("answered after %v, deadline is %v", elapsed, h.deadline)
	}
	if code != http.StatusServiceUnavailable || body.Data.Services["user-service"] != "unhealthy" {
		t.Fatalf("got %d %v, want 503 with user-service unhealthy", code, body.Data.Services)
	}
}

func TestHealthConcurrentRequestsShareOneRound(t *testing.T) {
	var calls [4]atomic.Int32
	counted := func(i int) func(context.Context) error {
		return func(context.Context) error {
			calls[i].Add(1)
			time.Sleep(20 * time.Millisecond)
			return nil
		}
	}
	h := newHealthHandler(fourChecks(counted(0), counted(1), counted(2), counted(3)), testLogger)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", h.HealthCheck)
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
			if w.Code != http.StatusOK {
				t.Errorf("code = %d", w.Code)
			}
		}()
	}
	wg.Wait()

	for i := range calls {
		if n := calls[i].Load(); n != 1 {
			t.Fatalf("check %d ran %d times for 50 requests, want 1", i, n)
		}
	}
}
