package errors

import (
	"net/http"
)

type PostError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
}

func (e *PostError) Error() string {
	return e.Message
}

func NewPostError(code, message string, statusCode int) *PostError {
	return &PostError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

var (
	ErrPostNotFound        = NewPostError("POST_NOT_FOUND", "Post not found", http.StatusNotFound)
	ErrPostAlreadyExists   = NewPostError("POST_ALREADY_EXISTS", "Post with this slug already exists", http.StatusConflict)
	ErrInvalidPostData     = NewPostError("INVALID_POST_DATA", "Invalid post data provided", http.StatusBadRequest)
	ErrPostCreationFailed  = NewPostError("POST_CREATION_FAILED", "Failed to create post", http.StatusInternalServerError)
	ErrPostUpdateFailed    = NewPostError("POST_UPDATE_FAILED", "Failed to update post", http.StatusInternalServerError)
	ErrPostDeletionFailed  = NewPostError("POST_DELETION_FAILED", "Failed to delete post", http.StatusInternalServerError)
	ErrPostListFailed      = NewPostError("POST_LIST_FAILED", "Failed to retrieve posts", http.StatusInternalServerError)
	ErrPostSearchFailed    = NewPostError("POST_SEARCH_FAILED", "Failed to search posts", http.StatusInternalServerError)
	ErrPostStatsFailed     = NewPostError("POST_STATS_FAILED", "Failed to retrieve post statistics", http.StatusInternalServerError)
	ErrUnauthorizedAccess  = NewPostError("UNAUTHORIZED_ACCESS", "You don't have permission to access this resource", http.StatusForbidden)
	ErrInvalidRequest      = NewPostError("INVALID_REQUEST", "Invalid request parameters", http.StatusBadRequest)
	ErrServiceUnavailable  = NewPostError("SERVICE_UNAVAILABLE", "Post service temporarily unavailable", http.StatusServiceUnavailable)
)