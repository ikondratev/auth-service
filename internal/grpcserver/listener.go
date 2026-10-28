package grpcserver

import (
	"context"
	"errors"
	"log/slog"
	"net"

	"github.com/ikondratev/auth-service/internal/authpb"
	"github.com/ikondratev/auth-service/internal/settings"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type Listener struct {
	engine   *grpc.Server
	health   *health.Server
	settings *settings.Settings
	log      *slog.Logger
}

func NewListener(s *settings.Settings, l *slog.Logger, auth *AuthServer) *Listener {
	engine := grpc.NewServer()
	hs := health.NewServer()
	healthpb.RegisterHealthServer(engine, hs)
	authpb.RegisterAuthServiceServer(engine, auth)

	return &Listener{
		engine:   engine,
		health:   hs,
		settings: s,
		log:      l,
	}
}

func (l *Listener) Start(errCh chan<- error) {
	lis, err := net.Listen("tcp", l.settings.GRPC.Port)
	if err != nil {
		errCh <- err
		return
	}

	l.health.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	l.log.Info("grpc: server started", "addr", l.settings.GRPC.Port, "env", l.settings.Env)
	errCh <- l.engine.Serve(lis)
}

func (l *Listener) Stop(ctx context.Context) error {
	l.health.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)

	stopped := make(chan struct{})
	go func() {
		l.engine.GracefulStop()
		close(stopped)
	}()

	select {
	case <-ctx.Done():
		l.engine.Stop()
		return ctx.Err()
	case <-stopped:
		return nil
	}
}

func IsStopped(err error) bool {
	return errors.Is(err, grpc.ErrServerStopped)
}
