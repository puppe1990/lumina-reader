package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"
)

func TestPostLocale_setsCookieAndRedirects(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/locale", strings.NewReader("locale=pt"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Host = "example.com"
	req.Header.Set("Referer", "https://example.com/dashboard")
	PostLocale(cais.Config{}).ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "/dashboard" {
		t.Errorf("Location = %q, want /dashboard", got)
	}
	var cookie *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == i18n.CookieName {
			cookie = c
			break
		}
	}
	if cookie == nil || cookie.Value != "pt" {
		t.Fatalf("cais_locale = %+v, want pt", cookie)
	}
}

func TestPostLocale_rejectsOffHostReferer(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/locale", strings.NewReader("locale=en"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Host = "example.com"
	req.Header.Set("Referer", "https://evil.example/phish")
	PostLocale(cais.Config{}).ServeHTTP(rr, req)
	if got := rr.Header().Get("Location"); got != "/" {
		t.Errorf("Location = %q, want /", got)
	}
}
