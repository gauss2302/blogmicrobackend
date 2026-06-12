// Package auth verifies the access tokens issued by auth-service so that
// notification-service can authenticate callers cryptographically instead of
// trusting an unauthenticated X-User-ID header.
package auth

import (
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v4"
)

// accessTokenType matches the "type" claim auth-service sets on access tokens.
const accessTokenType = "access"

var (
	// ErrInvalidToken is returned when a token is missing, malformed, expired,
	// or signed with the wrong key/algorithm.
	ErrInvalidToken = errors.New("invalid token")
	// ErrWrongTokenType is returned when a syntactically valid token is not an
	// access token (e.g. a refresh token is presented).
	ErrWrongTokenType = errors.New("wrong token type")
)

// Claims mirrors the subset of auth-service's JWT claims this service needs.
// It must stay compatible with auth-service/pkg/jwt.Claims.
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Type   string `json:"type"`
	jwt.RegisteredClaims
}

// Validator verifies HS256 access tokens using the signing secret shared with
// auth-service.
type Validator struct {
	secret []byte
}

// NewValidator builds a Validator from the shared JWT secret.
func NewValidator(secret string) *Validator {
	return &Validator{secret: []byte(secret)}
}

// ValidateAccessToken verifies the token's signature and expiry, enforces the
// HS256 algorithm (rejecting "none"/alg-confusion), confirms it is an access
// token, and returns the authenticated user id from the verified claims.
func (v *Validator) ValidateAccessToken(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Only accept HMAC-SHA256; never trust the alg header to pick the method.
		method, ok := token.Method.(*jwt.SigningMethodHMAC)
		if !ok || method.Alg() != "HS256" {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return v.secret, nil
	})
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if !token.Valid {
		return "", ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return "", ErrInvalidToken
	}
	if claims.Type != accessTokenType {
		return "", ErrWrongTokenType
	}
	if claims.UserID == "" {
		return "", ErrInvalidToken
	}

	return claims.UserID, nil
}
