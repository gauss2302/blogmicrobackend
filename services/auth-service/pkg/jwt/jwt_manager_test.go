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

func TestValidateTokenRejectsAnotherIssuer(t *testing.T) {
	const secret = "0123456789abcdef0123456789abcdef"
	claims := &entities.TokenClaims{UserID: "user-1", Type: "access"}

	ours := NewManager(secret, "blog-auth")
	token, err := ours.GenerateToken(claims, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ours.ValidateToken(token); err != nil {
		t.Fatalf("own issuer: %v", err)
	}

	// Same secret, different JWT_ISSUER: the signature checks out, the issuer does not.
	other, err := NewManager(secret, "someone-else").GenerateToken(claims, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ours.ValidateToken(other); err == nil {
		t.Fatal("token from another issuer accepted")
	}
}
