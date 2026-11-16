package auth

import "context"

// RevocationChecker reports whether an access token has been revoked.
// auth-service writes that fact to Redis under auth:blacklist:<token>; the
// HTTP APIs must consult it, because a valid signature alone stays true until
// the token expires.
type RevocationChecker interface {
	Revoked(ctx context.Context, token string) (bool, error)
}
