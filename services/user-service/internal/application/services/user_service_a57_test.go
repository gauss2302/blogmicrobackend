package services

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"user-service/internal/application/dto"
	"user-service/pkg/logger"
)

// fastest returns the shortest of n runs, to keep scheduler noise out.
func fastest(n int, f func()) time.Duration {
	best := time.Duration(1<<63 - 1)
	for range n {
		start := time.Now()
		f()
		if d := time.Since(start); d < best {
			best = d
		}
	}
	return best
}

// An unknown email must cost a bcrypt comparison like a wrong password does,
// or the response time tells which emails have accounts.
func TestValidateCredentialsTakesAsLongForUnknownEmail(t *testing.T) {
	svc := registerAlice(t)
	ctx := context.Background()
	_, _ = svc.ValidateCredentials(ctx, "nobody@example.com", "x") // build the dummy hash once

	wrong := fastest(3, func() { _, _ = svc.ValidateCredentials(ctx, "Alice@Example.com", "wrong-password") })
	unknown := fastest(3, func() { _, _ = svc.ValidateCredentials(ctx, "nobody@example.com", "wrong-password") })
	if unknown < wrong/2 {
		t.Fatalf("unknown email took %s, wrong password %s: unknown emails are distinguishable", unknown, wrong)
	}
}

func TestInfoLogsCarryNoEmailOrSearchQuery(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New("info")
	log.SetOutput(&buf)
	svc := NewUserService(newMemUserRepo(), &mockFollowRepo{}, log)
	ctx := context.Background()

	_, _ = svc.CreateUser(ctx, &dto.CreateUserRequest{Email: "secret.person@example.com", Name: "S", Password: "correct-horse"})
	_, _ = svc.GetUserByEmail(ctx, "secret.person@example.com")
	_, _ = svc.GetUserByEmail(ctx, "missing.person@example.com")
	_, _ = svc.ValidateCredentials(ctx, "secret.person@example.com", "wrong-password")
	_, _ = svc.SearchUsers(ctx, &dto.SearchUsersRequest{Query: "very-private-query", Limit: 10})

	for _, leak := range []string{"secret.person", "missing.person", "very-private-query"} {
		if strings.Contains(buf.String(), leak) {
			t.Errorf("INFO log contains %q:\n%s", leak, buf.String())
		}
	}
}
