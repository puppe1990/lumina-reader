// Package reader orquestra o arquivo remoto (B2), o cache em disco e o parse do
// EPUB, gravando os capítulos no SQLite na primeira abertura do livro.
package reader

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/puppe1990/lumina/internal/epub"
	"github.com/puppe1990/lumina/internal/models"
)

type Remote interface {
	Open(ctx context.Context, key string) (io.ReadCloser, error)
}

type ChapterStore interface {
	ChaptersFor(bookID int64) ([]models.Chapter, error)
	SaveChapters(bookID int64, chapters []models.Chapter) error
	CountChapters(bookID int64) (int, error)
}

type Service struct {
	remote   Remote
	store    ChapterStore
	cacheDir string
	// mu serializa a extração: duas abas abrindo o mesmo livro recém-sincronizado
	// não devem baixar/parsear duas vezes.
	mu sync.Mutex
}

func New(remote Remote, store ChapterStore, cacheDir string) *Service {
	return &Service{remote: remote, store: store, cacheDir: cacheDir}
}

// EnsureFile devolve o caminho local do livro, baixando para o cache na primeira
// vez. O nome usa o SHA1 do B2 — arquivo novo = caminho novo, sem invalidação.
func (s *Service) EnsureFile(ctx context.Context, book models.Book) (string, error) {
	if caminho, err := s.cachePath(book); err == nil {
		return caminho, nil
	}
	if err := os.MkdirAll(s.cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("criar cache de livros: %w", err)
	}

	caminho := filepath.Join(s.cacheDir, s.cacheKey(book)+path.Ext(book.RemoteKey))
	corpo, err := s.remote.Open(ctx, book.RemoteKey)
	if err != nil {
		return "", fmt.Errorf("baixar %q do B2: %w", book.RemoteKey, err)
	}
	defer func() { _ = corpo.Close() }()

	temporario, err := os.CreateTemp(s.cacheDir, ".baixando-*")
	if err != nil {
		return "", fmt.Errorf("criar arquivo temporário: %w", err)
	}
	if _, err := io.Copy(temporario, corpo); err != nil {
		_ = temporario.Close()
		_ = os.Remove(temporario.Name())
		return "", fmt.Errorf("gravar %q no cache: %w", book.RemoteKey, err)
	}
	if err := temporario.Close(); err != nil {
		return "", fmt.Errorf("fechar cache de %q: %w", book.RemoteKey, err)
	}
	if err := os.Rename(temporario.Name(), caminho); err != nil {
		return "", fmt.Errorf("publicar cache de %q: %w", book.RemoteKey, err)
	}
	return caminho, nil
}

// Chapters devolve os capítulos limpos; o primeiro acesso baixa e parseia o EPUB
// e o resultado fica no SQLite (reabrir o livro é uma consulta local).
func (s *Service) Chapters(ctx context.Context, book models.Book) ([]models.Chapter, error) {
	if total, err := s.store.CountChapters(book.ID); err != nil {
		return nil, err
	} else if total > 0 {
		return s.store.ChaptersFor(book.ID)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if total, err := s.store.CountChapters(book.ID); err != nil {
		return nil, err
	} else if total > 0 {
		return s.store.ChaptersFor(book.ID)
	}

	caminho, err := s.EnsureFile(ctx, book)
	if err != nil {
		return nil, err
	}
	arquivo, err := os.Open(caminho)
	if err != nil {
		return nil, fmt.Errorf("abrir EPUB em cache %q: %w", caminho, err)
	}
	defer func() { _ = arquivo.Close() }()
	info, err := arquivo.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat EPUB %q: %w", caminho, err)
	}

	parsed, err := epub.Parse(arquivo, info.Size(), epub.ParseOptions{
		ResourceURL: func(caminhoInterno string) string {
			return fmt.Sprintf("/livros/%d/recurso/%s", book.ID, caminhoInterno)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("parsear EPUB %q: %w", book.RemoteKey, err)
	}

	capitulos := make([]models.Chapter, 0, len(parsed.Chapters))
	for i, capitulo := range parsed.Chapters {
		titulo := capitulo.Title
		if titulo == "" {
			titulo = fmt.Sprintf("Capítulo %d", i+1)
		}
		capitulos = append(capitulos, models.Chapter{
			Ordinal: i,
			Title:   titulo,
			HTML:    capitulo.HTML,
			Words:   capitulo.Words,
		})
	}
	if err := s.store.SaveChapters(book.ID, capitulos); err != nil {
		return nil, err
	}
	return capitulos, nil
}

// OpenFile entrega o original: usa o cache em disco quando existe, senão faz
// streaming direto do B2 (o download não precisa ser materializado).
func (s *Service) OpenFile(ctx context.Context, book models.Book) (io.ReadCloser, error) {
	if caminho, err := s.cachePath(book); err == nil {
		if arquivo, err := os.Open(caminho); err == nil {
			return arquivo, nil
		}
	}
	corpo, err := s.remote.Open(ctx, book.RemoteKey)
	if err != nil {
		return nil, fmt.Errorf("baixar %q do B2: %w", book.RemoteKey, err)
	}
	return corpo, nil
}

// ContentTypeFor mapeia a extensão do arquivo para o Content-Type do download.
func ContentTypeFor(nome string) string {
	return epub.ContentTypeFor(nome)
}

func (s *Service) cachePath(book models.Book) (string, error) {
	caminho := filepath.Join(s.cacheDir, s.cacheKey(book)+path.Ext(book.RemoteKey))
	info, err := os.Stat(caminho)
	if err != nil || info.Size() == 0 {
		return "", fmt.Errorf("cache ausente para %q", book.RemoteKey)
	}
	return caminho, nil
}

// cacheKey usa o SHA1 do B2 — arquivo novo = chave nova, sem invalidação.
func (s *Service) cacheKey(book models.Book) string {
	if book.SHA1 == "" || book.SHA1 == "none" {
		return strings.ReplaceAll(book.RemoteKey, "/", "_")
	}
	return book.SHA1
}

// Resource extrai uma imagem (ou outro arquivo interno) do EPUB para o leitor.
func (s *Service) Resource(ctx context.Context, book models.Book, nome string) ([]byte, string, error) {
	if book.Format != "epub" {
		return nil, "", fmt.Errorf("recurso %q: livro %d não é EPUB (formato %q)", nome, book.ID, book.Format)
	}
	caminho, err := s.EnsureFile(ctx, book)
	if err != nil {
		return nil, "", err
	}
	arquivo, err := os.Open(caminho)
	if err != nil {
		return nil, "", fmt.Errorf("abrir EPUB em cache %q: %w", caminho, err)
	}
	defer func() { _ = arquivo.Close() }()
	info, err := arquivo.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("stat EPUB %q: %w", caminho, err)
	}
	return epub.OpenResource(arquivo, info.Size(), nome)
}
