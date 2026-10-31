package handlers

import (
	"net/http"
	"strings"

	"github.com/puppe1990/amarra-cais/pkg/amarra/view"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/boot"
	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"
	"github.com/puppe1990/amarra-cais/pkg/cais/meta"
	"github.com/puppe1990/amarra-cais/pkg/cais/session"
)

// frameworkVersion is the installed Cais version behind the brand label. A dev
// build reports a Go pseudo-version (0.19.2-0.20261013030325-f939464a88cd), so
// the label keeps the semver core and the tooltip carries the full string.
func frameworkVersion() (label, full string) {
	full = boot.CaisVersion()
	label = full
	if i := strings.Index(label, "-0."); i > 0 {
		label = label[:i]
	}
	return label, full
}

func amarraData(r *http.Request, site meta.Site, extra map[string]any) map[string]any {
	s := meta.ForRequest(site, r)
	// #208: the layout gates auth chrome (rail, logout) on this flag, so a
	// visitor without a session does not see navigation that bounces them.
	_, s.LoggedIn = session.UserID(r)
	label, full := frameworkVersion()
	data := map[string]any{
		"Site":                 s,
		"CSRFToken":            s.CSRFToken,
		"Flash":                s.Flash,
		"FrameworkVersion":     label,
		"FrameworkVersionFull": full,
	}
	if cat := i18n.CatalogFromRequest(r); cat != nil {
		data["Locale"] = cat.Locale()
	}
	for k, v := range extra {
		data[k] = v
	}
	return data
}

// writeView names the layout at the call site: an app with a second layout (a
// marketing "landing") must not render public pages inside the app chrome (#66).
func writeView(w http.ResponseWriter, r *http.Request, views *view.Renderer, cfg cais.Config, layout, name string, data any, status int) {
	view.Write(w, r, views, view.Page{Layout: layout, Name: name, Data: data, Status: status}, cfg)
}
