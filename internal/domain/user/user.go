package user

import "context"

type User struct {
	ID           string
	Email        string
	PasswordHash string
	Status       int
}

type Repo interface {
	Create(ctx context.Context, u *User) error
	Find(ctx context.Context, email string) (*User, error)
}
