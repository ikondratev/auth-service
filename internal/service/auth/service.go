package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
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
	if err := verify(email, password); err != nil {
		return TokenPair{}, err
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

func (s *Service) Login(ctx context.Context, email, password string) (TokenPair, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if err := verify(email, password); err != nil {
		return TokenPair{}, err
	}

	u, err := s.users.Find(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TokenPair{}, ErrInvalidCredentials
		}
		return TokenPair{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return TokenPair{}, ErrInvalidCredentials
	}

	if u.Status != statuses["active"] {
		return TokenPair{}, ErrInvalidCredentials
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return TokenPair{}, errors.Join(ErrToken, fmt.Errorf("refresh token: %w", err))
	}

	refresh := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(refresh))
	sessionToken := &session.RefreshToken{
		ID:        uuid.NewString(),
		UserID:    u.ID,
		TokenHash: hex.EncodeToString(sum[:]),
		FamilyID:  uuid.NewString(),
		ExpiresAt: time.Now().Add(s.refreshTTL),
	}

	if err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		return s.sessions.Create(ctx, sessionToken)
	}); err != nil {
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

func (s *Service) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return TokenPair{}, ErrInvalidRefresh
	}

	sum := sha256.Sum256([]byte(refreshToken))
	hash := hex.EncodeToString(sum[:])

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return TokenPair{}, errors.Join(ErrToken, fmt.Errorf("refresh token: %w", err))
	}
	next := base64.RawURLEncoding.EncodeToString(raw)
	nextSum := sha256.Sum256([]byte(next))

	var (
		userID string
		email  string
	)

	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		current, err := s.sessions.FindByHash(ctx, hash)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrInvalidRefresh
			}
			return err
		}
		if current.RevokedAt != nil || !current.ExpiresAt.After(time.Now()) {
			return ErrInvalidRefresh
		}

		u, err := s.users.FindByID(ctx, current.UserID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrInvalidRefresh
			}
			return err
		}
		if u.Status != statuses["active"] {
			return ErrInvalidRefresh
		}

		if err := s.sessions.Revoke(ctx, current.ID, time.Now()); err != nil {
			return err
		}

		sessionToken := &session.RefreshToken{
			ID:        uuid.NewString(),
			UserID:    current.UserID,
			TokenHash: hex.EncodeToString(nextSum[:]),
			FamilyID:  current.FamilyID,
			ExpiresAt: time.Now().Add(s.refreshTTL),
		}
		if err := s.sessions.Create(ctx, sessionToken); err != nil {
			return err
		}

		userID = u.ID
		email = u.Email
		return nil
	})
	if err != nil {
		return TokenPair{}, err
	}

	access, exp, err := s.tokens.AccessToken(userID, email)
	if err != nil {
		return TokenPair{}, errors.Join(ErrToken, fmt.Errorf("access token err %w", err))
	}

	return TokenPair{
		AccessToken:     access,
		RefreshToken:    next,
		AccessExpiresAt: exp,
		TokenType:       tokenType,
	}, nil
}

func verify(email, password string) error {
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return ErrInvalidEmail
	}

	if len(password) < minPassLendth {
		return ErrInvalidPassword
	}

	return nil
}
