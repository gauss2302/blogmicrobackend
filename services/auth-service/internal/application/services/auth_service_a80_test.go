package services

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"auth-service/internal/application/errors"
	"auth-service/internal/config"
	"auth-service/pkg/logger"
)

// createFails answers every CreateUser with one gRPC error.
type createFails struct {
	loginUsers
	err error
}

func (u *createFails) CreateUser(context.Context, string, string, string, string, string) (UserInfoResult, error) {
	return nil, u.err
}

// A-80: input user-service rejects is a 400; only a real outage is a 503.
func TestRegisterMapsCreateUserErrors(t *testing.T) {
	for code, want := range map[codes.Code]error{
		codes.InvalidArgument: errors.ErrInvalidRequest,
		codes.AlreadyExists:   errors.ErrUserAlreadyExists,
		codes.Unavailable:     errors.ErrServiceUnavailable,
		codes.Internal:        errors.ErrServiceUnavailable,
	} {
		users := &createFails{err: status.Error(code, "user-service text")}
		svc := NewAuthService(&refreshTokenRepo{}, nil, users, config.JWTConfig{Secret: "0123456789abcdef0123456789abcdef"},
			config.GoogleConfig{}, logger.New("fatal"))
		if _, err := svc.Register(context.Background(), "a@example.com", "password1", "n"); err != want {
			t.Errorf("%v: err = %v, want %v", code, err, want)
		}
	}
}
