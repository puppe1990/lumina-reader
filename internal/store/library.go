package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/puppe1990/lumina-reader/internal/models"
)

var ErrBookNotFound = errors.New("livro não encontrado")

// BookFilter alimenta a estante. Status: lendo | lidos | nao_iniciados.
type BookFilter struct {
	Category string
	Format   string
	Status   string
	Search   string
}

const bookColumns = `b.id, b.remote_key, b.title, b.author, b.category, b.format, b.size_bytes, b.sha1, b.added_at`

// UpsertBook insere ou atualiza os metadados derivados do caminho remoto e
// informa se o livro é novo (o relatório de sincronização conta os dois casos).
func (s *SQLiteStore) UpsertBook(book models.Book) (bool, error) {
	var id int64
	err := s.db.QueryRow("SELECT id FROM books WHERE remote_key = ?", book.RemoteKey).Scan(&id)
	if err == nil {
		_, err := s.db.Exec(
			`UPDATE books SET title = ?, author = ?, category = ?, format = ?, size_bytes = ?, sha1 = ? WHERE id = ?`,
			book.Title, book.Author, book.Category, book.Format, book.SizeBytes, book.SHA1, id,
		)
		if err != nil {
			return false, fmt.Errorf("atualizar livro %q: %w", book.RemoteKey, err)
		}
		return false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("procurar livro %q: %w", book.RemoteKey, err)
	}
	_, err = s.db.Exec(
		`INSERT INTO books (remote_key, title, author, category, format, size_bytes, sha1) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		book.RemoteKey, book.Title, book.Author, book.Category, book.Format, book.SizeBytes, book.SHA1,
	)
	if err != nil {
		return false, fmt.Errorf("inserir livro %q: %w", book.RemoteKey, err)
	}
	return true, nil
}

func (s *SQLiteStore) CountUsers() (int, error) {
	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&total); err != nil {
		return 0, fmt.Errorf("contar usuários: %w", err)
	}
	return total, nil
}

func (s *SQLiteStore) FindBookByKey(remoteKey string) (models.Book, error) {
	row := s.db.QueryRow(`SELECT `+bookColumns+` FROM books b WHERE b.remote_key = ?`, remoteKey)
	return scanBook(row)
}

func (s *SQLiteStore) GetBook(id int64) (models.Book, error) {
	row := s.db.QueryRow(`SELECT `+bookColumns+` FROM books b WHERE b.id = ?`, id)
	return scanBook(row)
}

func scanBook(row interface{ Scan(...any) error }) (models.Book, error) {
	var b models.Book
	err := row.Scan(&b.ID, &b.RemoteKey, &b.Title, &b.Author, &b.Category, &b.Format, &b.SizeBytes, &b.SHA1, &b.AddedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Book{}, ErrBookNotFound
	}
	if err != nil {
		return models.Book{}, fmt.Errorf("ler livro: %w", err)
	}
	return b, nil
}

// ListBooks devolve a estante com o estado de leitura, mais recente primeiro
// (última leitura ou, sem progresso, quando entrou no banco).
func (s *SQLiteStore) ListBooks(filter BookFilter) ([]models.BookListItem, error) {
	consulta := `SELECT ` + bookColumns + `, p.percent, p.updated_at, b.added_at, b.id % 6
		FROM books b LEFT JOIN reading_progress p ON p.book_id = b.id WHERE 1 = 1`
	var args []any
	if filter.Category != "" {
		consulta += " AND b.category = ?"
		args = append(args, filter.Category)
	}
	if filter.Format != "" {
		consulta += " AND b.format = ?"
		args = append(args, filter.Format)
	}
	if filter.Search != "" {
		consulta += " AND (b.title LIKE ? OR b.author LIKE ?)"
		termo := "%" + strings.TrimSpace(filter.Search) + "%"
		args = append(args, termo, termo)
	}
	switch filter.Status {
	case "lendo":
		consulta += " AND p.book_id IS NOT NULL AND p.percent < 95"
	case "lidos":
		consulta += " AND p.percent >= 95"
	case "nao_iniciados":
		consulta += " AND p.book_id IS NULL"
	}
	consulta += " ORDER BY COALESCE(p.updated_at, b.added_at) DESC, b.title COLLATE NOCASE ASC"

	rows, err := s.db.Query(consulta, args...)
	if err != nil {
		return nil, fmt.Errorf("listar livros: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var livros []models.BookListItem
	for rows.Next() {
		var item models.BookListItem
		var percent sql.NullFloat64
		var progressoEm sql.NullTime
		var adicionadoEm time.Time
		if err := rows.Scan(
			&item.ID, &item.RemoteKey, &item.Title, &item.Author, &item.Category,
			&item.Format, &item.SizeBytes, &item.SHA1, &item.AddedAt,
			&percent, &progressoEm, &adicionadoEm, &item.Tone,
		); err != nil {
			return nil, fmt.Errorf("ler linha de livro: %w", err)
		}
		item.HasProgress = percent.Valid
		item.Percent = percent.Float64
		item.UpdatedAt = item.AddedAt
		if progressoEm.Valid && progressoEm.Time.After(item.AddedAt) {
			item.UpdatedAt = progressoEm.Time
		}
		livros = append(livros, item)
	}
	return livros, rows.Err()
}

func (s *SQLiteStore) CategoryCounts() ([]models.CategoryCount, error) {
	rows, err := s.db.Query(
		`SELECT category, COUNT(*) FROM books WHERE category <> '' GROUP BY category ORDER BY category COLLATE NOCASE`,
	)
	if err != nil {
		return nil, fmt.Errorf("contar categorias: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var cats []models.CategoryCount
	for rows.Next() {
		var cat models.CategoryCount
		if err := rows.Scan(&cat.Name, &cat.Total); err != nil {
			return nil, fmt.Errorf("ler categoria: %w", err)
		}
		cats = append(cats, cat)
	}
	return cats, rows.Err()
}

func (s *SQLiteStore) LibraryTotals() (models.LibraryTotals, error) {
	var totais models.LibraryTotals
	err := s.db.QueryRow(
		`SELECT COUNT(*),
		        COALESCE(SUM(CASE WHEN b.format = 'epub' THEN 1 ELSE 0 END), 0),
		        COALESCE(SUM(CASE WHEN b.format = 'pdf' THEN 1 ELSE 0 END), 0),
		        COALESCE(SUM(CASE WHEN p.book_id IS NOT NULL AND p.percent < 95 THEN 1 ELSE 0 END), 0),
		        COALESCE(SUM(CASE WHEN p.percent >= 95 THEN 1 ELSE 0 END), 0)
		 FROM books b LEFT JOIN reading_progress p ON p.book_id = b.id`,
	).Scan(&totais.Total, &totais.Epubs, &totais.Pdfs, &totais.Lendo, &totais.Lidos)
	if err != nil {
		return models.LibraryTotals{}, fmt.Errorf("totais da estante: %w", err)
	}
	return totais, nil
}

// SaveChapters substitui os capítulos do livro (idempotente: rodar de novo
// depois de um novo parse não duplica linhas).
func (s *SQLiteStore) SaveChapters(bookID int64, chapters []models.Chapter) error {
	tx, err := s.db.Raw().Begin()
	if err != nil {
		return fmt.Errorf("abrir transação de capítulos: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec("DELETE FROM book_chapters WHERE book_id = ?", bookID); err != nil {
		return fmt.Errorf("limpar capítulos do livro %d: %w", bookID, err)
	}
	for _, capitulo := range chapters {
		_, err := tx.Exec(
			"INSERT INTO book_chapters (book_id, ordinal, title, html, words) VALUES (?, ?, ?, ?, ?)",
			bookID, capitulo.Ordinal, capitulo.Title, capitulo.HTML, capitulo.Words,
		)
		if err != nil {
			return fmt.Errorf("salvar capítulo %d do livro %d: %w", capitulo.Ordinal, bookID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("confirmar capítulos do livro %d: %w", bookID, err)
	}
	return nil
}

func (s *SQLiteStore) ChaptersFor(bookID int64) ([]models.Chapter, error) {
	rows, err := s.db.Query(
		"SELECT ordinal, title, html, words FROM book_chapters WHERE book_id = ? ORDER BY ordinal",
		bookID,
	)
	if err != nil {
		return nil, fmt.Errorf("listar capítulos do livro %d: %w", bookID, err)
	}
	defer func() { _ = rows.Close() }()

	var capitulos []models.Chapter
	for rows.Next() {
		var c models.Chapter
		if err := rows.Scan(&c.Ordinal, &c.Title, &c.HTML, &c.Words); err != nil {
			return nil, fmt.Errorf("ler capítulo do livro %d: %w", bookID, err)
		}
		capitulos = append(capitulos, c)
	}
	return capitulos, rows.Err()
}

func (s *SQLiteStore) CountChapters(bookID int64) (int, error) {
	var total int
	err := s.db.QueryRow("SELECT COUNT(*) FROM book_chapters WHERE book_id = ?", bookID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("contar capítulos do livro %d: %w", bookID, err)
	}
	return total, nil
}

// SaveProgress grava a posição e calcula o percentual a partir do total de
// capítulos (PDF sem capítulos fica em 0 — a estante usa o estado "iniciado").
func (s *SQLiteStore) SaveProgress(bookID int64, chapterOrdinal int, scrollRatio float64) (models.Progress, error) {
	if scrollRatio < 0 {
		scrollRatio = 0
	}
	if scrollRatio > 1 {
		scrollRatio = 1
	}
	total, err := s.CountChapters(bookID)
	if err != nil {
		return models.Progress{}, err
	}
	percent := 0.0
	if total > 0 {
		percent = (float64(chapterOrdinal) + scrollRatio) / float64(total) * 100
		percent = clampPercent(percent)
	}
	_, err = s.db.Exec(
		`INSERT INTO reading_progress (book_id, chapter_ordinal, scroll_ratio, percent, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(book_id) DO UPDATE SET
		   chapter_ordinal = excluded.chapter_ordinal,
		   scroll_ratio = excluded.scroll_ratio,
		   percent = excluded.percent,
		   updated_at = CURRENT_TIMESTAMP`,
		bookID, chapterOrdinal, scrollRatio, percent,
	)
	if err != nil {
		return models.Progress{}, fmt.Errorf("salvar progresso do livro %d: %w", bookID, err)
	}
	return models.Progress{BookID: bookID, ChapterOrdinal: chapterOrdinal, ScrollRatio: scrollRatio, Percent: percent}, nil
}

func clampPercent(percent float64) float64 {
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}

func (s *SQLiteStore) ProgressFor(bookID int64) (models.Progress, bool, error) {
	var p models.Progress
	var bookmark sql.NullInt64
	err := s.db.QueryRow(
		`SELECT book_id, chapter_ordinal, scroll_ratio, percent, bookmark_ordinal, updated_at
		 FROM reading_progress WHERE book_id = ?`, bookID,
	).Scan(&p.BookID, &p.ChapterOrdinal, &p.ScrollRatio, &p.Percent, &bookmark, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Progress{}, false, nil
	}
	if err != nil {
		return models.Progress{}, false, fmt.Errorf("ler progresso do livro %d: %w", bookID, err)
	}
	if bookmark.Valid {
		ordinal := int(bookmark.Int64)
		p.BookmarkOrdinal = &ordinal
	}
	return p, true, nil
}

func (s *SQLiteStore) SetBookmark(bookID int64, chapterOrdinal int) error {
	_, err := s.db.Exec(
		`INSERT INTO reading_progress (book_id, chapter_ordinal, scroll_ratio, percent, bookmark_ordinal, updated_at)
		 VALUES (?, 0, 0, 0, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(book_id) DO UPDATE SET
		   bookmark_ordinal = excluded.bookmark_ordinal,
		   updated_at = CURRENT_TIMESTAMP`,
		bookID, chapterOrdinal,
	)
	if err != nil {
		return fmt.Errorf("salvar marcador do livro %d: %w", bookID, err)
	}
	return nil
}

func (s *SQLiteStore) ClearBookmark(bookID int64) error {
	_, err := s.db.Exec(
		"UPDATE reading_progress SET bookmark_ordinal = NULL, updated_at = CURRENT_TIMESTAMP WHERE book_id = ?", bookID,
	)
	if err != nil {
		return fmt.Errorf("limpar marcador do livro %d: %w", bookID, err)
	}
	return nil
}

func (s *SQLiteStore) AddReadingSeconds(day string, seconds int) error {
	_, err := s.db.Exec(
		`INSERT INTO reading_days (day, seconds) VALUES (?, ?)
		 ON CONFLICT(day) DO UPDATE SET seconds = seconds + excluded.seconds`,
		day, seconds,
	)
	if err != nil {
		return fmt.Errorf("registrar leitura do dia %q: %w", day, err)
	}
	return nil
}

func (s *SQLiteStore) ReadingSecondsOn(day string) (int, error) {
	var segundos int
	err := s.db.QueryRow("SELECT seconds FROM reading_days WHERE day = ?", day).Scan(&segundos)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("ler segundos do dia %q: %w", day, err)
	}
	return segundos, nil
}

func (s *SQLiteStore) ReadingDays() ([]string, error) {
	rows, err := s.db.Query("SELECT day FROM reading_days ORDER BY day DESC")
	if err != nil {
		return nil, fmt.Errorf("listar dias de leitura: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var dias []string
	for rows.Next() {
		var day string
		if err := rows.Scan(&day); err != nil {
			return nil, fmt.Errorf("ler dia de leitura: %w", err)
		}
		dias = append(dias, day)
	}
	return dias, rows.Err()
}

func (s *SQLiteStore) CreateNote(note models.Note) (int64, error) {
	result, err := s.db.Exec(
		"INSERT INTO book_notes (book_id, chapter_ordinal, color, excerpt, note_text) VALUES (?, ?, ?, ?, ?)",
		note.BookID, note.ChapterOrdinal, note.Color, note.Excerpt, note.NoteText,
	)
	if err != nil {
		return 0, fmt.Errorf("criar nota no livro %d: %w", note.BookID, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("id da nota criada: %w", err)
	}
	return id, nil
}

func (s *SQLiteStore) NotesForChapter(bookID int64, chapterOrdinal int) ([]models.Note, error) {
	rows, err := s.db.Query(
		`SELECT id, book_id, chapter_ordinal, color, excerpt, note_text, created_at
		 FROM book_notes WHERE book_id = ? AND chapter_ordinal = ? ORDER BY created_at, id`,
		bookID, chapterOrdinal,
	)
	if err != nil {
		return nil, fmt.Errorf("listar notas do capítulo %d: %w", chapterOrdinal, err)
	}
	defer func() { _ = rows.Close() }()
	return scanNotes(rows)
}

// ListNotes devolve o caderno; bookID 0 lista todas as anotações.
func (s *SQLiteStore) ListNotes(bookID int64) ([]models.NoteWithBook, error) {
	consulta := `SELECT n.id, n.book_id, n.chapter_ordinal, n.color, n.excerpt, n.note_text, n.created_at,
		b.title, b.author, b.format
		FROM book_notes n JOIN books b ON b.id = n.book_id`
	var args []any
	if bookID > 0 {
		consulta += " WHERE n.book_id = ?"
		args = append(args, bookID)
	}
	consulta += " ORDER BY n.created_at DESC, n.id DESC"

	rows, err := s.db.Query(consulta, args...)
	if err != nil {
		return nil, fmt.Errorf("listar notas: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var notas []models.NoteWithBook
	for rows.Next() {
		var n models.NoteWithBook
		if err := rows.Scan(
			&n.ID, &n.BookID, &n.ChapterOrdinal, &n.Color, &n.Excerpt, &n.NoteText, &n.CreatedAt,
			&n.BookTitle, &n.BookAuthor, &n.BookFormat,
		); err != nil {
			return nil, fmt.Errorf("ler nota: %w", err)
		}
		notas = append(notas, n)
	}
	return notas, rows.Err()
}

func scanNotes(rows *sql.Rows) ([]models.Note, error) {
	var notas []models.Note
	for rows.Next() {
		var n models.Note
		if err := rows.Scan(&n.ID, &n.BookID, &n.ChapterOrdinal, &n.Color, &n.Excerpt, &n.NoteText, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("ler nota: %w", err)
		}
		notas = append(notas, n)
	}
	return notas, rows.Err()
}

func (s *SQLiteStore) UpdateNoteText(id int64, texto string) error {
	result, err := s.db.Exec("UPDATE book_notes SET note_text = ? WHERE id = ?", texto, id)
	if err != nil {
		return fmt.Errorf("atualizar nota %d: %w", id, err)
	}
	if linhas, err := result.RowsAffected(); err == nil && linhas == 0 {
		return fmt.Errorf("atualizar nota %d: nenhuma nota com esse id", id)
	}
	return nil
}

func (s *SQLiteStore) DeleteNote(id int64) error {
	_, err := s.db.Exec("DELETE FROM book_notes WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("apagar nota %d: %w", id, err)
	}
	return nil
}
