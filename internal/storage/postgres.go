package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/ikondratev/auth-service/internal/settings"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type DB struct {
	Adapter *sql.DB
}

func New(ctx context.Context, settings *settings.Settings) (*DB, error) {
	db, err := sql.Open("pgx", settings.DB.Url)
	if err != nil {
		return nil, errors.Join(ErrOpenConnection, err)
	}

	db.SetConnMaxIdleTime(time.Duration(settings.DB.MaxLifeTime))
	db.SetMaxIdleConns(int(settings.DB.MaxIdleConnection))
	db.SetConnMaxLifetime(time.Duration(settings.DB.MaxLifeTime) * time.Second)
	db.SetMaxOpenConns(settings.DB.OpenConnection)

	pingCtx, cancel := context.WithTimeout(
		ctx,
		time.Duration(settings.DB.StartDelay)*time.Second,
	)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, errors.Join(ErrOpenConnection, err)
	}

	return &DB{Adapter: db}, nil
}

func (d *DB) Stop() error {
	if err := d.Adapter.Close(); err != nil {
		return errors.Join(ErrCloseConnection, err)
	}

	return nil
}
