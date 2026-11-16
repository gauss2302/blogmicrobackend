package grpc

import (
	"context"
	"testing"
	"time"

	"user-service/pkg/auth"
	"user-service/pkg/logger"

	userv1 "github.com/gauss2302/blogmicrobackend/proto/user/v1"
	"github.com/golang-jwt/jwt/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const followSecret = "test-secret-at-least-32-characters-long"

func accessToken(t *testing.T, userID string) string {
	t.Helper()
	claims := auth.Claims{
		UserID: userID,
		Type:   "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(followSecret))
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func TestFollowRejectsCallerWhoIsNotTheFollower(t *testing.T) {
	server := NewUserServer(nil, auth.NewValidator(followSecret), allowRevocations{}, logger.New("error"))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+accessToken(t, "attacker")))

	_, err := server.Follow(ctx, &userv1.FollowRequest{FollowerId: "victim", FolloweeId: "someone"})
	st, _ := status.FromError(err)
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("code = %s, want %s (%v)", st.Code(), codes.PermissionDenied, err)
	}
}

func TestFollowRejectsMissingCredentials(t *testing.T) {
	server := NewUserServer(nil, auth.NewValidator(followSecret), allowRevocations{}, logger.New("error"))

	_, err := server.Follow(context.Background(), &userv1.FollowRequest{FollowerId: "victim", FolloweeId: "someone"})
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Fatalf("code = %s, want %s", st.Code(), codes.Unauthenticated)
	}
}

type allowRevocations struct{}

func (allowRevocations) Revoked(context.Context, string) (bool, error) { return false, nil }
