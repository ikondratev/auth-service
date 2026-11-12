package session

import (
	"context"
	"time"
)

type RefreshToken struct {
	ID        string
	UserID    string
	TokenHash string
	FamilyID  string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

type Repo interface {
	Create(ctx context.Context, t *RefreshToken) error
	FindByHash(ctx context.Context, tokenHash string) (*RefreshToken, error)
	Revoke(ctx context.Context, id string, at time.Time) error
}
