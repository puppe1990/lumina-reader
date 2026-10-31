package store

import (
	"fmt"
	"time"

	"github.com/puppe1990/amarra-cais/pkg/cais/passwordreset"
)

// Reset tokens are bearer credentials: persist only passwordreset.Hash(token)
// and return the raw value to the notifier, so a DB, backup or SQL-log leak
// cannot be replayed (#222). Lookup/consume hash the presented token first.
func (s *SQLiteStore) CreatePasswordResetToken(userID int64) (string, error) {
	if err := s.ClearPasswordResetTokens(userID); err != nil {
		return "", err
	}

	token, err := passwordreset.NewToken()
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().UTC().Add(passwordreset.DefaultTTL).Format("2006-01-02 15:04:05")
	if _, err := s.db.Exec(
		"INSERT INTO password_reset_tokens (token_hash, user_id, expires_at) VALUES (?, ?, ?)",
		passwordreset.Hash(token), userID, expiresAt,
	); err != nil {
		return "", fmt.Errorf("insert reset token: %w", err)
	}
	return token, nil
}

func (s *SQLiteStore) ClearPasswordResetTokens(userID int64) error {
	if _, err := s.db.Exec("DELETE FROM password_reset_tokens WHERE user_id = ?", userID); err != nil {
		return fmt.Errorf("clear reset tokens: %w", err)
	}
	return nil
}

func (s *SQLiteStore) FindPasswordResetUserID(token string) (int64, bool) {
	var userID int64
	err := s.db.QueryRow(
		"SELECT user_id FROM password_reset_tokens WHERE token_hash = ? AND expires_at > datetime('now')",
		passwordreset.Hash(token),
	).Scan(&userID)
	if err != nil {
		return 0, false
	}
	return userID, true
}

func (s *SQLiteStore) ResetPasswordWithToken(token, passwordHash string) error {
	userID, ok := s.FindPasswordResetUserID(token)
	if !ok {
		return fmt.Errorf("invalid or expired reset token")
	}

	tx, err := s.db.Raw().Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec("UPDATE users SET password_hash = ? WHERE id = ?", passwordHash, userID); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM password_reset_tokens WHERE token_hash = ?", passwordreset.Hash(token)); err != nil {
		return fmt.Errorf("delete reset token: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM sessions WHERE user_id = ?", userID); err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reset: %w", err)
	}
	return nil
}
