package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	"post-service/pkg/logger"

	postv1 "github.com/gauss2302/blogmicrobackend/proto/post/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// fakePosts panics in GetStats and blocks in HealthCheck until release closes.
type fakePosts struct {
	postv1.UnimplementedPostServiceServer
	inHealth chan struct{}
	release  chan struct{}
}

func (f *fakePosts) GetStats(context.Context, *postv1.GetStatsRequest) (*postv1.PostStatsResponse, error) {
	var stats map[string]int
	stats["boom"]++ // nil map write: a real bug, not a deliberate panic() call
	return nil, nil
}

func (f *fakePosts) HealthCheck(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	close(f.inHealth)
	select {
	case <-f.release:
	case <-ctx.Done():
	}
	return &emptypb.Empty{}, nil
}

func serve(t *testing.T, fake *fakePosts) (*grpc.Server, postv1.PostServiceClient) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(RecoveryInterceptor(logger.New("fatal"))))
	postv1.RegisterPostServiceServer(srv, fake)
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return srv, postv1.NewPostServiceClient(conn)
}

func TestRecoveryInterceptorTurnsPanicIntoInternal(t *testing.T) {
	fake := &fakePosts{inHealth: make(chan struct{}), release: make(chan struct{})}
	close(fake.release)
	_, client := serve(t, fake)
	ctx := context.Background()

	if _, err := client.GetStats(ctx, &postv1.GetStatsRequest{}); status.Code(err) != codes.Internal {
		t.Fatalf("panicking handler: err = %v, want Internal", err)
	}
	// The server survived and still answers.
	if _, err := client.HealthCheck(ctx, &emptypb.Empty{}); err != nil {
		t.Fatalf("after the panic: %v", err)
	}
}

func TestGracefulStopHasADeadline(t *testing.T) {
	fake := &fakePosts{inHealth: make(chan struct{}), release: make(chan struct{})}
	defer close(fake.release)
	srv, client := serve(t, fake)

	go client.HealthCheck(context.Background(), &emptypb.Empty{}) //nolint:errcheck // cut off by Stop
	<-fake.inHealth

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	if GracefulStop(ctx, srv) {
		t.Error("stop with an RPC still running reported graceful")
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("stop took %s, want about the 200ms deadline", took)
	}
}

func TestGracefulStopWithoutRPCsIsGraceful(t *testing.T) {
	fake := &fakePosts{inHealth: make(chan struct{}), release: make(chan struct{})}
	srv, _ := serve(t, fake)
	if !GracefulStop(context.Background(), srv) {
		t.Error("idle server: stop was not graceful")
	}
}
