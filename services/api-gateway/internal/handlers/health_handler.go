package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"api-gateway/internal/clients"
	"api-gateway/pkg/logger"
	"api-gateway/pkg/utils"
)

type HealthHandler struct {
	authClient      *clients.AuthClient
	userClient      *clients.UserClient
	postClient      *clients.PostClient
	notificationURL string
	healthClient    *http.Client
	logger          *logger.Logger
}

func NewHealthHandler(authClient *clients.AuthClient, userClient *clients.UserClient, postClient *clients.PostClient, notificationURL string, logger *logger.Logger) *HealthHandler {
	return &HealthHandler{
		authClient:      authClient,
		userClient:      userClient,
		postClient:      postClient,
		notificationURL: strings.TrimSuffix(strings.TrimSpace(notificationURL), "/"),
		healthClient: &http.Client{
			Timeout: 3 * time.Second,
		},
		logger: logger,
	}
}

func (h *HealthHandler) HealthCheck(c *gin.Context) {
	services := map[string]string{
		"auth-service":         "healthy",
		"user-service":         "healthy",
		"post-service":         "healthy",
		"notification-service": "healthy",
	}

	// Check auth service
	if err := h.authClient.HealthCheck(c.Request.Context()); err != nil {
		services["auth-service"] = "unhealthy"
		h.logger.Warn("Auth service health check failed: " + err.Error())
	}

	// Check user service
	if err := h.userClient.HealthCheck(c.Request.Context()); err != nil {
		services["user-service"] = "unhealthy"
		h.logger.Warn("User service health check failed: " + err.Error())
	}

	// Check post service
	if err := h.postClient.HealthCheck(c.Request.Context()); err != nil {
		services["post-service"] = "unhealthy"
		h.logger.Warn("Post service health check failed: " + err.Error())
	}

	// Check notification service (HTTP health endpoint)
	if err := h.checkNotificationService(c.Request.Context()); err != nil {
		services["notification-service"] = "unhealthy"
		h.logger.Warn("Notification service health check failed: " + err.Error())
	}

	// Determine overall status
	overallStatus := "healthy"
	for _, status := range services {
		if status == "unhealthy" {
			overallStatus = "degraded"
			break
		}
	}

	response := gin.H{
		"status":   overallStatus,
		"service":  "api-gateway",
		"services": services,
	}

	statusCode := http.StatusOK
	if overallStatus == "degraded" {
		statusCode = http.StatusServiceUnavailable
	}

	utils.SuccessResponse(c, statusCode, "Health check completed", response)
}

func (h *HealthHandler) checkNotificationService(ctx context.Context) error {
	if h.notificationURL == "" {
		return fmt.Errorf("notification service URL is not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.notificationURL+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := h.healthClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
