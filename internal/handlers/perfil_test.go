package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/session"

	appi18n "github.com/puppe1990/lumina-reader/internal/i18n"
	"github.com/puppe1990/lumina-reader/internal/store"
)

func newProfileHandler(t *testing.T) (*ProfileHandler, store.Store, int64) {
	t.Helper()
	s := setupTestStore(t)
	hash, err := session.HashPassword("password")
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.CreateUser("dono@example.com", hash)
	if err != nil {
		t.Fatal(err)
	}
	h := NewProfileHandler(setupTestViews(t), s, testSite(), cais.Config{}, appi18n.DefaultCatalog())
	return h, s, id
}

func pedirTrocaSenha(h *ProfileHandler, id int64, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/perfil/senha", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = session.WithUserID(req, id)
	rr := httptest.NewRecorder()
	h.ChangePassword(rr, req)
	return rr
}

func TestPerfil_PaginaMostraFormulario(t *testing.T) {
	h, _, id := newProfileHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/perfil", nil)
	req = session.WithUserID(req, id)
	rr := httptest.NewRecorder()
	h.Page(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	corpo := rr.Body.String()
	for _, alvo := range []string{"dono@example.com", "Senha atual", "Senha nova", "Confirmar a senha nova", `href="/perfil"`} {
		if !strings.Contains(corpo, alvo) {
			t.Errorf("página sem %q", alvo)
		}
	}
}

func TestPerfil_TrocaSenhaComSucesso(t *testing.T) {
	h, s, id := newProfileHandler(t)
	form := url.Values{"atual": {"password"}, "nova": {"SenhaNova2026"}, "confirmar": {"SenhaNova2026"}}
	rr := pedirTrocaSenha(h, id, form)
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/perfil" {
		t.Fatalf("status = %d, location %q", rr.Code, rr.Header().Get("Location"))
	}
	user, err := s.GetUser(id)
	if err != nil {
		t.Fatal(err)
	}
	if !session.VerifyPassword(user.PasswordHash, "SenhaNova2026") {
		t.Errorf("senha nova não foi gravada")
	}
	if session.VerifyPassword(user.PasswordHash, "password") {
		t.Errorf("senha antiga ainda vale")
	}
}

func TestPerfil_SenhaAtualErrada(t *testing.T) {
	h, s, id := newProfileHandler(t)
	rr := pedirTrocaSenha(h, id, url.Values{"atual": {"errada"}, "nova": {"SenhaNova2026"}, "confirmar": {"SenhaNova2026"}})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "senha atual não confere") {
		t.Errorf("faltou o erro da senha atual")
	}
	user, _ := s.GetUser(id)
	if !session.VerifyPassword(user.PasswordHash, "password") {
		t.Errorf("a senha não deveria mudar com a atual errada")
	}
}

func TestPerfil_ValidacoesDaSenhaNova(t *testing.T) {
	casos := []struct {
		nome   string
		form   url.Values
		trecho string
	}{
		{"curta", url.Values{"atual": {"password"}, "nova": {"curta"}, "confirmar": {"curta"}}, "pelo menos 8"},
		{"confirmação diferente", url.Values{"atual": {"password"}, "nova": {"SenhaNova2026"}, "confirmar": {"OutraSenha123"}}, "confirmação não bate"},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			h, s, id := newProfileHandler(t)
			rr := pedirTrocaSenha(h, id, caso.form)
			if rr.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422", rr.Code)
			}
			if !strings.Contains(rr.Body.String(), caso.trecho) {
				t.Errorf("faltou %q no corpo", caso.trecho)
			}
			user, _ := s.GetUser(id)
			if !session.VerifyPassword(user.PasswordHash, "password") {
				t.Errorf("a senha não deveria mudar")
			}
		})
	}
}

func TestPerfil_SemSessaoRedireciona(t *testing.T) {
	h, _, _ := newProfileHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/perfil", nil)
	rr := httptest.NewRecorder()
	h.Page(rr, req)
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/login" {
		t.Fatalf("status = %d, location %q", rr.Code, rr.Header().Get("Location"))
	}
}

func TestStore_UpdateUserPassword(t *testing.T) {
	s := setupTestStore(t)
	hash, err := session.HashPassword("primeira123")
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.CreateUser("conta@example.com", hash)
	if err != nil {
		t.Fatal(err)
	}
	novo, err := session.HashPassword("segunda4567")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateUserPassword(id, novo); err != nil {
		t.Fatalf("UpdateUserPassword: %v", err)
	}
	user, err := s.GetUser(id)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if !session.VerifyPassword(user.PasswordHash, "segunda4567") {
		t.Errorf("hash não foi atualizado")
	}
	if _, err := s.GetUser(999); err == nil {
		t.Errorf("GetUser de id inexistente deveria falhar")
	}
}
