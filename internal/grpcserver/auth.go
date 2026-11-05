package grpcserver

import (
	"context"
	"errors"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ikondratev/auth-service/internal/authpb"
	authsvc "github.com/ikondratev/auth-service/internal/service/auth"
)

type AuthServer struct {
	svc    *authsvc.Service
	logger *slog.Logger
	authpb.UnimplementedAuthServiceServer
}

func NewAuthServer(l *slog.Logger, svc *authsvc.Service) *AuthServer {
	return &AuthServer{
		logger: l,
		svc:    svc,
	}
}

func (s *AuthServer) Register(ctx context.Context, req *authpb.RegisterRequest) (*authpb.TokenPair, error) {
	pair, err := s.svc.Register(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		switch {
		case errors.Is(err, authsvc.ErrInvalidEmail), errors.Is(err, authsvc.ErrInvalidPassword):
			return nil, status.Error(codes.InvalidArgument, "invalid email or password")
		case errors.Is(err, authsvc.ErrEmailTaken):
			return nil, status.Error(codes.AlreadyExists, "email already exists")
		default:
			return nil, status.Error(codes.Internal, "register failed")
		}
	}

	return &authpb.TokenPair{
		AccessToken:     pair.AccessToken,
		RefreshToken:    pair.RefreshToken,
		AccessExpiresAt: pair.AccessExpiresAt.Unix(),
		TokenType:       pair.TokenType,
	}, nil

}
func (s *AuthServer) Login(context.Context, *authpb.LoginRequest) (*authpb.TokenPair, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}
func (s *AuthServer) Refresh(context.Context, *authpb.RefreshRequest) (*authpb.TokenPair, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}
func (s *AuthServer) Logout(context.Context, *authpb.LogoutRequest) (*authpb.LogoutResponse, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}
