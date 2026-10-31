package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/puppe1990/amarra-cais/pkg/amarra/view"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/httpx"
	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"
	"github.com/puppe1990/amarra-cais/pkg/cais/meta"

	"github.com/puppe1990/lumina-reader/internal/reader"
	"github.com/puppe1990/lumina-reader/internal/store"
)

type ReaderHandler struct {
	views   *view.Renderer
	site    meta.Site
	catalog *i18n.Catalog
	cfg     cais.Config
	store   store.Store
	leitor  *reader.Service
	agora   func() time.Time
}

func NewReaderHandler(views *view.Renderer, site meta.Site, catalog *i18n.Catalog, cfg cais.Config, s store.Store, leitor *reader.Service) *ReaderHandler {
	return &ReaderHandler{views: views, site: site, catalog: catalog, cfg: cfg, store: s, leitor: leitor, agora: time.Now}
}

// ChapterRef alimenta o sumário do leitor sem carregar o HTML dos capítulos.
type ChapterRef struct {
	Ordinal int
	Numero  int
	Title   string
	Ativo   bool
}

func (h *ReaderHandler) Chapter(w http.ResponseWriter, r *http.Request, bookID int64, n string) {
	book, err := h.store.GetBook(bookID)
	if err != nil {
		h.notFound(w, r)
		return
	}
	if book.Format != "epub" {
		http.Redirect(w, r, "/livros/"+itoa(bookID), http.StatusSeeOther)
		return
	}

	capitulos, err := h.leitor.Chapters(r.Context(), book)
	if err != nil {
		writeView(w, r, h.views, h.cfg, "app", "erro", amarraData(r, h.site, map[string]any{
			"Title":   book.Title,
			"ErroMsg": "Não consegui preparar este livro: " + err.Error(),
		}), http.StatusInternalServerError)
		return
	}

	ordinal, err := strconv.Atoi(n)
	if err != nil || ordinal < 0 || ordinal >= len(capitulos) {
		h.notFound(w, r)
		return
	}

	progresso, temProgresso, err := h.store.ProgressFor(bookID)
	if err != nil {
		h.notFound(w, r)
		return
	}
	notas, err := h.store.NotesForChapter(bookID, ordinal)
	if err != nil {
		h.notFound(w, r)
		return
	}

	sumario := make([]ChapterRef, len(capitulos))
	for i, capitulo := range capitulos {
		sumario[i] = ChapterRef{Ordinal: i, Numero: i + 1, Title: capitulo.Title, Ativo: i == ordinal}
	}
	notasJSON, err := json.Marshal(notas)
	if err != nil {
		notasJSON = []byte("[]")
	}

	percent := 0.0
	restaurar := 0.0
	marcado := false
	if temProgresso {
		percent = progresso.Percent
		if progresso.ChapterOrdinal == ordinal {
			restaurar = progresso.ScrollRatio
		}
		if progresso.BookmarkOrdinal != nil && *progresso.BookmarkOrdinal == ordinal {
			marcado = true
		}
	}

	capitulo := capitulos[ordinal]
	writeView(w, r, h.views, h.cfg, "app", "leitor", amarraData(r, h.site, map[string]any{
		"Title":         book.Title + " · " + capitulo.Title,
		"ActiveNav":     "lendo",
		"Book":          book,
		"Chapter":       capitulo,
		"ChapterHTML":   template.HTML(capitulo.HTML),
		"ChapterNumero": "Capítulo " + strs(ordinal+1),
		"Ordinal":       ordinal,
		"Total":         len(capitulos),
		"Prev":          prevOrdinal(ordinal),
		"Next":          nextOrdinal(ordinal, len(capitulos)),
		"Position":      strs(ordinal+1) + " de " + strs(len(capitulos)),
		"Percent":       int(percent + 0.5),
		"MinutesLeft":   minutesFor(capitulo.Words),
		"TOC":           sumario,
		"NotesJSON":     template.JS(notasJSON),
		"Marker":        marcado,
		"Restore":       restaurar,
	}), 0)
}

// Search procura o termo no texto do livro e devolve trechos em JSON — o
// diálogo de busca do leitor consome esse endpoint.
func (h *ReaderHandler) Search(w http.ResponseWriter, r *http.Request, bookID int64) {
	book, err := h.store.GetBook(bookID)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]any{"ok": false, "erro": "livro não encontrado"})
		return
	}
	termo := strings.TrimSpace(r.URL.Query().Get("q"))
	if book.Format != "epub" || len([]rune(termo)) < 2 {
		respondJSON(w, http.StatusOK, map[string]any{"ok": true, "hits": []any{}})
		return
	}
	capitulos, err := h.leitor.Chapters(r.Context(), book)
	if err != nil {
		respondJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "erro": "não consegui preparar o livro"})
		return
	}
	hits := reader.BuscarNoTexto(termo, capitulos, 3, 40)
	respondJSON(w, http.StatusOK, map[string]any{"ok": true, "hits": hits})
}

func (h *ReaderHandler) notFound(w http.ResponseWriter, r *http.Request) {
	writeView(w, r, h.views, h.cfg, "app", "404", amarraData(r, h.site, map[string]any{
		"Title": "Não encontrado",
	}), http.StatusNotFound)
}

// Progress recebe a posição do leitor (capítulo + rolagem) e devolve o
// percentual do livro já calculado para o rodapé.
func (h *ReaderHandler) Progress(w http.ResponseWriter, r *http.Request, bookID int64) {
	if err := httpx.ParseFormOrJSON(r); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "erro": "corpo inválido"})
		return
	}
	ordinal, err := strconv.Atoi(strings.TrimSpace(r.FormValue("cap")))
	if err != nil || ordinal < 0 {
		respondJSON(w, http.StatusUnprocessableEntity, map[string]any{"ok": false, "erro": "capítulo inválido"})
		return
	}
	ratio, err := strconv.ParseFloat(strings.TrimSpace(r.FormValue("ratio")), 64)
	if err != nil {
		ratio = 0
	}
	progresso, err := h.store.SaveProgress(bookID, ordinal, ratio)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "erro": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true, "percent": progresso.Percent})
}

// Pulse acumula o tempo de leitura (o JS manda a cada 30s com a aba visível).
func (h *ReaderHandler) Pulse(w http.ResponseWriter, r *http.Request, bookID int64) {
	_ = bookID
	if err := httpx.ParseFormOrJSON(r); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "erro": "corpo inválido"})
		return
	}
	segundos, err := strconv.Atoi(strings.TrimSpace(r.FormValue("segundos")))
	if err != nil || segundos <= 0 {
		respondJSON(w, http.StatusUnprocessableEntity, map[string]any{"ok": false, "erro": "segundos inválidos"})
		return
	}
	if segundos > 300 {
		segundos = 300
	}
	if err := h.store.AddReadingSeconds(hojeBRT(h.agora()), segundos); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "erro": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// Bookmark liga/desliga o marcador de página no capítulo atual.
func (h *ReaderHandler) Bookmark(w http.ResponseWriter, r *http.Request, bookID int64) {
	if err := httpx.ParseFormOrJSON(r); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "erro": "corpo inválido"})
		return
	}
	ordinal, err := strconv.Atoi(strings.TrimSpace(r.FormValue("cap")))
	if err != nil || ordinal < 0 {
		respondJSON(w, http.StatusUnprocessableEntity, map[string]any{"ok": false, "erro": "capítulo inválido"})
		return
	}
	ativo := strings.EqualFold(strings.TrimSpace(r.FormValue("ativo")), "true")
	if ativo {
		err = h.store.SetBookmark(bookID, ordinal)
	} else {
		err = h.store.ClearBookmark(bookID)
	}
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "erro": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true, "ativo": ativo})
}

func prevOrdinal(ordinal int) int {
	if ordinal <= 0 {
		return -1
	}
	return ordinal - 1
}

func nextOrdinal(ordinal, total int) int {
	if ordinal+1 >= total {
		return -1
	}
	return ordinal + 1
}

func strs(v int) string { return strconv.Itoa(v) }
