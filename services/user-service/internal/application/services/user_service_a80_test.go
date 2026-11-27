package services

import (
	"context"
	"strings"
	"testing"

	"user-service/internal/application/dto"
	apperrors "user-service/internal/application/errors"
	"user-service/internal/domain/entities"
	"user-service/pkg/logger"
)

// noLookupRepo fails the test if CreateUser reaches the database.
type noLookupRepo struct {
	mockUserRepo
	t *testing.T
}

func (r *noLookupRepo) GetByEmail(context.Context, string) (*entities.User, error) {
	r.t.Fatal("GetByEmail called before validation")
	return nil, nil
}

func (r *noLookupRepo) Create(context.Context, *entities.User) error {
	r.t.Fatal("Create called with invalid input")
	return nil
}

// A-80: input Postgres would reject is a 400 before any query, not a failed
// SELECT answered as 503.
func TestCreateUser_InvalidInputSkipsDatabase(t *testing.T) {
	for name, req := range map[string]*dto.CreateUserRequest{
		"NUL in email":     {Email: "a\x00b@example.com", Name: "n"},
		"NUL in name":      {Email: "a@example.com", Name: "a\x00b"},
		"NUL in picture":   {Email: "a@example.com", Name: "n", Picture: "https://e.com/\x00"},
		"name too long":    {Email: "a@example.com", Name: strings.Repeat("я", 101)},
		"picture too long": {Email: "a@example.com", Name: "n", Picture: "https://e.com/" + strings.Repeat("x", 500)},
	} {
		t.Run(name, func(t *testing.T) {
			svc := NewUserService(&noLookupRepo{t: t}, &mockFollowRepo{}, logger.New("error"))
			if _, err := svc.CreateUser(context.Background(), req); err != apperrors.ErrInvalidUserData {
				t.Fatalf("err = %v, want ErrInvalidUserData", err)
			}
		})
	}
}
