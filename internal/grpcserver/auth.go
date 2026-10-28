package grpcserver

import (
	"context"
	"log/slog"

	"github.com/ikondratev/auth-service/internal/authpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthServer struct {
	logger *slog.Logger
	authpb.UnimplementedAuthServiceServer
}

func NewAuthServer(l *slog.Logger) *AuthServer {
	return &AuthServer{logger: l}
}

func (s *AuthServer) Register(context.Context, *authpb.RegisterRequest) (*authpb.TokenPair, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
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
