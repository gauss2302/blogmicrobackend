package grpc

import (
	"context"
	"net/http"

	appErrors "auth-service/internal/application/errors"
	"auth-service/internal/application/services"
	"auth-service/internal/application/services/dto"
	"auth-service/pkg/logger"

	authv1 "github.com/nikitashilov/microblog_grpc/proto/auth/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// AuthServer exposes AuthService functionality over gRPC.
type AuthServer struct {
	authv1.UnimplementedAuthServiceServer
	service *services.AuthService
	logger  *logger.Logger
}

func NewAuthServer(service *services.AuthService, logger *logger.Logger) *AuthServer {
	return &AuthServer{service: service, logger: logger}
}

func (s *AuthServer) GetGoogleAuthURL(ctx context.Context, _ *emptypb.Empty) (*authv1.GetGoogleAuthURLResponse, error) {
	resp, err := s.service.GetGoogleAuthURL(ctx)
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	return &authv1.GetGoogleAuthURLResponse{
		AuthUrl: resp.AuthURL,
		State:   resp.State,
	}, nil
}

func (s *AuthServer) HandleGoogleCallback(ctx context.Context, req *authv1.GoogleCallbackRequest) (*authv1.GoogleCallbackResponse, error) {
	dtoReq := &dto.GoogleCallbackRequest{
		State: req.GetState(),
		Code:  req.GetCode(),
	}

	resp, err := s.service.HandleGoogleCallback(ctx, dtoReq)
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	return &authv1.GoogleCallbackResponse{AuthCode: resp.AuthCode}, nil
}

func (s *AuthServer) ExchangeAuthCode(ctx context.Context, req *authv1.ExchangeAuthCodeRequest) (*authv1.ExchangeAuthCodeResponse, error) {
	dtoReq := &dto.ExchangeAuthCodeRequest{AuthCode: req.GetAuthCode()}

	resp, err := s.service.ExchangeAuthCode(ctx, dtoReq)
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	return &authv1.ExchangeAuthCodeResponse{
		User:   toProtoUser(resp.User),
		Tokens: toProtoTokens(resp.Tokens),
	}, nil
}

func (s *AuthServer) RefreshToken(ctx context.Context, req *authv1.RefreshTokenRequest) (*authv1.RefreshTokenResponse, error) {
	dtoReq := &dto.RefreshTokenRequest{RefreshToken: req.GetRefreshToken()}

	resp, err := s.service.RefreshToken(ctx, dtoReq)
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	return &authv1.RefreshTokenResponse{
		User:   toProtoUser(resp.User),
		Tokens: toProtoTokens(resp.Tokens),
	}, nil
}

func (s *AuthServer) Logout(ctx context.Context, req *authv1.LogoutRequest) (*emptypb.Empty, error) {
	dtoReq := &dto.LogoutRequest{AccessToken: req.GetAccessToken()}

	if err := s.service.Logout(ctx, dtoReq); err != nil {
		return nil, s.toGRPCError(err)
	}

	return &emptypb.Empty{}, nil
}

func (s *AuthServer) ValidateToken(ctx context.Context, req *authv1.ValidateTokenRequest) (*authv1.ValidateTokenResponse, error) {
	resp, err := s.service.ValidateToken(ctx, req.GetToken())
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	return &authv1.ValidateTokenResponse{
		Valid:  resp.Valid,
		UserId: resp.UserID,
		Email:  resp.Email,
	}, nil
}

func (s *AuthServer) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.RegisterResponse, error) {
	resp, err := s.service.Register(ctx, req.GetEmail(), req.GetPassword(), req.GetName())
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	return &authv1.RegisterResponse{
		User:   toProtoUser(resp.User),
		Tokens: toProtoTokens(resp.Tokens),
	}, nil
}

func (s *AuthServer) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	resp, err := s.service.Login(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	return &authv1.LoginResponse{
		User:   toProtoUser(resp.User),
		Tokens: toProtoTokens(resp.Tokens),
	}, nil
}

func (s *AuthServer) HealthCheck(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s *AuthServer) toGRPCError(err error) error {
	if err == nil {
		return nil
	}

	if authErr, ok := err.(*appErrors.AuthError); ok {
		switch authErr.StatusCode {
		case http.StatusBadRequest:
			return status.Error(codes.InvalidArgument, authErr.Message)
		case http.StatusUnauthorized:
			return status.Error(codes.Unauthenticated, authErr.Message)
		case http.StatusForbidden:
			return status.Error(codes.PermissionDenied, authErr.Message)
		case http.StatusNotFound:
			return status.Error(codes.NotFound, authErr.Message)
		case http.StatusConflict:
			return status.Error(codes.AlreadyExists, authErr.Message)
		case http.StatusTooManyRequests:
			return status.Error(codes.ResourceExhausted, authErr.Message)
		case http.StatusServiceUnavailable:
			return status.Error(codes.Unavailable, authErr.Message)
		default:
			return status.Error(codes.Internal, authErr.Message)
		}
	}

	s.logger.Error("unexpected error: " + err.Error())
	return status.Error(codes.Internal, "internal server error")
}

func toProtoUser(user *dto.UserInfo) *authv1.UserInfo {
	if user == nil {
		return nil
	}

	return &authv1.UserInfo{
		Id:      user.ID,
		Email:   user.Email,
		Name:    user.Name,
		Picture: user.Picture,
	}
}

func toProtoTokens(tokens *dto.TokenPair) *authv1.TokenPair {
	if tokens == nil {
		return nil
	}

	return &authv1.TokenPair{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		TokenType:    tokens.TokenType,
		ExpiresIn:    int32(tokens.ExpiresIn),
	}
}
