package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"github.com/ikondratev/auth-service/internal/domain/session"
	"github.com/ikondratev/auth-service/internal/domain/user"
)

const (
	minPassLendth = 8
	tokenType     = "Bearer"
)

var statuses = map[string]int{
	"active":   1,
	"disabled": 0,
}

type Service struct {
	tx         Transactor
	users      user.Repo
	sessions   session.Repo
	tokens     AccessIssuer
	refreshTTL time.Duration
}

type TokenPair struct {
	AccessToken     string
	RefreshToken    string
	AccessExpiresAt time.Time
	TokenType       string
}

type Transactor interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type AccessIssuer interface {
	AccessToken(userID, email string) (string, time.Time, error)
}

func New(
	tx Transactor,
	users user.Repo,
	sessions session.Repo,
	tokens AccessIssuer,
	refreshTTL time.Duration,
) *Service {
	return &Service{
		tx:         tx,
		users:      users,
		sessions:   sessions,
		tokens:     tokens,
		refreshTTL: refreshTTL,
	}
}

func (s *Service) Register(ctx context.Context, email, password string) (TokenPair, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return TokenPair{}, ErrInvalidEmail
	}

	if len(password) < minPassLendth {
		return TokenPair{}, ErrInvalidPassword
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return TokenPair{}, ErrInvalidPassword
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return TokenPair{}, errors.Join(ErrToken, fmt.Errorf("refresh token: %w", err))
	}

	refresh := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(refresh))

	u := &user.User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: string(hash),
		Status:       statuses["active"],
	}
	sessionToken := &session.RefreshToken{
		ID:        uuid.NewString(),
		UserID:    u.ID,
		TokenHash: hex.EncodeToString(sum[:]),
		FamilyID:  uuid.NewString(),
		ExpiresAt: time.Now().Add(s.refreshTTL),
	}

	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.users.Create(ctx, u); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == alreadyRegisterErrCode {
				return ErrEmailTaken
			}
			return err
		}
		return s.sessions.Create(ctx, sessionToken)
	})
	if err != nil {
		return TokenPair{}, err
	}

	access, exp, err := s.tokens.AccessToken(u.ID, u.Email)
	if err != nil {
		return TokenPair{}, errors.Join(ErrToken, fmt.Errorf("access token err %w", err))
	}

	return TokenPair{
		AccessToken:     access,
		RefreshToken:    refresh,
		AccessExpiresAt: exp,
		TokenType:       tokenType,
	}, nil
}
