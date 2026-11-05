package postgres

import (
	"context"
	"database/sql"

	"github.com/ikondratev/auth-service/internal/domain/user"
)

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

var _ user.Repo = (*UserRepo)(nil)

func (r *UserRepo) Create(ctx context.Context, u *user.User) error {
	tx, err := requireTx(ctx)
	if err != nil {
		return err
	}

	const query = `
		INSERT INTO users (id, email, password_hash, status)
		VALUES ($1, $2, $3, $4)
	`
	_, err = tx.ExecContext(ctx, query, u.ID, u.Email, u.PasswordHash, u.Status)
	return err
}
