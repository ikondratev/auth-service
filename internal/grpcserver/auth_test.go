package grpcserver_test

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ikondratev/auth-service/internal/authpb"
	"github.com/ikondratev/auth-service/internal/domain/session"
	"github.com/ikondratev/auth-service/internal/domain/user"
	"github.com/ikondratev/auth-service/internal/grpcserver"
	authsvc "github.com/ikondratev/auth-service/internal/service/auth"
)

type fakeTx struct{}

func (fakeTx) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type fakeUsers struct{}

func (fakeUsers) Create(context.Context, *user.User) error { return nil }
func (fakeUsers) Find(context.Context, string) (*user.User, error) {
	return nil, sql.ErrNoRows
}
func (fakeUsers) FindByID(context.Context, string) (*user.User, error) {
	return nil, sql.ErrNoRows
}

type fakeSessions struct {
	err error
}

func (fakeSessions) Create(context.Context, *session.RefreshToken) error { return nil }
func (f fakeSessions) FindByHash(context.Context, string) (*session.RefreshToken, error) {
	if f.err != nil {
		return nil, f.err
	}
	return nil, sql.ErrNoRows
}
func (fakeSessions) Revoke(context.Context, string, time.Time) error       { return nil }
func (fakeSessions) RevokeFamily(context.Context, string, time.Time) error { return nil }

type fakeIssuer struct{}

func (fakeIssuer) AccessToken(string, string) (string, time.Time, error) {
	return "", time.Time{}, nil
}

func newServer(sessions fakeSessions) *grpcserver.AuthServer {
	svc := authsvc.New(fakeTx{}, fakeUsers{}, sessions, fakeIssuer{}, time.Hour)
	return grpcserver.NewAuthServer(slog.New(slog.DiscardHandler), svc)
}

func TestLogoutMapsInvalidRefresh(t *testing.T) {
	srv := newServer(fakeSessions{})

	_, err := srv.Logout(context.Background(), &authpb.LogoutRequest{RefreshToken: "   "})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("got %v", err)
	}
	if st.Message() != "invalid refresh token" {
		t.Fatalf("message = %q", st.Message())
	}
}

func TestRefreshMapsStoreError(t *testing.T) {
	srv := newServer(fakeSessions{err: errors.New("db down")})

	_, err := srv.Refresh(context.Background(), &authpb.RefreshRequest{RefreshToken: "presented-refresh"})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Fatalf("got %v", err)
	}
}
