package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/session"

	appi18n "github.com/puppe1990/lumina/internal/i18n"
	"github.com/puppe1990/lumina/internal/store"
)

func newAuthHandler(t *testing.T) (*AuthHandler, store.Store) {
	t.Helper()
	s := setupTestStore(t)
	h := NewAuthHandler(setupTestViews(t), s, testSite(), s.Sessions(), cais.Config{}, appi18n.DefaultCatalog())
	return h, s
}

// newAuthHandlerForSignup mantém o nome histórico usado no teste de revogação
// (o app não tem mais cadastro) e cria o usuário demo que o fluxo espera.
func newAuthHandlerForSignup(t *testing.T) (*AuthHandler, store.Store) {
	t.Helper()
	h, s := newAuthHandler(t)
	hash, err := session.HashPassword("password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser("demo@example.com", hash); err != nil {
		t.Fatal(err)
	}
	return h, s
}

func TestAuth_RevocationFailureReturnsServerError(t *testing.T) {
	for _, action := range []string{"login", "logout"} {
		t.Run(action, func(t *testing.T) {
			h, s := newAuthHandlerForSignup(t)
			h.cfg = cais.Config{Env: "production"}
			user, err := s.FindUserByEmail("demo@example.com")
			if err != nil {
				t.Fatal(err)
			}
			token, err := s.Sessions().Create(user.ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.DB().Exec("CREATE TRIGGER refuse_delete BEFORE DELETE ON sessions BEGIN SELECT RAISE(FAIL, 'simulated write failure'); END"); err != nil {
				t.Fatal(err)
			}
			form := url.Values{"email": {"demo@example.com"}, "password": {"password"}}
			req := httptest.NewRequest("POST", "/"+action, strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(&http.Cookie{Name: session.DefaultCookieName, Value: token})
			rr := httptest.NewRecorder()
			if action == "login" {
				h.LoginPost(rr, req)
			} else {
				h.LogoutPost(rr, req)
			}
			if rr.Code != 500 || rr.Header().Get("Location") != "" {
				t.Fatalf("failed revocation: status %d, location %q", rr.Code, rr.Header().Get("Location"))
			}
			if len(rr.Result().Cookies()) != 0 {
				t.Fatal("failed revocation must leave cookie unchanged")
			}
			if strings.Contains(rr.Body.String(), token) || strings.Contains(rr.Body.String(), "simulated") {
				t.Fatal("production error must be sanitized")
			}
			if _, ok := s.Sessions().Get(token); !ok {
				t.Fatal("trigger must preserve original session")
			}
		})
	}
}
func TestAuth_Login_redirectsWhenAuthenticated(t *testing.T) {
	h, s := newAuthHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req = session.WithUserID(req, 1)
	rr := httptest.NewRecorder()
	h.Login(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rr.Code)
	}
	_ = s
}

func TestAuth_LoginPost_invalidCredentials(t *testing.T) {
	h, _ := newAuthHandler(t)

	form := url.Values{"email": {"nobody@example.com"}, "password": {"wrong"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.LoginPost(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Invalid email or password") {
		t.Errorf("missing credentials error, got: %s", rr.Body.String())
	}
}

func TestAuth_LoginPost_validCredentials_redirects(t *testing.T) {
	s, err := store.NewSQLiteStore(":memory:", "development")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	h := NewAuthHandler(setupTestViews(t), s, testSite(), s.Sessions(), cais.Config{}, appi18n.DefaultCatalog())

	form := url.Values{"email": {"demo@example.com"}, "password": {"password"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.LoginPost(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303, body: %s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Location") != "/" {
		t.Errorf("Location = %q, want /", rr.Header().Get("Location"))
	}
	foundFlash := false
	for _, c := range rr.Result().Cookies() {
		if c.Name == "cais_flash" && c.Value != "" {
			foundFlash = true
			break
		}
	}
	if !foundFlash {
		t.Error("login must set cais_flash cookie via flash.Set")
	}
}
