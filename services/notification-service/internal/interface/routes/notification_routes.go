package routes

import (
	"github.com/gin-gonic/gin"
	"notification-service/internal/application/services"
	"notification-service/internal/interface/http/handler"
	"notification-service/internal/interface/http/middleware"
	"notification-service/pkg/auth"
	"notification-service/pkg/logger"
)

func SetupNotificationRoutes(router *gin.Engine, notificationService *services.NotificationService, validator *auth.Validator, trustMode string, logger *logger.Logger) {
	notificationHandler := handler.NewNotificationHandler(notificationService, logger)

	// Global Middleware
	router.Use(middleware.ErrorHandler(logger))
	router.Use(middleware.RequestLogger(logger))
	router.Use(middleware.CORS())

	router.GET("/health", notificationHandler.HealthCheck)

	v1 := router.Group("/api/v1")

	{
		notifications := v1.Group("/notifications")
		{
			protected := notifications.Group("")
			protected.Use(middleware.AuthMiddleware(validator, trustMode, logger))
			{
				protected.POST("", notificationHandler.CreateNotification)
				protected.GET("", notificationHandler.ListNotifications)
				protected.GET("/unread-count", notificationHandler.GetUnreadCount)
				protected.GET("/:id", notificationHandler.GetNotification)
				protected.PUT("/mark-read", notificationHandler.MarkAsRead)
				protected.DELETE("/:id", notificationHandler.DeleteNotification)
			}
		}
	}
}
