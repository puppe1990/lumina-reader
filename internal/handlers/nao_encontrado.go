package handlers

import (
	"net/http"

	"github.com/puppe1990/amarra-cais/pkg/amarra/view"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"
	"github.com/puppe1990/amarra-cais/pkg/cais/meta"
)

// NotFoundHandler serve as rotas sem match e os params que não parseiam
// (cais.IntParam) com a mesma página do layout.
type NotFoundHandler struct {
	views   *view.Renderer
	site    meta.Site
	catalog *i18n.Catalog
	cfg     cais.Config
}

func NewNotFoundHandler(views *view.Renderer, site meta.Site, catalog *i18n.Catalog, cfg cais.Config) *NotFoundHandler {
	return &NotFoundHandler{views: views, site: site, catalog: catalog, cfg: cfg}
}

func (h *NotFoundHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	writeView(w, r, h.views, h.cfg, "app", "404", amarraData(r, h.site, map[string]any{
		"Title": "Não encontrado",
	}), http.StatusNotFound)
}
