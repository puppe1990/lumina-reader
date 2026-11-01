package reader

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/puppe1990/lumina-reader/internal/models"
)

func TestTextoDoLivro_EPUBUsaCapitulosComRotuloENormalizaHTML(t *testing.T) {
	lojas := newMemoryChapters()
	lojas.porLivro[7] = []models.Chapter{
		{Ordinal: 0, Title: "Abertura", HTML: "<p>Olá&nbsp;<b>mundo</b> do livro.</p>"},
		{Ordinal: 1, Title: "Capítulo Dois", HTML: "<div><p>Segundo   capítulo, com <i>ênfase</i>.</p></div>"},
		{Ordinal: 2, Title: "Vazio", HTML: "   "},
	}
	servico := New(&fakeRemote{}, lojas, t.TempDir())

	unidades, err := servico.TextoDoLivro(context.Background(), models.Book{ID: 7, Format: "epub"})
	if err != nil {
		t.Fatalf("TextoDoLivro erro = %v", err)
	}
	if len(unidades) != 2 {
		t.Fatalf("unidades = %d, want 2 (a vazia sai)", len(unidades))
	}
	if unidades[0].Rotulo != "capítulo 1 — Abertura" {
		t.Errorf("rótulo = %q", unidades[0].Rotulo)
	}
	if unidades[0].Texto != "Olá mundo do livro." {
		t.Errorf("texto = %q (tags e entidades normalizadas)", unidades[0].Texto)
	}
	if !strings.Contains(unidades[1].Texto, "Segundo capítulo") {
		t.Errorf("texto = %q", unidades[1].Texto)
	}
}

func TestTextoDoLivro_PDFExtraiPaginasECacheia(t *testing.T) {
	chamadas := 0
	servico := New(&fakeRemote{dados: []byte("%PDF-1.4 falso")}, newMemoryChapters(), t.TempDir())
	servico.extrairPDF = func(ctx context.Context, caminho string) (string, error) {
		chamadas++
		return "Primeira página com bastante texto aqui.\fSegunda página também com texto.\f", nil
	}
	livro := models.Book{ID: 9, Format: "pdf", RemoteKey: "Cat/Livro.pdf", SHA1: "sha-do-pdf"}

	unidades, err := servico.TextoDoLivro(context.Background(), livro)
	if err != nil {
		t.Fatalf("TextoDoLivro erro = %v", err)
	}
	if len(unidades) != 2 || unidades[0].Rotulo != "página 1" || unidades[1].Rotulo != "página 2" {
		t.Fatalf("unidades = %+v", unidades)
	}
	if _, err := servico.TextoDoLivro(context.Background(), livro); err != nil {
		t.Fatal(err)
	}
	if chamadas != 1 {
		t.Errorf("pdftotext rodou %d vezes, want 1 (cache por SHA-1)", chamadas)
	}
}

func TestTextoDoLivro_FormatoSemSuporte(t *testing.T) {
	servico := New(&fakeRemote{}, newMemoryChapters(), t.TempDir())
	if _, err := servico.TextoDoLivro(context.Background(), models.Book{ID: 1, Format: "mobi"}); !errors.Is(err, ErrSemTexto) {
		t.Fatalf("erro = %v, want ErrSemTexto", err)
	}
}

func TestTextoDoLivro_PDFEscaneado(t *testing.T) {
	servico := New(&fakeRemote{dados: []byte("%PDF-1.4 falso")}, newMemoryChapters(), t.TempDir())
	servico.extrairPDF = func(ctx context.Context, caminho string) (string, error) {
		return "\f \f\f", nil
	}
	_, err := servico.TextoDoLivro(context.Background(), models.Book{ID: 2, Format: "pdf", RemoteKey: "a.pdf", SHA1: "x"})
	if !errors.Is(err, ErrSemTexto) {
		t.Fatalf("erro = %v, want ErrSemTexto para PDF sem texto", err)
	}
}

func TestTextoDoLivro_CacheRespeitaOLimite(t *testing.T) {
	servico := New(&fakeRemote{}, newMemoryChapters(), t.TempDir())
	for indice := 0; indice < maxLivrosEmMemoria+2; indice++ {
		servico.textoCache.guardar(string(rune('a'+indice)), []Unidade{{Rotulo: "página 1", Texto: "x"}})
	}
	if total := len(servico.textoCache.itens); total != maxLivrosEmMemoria {
		t.Errorf("cache guardou %d livros, want %d", total, maxLivrosEmMemoria)
	}
}
