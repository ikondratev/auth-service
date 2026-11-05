package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ikondratev/auth-service/internal/storage"
)

func requireTx(ctx context.Context) (*sql.Tx, error) {
	tx, ok := storage.TxFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("missing transaction")
	}
	return tx, nil
}
