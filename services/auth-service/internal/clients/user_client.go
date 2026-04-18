package clients

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"auth-service/internal/config"
	userv1 "github.com/nikitashilov/microblog_grpc/proto/user/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

const defaultUserTimeout = 10 * time.Second

// UserClient wraps gRPC communication with the user service (CreateUser, ValidateCredentials).
type UserClient struct {
	conn   *grpc.ClientConn
	client userv1.UserServiceClient
}

// NewUserClient creates a gRPC client for the user service.
func NewUserClient(addr string, tlsCfg config.GRPCTLSConfig) (*UserClient, error) {
	creds, err := buildClientTransportCredentials(tlsCfg)
	if err != nil {
		return nil, fmt.Errorf("build user client transport credentials: %w", err)
	}

	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(creds),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to user gRPC service: %w", err)
	}

	return &UserClient{
		conn:   conn,
		client: userv1.NewUserServiceClient(conn),
	}, nil
}

// CreateUser creates a user in user-service (id optional for email/password signup).
func (c *UserClient) CreateUser(ctx context.Context, id, email, name, picture, password string) (*userv1.User, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultUserTimeout)
	defer cancel()

	req := &userv1.CreateUserRequest{
		Id:       id,
		Email:    email,
		Name:     name,
		Picture:  picture,
		Password: password,
	}

	resp, err := c.client.CreateUser(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// ValidateCredentials validates email/password and returns user id, email, name, picture.
func (c *UserClient) ValidateCredentials(ctx context.Context, email, password string) (*userv1.ValidateCredentialsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultUserTimeout)
	defer cancel()

	req := &userv1.ValidateCredentialsRequest{
		Email:    email,
		Password: password,
	}

	resp, err := c.client.ValidateCredentials(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// GetUserByEmail returns a user record by email.
func (c *UserClient) GetUserByEmail(ctx context.Context, email string) (*userv1.User, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultUserTimeout)
	defer cancel()

	req := &userv1.GetUserByEmailRequest{
		Email: email,
	}

	resp, err := c.client.GetUserByEmail(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// Close closes the gRPC connection.
func (c *UserClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func buildClientTransportCredentials(tlsCfg config.GRPCTLSConfig) (credentials.TransportCredentials, error) {
	if !tlsCfg.Enabled {
		return insecure.NewCredentials(), nil
	}

	caPEM, err := os.ReadFile(tlsCfg.CAFile)
	if err != nil {
		return nil, fmt.Errorf("read gRPC CA file: %w", err)
	}

	rootCAs := x509.NewCertPool()
	if ok := rootCAs.AppendCertsFromPEM(caPEM); !ok {
		return nil, fmt.Errorf("parse gRPC CA certificate")
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    rootCAs,
	}

	if tlsCfg.CertFile != "" && tlsCfg.KeyFile != "" {
		clientCert, certErr := tls.LoadX509KeyPair(tlsCfg.CertFile, tlsCfg.KeyFile)
		if certErr != nil {
			return nil, fmt.Errorf("load gRPC client certificate: %w", certErr)
		}
		tlsConfig.Certificates = []tls.Certificate{clientCert}
	}

	return credentials.NewTLS(tlsConfig), nil
}
