package errors

import "net/http"

type NotificationError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
}

func (e *NotificationError) Error() string {
	return e.Message
}

func NewNotificationError(code, message string, statusCode int) *NotificationError {
	return &NotificationError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

var (
	ErrNotificationNotFound       = NewNotificationError("NOTIFICATION_NOT_FOUND", "Notification not found", http.StatusNotFound)
	ErrInvalidNotificationData    = NewNotificationError("INVALID_NOTIFICATION_DATA", "Invalid notification data provided", http.StatusBadRequest)
	ErrNotificationCreationFailed = NewNotificationError("NOTIFICATION_CREATION_FAILED", "Failed to create notification", http.StatusInternalServerError)
	ErrNotificationUpdateFailed   = NewNotificationError("NOTIFICATION_UPDATE_FAILED", "Failed to update notification", http.StatusInternalServerError)
	ErrNotificationDeletionFailed = NewNotificationError("NOTIFICATION_DELETION_FAILED", "Failed to delete notification", http.StatusInternalServerError)
	ErrNotificationListFailed     = NewNotificationError("NOTIFICATION_LIST_FAILED", "Failed to retrieve notifications", http.StatusInternalServerError)
	ErrUnauthorizedAccess         = NewNotificationError("UNAUTHORIZED_ACCESS", "You don't have permission to access this resource", http.StatusForbidden)
	ErrInvalidRequest             = NewNotificationError("INVALID_REQUEST", "Invalid request parameters", http.StatusBadRequest)
	ErrServiceUnavailable         = NewNotificationError("SERVICE_UNAVAILABLE", "Notification service temporarily unavailable", http.StatusServiceUnavailable)
	ErrMessageProcessingFailed    = NewNotificationError("MESSAGE_PROCESSING_FAILED", "Failed to process message", http.StatusInternalServerError)
)
