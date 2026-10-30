package postgres

import (
	"context"
	"database/sql"

	"github.com/ikondratev/auth-service/internal/domain/session"
)

type RefreshRepo struct {
	db *sql.DB
}

func NewRefreshRepo(db *sql.DB) *RefreshRepo {
	return &RefreshRepo{db: db}
}

var _ session.Repo = (*RefreshRepo)(nil)

func (r *RefreshRepo) Create(ctx context.Context, t *session.RefreshToken) error {
	tx, err := requireTx(ctx)
	if err != nil {
		return err
	}

	const query = `
		INSERT INTO refresh_tokens (id, user_id, token_hash, family_id, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err = tx.ExecContext(ctx, query, t.ID, t.UserID, t.TokenHash, t.FamilyID, t.ExpiresAt)
	return err
}
