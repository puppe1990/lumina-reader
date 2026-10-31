package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/puppe1990/amarra-cais/pkg/amarra/view"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"
	"github.com/puppe1990/amarra-cais/pkg/cais/meta"

	"github.com/puppe1990/lumina-reader/internal/models"
	"github.com/puppe1990/lumina-reader/internal/store"
)

// fusoBRT fixa o dia de leitura na hora de São Paulo: o servidor roda em UTC e
// "leu hoje" não pode virar amanhã às 21h.
var fusoBRT = time.FixedZone("BRT", -3*60*60)

func hojeBRT(agora time.Time) string { return agora.In(fusoBRT).Format("2006-01-02") }

func saudacao(agora time.Time) string {
	switch hora := agora.In(fusoBRT).Hour(); {
	case hora >= 5 && hora < 12:
		return "Bom dia"
	case hora >= 12 && hora < 18:
		return "Boa tarde"
	default:
		return "Boa noite"
	}
}

func respondJSON(w http.ResponseWriter, status int, corpo map[string]any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(corpo)
}

// ContinueReading é o livro em andamento para o hero da estante.
type ContinueReading struct {
	Book         models.BookListItem
	Ordinal      int
	Total        int
	ChapterLabel string
}

type LibraryHandler struct {
	views       *view.Renderer
	site        meta.Site
	catalog     *i18n.Catalog
	cfg         cais.Config
	store       store.Store
	syncEnabled bool
	agora       func() time.Time
}

func NewLibraryHandler(views *view.Renderer, site meta.Site, catalog *i18n.Catalog, cfg cais.Config, s store.Store) *LibraryHandler {
	return &LibraryHandler{views: views, site: site, catalog: catalog, cfg: cfg, store: s, agora: time.Now}
}

func (h *LibraryHandler) SetSyncEnabled(enabled bool) { h.syncEnabled = enabled }

func (h *LibraryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	filtro := r.URL.Query().Get("f")
	categoria := r.URL.Query().Get("cat")

	// O chip de formato usa o mesmo ?f= dos chips de status.
	filtroFormato := ""
	if filtro == "pdf" || filtro == "epub" {
		filtroFormato = filtro
		filtro = ""
	}

	livros, err := h.store.ListBooks(store.BookFilter{Status: filtro, Format: filtroFormato, Category: categoria})
	if err != nil {
		writeView(w, r, h.views, h.cfg, "app", "erro", amarraData(r, h.site, map[string]any{
			"Title":   "Biblioteca",
			"ErroMsg": err.Error(),
		}), http.StatusInternalServerError)
		return
	}
	todos, err := h.store.ListBooks(store.BookFilter{})
	if err != nil {
		todos = livros
	}
	totais, err := h.store.LibraryTotals()
	if err != nil {
		totais = models.LibraryTotals{}
	}
	stats, err := h.leituraStats()
	if err != nil {
		stats = models.ReadingStats{GoalMinutes: models.ReadingGoalMinutes}
	}

	writeView(w, r, h.views, h.cfg, "app", "home", amarraData(r, h.site, map[string]any{
		"Title":        "Biblioteca",
		"ActiveNav":    "biblioteca",
		"Greeting":     saudacao(h.agora()),
		"Books":        livros,
		"Totals":       totais,
		"Stats":        stats,
		"GoalPercent":  goalPercent(stats),
		"MinutesToday": stats.SecondsToday / 60,
		"Filter":       filtro,
		"Category":     categoria,
		"Continue":     h.continuarLendo(todos),
		"SyncEnabled":  h.syncEnabled,
	}), 0)
}

func goalPercent(stats models.ReadingStats) int {
	if stats.GoalMinutes <= 0 {
		return 0
	}
	percent := stats.SecondsToday * 100 / (stats.GoalMinutes * 60)
	if percent > 100 {
		percent = 100
	}
	return percent
}

func (h *LibraryHandler) leituraStats() (models.ReadingStats, error) {
	dias, err := h.store.ReadingDays()
	if err != nil {
		return models.ReadingStats{}, err
	}
	hoje := hojeBRT(h.agora())
	segundos, err := h.store.ReadingSecondsOn(hoje)
	if err != nil {
		return models.ReadingStats{}, err
	}
	return models.ReadingStats{
		StreakDays:   models.Streak(dias, hoje),
		SecondsToday: segundos,
		GoalMinutes:  models.ReadingGoalMinutes,
	}, nil
}

// continuarLendo escolhe o livro mais recente ainda em andamento (ou o último
// tocado, se todos terminaram) para o hero "Lendo agora".
func (h *LibraryHandler) continuarLendo(todos []models.BookListItem) *ContinueReading {
	escolhido := -1
	for i, livro := range todos {
		if livro.HasProgress && livro.Percent < 95 {
			escolhido = i
			break
		}
	}
	if escolhido < 0 {
		for i, livro := range todos {
			if livro.HasProgress {
				escolhido = i
				break
			}
		}
	}
	if escolhido < 0 {
		return nil
	}
	livro := todos[escolhido]
	continuar := &ContinueReading{Book: livro}
	if progresso, ok, err := h.store.ProgressFor(livro.ID); err == nil && ok {
		continuar.Ordinal = progresso.ChapterOrdinal
	}
	if total, err := h.store.CountChapters(livro.ID); err == nil {
		continuar.Total = total
	}
	if livro.Format == "epub" {
		continuar.ChapterLabel = "Capítulo " + itoa(int64(continuar.Ordinal)+1)
		if continuar.Total > 0 {
			continuar.ChapterLabel = "Capítulo " + itoa(int64(continuar.Ordinal)+1) + " de " + itoa(int64(continuar.Total))
		}
	} else {
		continuar.ChapterLabel = "PDF aberto no navegador"
	}
	return continuar
}

// LendoHandler responde ao item "Lendo" da navegação: retoma marcador ou o
// último progresso, senão volta para a estante.
func (h *LibraryHandler) Resume(w http.ResponseWriter, r *http.Request) {
	livros, err := h.store.ListBooks(store.BookFilter{})
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	for _, livro := range livros {
		if !livro.HasProgress {
			continue
		}
		progresso, ok, err := h.store.ProgressFor(livro.ID)
		if err != nil || !ok {
			continue
		}
		http.Redirect(w, r, resumeTarget(livro, progresso), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func resumeTarget(livro models.BookListItem, progresso models.Progress) string {
	base := "/livros/" + itoa(livro.ID)
	if livro.Format != "epub" {
		return base
	}
	ordinal := progresso.ChapterOrdinal
	if progresso.BookmarkOrdinal != nil {
		ordinal = *progresso.BookmarkOrdinal
	}
	return base + "/cap/" + strs(ordinal)
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	negativo := v < 0
	if negativo {
		v = -v
	}
	var digitos []byte
	for v > 0 {
		digitos = append([]byte{byte('0' + v%10)}, digitos...)
		v /= 10
	}
	if negativo {
		return "-" + string(digitos)
	}
	return string(digitos)
}
