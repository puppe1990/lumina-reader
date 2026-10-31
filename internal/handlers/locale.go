package handlers

import (
	"net/http"
	"net/url"

	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"
)

// PostLocale persists the kit <.locale-toggle /> choice and redirects back.
func PostLocale(cfg cais.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		i18n.SetCookie(w, r.FormValue("locale"), cfg.CookieSecure())
		http.Redirect(w, r, localeNextPath(r), http.StatusSeeOther)
	}
}

func localeNextPath(r *http.Request) string {
	ref := r.Header.Get("Referer")
	if ref == "" {
		return "/"
	}
	u, err := url.Parse(ref)
	if err != nil || u.Path == "" {
		return "/"
	}
	if u.Host != "" && u.Host != r.Host {
		return "/"
	}
	if u.RawQuery != "" {
		return u.Path + "?" + u.RawQuery
	}
	return u.Path
}
