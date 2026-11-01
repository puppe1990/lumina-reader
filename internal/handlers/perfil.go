package handlers

import (
	"errors"
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

// ChangeEmail troca o email de login (pede a senha atual para confirmar).
func (h *ProfileHandler) ChangeEmail(w http.ResponseWriter, r *http.Request) {
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
		h.render(w, r, map[string]any{"Errors": validate.FieldErrors{"email": "Não consegui carregar sua conta."}}, http.StatusInternalServerError)
		return
	}

	novo := strings.ToLower(strings.TrimSpace(r.FormValue("novo_email")))
	senha := r.FormValue("senha_email")
	erros := validate.FieldErrors{}
	if err := validate.Email(novo); err != nil {
		erros["novo_email"] = "Digite um email válido."
	}
	if novo != "" && strings.EqualFold(novo, user.Email) {
		erros["novo_email"] = "É o mesmo email de agora."
	}
	if !session.VerifyPassword(user.PasswordHash, senha) {
		erros["senha_email"] = "A senha não confere."
	}
	if len(erros) == 0 {
		if err := h.store.UpdateUserEmail(userID, novo); err != nil {
			if errors.Is(err, store.ErrEmailTaken) {
				erros["novo_email"] = "Este email já é usado por outra conta."
			} else {
				erros["novo_email"] = "Não consegui salvar o email."
			}
		}
	}
	if len(erros) > 0 {
		h.render(w, r, map[string]any{"Errors": erros}, http.StatusUnprocessableEntity)
		return
	}
	flash.Set(w, "notice", "Email atualizado para "+novo+".", h.cfg.CookieSecure())
	http.Redirect(w, r, "/perfil", http.StatusSeeOther)
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
