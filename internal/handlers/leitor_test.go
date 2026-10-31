package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/puppe1990/amarra-cais/pkg/cais"

	"github.com/puppe1990/lumina-reader/internal/models"
	"github.com/puppe1990/lumina-reader/internal/reader"
	"github.com/puppe1990/lumina-reader/internal/store"
)

type remoteIndisponivel struct{}

func (remoteIndisponivel) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	return nil, context.Canceled // capítulos em cache: o handler não deve precisar do B2
}

func seedCapitulos(t *testing.T, s store.Store, bookID int64, titulos ...string) {
	t.Helper()
	capitulos := make([]models.Chapter, len(titulos))
	for i, titulo := range titulos {
		capitulos[i] = models.Chapter{
			Ordinal: i, Title: titulo,
			HTML:  "<h1>" + titulo + "</h1><p>Texto do capítulo " + titulo + ".</p>",
			Words: 120,
		}
	}
	if err := s.SaveChapters(bookID, capitulos); err != nil {
		t.Fatal(err)
	}
}

func newReaderHandler(t *testing.T, s store.Store) *ReaderHandler {
	t.Helper()
	leitor := reader.New(remoteIndisponivel{}, s, t.TempDir())
	return NewReaderHandler(setupTestViews(t), testSite(), appCatalog(), cais.Config{Env: "test"}, s, leitor)
}

func TestReader_ChapterRendersNavigationAndProgress(t *testing.T) {
	s := setupTestStore(t)
	id := seedLivro(t, s, "Cat/A/Livro.epub", "Livro Ep", "Autora", "Cat", "epub")
	seedCapitulos(t, s, id, "Abertura", "Meio", "Desfecho")

	h := newReaderHandler(t, s)
	req := httptest.NewRequest(http.MethodGet, "/livros/"+itoa64(id)+"/cap/1", nil)
	rr := httptest.NewRecorder()
	h.Chapter(rr, req, id, "1")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	for _, esperado := range []string{"Meio", "Texto do capítulo Meio", "Posição 2 de 3", "Abertura", "Desfecho"} {
		if !strings.Contains(body, esperado) {
			t.Errorf("leitor sem %q", esperado)
		}
	}
}

func TestReader_PdfRedirectsToBookPage(t *testing.T) {
	s := setupTestStore(t)
	id := seedLivro(t, s, "Cat/A/Livro.pdf", "Livro Pdf", "", "Cat", "pdf")

	h := newReaderHandler(t, s)
	req := httptest.NewRequest(http.MethodGet, "/livros/"+itoa64(id)+"/cap/0", nil)
	rr := httptest.NewRecorder()
	h.Chapter(rr, req, id, "0")

	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/livros/"+itoa64(id) {
		t.Errorf("PDF deveria voltar para a página do livro: %d %q", rr.Code, rr.Header().Get("Location"))
	}
}

func TestReader_ProgressPostSavesPercent(t *testing.T) {
	s := setupTestStore(t)
	id := seedLivro(t, s, "Cat/A/Livro.epub", "Livro Ep", "", "Cat", "epub")
	seedCapitulos(t, s, id, "Um", "Dois", "Três", "Quatro")

	h := newReaderHandler(t, s)
	corpo := strings.NewReader(`{"cap":1,"ratio":0.5}`)
	req := httptest.NewRequest(http.MethodPost, "/livros/"+itoa64(id)+"/progresso", corpo)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Progress(rr, req, id)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%.200s", rr.Code, rr.Body.String())
	}
	var resposta struct {
		OK      bool    `json:"ok"`
		Percent float64 `json:"percent"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resposta); err != nil {
		t.Fatalf("resposta não é JSON: %v", err)
	}
	if !resposta.OK || resposta.Percent != 37.5 {
		t.Errorf("resposta = %+v, want ok 37.5", resposta)
	}
	progresso, ok, _ := s.ProgressFor(id)
	if !ok || progresso.ChapterOrdinal != 1 {
		t.Errorf("progresso não salvo: %+v ok=%v", progresso, ok)
	}
}

func TestReader_PulseAddsSeconds(t *testing.T) {
	s := setupTestStore(t)
	id := seedLivro(t, s, "Cat/A/Livro.epub", "Livro Ep", "", "Cat", "epub")

	h := newReaderHandler(t, s)
	h.agora = testClock("2026-10-31T15:00:00-03:00")
	req := httptest.NewRequest(http.MethodPost, "/livros/"+itoa64(id)+"/pulso", strings.NewReader(`{"segundos":30}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Pulse(rr, req, id)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	segundos, err := s.ReadingSecondsOn("2026-10-31")
	if err != nil || segundos != 30 {
		t.Errorf("segundos = %d err=%v, want 30", segundos, err)
	}
}

func TestReader_PDFPageRendersAndProgressUsesPages(t *testing.T) {
	s := setupTestStore(t)
	id := seedLivro(t, s, "Cat/A/Livro.pdf", "Livro Pdf", "", "Cat", "pdf")
	h := newReaderHandler(t, s)

	req := httptest.NewRequest(http.MethodGet, "/livros/"+itoa64(id)+"/pdf", nil)
	rr := httptest.NewRecorder()
	h.PDF(rr, req, id)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "data-pdf-root") {
		t.Fatalf("página do PDF: %d %.200s", rr.Code, rr.Body.String())
	}

	// EPUB não entra no leitor de PDF.
	epub := seedLivro(t, s, "Cat/A/Livro.epub", "Livro Ep", "", "Cat", "epub")
	req = httptest.NewRequest(http.MethodGet, "/livros/"+itoa64(epub)+"/pdf", nil)
	req.SetPathValue("id", itoa64(epub))
	rr = httptest.NewRecorder()
	h.PDF(rr, req, epub)
	if rr.Code != http.StatusSeeOther {
		t.Errorf("EPUB deveria voltar para a página do livro: %d", rr.Code)
	}

	// Progresso do PDF exige total e grava página/percentual.
	corpo := strings.NewReader(`{"cap":29,"ratio":0,"total":120}`)
	req = httptest.NewRequest(http.MethodPost, "/livros/"+itoa64(id)+"/progresso", corpo)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Progress(rr, req, id)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"percent":25`) {
		t.Fatalf("progresso do PDF: %d %.200s", rr.Code, rr.Body.String())
	}
	livro, _ := s.GetBook(id)
	if livro.PdfPages != 120 {
		t.Errorf("PdfPages = %d, want 120", livro.PdfPages)
	}

	req = httptest.NewRequest(http.MethodPost, "/livros/"+itoa64(id)+"/progresso", strings.NewReader(`{"cap":3}`))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Progress(rr, req, id)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("sem total deveria ser 422: %d", rr.Code)
	}
}

func TestReader_SearchReturnsHits(t *testing.T) {
	s := setupTestStore(t)
	id := seedLivro(t, s, "Cat/A/Livro.epub", "Livro Ep", "", "Cat", "epub")
	seedCapitulos(t, s, id, "Abertura", "Meio", "Desfecho")

	h := newReaderHandler(t, s)
	req := httptest.NewRequest(http.MethodGet, "/livros/"+itoa64(id)+"/busca?q=capítulo", nil)
	rr := httptest.NewRecorder()
	h.Search(rr, req, id)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	var resposta struct {
		OK   bool `json:"ok"`
		Hits []struct {
			ChapterOrdinal int    `json:"ChapterOrdinal"`
			Snippet        string `json:"Snippet"`
		} `json:"Hits"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resposta); err != nil {
		t.Fatalf("resposta não é JSON: %v", err)
	}
	// A resposta serializa "hits" (minúsculo) — normaliza pelo corpo cru.
	if !resposta.OK || !strings.Contains(rr.Body.String(), `"Snippet"`) {
		t.Fatalf("resposta = %.300s", rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/livros/"+itoa64(id)+"/busca?q=a", nil)
	rr = httptest.NewRecorder()
	h.Search(rr, req, id)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"hits":[]`) {
		t.Errorf("termo curto deveria devolver hits vazio: %.200s", rr.Body.String())
	}
}

func TestReader_BookmarkToggles(t *testing.T) {
	s := setupTestStore(t)
	id := seedLivro(t, s, "Cat/A/Livro.epub", "Livro Ep", "", "Cat", "epub")
	seedCapitulos(t, s, id, "Um", "Dois")

	h := newReaderHandler(t, s)

	marcar := func(corpo string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/livros/"+itoa64(id)+"/marcador", strings.NewReader(corpo))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.Bookmark(rr, req, id)
		return rr
	}

	if rr := marcar(`{"cap":1,"ativo":true}`); rr.Code != http.StatusOK {
		t.Fatalf("marcar: status %d", rr.Code)
	}
	progresso, _, _ := s.ProgressFor(id)
	if progresso.BookmarkOrdinal == nil || *progresso.BookmarkOrdinal != 1 {
		t.Fatalf("marcador não salvo: %+v", progresso.BookmarkOrdinal)
	}

	if rr := marcar(`{"cap":1,"ativo":false}`); rr.Code != http.StatusOK {
		t.Fatalf("desmarcar: status %d", rr.Code)
	}
	progresso, _, _ = s.ProgressFor(id)
	if progresso.BookmarkOrdinal != nil {
		t.Errorf("marcador deveria estar limpo: %v", *progresso.BookmarkOrdinal)
	}
}
