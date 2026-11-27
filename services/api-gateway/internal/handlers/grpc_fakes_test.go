package handlers

import (
	"context"
	"net"
	"sync/atomic"
	"testing"

	"google.golang.org/grpc"

	"api-gateway/internal/clients"
	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
	authv1 "github.com/gauss2302/blogmicrobackend/proto/auth/v1"
	postv1 "github.com/gauss2302/blogmicrobackend/proto/post/v1"
	userv1 "github.com/gauss2302/blogmicrobackend/proto/user/v1"
)

// startGRPC serves fakes on a loopback port so tests drive the real gRPC
// clients and handlers end to end, status codes included.
func startGRPC(t *testing.T, register func(*grpc.Server)) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	register(srv)
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)
	return lis.Addr().String()
}

var testLogger = logger.New("fatal")

// fakeAuth counts calls and fails every faked method with err when set.
type fakeAuth struct {
	authv1.UnimplementedAuthServiceServer
	calls atomic.Int32
	err   error
}

func (f *fakeAuth) GetGoogleAuthURL(context.Context, *authv1.GetGoogleAuthURLRequest) (*authv1.GetGoogleAuthURLResponse, error) {
	f.calls.Add(1)
	if f.err != nil {
		return nil, f.err
	}
	return &authv1.GetGoogleAuthURLResponse{}, nil
}

func (f *fakeAuth) RefreshToken(context.Context, *authv1.RefreshTokenRequest) (*authv1.RefreshTokenResponse, error) {
	f.calls.Add(1)
	if f.err != nil {
		return nil, f.err
	}
	return &authv1.RefreshTokenResponse{}, nil
}

func (f *fakeAuth) Register(context.Context, *authv1.RegisterRequest) (*authv1.RegisterResponse, error) {
	f.calls.Add(1)
	if f.err != nil {
		return nil, f.err
	}
	return &authv1.RegisterResponse{}, nil
}

func newTestAuthHandler(t *testing.T, fake *fakeAuth) *AuthHandler {
	t.Helper()
	addr := startGRPC(t, func(s *grpc.Server) { authv1.RegisterAuthServiceServer(s, fake) })
	client, err := clients.NewAuthClient(addr, config.GRPCTLSConfig{}, testLogger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return NewAuthHandler(client, &config.Config{}, testLogger)
}

// fakeUsers and fakePosts record the offset of the last list call in a shared
// counter; -1 means no call.
type fakeUsers struct {
	userv1.UnimplementedUserServiceServer
	offset *atomic.Int32
}

type fakePosts struct {
	postv1.UnimplementedPostServiceServer
	offset *atomic.Int32
}

func (f *fakeUsers) ListUsers(_ context.Context, req *userv1.ListUsersRequest) (*userv1.ListUsersResponse, error) {
	f.offset.Store(req.GetOffset())
	return &userv1.ListUsersResponse{}, nil
}

func (f *fakePosts) ListPosts(_ context.Context, req *postv1.ListPostsRequest) (*postv1.ListPostsResponse, error) {
	f.offset.Store(req.GetOffset())
	return &postv1.ListPostsResponse{}, nil
}

func newTestListHandlers(t *testing.T, offset *atomic.Int32) (*UserHandler, *PostHandler) {
	t.Helper()
	addr := startGRPC(t, func(s *grpc.Server) {
		userv1.RegisterUserServiceServer(s, &fakeUsers{offset: offset})
		postv1.RegisterPostServiceServer(s, &fakePosts{offset: offset})
	})
	userClient, err := clients.NewUserClient(addr, config.GRPCTLSConfig{}, testLogger)
	if err != nil {
		t.Fatal(err)
	}
	postClient, err := clients.NewPostClient(addr, config.GRPCTLSConfig{}, testLogger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { userClient.Close(); postClient.Close() })
	return NewUserHandler(userClient, nil, postClient, testLogger), NewPostHandler(postClient, testLogger)
}
