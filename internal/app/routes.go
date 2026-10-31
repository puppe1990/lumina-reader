package app

import (
	"net/http"

	"github.com/puppe1990/amarra-cais/pkg/amarra/live"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/middleware"

	"github.com/puppe1990/lumina/internal/handlers"
)

func registerRoutes(r *cais.Router, deps Deps, cfg cais.Config) {
	library := handlers.NewLibraryHandler(deps.Views, deps.Site, deps.Catalog, cfg, deps.Store)
	library.SetSyncEnabled(deps.Sincronizador != nil)
	book := handlers.NewBookHandler(deps.Views, deps.Site, deps.Catalog, cfg, deps.Store, deps.Leitor)
	leitor := handlers.NewReaderHandler(deps.Views, deps.Site, deps.Catalog, cfg, deps.Store, deps.Leitor)
	caderno := handlers.NewNotesHandler(deps.Views, deps.Site, deps.Catalog, cfg, deps.Store)
	colecoes := handlers.NewCollectionsHandler(deps.Views, deps.Site, deps.Catalog, cfg, deps.Store)
	busca := handlers.NewSearchHandler(deps.Views, deps.Site, deps.Catalog, cfg, deps.Store)
	sincronizar := handlers.NewSyncHandler(deps.Views, deps.Site, deps.Catalog, cfg, deps.Store, deps.Sincronizador)
	naoEncontrado := handlers.NewNotFoundHandler(deps.Views, deps.Site, deps.Catalog, cfg)
	auth := handlers.NewAuthHandler(deps.Views, deps.Store, deps.Site, deps.Store.Sessions(), cfg, deps.Catalog)

	loginLimit := middleware.NewRateLimiter(10, cfg)
	exigir := func(h http.HandlerFunc) http.HandlerFunc {
		return middleware.RequireAuthFunc("/login", h)
	}

	r.Get("/login", auth.Login)
	r.Post("/login", loginLimit.Middleware(http.HandlerFunc(auth.LoginPost)).ServeHTTP)
	r.Post("/logout", auth.LogoutPost)
	r.Post("/locale", handlers.PostLocale(cfg))

	r.Get("/", exigir(library.ServeHTTP))
	r.Get("/lendo", exigir(library.Resume))
	r.Get("/livros/{id}", exigir(cais.IntParam("id", book.Show)))
	r.Get("/livros/{id}/cap/{n}", exigir(cais.IntStringParams("id", "n", leitor.Chapter)))
	r.Get("/livros/{id}/arquivo", exigir(cais.IntParam("id", book.File)))
	r.Get("/livros/{id}/recurso/{nome...}", exigir(cais.IntStringParams("id", "nome", book.Resource)))
	r.Post("/livros/{id}/progresso", exigir(cais.IntParam("id", leitor.Progress)))
	r.Post("/livros/{id}/pulso", exigir(cais.IntParam("id", leitor.Pulse)))
	r.Post("/livros/{id}/marcador", exigir(cais.IntParam("id", leitor.Bookmark)))
	r.Post("/livros/{id}/notas", exigir(cais.IntParam("id", caderno.Create)))
	r.Put("/notas/{id}", exigir(cais.IntParam("id", caderno.Update)))
	r.Delete("/notas/{id}", exigir(cais.IntParam("id", caderno.Delete)))
	r.Get("/notas", exigir(caderno.Page))
	r.Get("/colecoes", exigir(colecoes.ServeHTTP))
	r.Get("/buscar", exigir(busca.ServeHTTP))
	r.Post("/sincronizar", exigir(sincronizar.Post))

	r.NotFound(naoEncontrado.ServeHTTP)
}

func registerLiveViews(hub *live.Hub, deps Deps) {
	_ = hub
	_ = deps
	// cais:live-views
}
