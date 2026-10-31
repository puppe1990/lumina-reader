-- up
CREATE TABLE IF NOT EXISTS books (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    remote_key TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    author TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT '',
    format TEXT NOT NULL,
    size_bytes INTEGER NOT NULL DEFAULT 0,
    sha1 TEXT NOT NULL DEFAULT '',
    added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_books_category ON books(category);
CREATE INDEX IF NOT EXISTS idx_books_format ON books(format);

CREATE TABLE IF NOT EXISTS book_chapters (
    book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    html TEXT NOT NULL,
    words INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (book_id, ordinal)
);

CREATE TABLE IF NOT EXISTS reading_progress (
    book_id INTEGER PRIMARY KEY REFERENCES books(id) ON DELETE CASCADE,
    chapter_ordinal INTEGER NOT NULL DEFAULT 0,
    scroll_ratio REAL NOT NULL DEFAULT 0,
    percent REAL NOT NULL DEFAULT 0,
    bookmark_ordinal INTEGER,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS book_notes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    chapter_ordinal INTEGER NOT NULL,
    color TEXT NOT NULL DEFAULT 'amarelo',
    excerpt TEXT NOT NULL,
    note_text TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_book_notes_position ON book_notes(book_id, chapter_ordinal);

CREATE TABLE IF NOT EXISTS reading_days (
    day TEXT PRIMARY KEY,
    seconds INTEGER NOT NULL DEFAULT 0
);

-- down
DROP TABLE IF EXISTS reading_days;
DROP TABLE IF EXISTS book_notes;
DROP TABLE IF EXISTS reading_progress;
DROP TABLE IF EXISTS book_chapters;
DROP TABLE IF EXISTS books;
