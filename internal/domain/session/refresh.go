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
}

type Repo interface {
	Create(ctx context.Context, t *RefreshToken) error
}
