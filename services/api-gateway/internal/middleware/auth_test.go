package middleware

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"api-gateway/internal/clients"
	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
	authv1 "github.com/gauss2302/blogmicrobackend/proto/auth/v1"
)

type failingAuth struct {
	authv1.UnimplementedAuthServiceServer
	err error
}

func (f *failingAuth) ValidateToken(context.Context, *authv1.ValidateTokenRequest) (*authv1.ValidateTokenResponse, error) {
	return nil, f.err
}

func TestAuthMiddlewareStatusMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		code codes.Code
		want int
	}{
		{codes.Unauthenticated, http.StatusUnauthorized},
		{codes.InvalidArgument, http.StatusUnauthorized}, // e.g. a refresh token sent as access
		{codes.Unavailable, http.StatusServiceUnavailable},
		{codes.DeadlineExceeded, http.StatusGatewayTimeout},
		{codes.Internal, http.StatusInternalServerError},
	}
	for _, tt := range tests {
		lis, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		srv := grpc.NewServer()
		authv1.RegisterAuthServiceServer(srv, &failingAuth{err: status.Error(tt.code, "x")})
		go srv.Serve(lis)
		client, err := clients.NewAuthClient(lis.Addr().String(), config.GRPCTLSConfig{}, logger.New("fatal"))
		if err != nil {
			t.Fatal(err)
		}

		r := gin.New()
		r.Use(AuthMiddleware(client))
		r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer t")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		client.Close()
		srv.Stop()
		if w.Code != tt.want {
			t.Errorf("%v: status = %d, want %d", tt.code, w.Code, tt.want)
		}
	}
}
