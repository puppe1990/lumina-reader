package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/puppe1990/amarra-cais/pkg/amarra/live"
	"github.com/puppe1990/amarra-cais/pkg/amarra/view"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/devlog"
	"github.com/puppe1990/amarra-cais/pkg/cais/devreload"
	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"
	"github.com/puppe1990/amarra-cais/pkg/cais/jobsui"
	"github.com/puppe1990/amarra-cais/pkg/cais/meta"
	"github.com/puppe1990/amarra-cais/pkg/cais/middleware"
	"github.com/puppe1990/amarra-cais/pkg/cais/netutil"

	"github.com/puppe1990/lumina-reader/internal/handlers"
	appi18n "github.com/puppe1990/lumina-reader/internal/i18n"
	"github.com/puppe1990/lumina-reader/internal/reader"
	"github.com/puppe1990/lumina-reader/internal/store"
)

type Deps struct {
	Views     *view.Renderer
	Store     store.Store
	StaticDir string
	Site      meta.Site
	Catalog   *i18n.Catalog
	// Catalogs is the full locale set for LocaleMiddleware and the per-request
	// view renderer (#211). Catalog stays the boot fallback for handler copy.
	Catalogs map[string]*i18n.Catalog
	// DevReload is the development-only SSE hub that tells open tabs a template
	// or stylesheet changed. Nil in production and in tests (#340).
	DevReload *devreload.Hub
	// Leitor prepara e serve os livros do B2; nil cai num serviço sem B2 que só
	// mostra o erro configurado nas páginas.
	Leitor *reader.Service
	// Sincronizador espelha o balde B2 na estante; nil desliga o botão sincronizar.
	Sincronizador handlers.Sincronizador
	// ChatConversador é o provedor de LLM do "conversar com o livro"; nil
	// mantém o botão visível mas responde que falta configuração.
	ChatConversador handlers.ConversadorRLM
}

// remoteNaoConfigurado mantém o app bootável sem credenciais B2 (testes, dev
// sem .env): abrir livro responde o erro de configuração em vez de panicar.
type remoteNaoConfigurado struct{}

func (remoteNaoConfigurado) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("B2 não configurado: defina B2_APPLICATION_KEY_ID, B2_APPLICATION_KEY e B2_BUCKET")
}

type App struct {
	config cais.Config
	store  store.Store
	router *cais.Router
	server *http.Server
}

func New(cfg cais.Config, deps Deps) (*App, error) {
	if deps.Views == nil {
		return nil, fmt.Errorf("views are required")
	}
	if deps.Store == nil {
		return nil, fmt.Errorf("store is required")
	}

	site := deps.Site
	if site.AppName == "" {
		site = meta.SiteFrom("lumina", cfg.AppURL)
	}
	site.Env = cfg.Env
	deps.Site = site

	if deps.Leitor == nil {
		cacheDir := filepath.Join(filepath.Dir(cfg.DBPath), "cache")
		deps.Leitor = reader.New(remoteNaoConfigurado{}, deps.Store, cacheDir)
	}

	r := cais.NewRouter()
	r.Use(middleware.CSRF(cfg))
	r.Use(middleware.LoadSession(deps.Store.Sessions()))
	r.Use(middleware.Flash(cfg))
	catalogs := deps.Catalogs
	if len(catalogs) == 0 {
		catalogs = map[string]*i18n.Catalog{
			"en": appi18n.NewCatalog("en"),
			"pt": appi18n.NewCatalog("pt"),
		}
	}
	r.Use(i18n.LocaleMiddleware(catalogs, cfg.Locale))
	buf := devlog.Prepare(cfg.Env)
	if buf != nil {
		r.Use(middleware.LoggerTo(cfg, devlog.MirrorDefault(log.Writer())))
	} else {
		r.Use(middleware.Logger(cfg))
	}
	r.Use(middleware.Recover)
	r.Use(middleware.SecurityHeaders(cfg))
	r.StaticForEnv("/static", deps.StaticDir, cfg)

	registerRoutes(r, deps, cfg)
	hub := live.NewHub(live.Config{Env: cfg.Env})
	registerLiveViews(hub, deps)
	r.Handle("GET /amarra/live", hub.Handler())
	devlog.Register(r, cfg.Env, buf)
	deps.DevReload.Register(r, cfg.Env)
	if err := jobsui.Register(r, deps.Store.DB()); err != nil {
		return nil, fmt.Errorf("jobs dashboard: %w", err)
	}
	r.Get("/health", healthHandler(deps.Store, cfg))

	return &App{
		config: cfg,
		store:  deps.Store,
		router: r,
		server: &http.Server{
			Addr:              cfg.Port,
			Handler:           r,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			// O chat com o livro espera o provedor de LLM (modelos de raciocínio
			// passam de 30s com folga).
			WriteTimeout: 150 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}, nil
}

func healthHandler(s store.Store, cfg cais.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := "ok"
		code := http.StatusOK
		if err := s.Ping(); err != nil {
			status = "degraded"
			code = http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(netutil.HealthPayload(status, cfg.Port, cfg.Env))
	}
}

func (a *App) Handler() http.Handler {
	return a.router
}

func (a *App) Run() error {
	return a.RunContext(context.Background())
}

func (a *App) RunContext(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- a.server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.server.Shutdown(shutdownCtx); err != nil {
			_ = a.store.Close()
			return err
		}
		<-errCh
		_ = a.store.Close()
		return nil
	case err := <-errCh:
		_ = a.store.Close()
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}
