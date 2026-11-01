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

// senhaMinima é o piso da senha nova ao trocar em /perfil.
const senhaMinima = 8

// ProfileHandler cuida da conta do dono do acervo: ver o email e trocar a senha
// (não há cadastro público nem vários usuários).
type ProfileHandler struct {
	views   *view.Renderer
	store   store.Store
	site    meta.Site
	cfg     cais.Config
	catalog *i18n.Catalog
}

func NewProfileHandler(views *view.Renderer, s store.Store, site meta.Site, cfg cais.Config, catalog *i18n.Catalog) *ProfileHandler {
	return &ProfileHandler{views: views, store: s, site: site, cfg: cfg, catalog: catalog}
}

func (h *ProfileHandler) render(w http.ResponseWriter, r *http.Request, extra map[string]any, status int) {
	userID, ok := session.UserID(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	user, err := h.store.GetUser(userID)
	if err != nil {
		extra = map[string]any{"ErroMsg": "Não consegui carregar sua conta."}
	}
	if extra == nil {
		extra = map[string]any{}
	}
	extra["Title"] = "Conta"
	extra["Email"] = user.Email
	writeView(w, r, h.views, h.cfg, "app", "perfil", amarraData(r, h.site, extra), status)
}

// Page mostra o formulário de troca de senha.
func (h *ProfileHandler) Page(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, nil, 0)
}

// ChangePassword valida a senha atual e grava a nova.
func (h *ProfileHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if err := httpx.ParseFormOrJSON(r); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	userID, ok := session.UserID(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	user, err := h.store.GetUser(userID)
	if err != nil {
		h.render(w, r, map[string]any{"Errors": validate.FieldErrors{"atual": "Não consegui carregar sua conta."}}, http.StatusInternalServerError)
		return
	}

	atual := r.FormValue("atual")
	nova := r.FormValue("nova")
	confirmar := r.FormValue("confirmar")
	erros := validate.FieldErrors{}
	if !session.VerifyPassword(user.PasswordHash, atual) {
		erros["atual"] = "A senha atual não confere."
	}
	if len([]rune(nova)) < senhaMinima {
		erros["nova"] = "Use pelo menos 8 caracteres."
	}
	if strings.TrimSpace(nova) != strings.TrimSpace(confirmar) {
		erros["confirmar"] = "A confirmação não bate com a senha nova."
	}
	if len(erros) > 0 {
		h.render(w, r, map[string]any{"Errors": erros}, http.StatusUnprocessableEntity)
		return
	}

	hash, err := session.HashPassword(nova)
	if err != nil {
		h.render(w, r, map[string]any{"Errors": validate.FieldErrors{"nova": "Não consegui gerar a senha."}}, http.StatusInternalServerError)
		return
	}
	if err := h.store.UpdateUserPassword(userID, hash); err != nil {
		h.render(w, r, map[string]any{"Errors": validate.FieldErrors{"nova": "Não consegui salvar a senha."}}, http.StatusInternalServerError)
		return
	}
	flash.Set(w, "notice", "Senha alterada.", h.cfg.CookieSecure())
	http.Redirect(w, r, "/perfil", http.StatusSeeOther)
}
