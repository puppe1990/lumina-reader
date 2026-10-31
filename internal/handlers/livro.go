package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/puppe1990/amarra-cais/pkg/amarra/view"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"
	"github.com/puppe1990/amarra-cais/pkg/cais/meta"

	"github.com/puppe1990/lumina/internal/models"
	"github.com/puppe1990/lumina/internal/reader"
	"github.com/puppe1990/lumina/internal/store"
)

type BookHandler struct {
	views   *view.Renderer
	site    meta.Site
	catalog *i18n.Catalog
	cfg     cais.Config
	store   store.Store
	leitor  *reader.Service
}

func NewBookHandler(views *view.Renderer, site meta.Site, catalog *i18n.Catalog, cfg cais.Config, s store.Store, leitor *reader.Service) *BookHandler {
	return &BookHandler{views: views, site: site, catalog: catalog, cfg: cfg, store: s, leitor: leitor}
}

func (h *BookHandler) bookOrNotFound(w http.ResponseWriter, r *http.Request, id int64) (models.Book, bool) {
	book, err := h.store.GetBook(id)
	if errors.Is(err, store.ErrBookNotFound) {
		h.render404(w, r)
		return models.Book{}, false
	}
	if err != nil {
		h.renderError(w, r, err)
		return models.Book{}, false
	}
	return book, true
}

func (h *BookHandler) render404(w http.ResponseWriter, r *http.Request) {
	writeView(w, r, h.views, h.cfg, "app", "404", amarraData(r, h.site, map[string]any{
		"Title": "Não encontrado",
	}), http.StatusNotFound)
}

func (h *BookHandler) renderError(w http.ResponseWriter, r *http.Request, err error) {
	writeView(w, r, h.views, h.cfg, "app", "erro", amarraData(r, h.site, map[string]any{
		"Title":   "Erro",
		"ErroMsg": err.Error(),
	}), http.StatusInternalServerError)
}

func (h *BookHandler) Show(w http.ResponseWriter, r *http.Request, id int64) {
	book, ok := h.bookOrNotFound(w, r, id)
	if !ok {
		return
	}

	progresso, temProgresso, err := h.store.ProgressFor(book.ID)
	if err != nil {
		h.renderError(w, r, err)
		return
	}
	capitulos, err := h.store.ChaptersFor(book.ID)
	if err != nil {
		h.renderError(w, r, err)
		return
	}
	notas, err := h.store.ListNotes(book.ID)
	if err != nil {
		h.renderError(w, r, err)
		return
	}

	continuar := 0
	if temProgresso {
		continuar = progresso.ChapterOrdinal
	}
	palavras := 0
	for _, capitulo := range capitulos {
		palavras += capitulo.Words
	}

	writeView(w, r, h.views, h.cfg, "app", "livro", amarraData(r, h.site, map[string]any{
		"Title":        book.Title,
		"ActiveNav":    "biblioteca",
		"Book":         book,
		"Tone":         int(book.ID % 6),
		"Progress":     progresso,
		"HasProgress":  temProgresso,
		"Percent":      int(progresso.Percent + 0.5),
		"Chapters":     capitulos,
		"Notes":        notas,
		"Continue":     continuar,
		"MinutesTotal": minutesFor(palavras),
		"FileSize":     humanSize(book.SizeBytes),
	}), 0)
}

// File entrega o original: PDF abre inline no navegador; os demais formatos
// descem como anexo (o leitor web só renderiza EPUB).
func (h *BookHandler) File(w http.ResponseWriter, r *http.Request, id int64) {
	book, ok := h.bookOrNotFound(w, r, id)
	if !ok {
		return
	}
	stream, err := h.leitor.OpenFile(r.Context(), book)
	if err != nil {
		h.renderError(w, r, fmt.Errorf("abrir %q: %w", book.RemoteKey, err))
		return
	}
	defer func() { _ = stream.Close() }()

	nome := lastPathSegment(book.RemoteKey)
	disposicao := "inline"
	if r.URL.Query().Get("baixar") != "" || book.Format != "pdf" {
		disposicao = "attachment"
	}
	w.Header().Set("Content-Type", reader.ContentTypeFor(nome))
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename*=UTF-8''%s", disposicao, escapeQuoted(nome)))
	w.Header().Set("Cache-Control", "private, max-age=3600")
	if _, err := io.Copy(w, stream); err != nil {
		return // conexão caiu no meio do streaming; nada além de log implícito
	}
}

// Resource serve imagens e outros arquivos internos do EPUB.
func (h *BookHandler) Resource(w http.ResponseWriter, r *http.Request, id int64, nome string) {
	book, ok := h.bookOrNotFound(w, r, id)
	if !ok {
		return
	}
	conteudo, tipo, err := h.leitor.Resource(r.Context(), book, nome)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", tipo)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	_, _ = w.Write(conteudo)
}

func lastPathSegment(key string) string {
	partes := strings.Split(key, "/")
	return partes[len(partes)-1]
}

func escapeQuoted(nome string) string {
	substituido := strings.ReplaceAll(nome, "%", "%25")
	substituido = strings.ReplaceAll(substituido, " ", "%20")
	substituido = strings.ReplaceAll(substituido, "\"", "%22")
	return substituido
}

func minutesFor(palavras int) int {
	if palavras <= 0 {
		return 0
	}
	minutos := (palavras + 199) / 200 // ~200 palavras por minuto
	if minutos < 1 {
		minutos = 1
	}
	return minutos
}

func humanSize(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(bytes)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
