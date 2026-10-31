package models

import "time"

// Book é um arquivo do acervo que mora no B2; a linha local guarda só os
// metadados derivados do caminho remoto.
type Book struct {
	ID        int64
	RemoteKey string
	Title     string
	Author    string
	Category  string
	Format    string
	SizeBytes int64
	SHA1      string
	AddedAt   time.Time
}

// Chapter é um item da spine do EPUB já sanitizado, com o HTML pronto para o
// leitor (imagens reescritas para /livros/{id}/recurso/...).
type Chapter struct {
	Ordinal int
	Title   string
	HTML    string
	Words   int
}

type Progress struct {
	BookID          int64
	ChapterOrdinal  int
	ScrollRatio     float64
	Percent         float64
	BookmarkOrdinal *int
	UpdatedAt       time.Time
}

type Note struct {
	ID             int64
	BookID         int64
	ChapterOrdinal int
	Color          string
	Excerpt        string
	NoteText       string
	CreatedAt      time.Time
}

type NoteWithBook struct {
	Note
	BookTitle  string
	BookAuthor string
	BookFormat string
}

// BookListItem é o livro com o estado de leitura para a estante.
type BookListItem struct {
	Book
	HasProgress bool
	Percent     float64
	Favorite    bool
	UpdatedAt   time.Time
	// Tone escolhe a lombada tipográfica da capa (0..5); vem do id no SQL para
	// ser determinística e estável entre renders.
	Tone int
}

type CategoryCount struct {
	Name  string
	Total int
}

// Cover é a arte do livro extraída do arquivo (EPUB) ou da 1ª página (PDF).
// Missing marca "já tentamos e não tem" para não rebaixar o arquivo a cada
// request.
type Cover struct {
	BookID      int64
	ContentType string
	Image       []byte
	Missing     bool
}

type LibraryTotals struct {
	Total     int
	Epubs     int
	Pdfs      int
	Lendo     int
	Lidos     int
	Favoritos int
}

type ReadingStats struct {
	StreakDays   int
	SecondsToday int
	GoalMinutes  int
}
