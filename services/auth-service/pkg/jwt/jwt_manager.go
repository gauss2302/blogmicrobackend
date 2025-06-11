package jwt

import (
	"auth-service/internal/domain/entities"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type Manager struct {
	secret []byte
}

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Type   string `json:"type"`
	jwt.RegisteredClaims
}

func NewManager(secret string) *Manager {
	return &Manager{secret: []byte(secret)}
}

func (m *Manager) GenerateToken(tokenClaims *entities.TokenClaims, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID: tokenClaims.UserID,
		Email:  tokenClaims.Email,
		Type:   tokenClaims.Type,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "auth-service",
			Subject:   tokenClaims.UserID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *Manager) ValidateToken(tokenString string) (*entities.TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
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

	return &entities.TokenClaims{
		UserID: claims.UserID,
		Email:  claims.Email,
		Type:   claims.Type,
	}, nil
}