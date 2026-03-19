package errors

import (
	"net/http"
)

type AuthError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
}

func (e *AuthError) Error() string {
	return e.Message
}

func NewAuthError(code, message string, statusCode int) *AuthError {
	return &AuthError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

var (
	ErrInvalidGoogleCode   = NewAuthError("INVALID_GOOGLE_CODE", "Invalid Google authorization code", http.StatusUnauthorized)
	ErrInvalidOAuthState   = NewAuthError("INVALID_OAUTH_STATE", "Invalid or expired OAuth state", http.StatusUnauthorized)
	ErrInvalidRedirectURI  = NewAuthError("INVALID_REDIRECT_URI", "Invalid redirect URI", http.StatusBadRequest)
	ErrPKCERequired        = NewAuthError("PKCE_REQUIRED", "PKCE code verifier is required", http.StatusBadRequest)
	ErrInvalidCodeVerifier = NewAuthError("INVALID_CODE_VERIFIER", "Invalid PKCE code verifier", http.StatusBadRequest)
	ErrInvalidRefreshToken = NewAuthError("INVALID_REFRESH_TOKEN", "Invalid refresh token", http.StatusUnauthorized)
	ErrInvalidAccessToken  = NewAuthError("INVALID_ACCESS_TOKEN", "Invalid access token", http.StatusUnauthorized)
	ErrInvalidTokenType    = NewAuthError("INVALID_TOKEN_TYPE", "Invalid token type", http.StatusBadRequest)
	ErrTokenNotFound       = NewAuthError("TOKEN_NOT_FOUND", "Token not found", http.StatusUnauthorized)
	ErrTokenBlacklisted    = NewAuthError("TOKEN_BLACKLISTED", "Token has been revoked", http.StatusUnauthorized)
	ErrTokenGeneration     = NewAuthError("TOKEN_GENERATION_FAILED", "Failed to generate tokens", http.StatusInternalServerError)
	ErrTokenStorage        = NewAuthError("TOKEN_STORAGE_FAILED", "Failed to store tokens", http.StatusInternalServerError)
	ErrTokenValidation     = NewAuthError("TOKEN_VALIDATION_FAILED", "Failed to validate token", http.StatusInternalServerError)
	ErrTokenDeletion       = NewAuthError("TOKEN_DELETION_FAILED", "Failed to delete tokens", http.StatusInternalServerError)
	ErrInvalidRequest      = NewAuthError("INVALID_REQUEST", "Invalid request parameters", http.StatusBadRequest)
	ErrInvalidCredentials  = NewAuthError("INVALID_CREDENTIALS", "Invalid email or password", http.StatusUnauthorized)
	ErrUserAlreadyExists   = NewAuthError("USER_ALREADY_EXISTS", "User with this email already exists", http.StatusConflict)
	ErrServiceUnavailable  = NewAuthError("SERVICE_UNAVAILABLE", "Authentication service temporarily unavailable", http.StatusServiceUnavailable)
)
