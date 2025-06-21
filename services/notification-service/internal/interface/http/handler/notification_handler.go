package handler

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"notification-service/internal/application/dto"
	"notification-service/internal/application/errors"
	"notification-service/internal/application/services"
	"notification-service/internal/interface/validators"
	"notification-service/pkg/logger"
	"notification-service/pkg/utils"
)

type NotificationHandler struct {
	notificationService *services.NotificationService
	validator           *validators.NotificationValidator
	logger              *logger.Logger
}

func NewNotificationHandler(notificationService *services.NotificationService, logger *logger.Logger) *NotificationHandler {
	return &NotificationHandler{
		notificationService: notificationService,
		validator:           validators.NewNotificationValidator(),
		logger:              logger,
	}
}

func (h *NotificationHandler) CreateNotification(c *gin.Context) {
	var req dto.CreateNotificationRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid create notif req: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	if err := h.validator.ValidateCreateNotificationRequest(&req); err != nil {
		h.logger.Warn("create notif validation failed " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidNotificationData)
		return
	}

	response, err := h.notificationService.CreateNotification(c.Request.Context(), &req)
	if err != nil {
		if notificationErr, ok := err.(*errors.NotificationError); ok {
			utils.ErrorResponse(c, notificationErr)
		} else {
			h.logger.Error("unexpected err in create notif " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, "notif created successfully", response)
}

func (h *NotificationHandler) GetNotification(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetHeader("X-User-ID")

	if id == "" || userID == "" {
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
	}

	response, err := h.notificationService.GetNotification(c.Request.Context(), id, userID)
	if err != nil {
		if notificationErr, ok := err.(*errors.NotificationError); ok {
			utils.ErrorResponse(c, notificationErr)
		} else {
			h.logger.Error("unexpected error in get notif " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "notif retrieved successfully", response)
}

func (h *NotificationHandler) ListNotifications(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		utils.ErrorResponse(c, errors.ErrUnauthorizedAccess)
		return
	}

	var req dto.ListNotificationsRequest

	if err := c.ShouldBindQuery(&req); err != nil {
		h.logger.Warn("invalid list notif req: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	if req.Limit == 0 {
		req.Limit = 20
	}

	response, err := h.notificationService.ListNotifications(c.Request.Context(), userID, &req)
	if err != nil {
		if notificationErr, ok := err.(*errors.NotificationError); ok {
			utils.ErrorResponse(c, notificationErr)
		} else {
			h.logger.Error("unexpected error in list notif: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "notif retrieved sucessfully", response)
}

func (h *NotificationHandler) MarkAsRead(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		utils.ErrorResponse(c, errors.ErrUnauthorizedAccess)
		return
	}

	var req dto.MarkAsReadRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid mark as read req: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	if err := h.validator.ValidateMarkAsReadRequest(&req); err != nil {
		h.logger.Warn("mark as read validation failed: " + err.Error())
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	err := h.notificationService.MarkAsRead(c.Request.Context(), userID, &req)
	if err != nil {
		if notificationErr, ok := err.(*errors.NotificationError); ok {
			utils.ErrorResponse(c, notificationErr)
		} else {
			h.logger.Error("Unexpected error in mark as read: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "notifs marked as read successfully", nil)
}

func (h *NotificationHandler) DeleteNotification(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetHeader("X-User-ID")

	if id == "" || userID == "" {
		utils.ErrorResponse(c, errors.ErrInvalidRequest)
		return
	}

	err := h.notificationService.DeleteNotification(c.Request.Context(), id, userID)
	if err != nil {
		if notificationErr, ok := err.(*errors.NotificationError); ok {
			utils.ErrorResponse(c, notificationErr)
		} else {
			h.logger.Error("Unexpected error in delete notification: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Notification deleted successfully", nil)
}

func (h *NotificationHandler) GetUnreadCount(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		utils.ErrorResponse(c, errors.ErrUnauthorizedAccess)
		return
	}

	count, err := h.notificationService.GetUnreadCount(c.Request.Context(), userID)
	if err != nil {
		if notificationErr, ok := err.(*errors.NotificationError); ok {
			utils.ErrorResponse(c, notificationErr)
		} else {
			h.logger.Error("Unexpected error in get unread count: " + err.Error())
			utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		}
		return
	}

	response := map[string]interface{}{
		"unread_count": count,
	}

	utils.SuccessResponse(c, http.StatusOK, "Unread count retrieved successfully", response)
}

func (h *NotificationHandler) HealthCheck(c *gin.Context) {
	utils.SuccessResponse(c, http.StatusOK, "Notification service is healthy", gin.H{
		"service": "notification-service",
		"status":  "running",
	})
}
