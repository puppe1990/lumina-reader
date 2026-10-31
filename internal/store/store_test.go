package store

import (
	"errors"
	"testing"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewSQLiteStore(":memory:", "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestStore_Migrations(t *testing.T) {
	s := newTestStore(t)

	for _, table := range []string{"users", "sessions"} {
		var name string
		err := s.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name = ?", table).Scan(&name)
		if err != nil {
			t.Fatalf("%s table not found: %v", table, err)
		}
	}
}

func TestStore_CreateUser_ThenFind(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.CreateUser("alice@example.com", "hash"); err != nil {
		t.Fatal(err)
	}

	found, err := s.FindUserByEmail("alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if found.Email != "alice@example.com" {
		t.Errorf("Email = %q, want alice@example.com", found.Email)
	}

	if _, err := s.CreateUser("alice@example.com", "hash"); !errors.Is(err, ErrEmailTaken) {
		t.Errorf("duplicate CreateUser err = %v, want ErrEmailTaken", err)
	}
}
