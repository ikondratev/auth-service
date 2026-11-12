package auth_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"github.com/ikondratev/auth-service/internal/domain/session"
	"github.com/ikondratev/auth-service/internal/domain/user"
	authsvc "github.com/ikondratev/auth-service/internal/service/auth"
)

type fakeTx struct {
	calls int
}

func (f *fakeTx) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	f.calls++
	return fn(ctx)
}

type fakeUsers struct {
	calls    int
	user     *user.User
	err      error
	findUser *user.User
	findErr  error
	findID   string
}

func (f *fakeUsers) Create(_ context.Context, u *user.User) error {
	f.calls++
	copied := *u
	f.user = &copied
	return f.err
}

func (f *fakeUsers) Find(context.Context, string) (*user.User, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if f.findUser == nil {
		return nil, sql.ErrNoRows
	}
	copied := *f.findUser
	return &copied, nil
}

func (f *fakeUsers) FindByID(_ context.Context, id string) (*user.User, error) {
	f.findID = id
	if f.findErr != nil {
		return nil, f.findErr
	}
	if f.findUser == nil {
		return nil, sql.ErrNoRows
	}
	copied := *f.findUser
	return &copied, nil
}

type fakeSessions struct {
	calls       int
	token       *session.RefreshToken
	err         error
	findToken   *session.RefreshToken
	findErr     error
	revokeCalls int
	revokeErr   error
}

func (f *fakeSessions) Create(_ context.Context, t *session.RefreshToken) error {
	f.calls++
	copied := *t
	f.token = &copied
	return f.err
}

func (f *fakeSessions) FindByHash(context.Context, string) (*session.RefreshToken, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if f.findToken == nil {
		return nil, sql.ErrNoRows
	}
	copied := *f.findToken
	return &copied, nil
}

func (f *fakeSessions) Revoke(context.Context, string, time.Time) error {
	f.revokeCalls++
	return f.revokeErr

}

type fakeIssuer struct {
	calls  int
	userID string
	email  string
	access string
	exp    time.Time
	err    error
}

func (f *fakeIssuer) AccessToken(userID, email string) (string, time.Time, error) {
	f.calls++
	f.userID = userID
	f.email = email
	return f.access, f.exp, f.err
}

func newService(users *fakeUsers, sessions *fakeSessions, issuer *fakeIssuer) (*authsvc.Service, *fakeTx) {
	tx := &fakeTx{}
	svc := authsvc.New(tx, users, sessions, issuer, 2*time.Hour)
	return svc, tx
}

func TestRegisterSuccess(t *testing.T) {
	users := &fakeUsers{}
	sessions := &fakeSessions{}
	exp := time.Now().Add(15 * time.Minute).Truncate(time.Second)
	issuer := &fakeIssuer{access: "access-token", exp: exp}
	svc, tx := newService(users, sessions, issuer)

	pair, err := svc.Register(context.Background(), "  Ada@Example.com ", "password1")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if tx.calls != 1 || users.calls != 1 || sessions.calls != 1 || issuer.calls != 1 {
		t.Fatalf("calls tx=%d users=%d sessions=%d issuer=%d", tx.calls, users.calls, sessions.calls, issuer.calls)
	}
	if users.user.Email != "ada@example.com" || users.user.Status != 1 {
		t.Fatalf("user = %+v", users.user)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(users.user.PasswordHash), []byte("password1")); err != nil {
		t.Fatalf("password hash: %v", err)
	}
	if sessions.token.UserID != users.user.ID || sessions.token.ID == "" || sessions.token.FamilyID == "" {
		t.Fatalf("refresh = %+v", sessions.token)
	}

	raw, err := base64.RawURLEncoding.DecodeString(pair.RefreshToken)
	if err != nil || len(raw) != 32 {
		t.Fatalf("refresh token = %q", pair.RefreshToken)
	}
	sum := sha256.Sum256([]byte(pair.RefreshToken))
	if sessions.token.TokenHash != hex.EncodeToString(sum[:]) {
		t.Fatalf("stored hash does not match refresh token")
	}

	deadline := time.Now().Add(2 * time.Hour)
	if sessions.token.ExpiresAt.Before(deadline.Add(-5*time.Second)) || sessions.token.ExpiresAt.After(deadline.Add(5*time.Second)) {
		t.Fatalf("expires at %s", sessions.token.ExpiresAt)
	}
	if pair.AccessToken != "access-token" || !pair.AccessExpiresAt.Equal(exp) || pair.TokenType != "Bearer" {
		t.Fatalf("pair = %+v", pair)
	}
	if issuer.userID != users.user.ID || issuer.email != "ada@example.com" {
		t.Fatalf("issuer got %s %s", issuer.userID, issuer.email)
	}
}

func TestRegisterRejectsInput(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
		want     error
	}{
		{name: "email", email: "not-an-email", password: "password1", want: authsvc.ErrInvalidEmail},
		{name: "named address", email: "Ada <ada@example.com>", password: "password1", want: authsvc.ErrInvalidEmail},
		{name: "short password", email: "ada@example.com", password: "short", want: authsvc.ErrInvalidPassword},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &fakeUsers{}
			sessions := &fakeSessions{}
			issuer := &fakeIssuer{}
			svc, tx := newService(users, sessions, issuer)

			_, err := svc.Register(context.Background(), tt.email, tt.password)
			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
			if tx.calls != 0 || users.calls != 0 || sessions.calls != 0 || issuer.calls != 0 {
				t.Fatalf("invalid input was saved")
			}
		})
	}
}

func TestRegisterEmailTaken(t *testing.T) {
	users := &fakeUsers{err: &pgconn.PgError{Code: "23505"}}
	sessions := &fakeSessions{}
	issuer := &fakeIssuer{}
	svc, _ := newService(users, sessions, issuer)

	_, err := svc.Register(context.Background(), "ada@example.com", "password1")
	if !errors.Is(err, authsvc.ErrEmailTaken) {
		t.Fatalf("got %v", err)
	}
	if sessions.calls != 0 || issuer.calls != 0 {
		t.Fatalf("email conflict continued, sessions=%d issuer=%d", sessions.calls, issuer.calls)
	}
}

func TestRegisterStopsWhenUserCreateFails(t *testing.T) {
	dbErr := errors.New("db down")
	users := &fakeUsers{err: dbErr}
	sessions := &fakeSessions{}
	issuer := &fakeIssuer{}
	svc, _ := newService(users, sessions, issuer)

	_, err := svc.Register(context.Background(), "ada@example.com", "password1")
	if !errors.Is(err, dbErr) {
		t.Fatalf("got %v", err)
	}
	if sessions.calls != 0 || issuer.calls != 0 {
		t.Fatalf("create continued after user error")
	}
}

func TestRegisterReturnsTokenErrorAfterSave(t *testing.T) {
	users := &fakeUsers{}
	sessions := &fakeSessions{}
	issuer := &fakeIssuer{err: errors.New("sign failed")}
	svc, _ := newService(users, sessions, issuer)

	_, err := svc.Register(context.Background(), "ada@example.com", "password1")
	if !errors.Is(err, authsvc.ErrToken) {
		t.Fatalf("got %v", err)
	}
	if users.calls != 1 || sessions.calls != 1 {
		t.Fatalf("rows were not saved before signing")
	}
}

func TestLoginSuccess(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("password1"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	users := &fakeUsers{findUser: &user.User{
		ID:           "user-1",
		Email:        "ada@example.com",
		PasswordHash: string(hash),
		Status:       1,
	}}

	sessions := &fakeSessions{}
	exp := time.Now().Add(15 * time.Minute).Truncate(time.Second)
	issuer := &fakeIssuer{access: "access-token", exp: exp}
	svc, tx := newService(users, sessions, issuer)
	pair, err := svc.Login(context.Background(), "  Ada@Example.com ", "password1")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	if tx.calls != 1 || users.calls != 0 || sessions.calls != 1 || issuer.calls != 1 {
		t.Fatalf("calls tx=%d users=%d sessions=%d issuer=%d", tx.calls, users.calls, sessions.calls, issuer.calls)
	}

	if sessions.token.UserID != "user-1" || sessions.token.ID == "" || sessions.token.FamilyID == "" {
		t.Fatalf("refresh = %+v", sessions.token)
	}

	raw, err := base64.RawURLEncoding.DecodeString(pair.RefreshToken)
	if err != nil || len(raw) != 32 {
		t.Fatalf("refresh token = %q", pair.RefreshToken)
	}

	sum := sha256.Sum256([]byte(pair.RefreshToken))
	if sessions.token.TokenHash != hex.EncodeToString(sum[:]) {
		t.Fatalf("stored hash does not match refresh token")
	}

	deadline := time.Now().Add(2 * time.Hour)
	if sessions.token.ExpiresAt.Before(deadline.Add(-5*time.Second)) || sessions.token.ExpiresAt.After(deadline.Add(5*time.Second)) {
		t.Fatalf("expires at %s", sessions.token.ExpiresAt)
	}

	if pair.AccessToken != "access-token" || !pair.AccessExpiresAt.Equal(exp) || pair.TokenType != "Bearer" {
		t.Fatalf("pair = %+v", pair)
	}

	if issuer.userID != "user-1" || issuer.email != "ada@example.com" {
		t.Fatalf("issuer got %s %s", issuer.userID, issuer.email)
	}
}

func TestLoginRejectsInput(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
		want     error
	}{
		{name: "email", email: "not-an-email", password: "password1", want: authsvc.ErrInvalidEmail},
		{name: "named address", email: "Ada <ada@example.com>", password: "password1", want: authsvc.ErrInvalidEmail},
		{name: "short password", email: "ada@example.com", password: "short", want: authsvc.ErrInvalidPassword},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &fakeUsers{}
			sessions := &fakeSessions{}
			issuer := &fakeIssuer{}
			svc, tx := newService(users, sessions, issuer)
			_, err := svc.Login(context.Background(), tt.email, tt.password)
			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
			if tx.calls != 0 || sessions.calls != 0 || issuer.calls != 0 {
				t.Fatalf("invalid input continued")
			}
		})
	}
}

func TestLoginStopsWhenSessionCreateFails(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("password1"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	dbErr := errors.New("db down")
	users := &fakeUsers{findUser: &user.User{
		ID:           "user-1",
		Email:        "ada@example.com",
		PasswordHash: string(hash),
		Status:       1,
	}}

	sessions := &fakeSessions{err: dbErr}
	issuer := &fakeIssuer{}
	svc, _ := newService(users, sessions, issuer)
	_, err = svc.Login(context.Background(), "ada@example.com", "password1")
	if !errors.Is(err, dbErr) {
		t.Fatalf("got %v", err)
	}
	if issuer.calls != 0 {
		t.Fatalf("issuer called after session error")
	}
}

func TestLoginReturnsTokenErrorAfterSave(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("password1"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	users := &fakeUsers{findUser: &user.User{
		ID:           "user-1",
		Email:        "ada@example.com",
		PasswordHash: string(hash),
		Status:       1,
	}}

	sessions := &fakeSessions{}
	issuer := &fakeIssuer{err: errors.New("sign failed")}
	svc, _ := newService(users, sessions, issuer)
	_, err = svc.Login(context.Background(), "ada@example.com", "password1")
	if !errors.Is(err, authsvc.ErrToken) {
		t.Fatalf("got %v", err)
	}
	if sessions.calls != 1 || issuer.calls != 1 {
		t.Fatalf("session calls=%d issuer calls=%d", sessions.calls, issuer.calls)
	}
}

func TestLoginRejectsCredentials(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("password1"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	active := &user.User{
		ID:           "user-1",
		Email:        "ada@example.com",
		PasswordHash: string(hash),
		Status:       1,
	}

	disabled := *active
	disabled.Status = 0
	tests := []struct {
		name     string
		user     *user.User
		password string
	}{
		{name: "missing", password: "password1"},
		{name: "wrong password", user: active, password: "password2"},
		{name: "disabled", user: &disabled, password: "password1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &fakeUsers{findUser: tt.user}
			sessions := &fakeSessions{}
			issuer := &fakeIssuer{}
			svc, tx := newService(users, sessions, issuer)
			_, err := svc.Login(context.Background(), "ada@example.com", tt.password)
			if !errors.Is(err, authsvc.ErrInvalidCredentials) {
				t.Fatalf("got %v", err)
			}
			if tx.calls != 0 || sessions.calls != 0 || issuer.calls != 0 {
				t.Fatalf("credentials failure continued")
			}
		})
	}
}

func TestRefreshRotatesSession(t *testing.T) {
	presented := "presented-refresh"
	sum := sha256.Sum256([]byte(presented))
	users := &fakeUsers{findUser: &user.User{
		ID:     "user-1",
		Email:  "ada@example.com",
		Status: 1,
	}}
	sessions := &fakeSessions{findToken: &session.RefreshToken{
		ID:        "old-token",
		UserID:    "user-1",
		TokenHash: hex.EncodeToString(sum[:]),
		FamilyID:  "family-1",
		ExpiresAt: time.Now().Add(time.Hour),
	}}
	exp := time.Now().Add(15 * time.Minute).Truncate(time.Second)
	issuer := &fakeIssuer{access: "access-token", exp: exp}
	svc, tx := newService(users, sessions, issuer)

	pair, err := svc.Refresh(context.Background(), "  presented-refresh  ")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if users.findID != "user-1" {
		t.Fatalf("find id = %s", users.findID)
	}

	if tx.calls != 1 || sessions.revokeCalls != 1 || sessions.calls != 1 || issuer.calls != 1 {
		t.Fatalf("calls tx=%d revoke=%d create=%d issuer=%d", tx.calls, sessions.revokeCalls, sessions.calls, issuer.calls)
	}

	if sessions.token.FamilyID != "family-1" || sessions.token.UserID != "user-1" || sessions.token.ID == "old-token" {
		t.Fatalf("refresh = %+v", sessions.token)
	}

	got := sha256.Sum256([]byte(pair.RefreshToken))
	if sessions.token.TokenHash != hex.EncodeToString(got[:]) {
		t.Fatalf("stored hash = %s", sessions.token.TokenHash)
	}

	if pair.AccessToken != "access-token" || !pair.AccessExpiresAt.Equal(exp) || pair.TokenType != "Bearer" {
		t.Fatalf("pair = %+v", pair)
	}

	if issuer.userID != "user-1" || issuer.email != "ada@example.com" {
		t.Fatalf("issuer user=%s email=%s", issuer.userID, issuer.email)
	}
}

func TestRefreshRejectsToken(t *testing.T) {
	presented := "presented-refresh"
	sum := sha256.Sum256([]byte(presented))
	hash := hex.EncodeToString(sum[:])
	revokedAt := time.Now()
	active := &user.User{ID: "user-1", Email: "ada@example.com", Status: 1}
	disabled := *active
	disabled.Status = 0

	live := &session.RefreshToken{
		ID:        "old-token",
		UserID:    "user-1",
		TokenHash: hash,
		FamilyID:  "family-1",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	revoked := *live
	revoked.RevokedAt = &revokedAt
	expired := *live
	expired.ExpiresAt = time.Now().Add(-time.Minute)

	tests := []struct {
		name   string
		token  string
		user   *user.User
		row    *session.RefreshToken
		wantTx int
	}{
		{name: "empty", token: "   "},
		{name: "missing", token: presented, user: active, wantTx: 1},
		{name: "revoked", token: presented, user: active, row: &revoked, wantTx: 1},
		{name: "expired", token: presented, user: active, row: &expired, wantTx: 1},
		{name: "disabled", token: presented, user: &disabled, row: live, wantTx: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &fakeUsers{findUser: tt.user}
			sessions := &fakeSessions{findToken: tt.row}
			issuer := &fakeIssuer{}
			svc, tx := newService(users, sessions, issuer)

			_, err := svc.Refresh(context.Background(), tt.token)
			if !errors.Is(err, authsvc.ErrInvalidRefresh) {
				t.Fatalf("got %v", err)
			}
			if tx.calls != tt.wantTx || sessions.revokeCalls != 0 || sessions.calls != 0 || issuer.calls != 0 {
				t.Fatalf("calls tx=%d revoke=%d create=%d issuer=%d", tx.calls, sessions.revokeCalls, sessions.calls, issuer.calls)
			}
		})
	}
}

func TestRefreshStopsWhenWriteFails(t *testing.T) {
	presented := "presented-refresh"
	sum := sha256.Sum256([]byte(presented))
	dbErr := errors.New("db down")
	users := &fakeUsers{findUser: &user.User{
		ID:     "user-1",
		Email:  "ada@example.com",
		Status: 1,
	}}

	row := &session.RefreshToken{
		ID:        "old-token",
		UserID:    "user-1",
		TokenHash: hex.EncodeToString(sum[:]),
		FamilyID:  "family-1",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	tests := []struct {
		name       string
		revokeErr  error
		createErr  error
		wantRevoke int
		wantCreate int
	}{
		{name: "revoke", revokeErr: dbErr, wantRevoke: 1},
		{name: "create", createErr: dbErr, wantRevoke: 1, wantCreate: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := &fakeSessions{findToken: row, err: tt.createErr, revokeErr: tt.revokeErr}
			issuer := &fakeIssuer{}
			svc, _ := newService(users, sessions, issuer)

			_, err := svc.Refresh(context.Background(), presented)
			if !errors.Is(err, dbErr) {
				t.Fatalf("got %v", err)
			}
			if sessions.revokeCalls != tt.wantRevoke || sessions.calls != tt.wantCreate || issuer.calls != 0 {
				t.Fatalf("revoke=%d create=%d issuer=%d", sessions.revokeCalls, sessions.calls, issuer.calls)
			}
		})
	}
}

func TestRefreshReturnsTokenErrorAfterSave(t *testing.T) {
	presented := "presented-refresh"
	sum := sha256.Sum256([]byte(presented))
	users := &fakeUsers{findUser: &user.User{
		ID:     "user-1",
		Email:  "ada@example.com",
		Status: 1,
	}}

	sessions := &fakeSessions{findToken: &session.RefreshToken{
		ID:        "old-token",
		UserID:    "user-1",
		TokenHash: hex.EncodeToString(sum[:]),
		FamilyID:  "family-1",
		ExpiresAt: time.Now().Add(time.Hour),
	}}

	issuer := &fakeIssuer{err: errors.New("sign failed")}
	svc, _ := newService(users, sessions, issuer)

	_, err := svc.Refresh(context.Background(), presented)
	if !errors.Is(err, authsvc.ErrToken) {
		t.Fatalf("got %v", err)
	}

	if sessions.revokeCalls != 1 || sessions.calls != 1 || issuer.calls != 1 {
		t.Fatalf("revoke=%d create=%d issuer=%d", sessions.revokeCalls, sessions.calls, issuer.calls)
	}
}
