package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/puppe1990/amarra-cais/pkg/cais/devlog"
	"github.com/puppe1990/amarra-cais/pkg/cais/session"
	caissqlite "github.com/puppe1990/amarra-cais/pkg/cais/sqlite"
	"github.com/puppe1990/amarra-cais/pkg/cais/sqllog"
	_ "modernc.org/sqlite"

	"github.com/puppe1990/lumina-reader/internal/models"
)

var ErrEmailTaken = errors.New("email already registered")

type Store interface {
	FindUserByEmail(email string) (models.User, error)
	CreateUser(email, passwordHash string) (int64, error)
	CreatePasswordResetToken(userID int64) (string, error)
	ClearPasswordResetTokens(userID int64) error
	FindPasswordResetUserID(token string) (int64, bool)
	ResetPasswordWithToken(token, passwordHash string) error
	CountUsers() (int, error)

	UpsertBook(book models.Book) (bool, error)
	FindBookByKey(remoteKey string) (models.Book, error)
	GetBook(id int64) (models.Book, error)
	ListBooks(filter BookFilter) ([]models.BookListItem, error)
	CategoryCounts() ([]models.CategoryCount, error)
	LibraryTotals() (models.LibraryTotals, error)

	SaveChapters(bookID int64, chapters []models.Chapter) error
	ChaptersFor(bookID int64) ([]models.Chapter, error)
	CountChapters(bookID int64) (int, error)

	SaveProgress(bookID int64, chapterOrdinal int, scrollRatio float64) (models.Progress, error)
	SaveProgressPages(bookID int64, page, total int) (models.Progress, error)
	ProgressFor(bookID int64) (models.Progress, bool, error)
	SetBookmark(bookID int64, chapterOrdinal int) error
	ClearBookmark(bookID int64) error

	AddReadingSeconds(day string, seconds int) error
	ReadingSecondsOn(day string) (int, error)
	ReadingDays() ([]string, error)

	CreateNote(note models.Note) (int64, error)
	NotesForChapter(bookID int64, chapterOrdinal int) ([]models.Note, error)
	ListNotes(bookID int64) ([]models.NoteWithBook, error)
	UpdateNoteText(id int64, texto string) error
	DeleteNote(id int64) error

	CoverFor(bookID int64) (models.Cover, bool, error)
	SaveCover(cover models.Cover) error

	ToggleFavorite(bookID int64) (bool, error)
	IsFavorite(bookID int64) (bool, error)

	Sessions() session.Store
	Ping() error
	DB() *sql.DB
	Close() error
}

type SQLiteStore struct {
	db *sqllog.DB
}

func NewSQLiteStore(dsn string, env string) (*SQLiteStore, error) {
	if dsn != ":memory:" {
		dir := filepath.Dir(dsn)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}

	db, err := sql.Open("sqlite", caissqlite.DSN(dsn))
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := caissqlite.Configure(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("configure sqlite: %w", err)
	}

	if err := applyMigrations(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	cfg := sqllog.ConfigForEnv(env)
	if cfg.Enabled {
		cfg.Writer = devlog.MirrorDefault(os.Stdout)
	}
	wrapped := sqllog.Wrap(db, cfg)
	if err := seedAuthData(wrapped.Raw(), env); err != nil {
		_ = wrapped.Close()
		return nil, err
	}
	return &SQLiteStore{db: wrapped}, nil
}

func seedAuthData(db *sql.DB, env string) error {
	if env != "development" {
		return nil
	}
	if err := session.EnsureSQLiteSchema(db); err != nil {
		return err
	}
	hash, err := session.HashPassword("password")
	if err != nil {
		return err
	}
	_, err = db.Exec("INSERT OR IGNORE INTO users (email, password_hash, is_admin) VALUES (?, ?, 1)", "demo@example.com", hash)
	return err
}

func (s *SQLiteStore) FindUserByEmail(email string) (models.User, error) {
	var u models.User
	err := s.db.QueryRow(
		"SELECT id, email, password_hash, created_at FROM users WHERE email = ?",
		email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return models.User{}, fmt.Errorf("find user: %w", err)
	}
	return u, nil
}

func (s *SQLiteStore) CreateUser(email, passwordHash string) (int64, error) {
	result, err := s.db.Exec(
		"INSERT INTO users (email, password_hash) VALUES (?, ?)",
		email, passwordHash,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return 0, ErrEmailTaken
		}
		return 0, fmt.Errorf("create user: %w", err)
	}
	return result.LastInsertId()
}

func (s *SQLiteStore) Sessions() session.Store {
	return session.NewSQLiteStore(s.db.Raw())
}

func (s *SQLiteStore) Ping() error {
	return s.db.Raw().Ping()
}

func (s *SQLiteStore) DB() *sql.DB {
	return s.db.Raw()
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
