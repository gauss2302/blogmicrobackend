package jwt

import (
	"auth-service/internal/domain/entities"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type Manager struct {
	secret     []byte
	algorithms []string
	issuer     string
}

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Type   string `json:"type"`
	jwt.RegisteredClaims
}

func NewManager(secret, issuer string) *Manager {
	return &Manager{
		secret:     []byte(secret),
		algorithms: []string{"HS256"}, // Explicitly allow only secure algorithms
		issuer:     issuer,
	}
}

func (m *Manager) GenerateToken(tokenClaims *entities.TokenClaims, ttl time.Duration) (string, error) {
	// A random jti keeps two tokens for the same user issued in the same
	// second distinct; tokens are stored and rotated by their exact value.
	jti := make([]byte, 16)
	if _, err := rand.Read(jti); err != nil {
		return "", fmt.Errorf("generate jti: %w", err)
	}
	now := time.Now()
	claims := &Claims{
		UserID: tokenClaims.UserID,
		Email:  tokenClaims.Email,
		Type:   tokenClaims.Type,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    m.issuer,
			Subject:   tokenClaims.UserID,
			ID:        base64.RawURLEncoding.EncodeToString(jti),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *Manager) ValidateToken(tokenString string) (*entities.TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate algorithm
		if method, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		} else if method.Alg() != "HS256" {
			return nil, fmt.Errorf("unexpected signing algorithm: %v", method.Alg())
		}
		return m.secret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// JWT_ISSUER: a token signed with the same secret for another issuer is
	// not ours.
	if claims.Issuer != m.issuer {
		return nil, fmt.Errorf("invalid token issuer")
	}

	return &entities.TokenClaims{
		UserID: claims.UserID,
		Email:  claims.Email,
		Type:   claims.Type,
	}, nil
}
