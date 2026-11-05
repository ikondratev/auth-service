package storage

import (
	"context"
	"database/sql"
)

type txKey struct{}

func TxFrom(ctx context.Context) (*sql.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(*sql.Tx)
	return tx, ok
}

func withTx(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

type Transactor struct {
	DB *sql.DB
}

func NewTransactor(db *sql.DB) *Transactor {
	return &Transactor{DB: db}
}

func (t *Transactor) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := t.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err = fn(withTx(ctx, tx)); err != nil {
		return err
	}

	return tx.Commit()
}
