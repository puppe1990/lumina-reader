package reader

import (
	"context"
	"errors"
	"fmt"
	"html"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	"github.com/puppe1990/lumina-reader/internal/models"
)

// Unidade é um pedaço pesquisável do livro: capítulo no EPUB, página no PDF.
type Unidade struct {
	Rotulo string
	Texto  string
}

// ErrSemTexto marca formatos que o servidor não sabe ler (mobi/azw3).
var ErrSemTexto = errors.New("este formato não tem texto pesquisável no servidor")

// maxLivrosEmMemoria limita o cache do texto extraído dos PDFs (livros grandes
// ficam em ~1 MB cada; o cache evita rodar pdftotext a cada pergunta).
const maxLivrosEmMemoria = 3

var (
	tagsHTML   = regexp.MustCompile(`(?s)<[^>]+>`)
	espacosSol = regexp.MustCompile(`[\s\x{00a0}]+`)
)

// TextoDoLivro entrega o livro em unidades para o "conversar com o livro":
// capítulos do EPUB (já no SQLite) ou páginas do PDF (pdftotext, cacheado).
func (s *Service) TextoDoLivro(ctx context.Context, book models.Book) ([]Unidade, error) {
	switch book.Format {
	case "epub":
		return s.unidadesDeEPUB(ctx, book)
	case "pdf":
		return s.unidadesDePDF(ctx, book)
	}
	return nil, fmt.Errorf("%w: formato %q", ErrSemTexto, book.Format)
}

func (s *Service) unidadesDeEPUB(ctx context.Context, book models.Book) ([]Unidade, error) {
	capitulos, err := s.Chapters(ctx, book)
	if err != nil {
		return nil, err
	}
	unidades := make([]Unidade, 0, len(capitulos))
	for _, capitulo := range capitulos {
		texto := htmlParaTexto(capitulo.HTML)
		if texto == "" {
			continue
		}
		unidades = append(unidades, Unidade{
			Rotulo: fmt.Sprintf("capítulo %d — %s", capitulo.Ordinal+1, strings.TrimSpace(capitulo.Title)),
			Texto:  texto,
		})
	}
	return unidades, nil
}

func (s *Service) unidadesDePDF(ctx context.Context, book models.Book) ([]Unidade, error) {
	if unidades, ok := s.textoCache.buscar(s.cacheKey(book)); ok {
		return unidades, nil
	}
	caminho, err := s.EnsureFile(ctx, book)
	if err != nil {
		return nil, err
	}
	bruto, err := s.extrairPDF(ctx, caminho)
	if err != nil {
		return nil, err
	}
	paginas := strings.Split(bruto, "\f")
	unidades := make([]Unidade, 0, len(paginas))
	for indice, pagina := range paginas {
		texto := htmlParaTexto(pagina)
		if len(texto) < 20 {
			continue
		}
		unidades = append(unidades, Unidade{
			Rotulo: fmt.Sprintf("página %d", indice+1),
			Texto:  texto,
		})
	}
	if len(unidades) == 0 {
		return nil, fmt.Errorf("%w: PDF sem texto (digitalizado?)", ErrSemTexto)
	}
	s.textoCache.guardar(s.cacheKey(book), unidades)
	return unidades, nil
}

// extrairTextoPDF roda o pdftotext (poppler) e devolve o texto com \f entre as
// páginas. Campo da struct para os testes trocarem pela versão falsa.
func extrairTextoPDF(ctx context.Context, caminho string) (string, error) {
	ferramenta, err := exec.LookPath("pdftotext")
	if err != nil {
		return "", errors.New("pdftotext (poppler) não está instalado no servidor")
	}
	saida, err := exec.CommandContext(ctx, ferramenta, "-enc", "UTF-8", caminho, "-").Output()
	if err != nil {
		return "", fmt.Errorf("pdftotext falhou: %w", err)
	}
	return string(saida), nil
}

// htmlParaTexto tira as tags e normaliza os espaços do capítulo.
func htmlParaTexto(bruto string) string {
	texto := tagsHTML.ReplaceAllString(bruto, " ")
	texto = html.UnescapeString(texto)
	return strings.TrimSpace(espacosSol.ReplaceAllString(texto, " "))
}

// cacheTexto é um cache simples e limitado das unidades por livro.
type cacheTexto struct {
	mu     sync.Mutex
	chaves []string
	itens  map[string][]Unidade
}

func novoCacheTexto() *cacheTexto {
	return &cacheTexto{itens: map[string][]Unidade{}}
}

func (c *cacheTexto) buscar(chave string) ([]Unidade, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	unidades, ok := c.itens[chave]
	return unidades, ok
}

func (c *cacheTexto) guardar(chave string, unidades []Unidade) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, existe := c.itens[chave]; !existe {
		c.chaves = append(c.chaves, chave)
	}
	c.itens[chave] = unidades
	for len(c.chaves) > maxLivrosEmMemoria {
		antiga := c.chaves[0]
		c.chaves = c.chaves[1:]
		delete(c.itens, antiga)
	}
}
