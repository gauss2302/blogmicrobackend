package clients

import (
	"context"
	"fmt"
	"time"

	authv1 "github.com/nikitashilov/microblog_grpc/proto/auth/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"api-gateway/pkg/logger"
)

const defaultAuthTimeout = 10 * time.Second

var (
	// Keepalive parameters for gRPC client connections
	keepaliveTime                = 30 * time.Second
	keepaliveTimeout             = 5 * time.Second
	keepalivePermitWithoutStream = true
)

type AuthClient struct {
	conn   *grpc.ClientConn
	client authv1.AuthServiceClient
	logger *logger.Logger
}

func NewAuthClient(addr string, logger *logger.Logger) (*AuthClient, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                keepaliveTime,
			Timeout:             keepaliveTimeout,
			PermitWithoutStream: keepalivePermitWithoutStream,
		}),
		grpc.WithUnaryInterceptor(unaryClientLoggingInterceptor(logger)),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to auth gRPC service: %w", err)
	}

	return &AuthClient{
		conn:   conn,
		client: authv1.NewAuthServiceClient(conn),
		logger: logger,
	}, nil
}

func (c *AuthClient) GetGoogleAuthURL(ctx context.Context, req *authv1.GetGoogleAuthURLRequest) (*authv1.GetGoogleAuthURLResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultAuthTimeout)
	defer cancel()

	if req == nil {
		req = &authv1.GetGoogleAuthURLRequest{}
	}

	resp, err := c.client.GetGoogleAuthURL(ctx, req)
	if err != nil {
		return nil, c.wrapError("get google auth url", err)
	}

	return resp, nil
}

func (c *AuthClient) HandleGoogleCallback(ctx context.Context, state, code string) (*authv1.GoogleCallbackResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultAuthTimeout)
	defer cancel()

	req := &authv1.GoogleCallbackRequest{State: state, Code: code}
	resp, err := c.client.HandleGoogleCallback(ctx, req)
	if err != nil {
		return nil, c.wrapError("handle google callback", err)
	}

	return resp, nil
}

func (c *AuthClient) ExchangeAuthCode(ctx context.Context, authCode string) (*authv1.ExchangeAuthCodeResponse, error) {
	return c.ExchangeAuthCodeWithVerifier(ctx, authCode, "")
}

func (c *AuthClient) ExchangeAuthCodeWithVerifier(ctx context.Context, authCode, codeVerifier string) (*authv1.ExchangeAuthCodeResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultAuthTimeout)
	defer cancel()

	req := &authv1.ExchangeAuthCodeRequest{AuthCode: authCode, CodeVerifier: codeVerifier}
	resp, err := c.client.ExchangeAuthCode(ctx, req)
	if err != nil {
		return nil, c.wrapError("exchange auth code", err)
	}

	return resp, nil
}

func (c *AuthClient) RefreshToken(ctx context.Context, refreshToken string) (*authv1.RefreshTokenResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultAuthTimeout)
	defer cancel()

	req := &authv1.RefreshTokenRequest{RefreshToken: refreshToken}
	resp, err := c.client.RefreshToken(ctx, req)
	if err != nil {
		return nil, c.wrapError("refresh token", err)
	}

	return resp, nil
}

func (c *AuthClient) Logout(ctx context.Context, accessToken string) error {
	ctx, cancel := context.WithTimeout(ctx, defaultAuthTimeout)
	defer cancel()

	req := &authv1.LogoutRequest{AccessToken: accessToken}
	if _, err := c.client.Logout(ctx, req); err != nil {
		return c.wrapError("logout", err)
	}

	return nil
}

func (c *AuthClient) ValidateToken(ctx context.Context, token string) (*authv1.ValidateTokenResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultAuthTimeout)
	defer cancel()

	req := &authv1.ValidateTokenRequest{Token: token}
	resp, err := c.client.ValidateToken(ctx, req)
	if err != nil {
		return nil, c.wrapError("validate token", err)
	}

	return resp, nil
}

func (c *AuthClient) Register(ctx context.Context, email, password, name string) (*authv1.RegisterResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultAuthTimeout)
	defer cancel()

	req := &authv1.RegisterRequest{Email: email, Password: password, Name: name}
	resp, err := c.client.Register(ctx, req)
	if err != nil {
		return nil, c.wrapError("register", err)
	}

	return resp, nil
}

func (c *AuthClient) Login(ctx context.Context, email, password string) (*authv1.LoginResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultAuthTimeout)
	defer cancel()

	req := &authv1.LoginRequest{Email: email, Password: password}
	resp, err := c.client.Login(ctx, req)
	if err != nil {
		return nil, c.wrapError("login", err)
	}

	return resp, nil
}

func (c *AuthClient) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if _, err := c.client.HealthCheck(ctx, &emptypb.Empty{}); err != nil {
		return c.wrapError("health check", err)
	}

	return nil
}

func (c *AuthClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *AuthClient) wrapError(action string, err error) error {
	if err == nil {
		return nil
	}

	if st, ok := status.FromError(err); ok {
		return status.Errorf(st.Code(), "%s: %s", action, st.Message())
	}

	return fmt.Errorf("%s: %w", action, err)
}

// unaryClientLoggingInterceptor logs gRPC client requests and responses
func unaryClientLoggingInterceptor(logger *logger.Logger) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		duration := time.Since(start)

		if err != nil {
			if st, ok := status.FromError(err); ok {
				logger.Warn(fmt.Sprintf("gRPC call %s failed: %s (code: %s, duration: %v)", method, st.Message(), st.Code(), duration))
			} else {
				logger.Warn(fmt.Sprintf("gRPC call %s failed: %v (duration: %v)", method, err, duration))
			}
		} else {
			logger.Debug(fmt.Sprintf("gRPC call %s succeeded (duration: %v)", method, duration))
		}

		return err
	}
}

func IsUnauthenticatedError(err error) bool {
	if err == nil {
		return false
	}

	if st, ok := status.FromError(err); ok {
		return st.Code() == codes.Unauthenticated
	}

	return false
}
