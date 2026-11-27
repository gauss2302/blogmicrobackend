package grpc

import (
	"context"
	"fmt"
	"runtime/debug"

	"post-service/pkg/logger"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RecoveryInterceptor turns a handler panic into codes.Internal instead of a
// crashed process, logging the panic and its stack. Put it last in the chain,
// next to the handler, so metrics and logging record the Internal status.
func RecoveryInterceptor(log *logger.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Error(fmt.Sprintf("gRPC %s panic: %v\n%s", info.FullMethod, r, debug.Stack()))
				resp, err = nil, status.Error(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

// GracefulStop lets in-flight RPCs finish until ctx is done, then closes the
// rest. It reports whether the stop was graceful.
func GracefulStop(ctx context.Context, srv *grpc.Server) bool {
	stopped := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
		return true
	case <-ctx.Done():
		srv.Stop() // GracefulStop returns once Stop has closed everything
		<-stopped
		return false
	}
}
