package store

import (
	"testing"
	"time"

	"github.com/puppe1990/lumina-reader/internal/models"
)

func seedBook(t *testing.T, s *SQLiteStore, key, titulo, autor, category, format string) int64 {
	t.Helper()
	created, err := s.UpsertBook(models.Book{
		RemoteKey: key, Title: titulo, Author: autor, Category: category,
		Format: format, SizeBytes: 100, SHA1: "sha-" + titulo,
	})
	if err != nil {
		t.Fatalf("UpsertBook(%s): %v", key, err)
	}
	if !created {
		t.Fatalf("UpsertBook(%s) não criou livro novo", key)
	}
	livro, err := s.FindBookByKey(key)
	if err != nil {
		t.Fatalf("FindBookByKey(%s): %v", key, err)
	}
	return livro.ID
}

func TestStore_UpsertBook_CreatesThenUpdates(t *testing.T) {
	s := newTestStore(t)

	book := models.Book{RemoteKey: "Cat/A/Livro.pdf", Title: "Livro", Author: "A", Category: "Cat", Format: "pdf", SizeBytes: 10, SHA1: "s1"}
	created, err := s.UpsertBook(book)
	if err != nil || !created {
		t.Fatalf("primeiro UpsertBook: created=%v err=%v", created, err)
	}
	book.SHA1 = "s2"
	book.SizeBytes = 20
	created, err = s.UpsertBook(book)
	if err != nil || created {
		t.Fatalf("segundo UpsertBook: created=%v err=%v (want update)", created, err)
	}
	salvo, err := s.FindBookByKey("Cat/A/Livro.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if salvo.SHA1 != "s2" || salvo.SizeBytes != 20 {
		t.Errorf("livro não atualizado: %+v", salvo)
	}
}

func TestStore_ListBooks_FiltersAndSearch(t *testing.T) {
	s := newTestStore(t)
	epub := seedBook(t, s, "Cat/A/Epub Um.epub", "Epub Um", "Autora Um", "Cat", "epub")
	seedBook(t, s, "Cat/A/Pdf Dois.pdf", "Pdf Dois", "Autor Dois", "Cat", "pdf")
	seedBook(t, s, "Outra/A/Outro.epub", "Outro", "Autor Três", "Outra", "epub")

	if _, err := s.SaveProgress(epub, 0, 0.5); err != nil {
		t.Fatal(err)
	}

	todos, err := s.ListBooks(BookFilter{})
	if err != nil || len(todos) != 3 {
		t.Fatalf("ListBooks(todos) = %d livros, err=%v want 3", len(todos), err)
	}

	epubs, err := s.ListBooks(BookFilter{Format: "epub"})
	if err != nil || len(epubs) != 2 {
		t.Fatalf("ListBooks(epub) = %d, want 2", len(epubs))
	}

	lendo, err := s.ListBooks(BookFilter{Status: "lendo"})
	if err != nil || len(lendo) != 1 || lendo[0].Title != "Epub Um" {
		t.Fatalf("ListBooks(lendo) = %+v, want Epub Um", lendo)
	}
	if len(lendo) == 1 && !lendo[0].HasProgress {
		t.Errorf("lendo[0].HasProgress = false, want true")
	}

	naoLidos, err := s.ListBooks(BookFilter{Status: "nao_iniciados"})
	if err != nil || len(naoLidos) != 2 {
		t.Fatalf("ListBooks(nao_iniciados) = %d, want 2", len(naoLidos))
	}

	busca, err := s.ListBooks(BookFilter{Search: "pdf dois"})
	if err != nil || len(busca) != 1 || busca[0].Title != "Pdf Dois" {
		t.Fatalf("ListBooks(search) = %+v, want Pdf Dois", busca)
	}

	porCategoria, err := s.ListBooks(BookFilter{Category: "Cat"})
	if err != nil || len(porCategoria) != 2 {
		t.Fatalf("ListBooks(category) = %d, want 2", len(porCategoria))
	}
}

func TestStore_ListBooks_RecentFirst(t *testing.T) {
	s := newTestStore(t)
	um := seedBook(t, s, "Cat/A/Um.epub", "Um", "", "Cat", "epub")
	dois := seedBook(t, s, "Cat/A/Dois.epub", "Dois", "", "Cat", "epub")

	if _, err := s.SaveProgress(um, 0, 0.1); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond) // datetime('now') tem resolução de segundo
	if _, err := s.SaveProgress(dois, 0, 0.1); err != nil {
		t.Fatal(err)
	}

	lista, err := s.ListBooks(BookFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if lista[0].Title != "Dois" {
		t.Errorf("primeiro da lista = %q, want Dois (último lido)", lista[0].Title)
	}
}

func TestStore_Categories(t *testing.T) {
	s := newTestStore(t)
	seedBook(t, s, "Cat/A/Um.epub", "Um", "", "Cat", "epub")
	seedBook(t, s, "Cat/A/Dois.pdf", "Dois", "", "Cat", "pdf")
	seedBook(t, s, "Outra/A/Três.epub", "Três", "", "Outra", "epub")

	cats, err := s.CategoryCounts()
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) != 2 || cats[0].Name != "Cat" || cats[0].Total != 2 {
		t.Fatalf("CategoryCounts = %+v", cats)
	}
}

func TestStore_ChaptersReplaceAll(t *testing.T) {
	s := newTestStore(t)
	id := seedBook(t, s, "Cat/A/Livro.epub", "Livro", "", "Cat", "epub")

	err := s.SaveChapters(id, []models.Chapter{
		{Ordinal: 0, Title: "Um", HTML: "<p>um</p>", Words: 10},
		{Ordinal: 1, Title: "Dois", HTML: "<p>dois</p>", Words: 20},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = s.SaveChapters(id, []models.Chapter{{Ordinal: 0, Title: "Um", HTML: "<p>um v2</p>", Words: 11}})
	if err != nil {
		t.Fatal(err)
	}

	caps, err := s.ChaptersFor(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(caps) != 1 || caps[0].HTML != "<p>um v2</p>" || caps[0].Words != 11 {
		t.Fatalf("capítulos após substituição = %+v", caps)
	}
}

func TestStore_Progress_PercentAndBookmark(t *testing.T) {
	s := newTestStore(t)
	id := seedBook(t, s, "Cat/A/Livro.epub", "Livro", "", "Cat", "epub")
	if err := s.SaveChapters(id, []models.Chapter{
		{Ordinal: 0, Title: "Um", HTML: "<p>um</p>"},
		{Ordinal: 1, Title: "Dois", HTML: "<p>dois</p>"},
		{Ordinal: 2, Title: "Três", HTML: "<p>três</p>"},
		{Ordinal: 3, Title: "Quatro", HTML: "<p>quatro</p>"},
	}); err != nil {
		t.Fatal(err)
	}

	progresso, err := s.SaveProgress(id, 1, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if want := 37.5; progresso.Percent != want {
		t.Errorf("Percent = %v, want %v", progresso.Percent, want)
	}

	atual, ok, err := s.ProgressFor(id)
	if err != nil || !ok {
		t.Fatalf("ProgressFor: ok=%v err=%v", ok, err)
	}
	if atual.ChapterOrdinal != 1 || atual.ScrollRatio != 0.5 {
		t.Errorf("progresso = %+v", atual)
	}

	if err := s.SetBookmark(id, 2); err != nil {
		t.Fatal(err)
	}
	atual, _, _ = s.ProgressFor(id)
	if atual.BookmarkOrdinal == nil || *atual.BookmarkOrdinal != 2 {
		t.Errorf("BookmarkOrdinal = %v, want 2", atual.BookmarkOrdinal)
	}
	if err := s.ClearBookmark(id); err != nil {
		t.Fatal(err)
	}
	atual, _, _ = s.ProgressFor(id)
	if atual.BookmarkOrdinal != nil {
		t.Errorf("BookmarkOrdinal = %v, want nil depois do clear", atual.BookmarkOrdinal)
	}
}

func TestStore_ReadingDaysAndSeconds(t *testing.T) {
	s := newTestStore(t)

	for i := 0; i < 3; i++ {
		if err := s.AddReadingSeconds("2026-10-31", 60); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.AddReadingSeconds("2026-10-30", 120); err != nil {
		t.Fatal(err)
	}

	segundos, err := s.ReadingSecondsOn("2026-10-31")
	if err != nil || segundos != 180 {
		t.Fatalf("ReadingSecondsOn = %d, err=%v want 180", segundos, err)
	}

	dias, err := s.ReadingDays()
	if err != nil || len(dias) != 2 {
		t.Fatalf("ReadingDays = %v, err=%v", dias, err)
	}
	if dias[0] != "2026-10-31" {
		t.Errorf("dias[0] = %q, want mais recente primeiro", dias[0])
	}
}

func TestStore_NotesLifecycle(t *testing.T) {
	s := newTestStore(t)
	id := seedBook(t, s, "Cat/A/Livro.epub", "Livro", "Autora", "Cat", "epub")

	noteID, err := s.CreateNote(models.Note{
		BookID: id, ChapterOrdinal: 3, Color: "amarelo",
		Excerpt: "um trecho destacado", NoteText: "minha anotação",
	})
	if err != nil {
		t.Fatal(err)
	}

	doCapitulo, err := s.NotesForChapter(id, 3)
	if err != nil || len(doCapitulo) != 1 || doCapitulo[0].Excerpt != "um trecho destacado" {
		t.Fatalf("NotesForChapter = %+v err=%v", doCapitulo, err)
	}

	comLivro, err := s.ListNotes(id)
	if err != nil || len(comLivro) != 1 {
		t.Fatalf("ListNotes = %+v err=%v", comLivro, err)
	}
	if comLivro[0].BookTitle != "Livro" || comLivro[0].BookAuthor != "Autora" {
		t.Errorf("nota sem dados do livro: %+v", comLivro[0])
	}

	if err := s.UpdateNoteText(noteID, "anotação revisada"); err != nil {
		t.Fatal(err)
	}
	comLivro, _ = s.ListNotes(id)
	if comLivro[0].NoteText != "anotação revisada" {
		t.Errorf("NoteText = %q", comLivro[0].NoteText)
	}

	if err := s.DeleteNote(noteID); err != nil {
		t.Fatal(err)
	}
	comLivro, _ = s.ListNotes(id)
	if len(comLivro) != 0 {
		t.Errorf("nota não foi apagada: %+v", comLivro)
	}
}

func TestStore_LibraryTotals(t *testing.T) {
	s := newTestStore(t)
	seedBook(t, s, "Cat/A/Um.epub", "Um", "", "Cat", "epub")
	seedBook(t, s, "Cat/A/Dois.pdf", "Dois", "", "Cat", "pdf")

	totais, err := s.LibraryTotals()
	if err != nil {
		t.Fatal(err)
	}
	if totais.Total != 2 || totais.Epubs != 1 || totais.Pdfs != 1 {
		t.Errorf("LibraryTotals = %+v", totais)
	}
}

func TestStore_CoverLifecycle(t *testing.T) {
	s := newTestStore(t)
	id := seedBook(t, s, "Cat/A/Livro.epub", "Livro", "", "Cat", "epub")

	if _, encontrada, err := s.CoverFor(id); err != nil || encontrada {
		t.Fatalf("CoverFor antes de salvar: encontrada=%v err=%v", encontrada, err)
	}

	if err := s.SaveCover(models.Cover{BookID: id, ContentType: "image/png", Image: []byte("\x89PNG\r\n\x1a\n")}); err != nil {
		t.Fatal(err)
	}
	capa, encontrada, err := s.CoverFor(id)
	if err != nil || !encontrada {
		t.Fatalf("CoverFor: encontrada=%v err=%v", encontrada, err)
	}
	if capa.ContentType != "image/png" || string(capa.Image) != "\x89PNG\r\n\x1a\n" || capa.Missing {
		t.Errorf("capa = %+v", capa)
	}

	// Registrar "sem capa" substitui a linha (não cresce).
	if err := s.SaveCover(models.Cover{BookID: id, Missing: true}); err != nil {
		t.Fatal(err)
	}
	capa, encontrada, err = s.CoverFor(id)
	if err != nil || !encontrada || !capa.Missing || len(capa.Image) != 0 {
		t.Errorf("capa missing = %+v encontrada=%v err=%v", capa, encontrada, err)
	}
}
