package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"notification-service/internal/application/errors"
	"notification-service/pkg/auth"
	"notification-service/pkg/logger"
	"notification-service/pkg/utils"
)

const (
	// trustModeInsecureDev is the only INTERNAL_HTTP_TRUST_MODE that permits the
	// unauthenticated X-User-ID header fallback. It is forbidden in production by
	// config validation.
	trustModeInsecureDev = "insecure_dev"
	// ContextUserIDKey is the gin context key under which the authenticated user
	// id is stored. Handlers must read identity from here, never from a header.
	ContextUserIDKey = "userID"
)

// AuthMiddleware authenticates the caller from a signed access token carried in
// the Authorization: Bearer header and stores the verified user id in the gin
// context under ContextUserIDKey.
//
// A valid token is required in every mode (fail closed). As an explicit,
// production-forbidden convenience, when trustMode is "insecure_dev" and no
// bearer token is supplied, a plain X-User-ID header is accepted so the service
// can be exercised locally without minting tokens.
func AuthMiddleware(validator *auth.Validator, trustMode string, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token := bearerToken(c); token != "" {
			if validator == nil {
				log.Error("access token presented but no JWT secret is configured")
				utils.ErrorResponse(c, errors.ErrUnauthorizedAccess)
				c.Abort()
				return
			}
			userID, err := validator.ValidateAccessToken(token)
			if err != nil {
				log.Warn("rejected request with invalid access token: " + err.Error())
				utils.ErrorResponse(c, errors.ErrUnauthorizedAccess)
				c.Abort()
				return
			}
			c.Set(ContextUserIDKey, userID)
			c.Next()
			return
		}

		// No bearer token. Only the explicit insecure dev mode may fall back to
		// trusting the caller-supplied header.
		if trustMode == trustModeInsecureDev {
			if userID := c.GetHeader("X-User-ID"); userID != "" {
				log.Warn("insecure_dev: trusting unauthenticated X-User-ID header")
				c.Set(ContextUserIDKey, userID)
				c.Next()
				return
			}
		}

		utils.ErrorResponse(c, errors.ErrUnauthorizedAccess)
		c.Abort()
	}
}

// bearerToken extracts the token from an "Authorization: Bearer <token>" header.
func bearerToken(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// CORS sets cross-origin headers for the notification API. The API authenticates
// with a bearer token (not cookies), so credentials are intentionally NOT allowed:
// a wildcard origin without credentials cannot be used to read responses with a
// victim's token attached.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func ErrorHandler(logger *logger.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		if err, ok := recovered.(string); ok {
			logger.Error("panic recovered: " + err)
		} else if err, ok := recovered.(error); ok {
			logger.Error("panic recovered: " + err.Error())
		} else {
			logger.Error("unknown panic recovered")
		}

		utils.ErrorResponse(c, errors.ErrServiceUnavailable)
		c.Abort()
	})
}

func RequestLogger(logger *logger.Logger) gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		logger.Info(
			"Request: " + param.Method + " " + param.Path +
				" | Status: " + strconv.Itoa(param.StatusCode) +
				" | Latency: " + param.Latency.String(),
		)
		return ""
	})
}
