package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/puppe1990/amarra-cais/pkg/cais"

	"github.com/puppe1990/lumina-reader/internal/models"
	"github.com/puppe1990/lumina-reader/internal/store"
)

func seedLivro(t *testing.T, s store.Store, key, titulo, autor, category, format string) int64 {
	t.Helper()
	if _, err := s.UpsertBook(models.Book{
		RemoteKey: key, Title: titulo, Author: autor, Category: category, Format: format,
		SizeBytes: 1024, SHA1: "sha-" + key,
	}, nil); err != nil {
		t.Fatal(err)
	}
	livro, err := s.FindBookByKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return livro.ID
}

func newLibraryHandler(t *testing.T, s store.Store) *LibraryHandler {
	t.Helper()
	return NewLibraryHandler(setupTestViews(t), testSite(), appCatalog(), cais.Config{Env: "test"}, s)
}

func TestLibrary_Home_ShowsShelfAndGreeting(t *testing.T) {
	s := setupTestStore(t)
	seedLivro(t, s, "Cat/A/Um.epub", "Livro Um", "Autora Um", "Cat", "epub")
	seedLivro(t, s, "Cat/A/Dois.pdf", "Livro Dois", "Autor Dois", "Cat", "pdf")

	h := newLibraryHandler(t, s)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	for _, esperado := range []string{"id=\"amarra-main\"", "Livro Um", "Livro Dois", "Biblioteca", "Leitura diária"} {
		if !strings.Contains(body, esperado) {
			t.Errorf("body sem %q", esperado)
		}
	}
}

func TestLibrary_Home_FilterChipsNarrowTheShelf(t *testing.T) {
	s := setupTestStore(t)
	seedLivro(t, s, "Cat/A/Um.epub", "Livro Um", "", "Cat", "epub")
	seedLivro(t, s, "Cat/A/Dois.pdf", "Livro Dois", "", "Cat", "pdf")
	// O hero "lendo agora" ignora os filtros; este livro segura o hero para
	// os asserts da grade falarem só sobre o recorte.
	lendo := seedLivro(t, s, "Cat/A/Lendo.epub", "Lendo Um", "", "Cat", "epub")
	if _, err := s.SaveProgress(lendo, 0, 0.2); err != nil {
		t.Fatal(err)
	}

	h := newLibraryHandler(t, s)
	req := httptest.NewRequest(http.MethodGet, "/?f=lendo", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Lendo Um") {
		t.Errorf("filtro lendo deveria manter Lendo Um")
	}
	if strings.Contains(body, "Livro Dois") {
		t.Errorf("filtro lendo não deveria mostrar Livro Dois")
	}

	// O chip de formato (?f=pdf) filtra por formato, não por status.
	req = httptest.NewRequest(http.MethodGet, "/?f=pdf", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	body = rr.Body.String()
	if !strings.Contains(body, "Livro Dois") {
		t.Errorf("filtro pdf deveria manter Livro Dois")
	}
	if strings.Contains(body, "Livro Um") {
		t.Errorf("filtro pdf não deveria mostrar Livro Um")
	}
}

func TestLibrary_Home_EmptyStateInvitesSync(t *testing.T) {
	h := newLibraryHandler(t, setupTestStore(t))
	h.SetSyncEnabled(true)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Sincronizar") {
		t.Errorf("estante vazia deveria convidar a sincronizar, got: %.300s", body)
	}
}

func TestLibrary_Home_ContinueCardUsesLatestProgress(t *testing.T) {
	s := setupTestStore(t)
	um := seedLivro(t, s, "Cat/A/Um.epub", "Livro Um", "", "Cat", "epub")
	seedLivro(t, s, "Cat/A/Dois.pdf", "Livro Dois", "", "Cat", "pdf")
	if _, err := s.SaveProgress(um, 2, 0.5); err != nil {
		t.Fatal(err)
	}

	h := newLibraryHandler(t, s)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Lendo agora") || !strings.Contains(body, "Continuar lendo") {
		t.Errorf("hero de continuação ausente: %.400s", body)
	}
	if !strings.Contains(body, `/livros/`+itoa64(um)+`/cap/2`) {
		t.Errorf("CTA deveria apontar para o capítulo 2: %.400s", body)
	}
}

func TestLibrary_Home_StreakCardShowsStats(t *testing.T) {
	s := setupTestStore(t)
	seedLivro(t, s, "Cat/A/Um.epub", "Livro Um", "", "Cat", "epub")

	h := newLibraryHandler(t, s)
	h.agora = testClock("2026-10-31T15:00:00-03:00")
	if err := s.AddReadingSeconds("2026-10-31", 600); err != nil {
		t.Fatal(err)
	}
	if err := s.AddReadingSeconds("2026-10-30", 600); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "2 dias") {
		t.Errorf("card de sequência sem '2 dias': %.400s", body)
	}
	if !strings.Contains(body, "10 min") {
		t.Errorf("card de sequência sem minutos de hoje: %.400s", body)
	}
}

func TestCollections_ListsCategoriesWithCounts(t *testing.T) {
	s := setupTestStore(t)
	seedLivro(t, s, "Cat/A/Um.epub", "Um", "", "Cat", "epub")
	seedLivro(t, s, "Outra/A/Dois.pdf", "Dois", "", "Outra", "pdf")

	h := NewCollectionsHandler(setupTestViews(t), testSite(), appCatalog(), cais.Config{Env: "test"}, s)
	req := httptest.NewRequest(http.MethodGet, "/colecoes", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Cat") || !strings.Contains(body, "Outra") {
		t.Errorf("coleções sem categorias: %.400s", body)
	}
	if !strings.Contains(body, `?cat=Cat`) {
		t.Errorf("coleção deveria linkar o filtro por categoria")
	}
}

func TestSearch_FindsByTitleAndAuthor(t *testing.T) {
	s := setupTestStore(t)
	seedLivro(t, s, "Cat/A/Um.epub", "Grande Sertão", "João Rosa", "Cat", "epub")
	seedLivro(t, s, "Cat/A/Dois.pdf", "Outro Livro", "Maria Silva", "Cat", "pdf")

	h := NewSearchHandler(setupTestViews(t), testSite(), appCatalog(), cais.Config{Env: "test"}, s)
	req := httptest.NewRequest(http.MethodGet, "/buscar?q=sertão", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Grande Sertão") || strings.Contains(body, "Outro Livro") {
		t.Errorf("busca errada: %.400s", body)
	}

	req = httptest.NewRequest(http.MethodGet, "/buscar?q=maria", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if !strings.Contains(rr.Body.String(), "Outro Livro") {
		t.Errorf("busca por autora deveria achar Outro Livro")
	}
}

func itoa64(v int64) string {
	dígitos := "0123456789"
	if v == 0 {
		return "0"
	}
	var out []byte
	for v > 0 {
		out = append([]byte{dígitos[v%10]}, out...)
		v /= 10
	}
	return string(out)
}
