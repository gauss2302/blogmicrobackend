package errors

import (
	"net/http"
)

type UserError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
}

func (e *UserError) Error() string {
	return e.Message
}

func NewUserError(code, message string, statusCode int) *UserError {
	return &UserError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

var (
	ErrUserNotFound        = NewUserError("USER_NOT_FOUND", "User not found", http.StatusNotFound)
	ErrUserAlreadyExists   = NewUserError("USER_ALREADY_EXISTS", "User with this email already exists", http.StatusConflict)
	ErrInvalidUserData     = NewUserError("INVALID_USER_DATA", "Invalid user data provided", http.StatusBadRequest)
	ErrUserCreationFailed  = NewUserError("USER_CREATION_FAILED", "Failed to create user", http.StatusInternalServerError)
	ErrUserUpdateFailed    = NewUserError("USER_UPDATE_FAILED", "Failed to update user", http.StatusInternalServerError)
	ErrUserDeletionFailed  = NewUserError("USER_DELETION_FAILED", "Failed to delete user", http.StatusInternalServerError)
	ErrUserListFailed      = NewUserError("USER_LIST_FAILED", "Failed to retrieve users", http.StatusInternalServerError)
	ErrUserSearchFailed    = NewUserError("USER_SEARCH_FAILED", "Failed to search users", http.StatusInternalServerError)
	ErrUserStatsFailed     = NewUserError("USER_STATS_FAILED", "Failed to retrieve user statistics", http.StatusInternalServerError)
	ErrUnauthorizedAccess  = NewUserError("UNAUTHORIZED_ACCESS", "You don't have permission to access this resource", http.StatusForbidden)
	ErrInvalidRequest      = NewUserError("INVALID_REQUEST", "Invalid request parameters", http.StatusBadRequest)
	ErrServiceUnavailable  = NewUserError("SERVICE_UNAVAILABLE", "User service temporarily unavailable", http.StatusServiceUnavailable)
)