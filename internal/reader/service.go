// Package reader orquestra o arquivo remoto (B2), o cache em disco e o parse do
// EPUB, gravando os capítulos no SQLite na primeira abertura do livro.
package reader

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/image/draw"
	"golang.org/x/image/webp"

	"github.com/puppe1990/lumina-reader/internal/epub"
	"github.com/puppe1990/lumina-reader/internal/models"
)

type Remote interface {
	Open(ctx context.Context, key string) (io.ReadCloser, error)
}

type ChapterStore interface {
	ChaptersFor(bookID int64) ([]models.Chapter, error)
	SaveChapters(bookID int64, chapters []models.Chapter) error
	CountChapters(bookID int64) (int, error)
	CoverFor(bookID int64) (models.Cover, bool, error)
	SaveCover(cover models.Cover) error
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

const (
	capaMaxLargura            = 720
	capaMaxAltura             = 1080
	capaBytesSemRedimensionar = 400 << 10
)

// normalizarCapa deixa a arte leve para a grade: imagens grandes viram JPEG de
// até 720x1080; pequenas (ou formatos que não decodificamos, como SVG) passam
// intactas com o content-type original.
func normalizarCapa(imagem []byte, tipo string) ([]byte, string, error) {
	var decodificada image.Image
	var err error
	switch tipo {
	case "image/png":
		decodificada, err = png.Decode(bytes.NewReader(imagem))
	case "image/jpeg":
		decodificada, err = jpeg.Decode(bytes.NewReader(imagem))
	case "image/gif":
		decodificada, err = gif.Decode(bytes.NewReader(imagem))
	case "image/webp":
		decodificada, err = webp.Decode(bytes.NewReader(imagem))
	default:
		return imagem, tipo, nil
	}
	if err != nil {
		return imagem, tipo, nil
	}

	limites := decodificada.Bounds()
	if limites.Dx() <= capaMaxLargura && limites.Dy() <= capaMaxAltura && len(imagem) <= capaBytesSemRedimensionar {
		return imagem, tipo, nil
	}
	escala := math.Min(1, math.Min(float64(capaMaxLargura)/float64(limites.Dx()), float64(capaMaxAltura)/float64(limites.Dy())))
	largura := int(math.Max(1, float64(limites.Dx())*escala))
	altura := int(math.Max(1, float64(limites.Dy())*escala))
	destino := image.NewRGBA(image.Rect(0, 0, largura, altura))
	draw.CatmullRom.Scale(destino, destino.Bounds(), decodificada, limites, draw.Src, nil)

	var saida bytes.Buffer
	if err := jpeg.Encode(&saida, destino, &jpeg.Options{Quality: 82}); err != nil {
		return imagem, tipo, nil
	}
	return saida.Bytes(), "image/jpeg", nil
}

// ErrNoCover sinaliza livro sem capa identificável (o handler responde 404 e a
// UI cai na capa tipográfica).
var ErrNoCover = errors.New("livro sem capa")

// Cover devolve a arte do livro, extraída uma única vez e cacheada no SQLite.
// EPUB: imagem marcada como capa no arquivo; PDF: 1ª página via pdftoppm
// (quando o poppler está instalado).
func (s *Service) Cover(ctx context.Context, book models.Book) ([]byte, string, error) {
	if capa, encontrada, err := s.store.CoverFor(book.ID); err != nil {
		return nil, "", err
	} else if encontrada {
		if capa.Missing {
			return nil, "", ErrNoCover
		}
		return capa.Image, capa.ContentType, nil
	}

	imagem, tipo, err := s.extrairCapa(ctx, book)
	if errors.Is(err, ErrNoCover) {
		_ = s.store.SaveCover(models.Cover{BookID: book.ID, Missing: true})
		return nil, "", ErrNoCover
	}
	if err != nil {
		return nil, "", err
	}
	imagem, tipo, err = normalizarCapa(imagem, tipo)
	if err != nil {
		return nil, "", err
	}
	if err := s.store.SaveCover(models.Cover{BookID: book.ID, ContentType: tipo, Image: imagem}); err != nil {
		return nil, "", err
	}
	return imagem, tipo, nil
}

func (s *Service) extrairCapa(ctx context.Context, book models.Book) ([]byte, string, error) {
	switch book.Format {
	case "epub":
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
		imagem, tipo, err := epub.Cover(arquivo, info.Size())
		if err != nil {
			return nil, "", ErrNoCover
		}
		return imagem, tipo, nil
	case "pdf":
		return s.capaDePDF(ctx, book)
	default:
		return nil, "", ErrNoCover
	}
}

// capaDePDF renderiza a primeira página; sem poppler instalado não há capa.
func (s *Service) capaDePDF(ctx context.Context, book models.Book) ([]byte, string, error) {
	ferramenta, err := exec.LookPath("pdftoppm")
	if err != nil {
		return nil, "", ErrNoCover
	}
	caminho, err := s.EnsureFile(ctx, book)
	if err != nil {
		return nil, "", err
	}
	dir, err := os.MkdirTemp("", "lumina-capa-*")
	if err != nil {
		return nil, "", fmt.Errorf("criar tmp da capa: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	saida := filepath.Join(dir, "capa")
	comando := exec.CommandContext(ctx, ferramenta, "-f", "1", "-l", "1", "-singlefile", "-jpeg", "-r", "72", caminho, saida)
	if saidaComando, err := comando.CombinedOutput(); err != nil {
		return nil, "", fmt.Errorf("pdftoppm falhou para %q: %w (%s)", book.RemoteKey, err, string(saidaComando)[:min(200, len(saidaComando))])
	}
	imagem, err := os.ReadFile(saida + ".jpg")
	if err != nil || len(imagem) == 0 {
		return nil, "", ErrNoCover
	}
	return imagem, "image/jpeg", nil
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
