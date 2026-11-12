package postgres

import (
	"context"
	"database/sql"
	"time"

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

func (r *RefreshRepo) FindByHash(ctx context.Context, tokenHash string) (*session.RefreshToken, error) {
	tx, err := requireTx(ctx)
	if err != nil {
		return nil, err
	}

	const query = `
		SELECT id, user_id, token_hash, family_id, expires_at, revoked_at
		FROM refresh_tokens
		WHERE token_hash = $1
		FOR UPDATE
	`
	var t session.RefreshToken
	err = tx.QueryRowContext(ctx, query, tokenHash).Scan(
		&t.ID, &t.UserID, &t.TokenHash, &t.FamilyID, &t.ExpiresAt, &t.RevokedAt,
	)

	if err != nil {
		return nil, err
	}

	return &t, nil
}

func (r *RefreshRepo) Revoke(ctx context.Context, id string, at time.Time) error {
	tx, err := requireTx(ctx)
	if err != nil {
		return err
	}

	const query = `
		UPDATE refresh_tokens
		SET revoked_at = $2
		WHERE id = $1 AND revoked_at IS NULL
	`

	res, err := tx.ExecContext(ctx, query, id, at)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if n == 0 {
		return sql.ErrNoRows
	}

	return nil
}
