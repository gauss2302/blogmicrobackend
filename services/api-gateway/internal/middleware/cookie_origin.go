package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"api-gateway/internal/config"
	"api-gateway/pkg/utils"
)

// RefreshCookieOrigin rejects a request that carries the refresh-token cookie
// and an Origin outside CORS_ALLOWED_ORIGINS: a browser on another site must
// not be able to refresh or end a session with the cookie it attaches. No
// Origin (BFF, mobile, curl) passes, and so does a token sent in the body,
// which an attacker would have to know. A "*" entry does not match.
func RefreshCookieOrigin(cfg *config.Config) gin.HandlerFunc {
	name := cfg.Auth.RefreshTokenCookieName
	if name == "" {
		name = "refresh_token"
	}
	return func(c *gin.Context) {
		if !cfg.Auth.UseRefreshTokenCookie {
			c.Next()
			return
		}
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if tok, err := c.Cookie(name); err != nil || tok == "" || origin == "" {
			c.Next()
			return
		}
		if !containsIgnoreCase(cfg.CORS.AllowedOrigins, origin) {
			utils.ErrorResponse(c, http.StatusForbidden, "FORBIDDEN_ORIGIN", "Cross-origin request rejected")
			c.Abort()
			return
		}
		c.Next()
	}
}
