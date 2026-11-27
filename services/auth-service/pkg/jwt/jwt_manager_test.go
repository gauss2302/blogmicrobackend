package jwt

import (
	"testing"
	"time"

	"auth-service/internal/domain/entities"
)

func TestTokensIssuedTogetherDiffer(t *testing.T) {
	m := NewManager("0123456789abcdef0123456789abcdef", "auth-service")
	claims := &entities.TokenClaims{UserID: "user-1", Email: "u@example.com", Type: "refresh"}

	a, err := m.GenerateToken(claims, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.GenerateToken(claims, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("two tokens for the same user in the same second are identical")
	}
	for _, tok := range []string{a, b} {
		if got, err := m.ValidateToken(tok); err != nil || got.UserID != "user-1" {
			t.Fatalf("ValidateToken: %v, %+v", err, got)
		}
	}
}
