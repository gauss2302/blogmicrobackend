package clients

import (
	"context"
	"net"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
	postv1 "github.com/gauss2302/blogmicrobackend/proto/post/v1"
)

// fakeUnpublish records UnpublishUserPosts requests and answers with err.
type fakeUnpublish struct {
	postv1.UnimplementedPostServiceServer
	mu   sync.Mutex
	reqs []*postv1.UnpublishUserPostsRequest
	err  error
}

func (f *fakeUnpublish) UnpublishUserPosts(_ context.Context, req *postv1.UnpublishUserPostsRequest) (*emptypb.Empty, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reqs = append(f.reqs, req)
	if f.err != nil {
		return nil, f.err
	}
	return &emptypb.Empty{}, nil
}

func TestUnpublishUserPosts(t *testing.T) {
	fake := &fakeUnpublish{}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	postv1.RegisterPostServiceServer(srv, fake)
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)

	client, err := NewPostClient(lis.Addr().String(), config.GRPCTLSConfig{}, logger.New("fatal"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	ctx := context.Background()

	if err := client.UnpublishUserPosts(ctx, "u1", "u1"); err != nil {
		t.Fatalf("ok call: %v", err)
	}
	if got := fake.reqs[0]; got.GetUserId() != "u1" || got.GetActorId() != "u1" {
		t.Errorf("sent user_id=%q actor_id=%q, want u1/u1", got.GetUserId(), got.GetActorId())
	}

	// The caller (DeleteUser) must see Unavailable to retry or refuse.
	fake.err = status.Error(codes.Unavailable, "search not updated")
	if err := client.UnpublishUserPosts(ctx, "u1", "u1"); status.Code(err) != codes.Unavailable {
		t.Errorf("err = %v, want code Unavailable", err)
	}
}
