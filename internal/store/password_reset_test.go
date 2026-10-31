package store

import (
	"testing"
	"time"

	"github.com/puppe1990/amarra-cais/pkg/cais/passwordreset"
	"github.com/puppe1990/amarra-cais/pkg/cais/session"
)

func newResetTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewSQLiteStore(":memory:", "development")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestCreatePasswordResetToken_storesDigestNotRawToken(t *testing.T) {
	s := newResetTestStore(t)
	user, err := s.FindUserByEmail("demo@example.com")
	if err != nil {
		t.Fatal(err)
	}

	token, err := s.CreatePasswordResetToken(user.ID)
	if err != nil {
		t.Fatal(err)
	}

	var stored string
	if err := s.db.QueryRow(
		"SELECT token_hash FROM password_reset_tokens WHERE user_id = ?", user.ID,
	).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == token {
		t.Fatal("stored reset token must not equal the raw bearer token")
	}
	if stored != passwordreset.Hash(token) {
		t.Fatalf("stored = %q, want passwordreset.Hash(token)", stored)
	}
}

func TestFindPasswordResetUserID_ignoresExpired(t *testing.T) {
	s := newResetTestStore(t)
	user, err := s.FindUserByEmail("demo@example.com")
	if err != nil {
		t.Fatal(err)
	}

	expired := "expired-token"
	if _, err := s.db.Exec(
		"INSERT INTO password_reset_tokens (token_hash, user_id, expires_at) VALUES (?, ?, ?)",
		passwordreset.Hash(expired), user.ID, time.Now().UTC().Add(-time.Minute).Format("2006-01-02 15:04:05"),
	); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.FindPasswordResetUserID(expired); ok {
		t.Fatal("expired token must not resolve")
	}
}

func TestResetPasswordWithToken_consumesToken(t *testing.T) {
	s := newResetTestStore(t)
	user, err := s.FindUserByEmail("demo@example.com")
	if err != nil {
		t.Fatal(err)
	}
	token, err := s.CreatePasswordResetToken(user.ID)
	if err != nil {
		t.Fatal(err)
	}

	passwordHash, err := session.HashPassword("new-password-123")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ResetPasswordWithToken(token, passwordHash); err != nil {
		t.Fatalf("reset with valid token: %v", err)
	}
	if _, ok := s.FindPasswordResetUserID(token); ok {
		t.Fatal("token must be consumed after a successful reset")
	}

	updated, err := s.FindUserByEmail("demo@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !session.VerifyPassword(updated.PasswordHash, "new-password-123") {
		t.Fatal("password was not updated")
	}
}
