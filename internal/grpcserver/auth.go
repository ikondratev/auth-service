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
	return s.authHandle(ctx, "register", req.GetEmail(), req.GetPassword(), s.svc.Register)
}
func (s *AuthServer) Login(ctx context.Context, req *authpb.LoginRequest) (*authpb.TokenPair, error) {
	return s.authHandle(ctx, "login", req.GetEmail(), req.GetPassword(), s.svc.Login)

}
func (s *AuthServer) Refresh(context.Context, *authpb.RefreshRequest) (*authpb.TokenPair, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}
func (s *AuthServer) Logout(context.Context, *authpb.LogoutRequest) (*authpb.LogoutResponse, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}

func (s *AuthServer) authHandle(
	ctx context.Context,
	opt, email, password string,
	fn func(context.Context, string, string) (authsvc.TokenPair, error),
) (*authpb.TokenPair, error) {
	pair, err := fn(ctx, email, password)
	if err != nil {
		switch {
		case errors.Is(err, authsvc.ErrInvalidEmail):
			return nil, status.Error(codes.InvalidArgument, "invalid email")
		case errors.Is(err, authsvc.ErrInvalidPassword):
			return nil, status.Error(codes.InvalidArgument, "invalid password")
		case errors.Is(err, authsvc.ErrEmailTaken):
			return nil, status.Error(codes.AlreadyExists, "email already exists")
		case errors.Is(err, authsvc.ErrInvalidCredentials):
			return nil, status.Error(codes.Unauthenticated, "invalid email or password")
		default:
			s.logger.Error(opt+" failed", "err", err)
			return nil, status.Error(codes.Internal, opt+" failed")
		}
	}

	return &authpb.TokenPair{
		AccessToken:     pair.AccessToken,
		RefreshToken:    pair.RefreshToken,
		AccessExpiresAt: pair.AccessExpiresAt.Unix(),
		TokenType:       pair.TokenType,
	}, nil
}
