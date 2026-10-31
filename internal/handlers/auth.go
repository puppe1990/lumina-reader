package handlers

import (
	"net/http"
	"strings"

	"github.com/puppe1990/amarra-cais/pkg/amarra/view"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/flash"
	"github.com/puppe1990/amarra-cais/pkg/cais/httpx"
	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"
	"github.com/puppe1990/amarra-cais/pkg/cais/meta"
	"github.com/puppe1990/amarra-cais/pkg/cais/session"
	"github.com/puppe1990/amarra-cais/pkg/cais/validate"

	"github.com/puppe1990/lumina-reader/internal/store"
)

// AuthHandler é só o portão da estante: login e logout. A biblioteca é pessoal
// (espelha o acervo inteiro), então não existe cadastro público nem reset.
type AuthHandler struct {
	views    *view.Renderer
	store    store.Store
	site     meta.Site
	sessions session.Store
	cfg      cais.Config
	catalog  *i18n.Catalog
}

func NewAuthHandler(views *view.Renderer, s store.Store, site meta.Site, sessions session.Store, cfg cais.Config, catalog *i18n.Catalog) *AuthHandler {
	return &AuthHandler{views: views, store: s, site: site, sessions: sessions, cfg: cfg, catalog: catalog}
}

func (h *AuthHandler) t(r *http.Request, key string, args ...any) string {
	return i18n.CatalogOr(r, h.catalog).T(key, args...)
}

func (h *AuthHandler) renderAuth(w http.ResponseWriter, r *http.Request, name string, extra map[string]any, status int) {
	if extra == nil {
		extra = map[string]any{}
	}
	if _, ok := extra["Title"]; !ok {
		extra["Title"] = h.t(r, "auth.login_title")
	}
	writeView(w, r, h.views, h.cfg, "app", name, amarraData(r, h.site, extra), status)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if _, ok := session.UserID(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	email := ""
	if h.cfg.Env != "production" {
		email = "demo@example.com"
	}
	h.renderAuth(w, r, "login", map[string]any{
		"Title": h.t(r, "auth.login_title"),
		"Email": email,
	}, 0)
}

func (h *AuthHandler) LoginPost(w http.ResponseWriter, r *http.Request) {
	if err := httpx.ParseFormOrJSON(r); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	user, err := h.store.FindUserByEmail(email)
	if err != nil || !session.VerifyPassword(user.PasswordHash, password) {
		h.renderAuth(w, r, "login", map[string]any{
			"Title":  h.t(r, "auth.login_title"),
			"Email":  email,
			"Errors": validate.FieldErrors{"email": h.t(r, "auth.invalid_credentials")},
		}, http.StatusUnprocessableEntity)
		return
	}

	if err := session.SignIn(w, h.sessions, r, user.ID, session.CookieOptionsFromConfig(h.cfg)); err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	flash.Set(w, "notice", h.t(r, "auth.welcome"), h.cfg.CookieSecure())
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) LogoutPost(w http.ResponseWriter, r *http.Request) {
	if err := session.SignOut(w, h.sessions, r, session.CookieOptionsFromConfig(h.cfg)); err != nil {
		httpx.ServerError(w, err, h.cfg)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
