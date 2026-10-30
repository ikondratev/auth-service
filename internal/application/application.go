package application

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ikondratev/auth-service/internal/grpcserver"
	"github.com/ikondratev/auth-service/internal/logger"
	"github.com/ikondratev/auth-service/internal/settings"
	"github.com/ikondratev/auth-service/internal/storage"
	"github.com/ikondratev/auth-service/internal/token"
)

type Application struct {
	settings *settings.Settings
	server   *grpcserver.Listener
	logger   *slog.Logger
	db       *storage.DB
	token    *token.Issuer
}

func New(e string) (*Application, error) {
	settings, err := settings.New(e)
	if err != nil {
		return nil, err
	}

	logger := logger.New(e)

	issToken, err := token.NewIssuer(settings)
	if err != nil {
		return nil, err
	}

	auth := grpcserver.NewAuthServer(logger)

	db, err := storage.New(context.Background(), settings)
	if err != nil {
		logger.Error("app: open db error", "err", err)
		return nil, err
	}

	return &Application{
		settings: settings,
		logger:   logger,
		server:   grpcserver.NewListener(settings, logger, auth),
		db:       db,
		token:    issToken,
	}, nil
}

func (a *Application) Run() (err error) {
	defer func() {
		err = errors.Join(err, a.db.Stop())
	}()

	chErr := make(chan error)
	go a.server.Start(chErr)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err = <-chErr:
		if !grpcserver.IsStopped(err) {
			a.logger.Error("app: stop grpc server error", "err", err)
			return err
		}
	case sig := <-quit:
		a.logger.Info("app: received signal", "sig", sig)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(a.settings.Server.WaitingShutdownSec)*time.Second,
	)
	defer cancel()

	if err = a.server.Stop(ctx); err != nil {
		a.logger.Error("app: shutdown grpc server error", "err", err)
		return err
	}

	a.logger.Info("app: server stopped grasefully")
	return nil
}
