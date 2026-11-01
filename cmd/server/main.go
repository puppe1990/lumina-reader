package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/puppe1990/amarra-cais/pkg/amarra/view"
	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/boot"
	"github.com/puppe1990/amarra-cais/pkg/cais/devreload"
	"github.com/puppe1990/amarra-cais/pkg/cais/i18n"
	"github.com/puppe1990/amarra-cais/pkg/cais/meta"
	"github.com/puppe1990/amarra-cais/pkg/cais/session"

	"github.com/puppe1990/lumina-reader/internal/app"
	"github.com/puppe1990/lumina-reader/internal/b2"
	"github.com/puppe1990/lumina-reader/internal/chat"
	"github.com/puppe1990/lumina-reader/internal/handlers"
	appi18n "github.com/puppe1990/lumina-reader/internal/i18n"
	"github.com/puppe1990/lumina-reader/internal/library"
	"github.com/puppe1990/lumina-reader/internal/reader"
	"github.com/puppe1990/lumina-reader/internal/store"
	"github.com/puppe1990/lumina-reader/web"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg := cais.Load()
	if err := cfg.Validate(); err != nil {
		return err
	}
	preferredPort := cfg.Port
	port, shifted, err := cais.ResolvePort(cfg.Port, cfg.Env)
	if err != nil {
		return err
	}
	cfg.Port = port

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := bootstrapWithConfig(ctx, cfg)
	if err != nil {
		return err
	}

	shiftedFrom := ""
	if shifted {
		shiftedFrom = preferredPort
	}
	boot.Print(os.Stdout, boot.Options{
		AppName:         "lumina",
		Config:          cfg,
		Version:         boot.CaisVersion(),
		PortShiftedFrom: shiftedFrom,
	})
	return a.RunContext(ctx)
}

func bootstrapWithConfig(ctx context.Context, cfg cais.Config) (*app.App, error) {
	catalog := appi18n.NewCatalog(cfg.Locale)
	catalogs := map[string]*i18n.Catalog{
		"en": appi18n.NewCatalog("en"),
		"pt": appi18n.NewCatalog("pt"),
	}

	views, err := loadViews(cfg, catalog, catalogs)
	if err != nil {
		return nil, fmt.Errorf("views: %w", err)
	}
	devReload := devreload.Watch(ctx, cfg, views.ReloadFromDir)

	s, err := store.NewSQLiteStore(cfg.DBPath, cfg.Env)
	if err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	if err := bootstrapOwner(s); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("usuário inicial: %w", err)
	}
	if err := repararContaDoDono(s); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("conserto da conta: %w", err)
	}

	leitor, sincronizador := buildAcervo(cfg, s)
	if sincronizador != nil {
		go sincronizarSeVazio(ctx, s, sincronizador)
	}

	staticDir, err := cais.ResolveWebDir("static", cfg.StaticDir)
	if err != nil {
		_ = s.Close()
		return nil, err
	}

	return app.New(cfg, app.Deps{
		Views:           views,
		Store:           s,
		StaticDir:       staticDir,
		Site:            meta.SiteFrom("lumina", cfg.AppURL),
		Catalog:         catalog,
		Catalogs:        catalogs,
		DevReload:       devReload,
		Leitor:          leitor,
		Sincronizador:   sincronizador,
		ChatConversador: buildChat(),
	})
}

// buildChat monta o provedor do "conversar com o livro" (OpenAI-compatível —
// NVIDIA NIM por padrão). Sem LUMINA_CHAT_API_KEY o recurso fica desligado.
func buildChat() handlers.ConversadorRLM {
	chave := strings.TrimSpace(os.Getenv("LUMINA_CHAT_API_KEY"))
	if chave == "" {
		return nil
	}
	base := strings.TrimSpace(os.Getenv("LUMINA_CHAT_BASE_URL"))
	if base == "" {
		base = "https://integrate.api.nvidia.com/v1"
	}
	modelo := strings.TrimSpace(os.Getenv("LUMINA_CHAT_MODEL"))
	if modelo == "" {
		modelo = "minimaxai/minimax-m3.1-flash"
	}
	return chat.NovoCliente(base, chave, modelo)
}

// bootstrapOwner cria o único usuário do app a partir de
// OWNER_EMAIL/OWNER_PASSWORD enquanto a tabela estiver vazia (não há cadastro)
// e aplica um conserto de acesso quando OWNER_RESET_EMAIL está setado.
func bootstrapOwner(s *store.SQLiteStore) error {
	email := strings.TrimSpace(os.Getenv("OWNER_EMAIL"))
	senha := os.Getenv("OWNER_PASSWORD")
	if email == "" || senha == "" {
		return nil
	}
	total, err := s.CountUsers()
	if err != nil || total > 0 {
		return err
	}
	hash, err := session.HashPassword(senha)
	if err != nil {
		return err
	}
	if _, err := s.CreateUser(email, hash); err != nil {
		return err
	}
	log.Printf("lumina: usuário inicial %s criado", email)
	return nil
}

// repararContaDoDono troca email/senha do único usuário no boot quando
// OWNER_RESET_EMAIL (e opcionalmente OWNER_RESET_PASSWORD) estão setados.
// Serve de recuperação de acesso; remova as envs depois de usar.
func repararContaDoDono(s *store.SQLiteStore) error {
	email := strings.ToLower(strings.TrimSpace(os.Getenv("OWNER_RESET_EMAIL")))
	if email == "" {
		return nil
	}
	id, err := s.SingleUserID()
	if err != nil {
		return nil // tabela vazia: o seed acima cuida
	}
	if err := s.UpdateUserEmail(id, email); err != nil {
		return fmt.Errorf("conserto do email: %w", err)
	}
	if senha := os.Getenv("OWNER_RESET_PASSWORD"); senha != "" {
		hash, err := session.HashPassword(senha)
		if err != nil {
			return fmt.Errorf("conserto da senha: %w", err)
		}
		if err := s.UpdateUserPassword(id, hash); err != nil {
			return fmt.Errorf("conserto da senha: %w", err)
		}
	}
	log.Printf("lumina: conta do dono reparada no boot (email %s)", email)
	return nil
}

// baldeB2 liga o cliente B2 a um balde concreto para as interfaces de sync e
// leitura.
type baldeB2 struct {
	cliente *b2.Client
	bucket  string
}

func (b baldeB2) List(ctx context.Context) ([]library.RemoteFile, error) {
	return b.cliente.ListRemote(ctx, b.bucket)
}

func (b baldeB2) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	return b.cliente.Open(ctx, b.bucket, key)
}

// buildAcervo monta o leitor e o sincronizador; sem credenciais B2 o leitor
// responde o erro de configuração e o botão sincronizar fica desligado.
func buildAcervo(cfg cais.Config, s *store.SQLiteStore) (*reader.Service, interface {
	Sync(ctx context.Context) (library.Report, error)
}) {
	cacheDir := filepath.Join(filepath.Dir(cfg.DBPath), "cache")
	keyID := strings.TrimSpace(os.Getenv("B2_APPLICATION_KEY_ID"))
	key := strings.TrimSpace(os.Getenv("B2_APPLICATION_KEY"))
	bucket := strings.TrimSpace(os.Getenv("B2_BUCKET"))
	if keyID == "" || key == "" || bucket == "" {
		return reader.New(remoteSemB2{}, s, cacheDir), nil
	}
	balde := baldeB2{cliente: b2.New(keyID, key), bucket: bucket}
	return reader.New(balde, s, cacheDir), library.NewService(balde, s)
}

type remoteSemB2 struct{}

func (remoteSemB2) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("B2 não configurado: defina B2_APPLICATION_KEY_ID, B2_APPLICATION_KEY e B2_BUCKET")
}

// sincronizarSeVazio popula a estante na primeira subida (banco novo no deploy),
// em background para não atrasar o boot.
func sincronizarSeVazio(ctx context.Context, s *store.SQLiteStore, sincronizador interface {
	Sync(ctx context.Context) (library.Report, error)
}) {
	totais, err := s.LibraryTotals()
	if err != nil || totais.Total > 0 {
		return
	}
	report, err := sincronizador.Sync(ctx)
	if err != nil {
		log.Printf("lumina: sincronização inicial falhou: %v", err)
		return
	}
	log.Printf("lumina: sincronização inicial: %d livros (%d novos)", report.New+report.Updated, report.New)
}

func loadViews(cfg cais.Config, catalog *i18n.Catalog, catalogs map[string]*i18n.Catalog) (*view.Renderer, error) {
	tmplFS, err := templateFS(cfg)
	if err != nil {
		return nil, err
	}
	return view.Load(tmplFS, catalog, catalogs["en"], catalogs["pt"])
}

// templateFS reads web/templates from disk in development so the dev reload can
// swap the page set in-process; production keeps the embedded copy (#340).
func templateFS(cfg cais.Config) (fs.FS, error) {
	if cfg.Env != "development" {
		return fs.Sub(web.Templates, "templates")
	}
	dir, err := cais.ResolveWebDir("templates", cfg.TemplatesDir)
	if err != nil {
		return nil, err
	}
	info, statErr := os.Stat(dir)
	if statErr != nil || !info.IsDir() {
		return fs.Sub(web.Templates, "templates")
	}
	return os.DirFS(dir), nil
}
