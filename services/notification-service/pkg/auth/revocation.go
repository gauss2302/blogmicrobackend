package auth

import "context"

// RevocationChecker reports whether an access token has been revoked.
// auth-service writes that fact to Redis under auth:blacklist:<token>.
type RevocationChecker interface {
	Revoked(ctx context.Context, token string) (bool, error)
}
