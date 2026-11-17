package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/ikondratev/auth-service/internal/domain/session"
	"github.com/ikondratev/auth-service/internal/repo/postgres"
	"github.com/ikondratev/auth-service/internal/storage"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	dsn := os.Getenv("AUTH_TEST_DSN")
	if dsn == "" {
		os.Exit(m.Run())
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		panic(err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Ping(); err != nil {
		panic(err)
	}
	if err := migrate(db); err != nil {
		panic(err)
	}

	testDB = db
	os.Exit(m.Run())
}

func migrate(db *sql.DB) error {
	if _, err := db.Exec(`DROP TABLE IF EXISTS refresh_tokens`); err != nil {
		return err
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS users`); err != nil {
		return err
	}

	_, file, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "000001_create_users.up.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, stmt := range strings.Split(string(body), ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func newRepo(t *testing.T) (*postgres.RefreshRepo, *storage.Transactor) {
	t.Helper()
	if testDB == nil {
		t.Skip("AUTH_TEST_DSN is not set")
	}
	return postgres.NewRefreshRepo(testDB), storage.NewTransactor(testDB)
}

func insertUser(t *testing.T, tx *storage.Transactor, id string) {
	t.Helper()
	err := tx.WithinTx(context.Background(), func(ctx context.Context) error {
		sqlTx, err := requireTestTx(ctx)
		if err != nil {
			return err
		}
		_, err = sqlTx.ExecContext(ctx, `
			INSERT INTO users (id, email, password_hash, status)
			VALUES ($1, $2, 'hash', 1)
		`, id, id+"@example.com")
		return err
	})
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
}

func requireTestTx(ctx context.Context) (*sql.Tx, error) {
	sqlTx, ok := storage.TxFrom(ctx)
	if !ok {
		return nil, errors.New("missing transaction")
	}
	return sqlTx, nil
}

func TestFindByHash(t *testing.T) {
	repo, tx := newRepo(t)
	userID := uuid.NewString()
	insertUser(t, tx, userID)

	expiresAt := time.Now().Add(time.Hour).Truncate(time.Microsecond).UTC()
	token := &session.RefreshToken{
		ID:        uuid.NewString(),
		UserID:    userID,
		TokenHash: uuid.NewString(),
		FamilyID:  uuid.NewString(),
		ExpiresAt: expiresAt,
	}
	err := tx.WithinTx(context.Background(), func(ctx context.Context) error {
		return repo.Create(ctx, token)
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	var got *session.RefreshToken
	err = tx.WithinTx(context.Background(), func(ctx context.Context) error {
		var findErr error
		got, findErr = repo.FindByHash(ctx, token.TokenHash)
		return findErr
	})
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.ID != token.ID || got.UserID != token.UserID || got.TokenHash != token.TokenHash || got.FamilyID != token.FamilyID {
		t.Fatalf("token = %+v", got)
	}
	if !got.ExpiresAt.Equal(expiresAt) || got.RevokedAt != nil {
		t.Fatalf("times expires=%s revoked=%v", got.ExpiresAt, got.RevokedAt)
	}

	err = tx.WithinTx(context.Background(), func(ctx context.Context) error {
		_, findErr := repo.FindByHash(ctx, "missing")
		return findErr
	})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing: got %v", err)
	}

	if _, err := repo.FindByHash(context.Background(), token.TokenHash); err == nil {
		t.Fatal("find outside a transaction succeeded")
	}
}

func TestRevoke(t *testing.T) {
	repo, tx := newRepo(t)
	userID := uuid.NewString()
	insertUser(t, tx, userID)

	token := &session.RefreshToken{
		ID:        uuid.NewString(),
		UserID:    userID,
		TokenHash: uuid.NewString(),
		FamilyID:  uuid.NewString(),
		ExpiresAt: time.Now().Add(time.Hour).UTC(),
	}
	err := tx.WithinTx(context.Background(), func(ctx context.Context) error {
		return repo.Create(ctx, token)
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	revokedAt := time.Now().Truncate(time.Microsecond).UTC()
	err = tx.WithinTx(context.Background(), func(ctx context.Context) error {
		return repo.Revoke(ctx, token.ID, revokedAt)
	})
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}

	var got *session.RefreshToken
	err = tx.WithinTx(context.Background(), func(ctx context.Context) error {
		var findErr error
		got, findErr = repo.FindByHash(ctx, token.TokenHash)
		return findErr
	})
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.RevokedAt == nil || !got.RevokedAt.Equal(revokedAt) {
		t.Fatalf("revoked_at = %v", got.RevokedAt)
	}

	err = tx.WithinTx(context.Background(), func(ctx context.Context) error {
		return repo.Revoke(ctx, token.ID, time.Now())
	})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("second revoke: got %v", err)
	}
}

func TestRevokeFamily(t *testing.T) {
	repo, tx := newRepo(t)
	userID := uuid.NewString()
	insertUser(t, tx, userID)

	familyID := uuid.NewString()
	otherFamily := uuid.NewString()
	live := newToken(userID, familyID)
	already := newToken(userID, familyID)
	other := newToken(userID, otherFamily)
	err := tx.WithinTx(context.Background(), func(ctx context.Context) error {
		for _, token := range []*session.RefreshToken{live, already, other} {
			if err := repo.Create(ctx, token); err != nil {
				return err
			}
		}
		return repo.Revoke(ctx, already.ID, time.Now().Add(-time.Minute).UTC())
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	revokedAt := time.Now().Truncate(time.Microsecond).UTC()
	err = tx.WithinTx(context.Background(), func(ctx context.Context) error {
		return repo.RevokeFamily(ctx, familyID, revokedAt)
	})
	if err != nil {
		t.Fatalf("revoke family: %v", err)
	}

	assertRevokedAt := func(hash string, want *time.Time) {
		t.Helper()
		var got *session.RefreshToken
		err := tx.WithinTx(context.Background(), func(ctx context.Context) error {
			var findErr error
			got, findErr = repo.FindByHash(ctx, hash)
			return findErr
		})
		if err != nil {
			t.Fatalf("find %s: %v", hash, err)
		}
		if want == nil {
			if got.RevokedAt != nil {
				t.Fatalf("revoked_at = %v", got.RevokedAt)
			}
			return
		}
		if got.RevokedAt == nil || !got.RevokedAt.Equal(*want) {
			t.Fatalf("revoked_at = %v, want %v", got.RevokedAt, want)
		}
	}
	assertRevokedAt(live.TokenHash, &revokedAt)
	assertRevokedAt(other.TokenHash, nil)

	err = tx.WithinTx(context.Background(), func(ctx context.Context) error {
		return repo.RevokeFamily(ctx, familyID, time.Now())
	})
	if err != nil {
		t.Fatalf("second revoke family: %v", err)
	}
}

func newToken(userID, familyID string) *session.RefreshToken {
	return &session.RefreshToken{
		ID:        uuid.NewString(),
		UserID:    userID,
		TokenHash: uuid.NewString(),
		FamilyID:  familyID,
		ExpiresAt: time.Now().Add(time.Hour).UTC(),
	}
}
