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
	}, nil)
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
	created, err := s.UpsertBook(book, nil)
	if err != nil || !created {
		t.Fatalf("primeiro UpsertBook: created=%v err=%v", created, err)
	}
	book.SHA1 = "s2"
	book.SizeBytes = 20
	created, err = s.UpsertBook(book, nil)
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

func TestStore_SaveProgressPages(t *testing.T) {
	s := newTestStore(t)
	id := seedBook(t, s, "Cat/A/Livro.pdf", "Livro PDF", "", "Cat", "pdf")

	progresso, err := s.SaveProgressPages(id, 30, 120)
	if err != nil {
		t.Fatalf("SaveProgressPages erro = %v", err)
	}
	if progresso.Percent != 25 || progresso.ChapterOrdinal != 29 {
		t.Errorf("progresso = %+v, want 25%% e página 30 (ordinal 29)", progresso)
	}
	livro, err := s.GetBook(id)
	if err != nil || livro.PdfPages != 120 {
		t.Errorf("PdfPages = %d err=%v, want 120", livro.PdfPages, err)
	}

	// Clamp: página além do total vira a última; abaixo de 1 vira a primeira.
	progresso, err = s.SaveProgressPages(id, 999, 120)
	if err != nil || progresso.Percent != 100 {
		t.Errorf("clamp superior: %+v err=%v", progresso, err)
	}
	progresso, err = s.SaveProgressPages(id, 0, 120)
	if err != nil || progresso.ChapterOrdinal != 0 {
		t.Errorf("clamp inferior: %+v err=%v", progresso, err)
	}
	if _, err := s.SaveProgressPages(id, 5, 0); err == nil {
		t.Error("total 0 deveria falhar")
	}
}

func TestStore_FavoritesToggleAndFilter(t *testing.T) {
	s := newTestStore(t)
	um := seedBook(t, s, "Cat/A/Um.epub", "Um", "", "Cat", "epub")
	seedBook(t, s, "Cat/A/Dois.epub", "Dois", "", "Cat", "epub")

	favorito, err := s.ToggleFavorite(um)
	if err != nil || !favorito {
		t.Fatalf("ToggleFavorite: favorito=%v err=%v", favorito, err)
	}
	if ok, err := s.IsFavorite(um); err != nil || !ok {
		t.Fatalf("IsFavorite = %v err=%v", ok, err)
	}

	favoritos, err := s.ListBooks(BookFilter{Status: "favoritos"})
	if err != nil || len(favoritos) != 1 || favoritos[0].Title != "Um" {
		t.Fatalf("ListBooks(favoritos) = %+v err=%v", favoritos, err)
	}
	if !favoritos[0].Favorite {
		t.Errorf("ListItem.Favorite = false, want true")
	}
	todos, _ := s.ListBooks(BookFilter{})
	for _, livro := range todos {
		if livro.Title == "Dois" && livro.Favorite {
			t.Errorf("Dois não deveria estar favoritado")
		}
	}

	totais, err := s.LibraryTotals()
	if err != nil || totais.Favoritos != 1 {
		t.Errorf("LibraryTotals.Favoritos = %d err=%v, want 1", totais.Favoritos, err)
	}

	// Toggle de novo remove; upsert (novo sync) preserva o estado.
	if favorito, err := s.ToggleFavorite(um); err != nil || favorito {
		t.Fatalf("segundo ToggleFavorite: favorito=%v err=%v (want false)", favorito, err)
	}
	if _, err := s.ToggleFavorite(um); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertBook(models.Book{RemoteKey: "Cat/A/Um.epub", Title: "Um", Category: "Cat", Format: "epub", SHA1: "novo"}, nil); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.IsFavorite(um); !ok {
		t.Errorf("sync (UpsertBook) não pode derrubar o favorito")
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

func TestStore_UpsertBook_RechaveiaPeloSHA1QuandoAChaveAntigaSaiu(t *testing.T) {
	s := newTestStore(t)
	antigo := models.Book{RemoteKey: "Cat/A/Livro/Livro.pdf", Title: "Livro", Category: "Cat", Format: "pdf", SizeBytes: 10, SHA1: "sha-conteudo"}
	if created, err := s.UpsertBook(antigo, nil); err != nil || !created {
		t.Fatalf("insert inicial: created=%v err=%v", created, err)
	}
	livro, err := s.FindBookByKey(antigo.RemoteKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveProgressPages(livro.ID, 7, 200); err != nil {
		t.Fatal(err)
	}

	novo := models.Book{RemoteKey: "Cat/B/Livro Padronizado/Livro Padronizado.pdf", Title: "Livro Padronizado", Category: "Cat", Format: "pdf", SizeBytes: 10, SHA1: "sha-conteudo"}
	created, err := s.UpsertBook(novo, map[string]bool{novo.RemoteKey: true})
	if err != nil {
		t.Fatalf("UpsertBook renomeado: %v", err)
	}
	if created {
		t.Errorf("renomeação não pode criar livro novo (id mudaria e o progresso se perderia)")
	}
	if _, err := s.FindBookByKey(antigo.RemoteKey); err == nil {
		t.Errorf("chave antiga ainda existe na estante")
	}
	migrado, err := s.FindBookByKey(novo.RemoteKey)
	if err != nil {
		t.Fatalf("chave nova não está na estante: %v", err)
	}
	if migrado.ID != livro.ID {
		t.Errorf("id mudou na renomeação: %d -> %d", livro.ID, migrado.ID)
	}
	progresso, ok, err := s.ProgressFor(migrado.ID)
	if err != nil || !ok || progresso.ChapterOrdinal != 6 {
		t.Errorf("progresso perdido na renomeação: %+v ok=%v err=%v", progresso, ok, err)
	}
}

func TestStore_UpsertBook_NaoRechaveiaSeAChaveAntigaAindaVive(t *testing.T) {
	s := newTestStore(t)
	um := models.Book{RemoteKey: "Cat/A/Um.pdf", Title: "Um", Category: "Cat", Format: "pdf", SHA1: "sha-repetido"}
	if _, err := s.UpsertBook(um, nil); err != nil {
		t.Fatal(err)
	}
	dois := models.Book{RemoteKey: "Cat/A/Dois.pdf", Title: "Dois", Category: "Cat", Format: "pdf", SHA1: "sha-repetido"}
	created, err := s.UpsertBook(dois, map[string]bool{um.RemoteKey: true, dois.RemoteKey: true})
	if err != nil {
		t.Fatalf("UpsertBook: %v", err)
	}
	if !created {
		t.Errorf("cópias distintas com o mesmo sha1 devem virar livros separados")
	}
	if livros, err := s.ListBooks(BookFilter{}); err != nil || len(livros) != 2 {
		t.Errorf("ListBooks = %d err=%v, want 2", len(livros), err)
	}
}

func TestStore_PruneBooks_RemoveOrfaosComRastros(t *testing.T) {
	s := newTestStore(t)
	fika := seedBook(t, s, "Cat/A/Fica.pdf", "Fica", "Autora", "Cat", "pdf")
	sai := seedBook(t, s, "Cat/A/Sai.pdf", "Sai", "Autor", "Cat", "pdf")

	if _, err := s.SaveProgressPages(sai, 3, 100); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ToggleFavorite(sai); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveCover(models.Cover{BookID: sai, ContentType: "image/png", Image: []byte("png")}); err != nil {
		t.Fatal(err)
	}

	removidos, err := s.PruneBooks(map[string]bool{"Cat/A/Fica.pdf": true})
	if err != nil {
		t.Fatalf("PruneBooks: %v", err)
	}
	if removidos != 1 {
		t.Errorf("removidos = %d, want 1", removidos)
	}
	if _, err := s.GetBook(sai); err == nil {
		t.Errorf("livro podado ainda existe")
	}
	if _, err := s.GetBook(fika); err != nil {
		t.Errorf("livro vivo foi removido: %v", err)
	}
	if _, ok, _ := s.ProgressFor(sai); ok {
		t.Errorf("progresso do livro podado ficou para trás")
	}
	if _, encontrada, _ := s.CoverFor(sai); encontrada {
		t.Errorf("capa do livro podado ficou para trás")
	}
	var favoritos int
	if err := s.db.Raw().QueryRow("SELECT COUNT(*) FROM book_favorites WHERE book_id = ?", sai).Scan(&favoritos); err != nil {
		t.Fatal(err)
	}
	if favoritos != 0 {
		t.Errorf("favorito do livro podado ficou para trás")
	}
}
