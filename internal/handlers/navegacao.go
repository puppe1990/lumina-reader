package handlers

import (
	"net/http"
	"strings"

	"github.com/puppe1990/amarra-cais/pkg/amarra/view"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"
	"github.com/puppe1990/amarra-cais/pkg/cais/meta"

	"github.com/puppe1990/lumina-reader/internal/models"
	"github.com/puppe1990/lumina-reader/internal/store"
)

type CollectionsHandler struct {
	views   *view.Renderer
	site    meta.Site
	catalog *i18n.Catalog
	cfg     cais.Config
	store   store.Store
}

func NewCollectionsHandler(views *view.Renderer, site meta.Site, catalog *i18n.Catalog, cfg cais.Config, s store.Store) *CollectionsHandler {
	return &CollectionsHandler{views: views, site: site, catalog: catalog, cfg: cfg, store: s}
}

func (h *CollectionsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cats, err := h.store.CategoryCounts()
	if err != nil {
		writeView(w, r, h.views, h.cfg, "app", "erro", amarraData(r, h.site, map[string]any{
			"Title":   "Coleções",
			"ErroMsg": err.Error(),
		}), http.StatusInternalServerError)
		return
	}
	totais, err := h.store.LibraryTotals()
	if err != nil {
		totais = models.LibraryTotals{}
	}

	writeView(w, r, h.views, h.cfg, "app", "colecoes", amarraData(r, h.site, map[string]any{
		"Title":      "Coleções",
		"ActiveNav":  "colecoes",
		"Categories": cats,
		"Totals":     totais,
	}), 0)
}

type SearchHandler struct {
	views   *view.Renderer
	site    meta.Site
	catalog *i18n.Catalog
	cfg     cais.Config
	store   store.Store
}

func NewSearchHandler(views *view.Renderer, site meta.Site, catalog *i18n.Catalog, cfg cais.Config, s store.Store) *SearchHandler {
	return &SearchHandler{views: views, site: site, catalog: catalog, cfg: cfg, store: s}
}

func (h *SearchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	termo := strings.TrimSpace(r.URL.Query().Get("q"))
	var livros []models.BookListItem

	if termo != "" {
		achados, err := h.store.ListBooks(store.BookFilter{Search: termo})
		if err != nil {
			writeView(w, r, h.views, h.cfg, "app", "erro", amarraData(r, h.site, map[string]any{
				"Title":   "Buscar",
				"ErroMsg": err.Error(),
			}), http.StatusInternalServerError)
			return
		}
		livros = achados
	}

	writeView(w, r, h.views, h.cfg, "app", "buscar", amarraData(r, h.site, map[string]any{
		"Title":     "Buscar",
		"ActiveNav": "buscar",
		"Query":     termo,
		"Books":     livros,
	}), 0)
}
