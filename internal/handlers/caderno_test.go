package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/puppe1990/amarra-cais/pkg/cais"

	"github.com/puppe1990/lumina-reader/internal/library"
	"github.com/puppe1990/lumina-reader/internal/models"
	"github.com/puppe1990/lumina-reader/internal/reader"
	"github.com/puppe1990/lumina-reader/internal/store"
)

func newBookHandler(t *testing.T, s store.Store) *BookHandler {
	t.Helper()
	leitor := reader.New(remoteIndisponivel{}, s, t.TempDir())
	return NewBookHandler(setupTestViews(t), testSite(), appCatalog(), cais.Config{Env: "test"}, s, leitor)
}

func TestBook_Show_EpubListsChaptersAndProgress(t *testing.T) {
	s := setupTestStore(t)
	id := seedLivro(t, s, "Cat/A/Livro.epub", "Livro Ep", "Autora", "Cat", "epub")
	seedCapitulos(t, s, id, "Abertura", "Meio")
	if _, err := s.SaveProgress(id, 1, 0.25); err != nil {
		t.Fatal(err)
	}

	h := newBookHandler(t, s)
	req := httptest.NewRequest(http.MethodGet, "/livros/"+itoa64(id), nil)
	rr := httptest.NewRecorder()
	h.Show(rr, req, id)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	body := rr.Body.String()
	for _, esperado := range []string{"Livro Ep", "Abertura", "Meio", "Continuar lendo"} {
		if !strings.Contains(body, esperado) {
			t.Errorf("detalhe sem %q", esperado)
		}
	}
}

func TestBook_Show_PdfOffersOpenLink(t *testing.T) {
	s := setupTestStore(t)
	id := seedLivro(t, s, "Cat/A/Livro.pdf", "Livro Pdf", "", "Cat", "pdf")

	h := newBookHandler(t, s)
	req := httptest.NewRequest(http.MethodGet, "/livros/"+itoa64(id), nil)
	rr := httptest.NewRecorder()
	h.Show(rr, req, id)

	body := rr.Body.String()
	if !strings.Contains(body, "/livros/"+itoa64(id)+"/arquivo") {
		t.Errorf("PDF sem link para abrir o arquivo")
	}
}

func TestBook_Show_UnknownReturns404(t *testing.T) {
	h := newBookHandler(t, setupTestStore(t))
	req := httptest.NewRequest(http.MethodGet, "/livros/999", nil)
	rr := httptest.NewRecorder()
	h.Show(rr, req, 999)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestNotes_CreateWritesHighlight(t *testing.T) {
	s := setupTestStore(t)
	id := seedLivro(t, s, "Cat/A/Livro.epub", "Livro Ep", "", "Cat", "epub")

	h := NewNotesHandler(setupTestViews(t), testSite(), appCatalog(), cais.Config{Env: "test"}, s)
	form := url.Values{
		"cap":    {"2"},
		"cor":    {"verde"},
		"trecho": {"Um trecho destacado com vírgula, ponto e mais texto"},
		"nota":   {"Minha anotação"},
	}
	req := httptest.NewRequest(http.MethodPost, "/livros/"+itoa64(id)+"/notas", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.Create(rr, req, id)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%.300s", rr.Code, rr.Body.String())
	}
	notas, err := s.NotesForChapter(id, 2)
	if err != nil || len(notas) != 1 {
		t.Fatalf("nota não criada: %+v err=%v", notas, err)
	}
	if notas[0].Color != "verde" || notas[0].NoteText != "Minha anotação" {
		t.Errorf("nota = %+v", notas[0])
	}
	if !strings.Contains(notas[0].Excerpt, "vírgula") {
		t.Errorf("trecho = %q", notas[0].Excerpt)
	}
}

func TestNotes_PageListsWithBook(t *testing.T) {
	s := setupTestStore(t)
	id := seedLivro(t, s, "Cat/A/Livro.epub", "Livro Ep", "Autora", "Cat", "epub")
	if _, err := s.CreateNote(models.Note{BookID: id, ChapterOrdinal: 3, Color: "amarelo", Excerpt: "um trecho"}); err != nil {
		t.Fatal(err)
	}

	h := NewNotesHandler(setupTestViews(t), testSite(), appCatalog(), cais.Config{Env: "test"}, s)
	req := httptest.NewRequest(http.MethodGet, "/notas", nil)
	rr := httptest.NewRecorder()
	h.Page(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Caderno") || !strings.Contains(body, "Livro Ep") || !strings.Contains(body, "um trecho") {
		t.Errorf("caderno sem conteúdo: %.500s", body)
	}
}

func TestNotes_DeleteAndUpdate(t *testing.T) {
	s := setupTestStore(t)
	id := seedLivro(t, s, "Cat/A/Livro.epub", "Livro Ep", "", "Cat", "epub")
	if _, err := s.CreateNote(models.Note{BookID: id, ChapterOrdinal: 0, Color: "azul", Excerpt: "trecho"}); err != nil {
		t.Fatal(err)
	}

	h := NewNotesHandler(setupTestViews(t), testSite(), appCatalog(), cais.Config{Env: "test"}, s)

	mudar := httptest.NewRequest(http.MethodPut, "/notas/1", strings.NewReader(url.Values{"nota": {"revisada"}}.Encode()))
	mudar.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.Update(rr, mudar, 1)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("update status = %d", rr.Code)
	}
	notas, _ := s.ListNotes(0)
	if len(notas) != 1 || notas[0].NoteText != "revisada" {
		t.Errorf("nota não atualizada: %+v", notas)
	}

	apagar := httptest.NewRequest(http.MethodDelete, "/notas/1", nil)
	rr = httptest.NewRecorder()
	h.Delete(rr, apagar, 1)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("delete status = %d", rr.Code)
	}
	notas, _ = s.ListNotes(0)
	if len(notas) != 0 {
		t.Errorf("nota não apagada: %+v", notas)
	}
}

type fakeSync struct {
	chamado bool
	report  library.Report
	err     error
}

func (f *fakeSync) Sync(ctx context.Context) (library.Report, error) {
	f.chamado = true
	return f.report, f.err
}

func TestSync_PostRunsAndFlashesReport(t *testing.T) {
	s := setupTestStore(t)
	fake := &fakeSync{report: library.Report{Total: 3, New: 2, Updated: 1}}
	h := NewSyncHandler(setupTestViews(t), testSite(), appCatalog(), cais.Config{Env: "test"}, s, fake)

	req := httptest.NewRequest(http.MethodPost, "/sincronizar", nil)
	rr := httptest.NewRecorder()
	h.Post(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rr.Code)
	}
	if rr.Header().Get("Location") != "/" {
		t.Errorf("Location = %q", rr.Header().Get("Location"))
	}
	if !fake.chamado {
		t.Errorf("serviço de sincronização não foi chamado")
	}
}

func TestSync_PostWithoutB2FlashesHelp(t *testing.T) {
	h := NewSyncHandler(setupTestViews(t), testSite(), appCatalog(), cais.Config{Env: "test"}, setupTestStore(t), nil)
	req := httptest.NewRequest(http.MethodPost, "/sincronizar", nil)
	rr := httptest.NewRecorder()
	h.Post(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rr.Code)
	}
}
