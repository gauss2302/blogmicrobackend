package routes

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"

	"user-service/internal/application/services"
	"user-service/internal/domain/entities"
	"user-service/pkg/auth"
	"user-service/pkg/logger"
)

const testSecret = "test-secret-at-least-32-characters-long"

// fakeUserRepo serves two fixed users for the read paths the HTTP API uses.
type fakeUserRepo struct{ users []*entities.User }

func (r *fakeUserRepo) Create(context.Context, *entities.User) error { return nil }
func (r *fakeUserRepo) GetByID(_ context.Context, id string) (*entities.User, error) {
	for _, u := range r.users {
		if u.ID == id {
			c := *u
			return &c, nil
		}
	}
	return nil, errors.New("user not found")
}
func (r *fakeUserRepo) GetByEmail(context.Context, string) (*entities.User, error) {
	return nil, errors.New("user not found")
}
func (r *fakeUserRepo) Update(context.Context, *entities.User) error { return nil }
func (r *fakeUserRepo) Delete(context.Context, string) error         { return nil }
func (r *fakeUserRepo) List(context.Context, int, int) ([]*entities.User, error) {
	return r.users, nil
}
func (r *fakeUserRepo) Search(context.Context, string, int, int) ([]*entities.User, error) {
	return r.users, nil
}
func (r *fakeUserRepo) Exists(context.Context, string) (bool, error)       { return true, nil }
func (r *fakeUserRepo) GetActiveUsersCount(context.Context) (int64, error) { return 2, nil }

type notRevoked struct{}

func (notRevoked) Revoked(context.Context, string) (bool, error) { return false, nil }

func newRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := &fakeUserRepo{users: []*entities.User{
		{ID: "alice", Email: "alice@example.com", Name: "Alice", IsActive: true},
		{ID: "bob", Email: "bob@example.com", Name: "Bob", IsActive: true},
	}}
	log := logger.New("error")
	router := gin.New()
	SetupUserRoutes(router, services.NewUserService(repo, nil, log), auth.NewValidator(testSecret), notRevoked{}, "", log)
	return router
}

func accessToken(t *testing.T, userID string) string {
	t.Helper()
	claims := auth.Claims{
		UserID: userID,
		Type:   "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

// do returns the status and raw body of a request made as actor ("" = anonymous).
func do(t *testing.T, router *gin.Engine, method, path, actor string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(`{"id":"x","email":"x@example.com","name":"X","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	if actor != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken(t, actor))
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

// hasEmailKey reports whether any object in the JSON body has an "email" key.
func hasEmailKey(t *testing.T, body string) bool {
	t.Helper()
	var v interface{}
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatalf("decode %q: %v", body, err)
	}
	var walk func(interface{}) bool
	walk = func(v interface{}) bool {
		switch x := v.(type) {
		case map[string]interface{}:
			if _, ok := x["email"]; ok {
				return true
			}
			for _, e := range x {
				if walk(e) {
					return true
				}
			}
		case []interface{}:
			for _, e := range x {
				if walk(e) {
					return true
				}
			}
		}
		return false
	}
	return walk(v)
}

func TestNoEmailInPublicAndListResponses(t *testing.T) {
	router := newRouter(t)
	for _, tc := range []struct{ path, actor string }{
		{"/api/v1/users/alice/profile", ""},
		{"/api/v1/users/alice/profile", "alice"},
		{"/api/v1/users/search?q=al", ""},
		{"/api/v1/users", "alice"},
	} {
		code, body := do(t, router, http.MethodGet, tc.path, tc.actor)
		if code != http.StatusOK {
			t.Fatalf("GET %s as %q: status %d, body %s", tc.path, tc.actor, code, body)
		}
		if !strings.Contains(body, `"Alice"`) {
			t.Fatalf("GET %s: body has no user: %s", tc.path, body)
		}
		if hasEmailKey(t, body) {
			t.Errorf("GET %s as %q exposes email: %s", tc.path, tc.actor, body)
		}
	}
}

func TestGetUserShowsEmailToOwnerOnly(t *testing.T) {
	router := newRouter(t)

	code, body := do(t, router, http.MethodGet, "/api/v1/users/alice", "alice")
	if code != http.StatusOK || !strings.Contains(body, `"email":"alice@example.com"`) {
		t.Errorf("owner: status %d, body %s; want email", code, body)
	}

	code, body = do(t, router, http.MethodGet, "/api/v1/users/alice", "bob")
	if code != http.StatusOK || hasEmailKey(t, body) {
		t.Errorf("other user: status %d, body %s; want no email", code, body)
	}
}

func TestCreateUserRouteRemoved(t *testing.T) {
	router := newRouter(t)
	code, body := do(t, router, http.MethodPost, "/api/v1/users", "alice")
	if code != http.StatusNotFound {
		t.Errorf("POST /api/v1/users: status %d, body %s; want 404", code, body)
	}
}
