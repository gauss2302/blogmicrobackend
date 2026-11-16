package middleware

import (
	"net/http"
	"strings"

	"user-service/internal/application/errors"
	"user-service/pkg/auth"
	"user-service/pkg/logger"
	"user-service/pkg/utils"

	"github.com/gin-gonic/gin"
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
func AuthMiddleware(validator *auth.Validator, revocations auth.RevocationChecker, trustMode string, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token := bearerToken(c); token != "" {
			userID, ok := acceptAccessToken(c, token, validator, revocations, trustMode, log)
			if !ok {
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

// ActorID returns the authenticated user id stored by the auth middleware, or
// an empty string when the request is anonymous.
// acceptAccessToken verifies the signature and rejects revoked tokens.
// A Redis error fails closed: a token whose revocation status is unknown is
// not treated as valid. insecure_dev may omit the checker; every other mode
// must have one, or a logged-out token would stay usable until it expires.
func acceptAccessToken(c *gin.Context, token string, validator *auth.Validator, revocations auth.RevocationChecker, trustMode string, log *logger.Logger) (string, bool) {
	if validator == nil {
		log.Error("access token presented but no JWT secret is configured")
		utils.ErrorResponse(c, errors.ErrUnauthorizedAccess)
		c.Abort()
		return "", false
	}
	userID, err := validator.ValidateAccessToken(token)
	if err != nil {
		log.Warn("rejected request with invalid access token: " + err.Error())
		utils.ErrorResponse(c, errors.ErrUnauthorizedAccess)
		c.Abort()
		return "", false
	}
	if revocations == nil {
		if trustMode == trustModeInsecureDev {
			return userID, true
		}
		log.Error("revocation checker is not configured")
		utils.ErrorResponse(c, errors.ErrUnauthorizedAccess)
		c.Abort()
		return "", false
	}
	revoked, err := revocations.Revoked(c.Request.Context(), token)
	if err != nil {
		log.Error("revocation check failed: " + err.Error())
		utils.ErrorResponse(c, errors.ErrUnauthorizedAccess)
		c.Abort()
		return "", false
	}
	if revoked {
		log.Warn("rejected revoked access token")
		utils.ErrorResponse(c, errors.ErrUnauthorizedAccess)
		c.Abort()
		return "", false
	}
	return userID, true
}

func ActorID(c *gin.Context) string {
	userID, ok := c.Get(ContextUserIDKey)
	if !ok {
		return ""
	}
	id, ok := userID.(string)
	if !ok {
		return ""
	}
	return id
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

// CORS sets cross-origin headers for the user API. The API authenticates with a
// bearer token (not cookies), so credentials are intentionally NOT allowed: a
// wildcard origin without credentials cannot be used to read responses with a
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
