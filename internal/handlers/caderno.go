package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/puppe1990/amarra-cais/pkg/amarra/view"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/flash"
	"github.com/puppe1990/amarra-cais/pkg/cais/httpx"
	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"
	"github.com/puppe1990/amarra-cais/pkg/cais/meta"

	"github.com/puppe1990/lumina-reader/internal/models"
	"github.com/puppe1990/lumina-reader/internal/store"
)

var coresDeDestaque = map[string]bool{
	"amarelo": true, "verde": true, "azul": true, "rosa": true,
}

const (
	maxTrecho = 2000
	maxNota   = 4000
)

type NotesHandler struct {
	views   *view.Renderer
	site    meta.Site
	catalog *i18n.Catalog
	cfg     cais.Config
	store   store.Store
}

func NewNotesHandler(views *view.Renderer, site meta.Site, catalog *i18n.Catalog, cfg cais.Config, s store.Store) *NotesHandler {
	return &NotesHandler{views: views, site: site, catalog: catalog, cfg: cfg, store: s}
}

func (h *NotesHandler) Page(w http.ResponseWriter, r *http.Request) {
	livroID, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("livro")), 10, 64)

	notas, err := h.store.ListNotes(livroID)
	if err != nil {
		writeView(w, r, h.views, h.cfg, "app", "erro", amarraData(r, h.site, map[string]any{
			"Title":   "Caderno",
			"ErroMsg": err.Error(),
		}), http.StatusInternalServerError)
		return
	}

	var filtroLivro *models.Book
	if livroID > 0 {
		if livro, err := h.store.GetBook(livroID); err == nil {
			filtroLivro = &livro
		}
	}

	agora := time.Now()
	views := make([]NotaView, len(notas))
	for i, nota := range notas {
		views[i] = NotaView{
			NoteWithBook: nota,
			ChapterLabel: "Cap. " + strs(nota.ChapterOrdinal+1),
			BookHref:     "/livros/" + itoa(nota.BookID) + "/cap/" + strs(nota.ChapterOrdinal),
			Quando:       quandoHumano(nota.CreatedAt, agora),
		}
	}

	data := amarraData(r, h.site, map[string]any{
		"Title":      "Caderno",
		"ActiveNav":  "notas",
		"Notes":      views,
		"FilterBook": filtroLivro,
	})
	data["NotesTotal"] = len(notas)
	data["BooksCount"] = distinctBooks(notas)
	data["ChaptersCount"] = distinctChapters(notas)

	writeView(w, r, h.views, h.cfg, "app", "notas", data, 0)
}

// NotaView é a anotação pronta para o caderno: rótulos 1-based e tempo humano.
type NotaView struct {
	models.NoteWithBook
	ChapterLabel string
	BookHref     string
	Quando       string
}

// quandoHumano segue o tom do caderno do design: AAAA-MM-DD, "Hoje às 14:20",
// "Ontem às 21:05", "3 dias atrás", "12 de set.".
func quandoHumano(quando time.Time, agora time.Time) string {
	local := quando.In(fusoBRT)
	hoje := agora.In(fusoBRT)
	if local.Format("2006-01-02") == hoje.Format("2006-01-02") {
		return "Hoje às " + local.Format("15:04")
	}
	if local.Format("2006-01-02") == hoje.AddDate(0, 0, -1).Format("2006-01-02") {
		return "Ontem às " + local.Format("15:04")
	}
	dias := int(hoje.Sub(local).Hours() / 24)
	switch {
	case dias >= 2 && dias < 7:
		return strs(dias) + " dias atrás"
	case dias < 60:
		return "em " + local.Format("02/01")
	default:
		return local.Format("02/01/2006")
	}
}

func distinctBooks(notas []models.NoteWithBook) int {
	vistos := map[int64]bool{}
	for _, nota := range notas {
		vistos[nota.BookID] = true
	}
	return len(vistos)
}

func distinctChapters(notas []models.NoteWithBook) int {
	vistos := map[string]bool{}
	for _, nota := range notas {
		vistos[strs(int(nota.BookID))+":"+strs(nota.ChapterOrdinal)] = true
	}
	return len(vistos)
}

// Create salva um destaque vindo da barra de seleção do leitor (fetch JSON).
func (h *NotesHandler) Create(w http.ResponseWriter, r *http.Request, bookID int64) {
	if _, err := h.store.GetBook(bookID); err != nil {
		respondJSON(w, http.StatusNotFound, map[string]any{"ok": false, "erro": "livro não encontrado"})
		return
	}
	if err := httpx.ParseFormOrJSON(r); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "erro": "corpo inválido"})
		return
	}

	trecho := limitar(strings.TrimSpace(r.FormValue("trecho")), maxTrecho)
	if trecho == "" {
		respondJSON(w, http.StatusUnprocessableEntity, map[string]any{"ok": false, "erro": "trecho vazio"})
		return
	}
	cor := strings.TrimSpace(r.FormValue("cor"))
	if !coresDeDestaque[cor] {
		cor = "amarelo"
	}
	ordinal, err := strconv.Atoi(strings.TrimSpace(r.FormValue("cap")))
	if err != nil || ordinal < 0 {
		ordinal = 0
	}

	id, err := h.store.CreateNote(models.Note{
		BookID:         bookID,
		ChapterOrdinal: ordinal,
		Color:          cor,
		Excerpt:        trecho,
		NoteText:       limitar(strings.TrimSpace(r.FormValue("nota")), maxNota),
	})
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "erro": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
}

func (h *NotesHandler) Update(w http.ResponseWriter, r *http.Request, id int64) {
	if err := httpx.ParseFormOrJSON(r); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	texto := limitar(strings.TrimSpace(r.FormValue("nota")), maxNota)
	if err := h.store.UpdateNoteText(id, texto); err != nil {
		http.NotFound(w, r)
		return
	}
	flash.Set(w, "notice", "Anotação salva.", h.cfg.CookieSecure())
	http.Redirect(w, r, "/notas", http.StatusSeeOther)
}

func (h *NotesHandler) Delete(w http.ResponseWriter, r *http.Request, id int64) {
	if err := h.store.DeleteNote(id); err != nil {
		http.NotFound(w, r)
		return
	}
	flash.Set(w, "notice", "Destaque removido.", h.cfg.CookieSecure())
	http.Redirect(w, r, "/notas", http.StatusSeeOther)
}

func limitar(texto string, max int) string {
	if len(texto) <= max {
		return texto
	}
	return strings.ToValidUTF8(texto[:max], "")
}
