package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/puppe1990/amarra-cais/pkg/amarra/view"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/flash"
	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"
	"github.com/puppe1990/amarra-cais/pkg/cais/meta"

	"github.com/puppe1990/lumina/internal/library"
	"github.com/puppe1990/lumina/internal/store"
)

// Sincronizador espelha o balde B2 na estante (nil = B2 não configurado).
type Sincronizador interface {
	Sync(ctx context.Context) (library.Report, error)
}

type SyncHandler struct {
	views         *view.Renderer
	site          meta.Site
	catalog       *i18n.Catalog
	cfg           cais.Config
	store         store.Store
	sincronizador Sincronizador
}

func NewSyncHandler(views *view.Renderer, site meta.Site, catalog *i18n.Catalog, cfg cais.Config, s store.Store, sincronizador Sincronizador) *SyncHandler {
	return &SyncHandler{views: views, site: site, catalog: catalog, cfg: cfg, store: s, sincronizador: sincronizador}
}

func (h *SyncHandler) Post(w http.ResponseWriter, r *http.Request) {
	if h.sincronizador == nil {
		flash.Set(w, "notice", "Configure o Backblaze (B2_APPLICATION_KEY_ID, B2_APPLICATION_KEY e B2_BUCKET) para sincronizar a estante.", h.cfg.CookieSecure())
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	report, err := h.sincronizador.Sync(r.Context())
	if err != nil {
		flash.Set(w, "notice", "Falha ao sincronizar: "+err.Error(), h.cfg.CookieSecure())
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	flash.Set(w, "notice", fmt.Sprintf("Estante sincronizada: %d livros (%d novos).", report.New+report.Updated, report.New), h.cfg.CookieSecure())
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
