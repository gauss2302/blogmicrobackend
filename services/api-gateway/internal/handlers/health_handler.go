package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"api-gateway/internal/clients"
	"api-gateway/pkg/logger"
	"api-gateway/pkg/utils"
)

type HealthHandler struct {
	authClient         *clients.AuthClient
	userClient         *clients.UserClient
	postClient 			*clients.PostClient
	logger             *logger.Logger
}

func NewHealthHandler(authClient *clients.AuthClient, userClient *clients.UserClient,postClient *clients.PostClient, logger *logger.Logger) *HealthHandler {
	return &HealthHandler{
		authClient:         authClient,
		userClient:         userClient,
		postClient: 			postClient,
		logger:             logger,
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
	if err := h.authClient.HealthCheck(); err != nil {
		services["auth-service"] = "unhealthy"
		h.logger.Warn("Auth service health check failed: " + err.Error())
	}

	// Check user service
	if err := h.userClient.HealthCheck(); err != nil {
		services["user-service"] = "unhealthy"
		h.logger.Warn("User service health check failed: " + err.Error())
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