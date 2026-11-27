package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"api-gateway/internal/clients"
	"api-gateway/internal/config"
	authv1 "github.com/gauss2302/blogmicrobackend/proto/auth/v1"
	userv1 "github.com/gauss2302/blogmicrobackend/proto/user/v1"
)

type callLog struct {
	mu    sync.Mutex
	calls []string
}

func (l *callLog) add(call string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, call)
}

func (l *callLog) get() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.calls)
}

type deleteAuth struct {
	authv1.UnimplementedAuthServiceServer
	log *callLog
	err error
}

func (f *deleteAuth) Logout(_ context.Context, req *authv1.LogoutRequest) (*emptypb.Empty, error) {
	f.log.add("logout:" + req.GetAccessToken())
	if f.err != nil {
		return nil, f.err
	}
	return &emptypb.Empty{}, nil
}

type deleteUsers struct {
	userv1.UnimplementedUserServiceServer
	log *callLog
}

func (f *deleteUsers) DeleteUser(_ context.Context, req *userv1.DeleteUserRequest) (*emptypb.Empty, error) {
	f.log.add("delete:" + req.GetId())
	if req.GetId() != req.GetActorId() {
		return nil, status.Error(codes.PermissionDenied, "not your account")
	}
	return &emptypb.Empty{}, nil
}

func deleteAccount(t *testing.T, authErr error, id string) (int, []string) {
	t.Helper()
	log := &callLog{}
	addr := startGRPC(t, func(s *grpc.Server) {
		authv1.RegisterAuthServiceServer(s, &deleteAuth{log: log, err: authErr})
		userv1.RegisterUserServiceServer(s, &deleteUsers{log: log})
	})
	authClient, err := clients.NewAuthClient(addr, config.GRPCTLSConfig{}, testLogger)
	if err != nil {
		t.Fatal(err)
	}
	userClient, err := clients.NewUserClient(addr, config.GRPCTLSConfig{}, testLogger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { authClient.Close(); userClient.Close() })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.DELETE("/users/:id", func(c *gin.Context) {
		c.Set("userID", "u1")
		c.Set("token", "access-u1")
	}, NewUserHandler(userClient, authClient, testLogger).DeleteUser)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/users/"+id, nil))
	return rec.Code, log.get()
}

func TestDeleteOwnAccountRevokesSessionsFirst(t *testing.T) {
	code, calls := deleteAccount(t, nil, "u1")
	if code != http.StatusOK || !slices.Equal(calls, []string{"logout:access-u1", "delete:u1"}) {
		t.Fatalf("status %d calls %v, want 200 and logout before delete", code, calls)
	}
}

func TestDeleteAccountKeptWhenRevokeFails(t *testing.T) {
	code, calls := deleteAccount(t, status.Error(codes.Unavailable, "redis down"), "u1")
	if code != http.StatusServiceUnavailable || !slices.Equal(calls, []string{"logout:access-u1"}) {
		t.Fatalf("status %d calls %v, want 503 and no delete", code, calls)
	}
}

func TestDeleteOtherAccountDoesNotRevoke(t *testing.T) {
	code, calls := deleteAccount(t, nil, "u2")
	if code != http.StatusForbidden || !slices.Equal(calls, []string{"delete:u2"}) {
		t.Fatalf("status %d calls %v, want 403 and no logout", code, calls)
	}
}
