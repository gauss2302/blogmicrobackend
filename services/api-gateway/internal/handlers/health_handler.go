package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"api-gateway/internal/clients"
	"api-gateway/pkg/logger"
	"api-gateway/pkg/utils"
)

const (
	// healthDeadline bounds one round of dependency checks, which run in parallel.
	healthDeadline = 2 * time.Second
	// healthCacheTTL caps backend load: however often /health is hit (directly
	// or anonymously through the frontend BFF), backends see one round per TTL.
	healthCacheTTL = 5 * time.Second
)

type healthCheck struct {
	name string
	// critical dependencies make the gateway unhealthy (503); the others only
	// degrade it (200).
	critical bool
	check    func(ctx context.Context) error
}

type HealthHandler struct {
	checks   []healthCheck
	deadline time.Duration
	ttl      time.Duration
	logger   *logger.Logger

	// ponytail: one lock serialises refreshes; callers wait at most one deadline.
	mu       sync.Mutex
	cachedAt time.Time
	code     int
	body     gin.H
}

func NewHealthHandler(authClient *clients.AuthClient, userClient *clients.UserClient, postClient *clients.PostClient, notificationURL string, logger *logger.Logger) *HealthHandler {
	notificationURL = strings.TrimSuffix(strings.TrimSpace(notificationURL), "/")
	httpClient := &http.Client{Timeout: 3 * time.Second}
	return newHealthHandler([]healthCheck{
		{name: "auth-service", critical: true, check: authClient.HealthCheck},
		{name: "user-service", critical: true, check: userClient.HealthCheck},
		{name: "post-service", critical: true, check: postClient.HealthCheck},
		{name: "notification-service", check: func(ctx context.Context) error {
			return checkHTTPHealth(ctx, httpClient, notificationURL)
		}},
	}, logger)
}

func newHealthHandler(checks []healthCheck, logger *logger.Logger) *HealthHandler {
	return &HealthHandler{checks: checks, deadline: healthDeadline, ttl: healthCacheTTL, logger: logger}
}

// HealthCheck answers healthy (200), degraded (200: only a non-critical
// dependency is down) or unhealthy (503: a critical dependency is down).
func (h *HealthHandler) HealthCheck(c *gin.Context) {
	code, body := h.result()
	utils.SuccessResponse(c, code, "Health check completed", body)
}

func (h *HealthHandler) result() (int, gin.H) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.body != nil && time.Since(h.cachedAt) < h.ttl {
		return h.code, h.body
	}

	// Not the request context: the result is shared, and one client going
	// away must not cancel the round for everyone else.
	ctx, cancel := context.WithTimeout(context.Background(), h.deadline)
	defer cancel()

	type outcome struct {
		i   int
		err error
	}
	results := make(chan outcome, len(h.checks))
	for i, chk := range h.checks {
		go func() { results <- outcome{i, chk.check(ctx)} }()
	}

	states := make([]string, len(h.checks))
wait:
	for range h.checks {
		select {
		case r := <-results:
			states[r.i] = "healthy"
			if r.err != nil {
				states[r.i] = "unhealthy"
				h.logger.Warn(h.checks[r.i].name + " health check failed: " + r.err.Error())
			}
		case <-ctx.Done():
			break wait
		}
	}

	services := make(map[string]string, len(h.checks))
	criticalDown, optionalDown := false, false
	for i, chk := range h.checks {
		if states[i] == "" {
			states[i] = "unhealthy"
			h.logger.Warn(chk.name + " health check timed out")
		}
		services[chk.name] = states[i]
		if states[i] != "healthy" {
			if chk.critical {
				criticalDown = true
			} else {
				optionalDown = true
			}
		}
	}

	overall, code := "healthy", http.StatusOK
	switch {
	case criticalDown:
		overall, code = "unhealthy", http.StatusServiceUnavailable
	case optionalDown:
		overall = "degraded"
	}

	h.code = code
	h.body = gin.H{"status": overall, "service": "api-gateway", "services": services}
	h.cachedAt = time.Now()
	return h.code, h.body
}

func checkHTTPHealth(ctx context.Context, client *http.Client, baseURL string) error {
	if baseURL == "" {
		return fmt.Errorf("service URL is not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
