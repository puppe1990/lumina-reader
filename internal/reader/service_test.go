package reader

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/puppe1990/lumina/internal/models"
)

type fakeRemote struct {
	opens int
	dados []byte
	err   error
}

func (f *fakeRemote) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	f.opens++
	if f.err != nil {
		return nil, f.err
	}
	return io.NopCloser(bytes.NewReader(f.dados)), nil
}

type memoryChapters struct {
	porLivro map[int64][]models.Chapter
	saves    int
}

func newMemoryChapters() *memoryChapters {
	return &memoryChapters{porLivro: map[int64][]models.Chapter{}}
}

func (m *memoryChapters) ChaptersFor(bookID int64) ([]models.Chapter, error) {
	return m.porLivro[bookID], nil
}

func (m *memoryChapters) SaveChapters(bookID int64, chapters []models.Chapter) error {
	m.saves++
	m.porLivro[bookID] = chapters
	return nil
}

func (m *memoryChapters) CountChapters(bookID int64) (int, error) {
	return len(m.porLivro[bookID]), nil
}

func epubDeTeste(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	escrever := func(nome, conteudo string) {
		w, err := zw.Create(nome)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(conteudo))
	}
	escrever("mimetype", "application/epub+zip")
	escrever("META-INF/container.xml", `<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`)
	escrever("OEBPS/content.opf", `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Livro de Teste</dc:title><dc:creator>Autora de Teste</dc:creator></metadata><manifest><item id="c1" href="c1.xhtml" media-type="application/xhtml+xml"/><item id="c2" href="c2.xhtml" media-type="application/xhtml+xml"/><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/></manifest><spine><itemref idref="c1"/><itemref idref="c2"/></spine></package>`)
	escrever("OEBPS/nav.xhtml", `<html xmlns="http://www.w3.org/1999/xhtml"><body><nav epub:type="toc"><ol><li><a href="c1.xhtml">Abertura</a></li><li><a href="c2.xhtml">Desfecho</a></li></ol></nav></body></html>`)
	escrever("OEBPS/c1.xhtml", `<html xmlns="http://www.w3.org/1999/xhtml"><body><h1>Abertura</h1><p>Primeiro capítulo.</p><img src="images/foto.png"/></body></html>`)
	escrever("OEBPS/c2.xhtml", `<html xmlns="http://www.w3.org/1999/xhtml"><body><h1>Desfecho</h1><p>Segundo capítulo.</p></body></html>`)
	img, err := zw.Create("OEBPS/images/foto.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = img.Write([]byte("\x89PNG\r\n\x1a\n"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestChapters_ParsesOnceThenUsesStore(t *testing.T) {
	remoto := &fakeRemote{dados: epubDeTeste(t)}
	store := newMemoryChapters()
	servico := New(remoto, store, t.TempDir())
	livro := models.Book{ID: 1, RemoteKey: "Cat/A/Livro.epub", SHA1: "abc123", Format: "epub"}

	capitulos, err := servico.Chapters(context.Background(), livro)
	if err != nil {
		t.Fatalf("Chapters erro = %v", err)
	}
	if len(capitulos) != 2 || capitulos[0].Title != "Abertura" {
		t.Fatalf("capítulos = %+v", capitulos)
	}
	if !bytes.Contains([]byte(capitulos[0].HTML), []byte("/livros/1/recurso/OEBPS/images/foto.png")) {
		t.Errorf("HTML sem imagem reescrita: %q", capitulos[0].HTML)
	}
	if remoto.opens != 1 || store.saves != 1 {
		t.Errorf("opens=%d saves=%d, want 1/1", remoto.opens, store.saves)
	}

	if _, err := servico.Chapters(context.Background(), livro); err != nil {
		t.Fatalf("segundo Chapters erro = %v", err)
	}
	if remoto.opens != 1 || store.saves != 1 {
		t.Errorf("segunda abertura não deveria rebaixar nem reparsar: opens=%d saves=%d", remoto.opens, store.saves)
	}
}

func TestResource_ReadsImageFromCachedFile(t *testing.T) {
	remoto := &fakeRemote{dados: epubDeTeste(t)}
	servico := New(remoto, newMemoryChapters(), t.TempDir())
	livro := models.Book{ID: 1, RemoteKey: "Cat/A/Livro.epub", SHA1: "abc123", Format: "epub"}

	conteudo, tipo, err := servico.Resource(context.Background(), livro, "OEBPS/images/foto.png")
	if err != nil {
		t.Fatalf("Resource erro = %v", err)
	}
	if len(conteudo) == 0 || tipo != "image/png" {
		t.Errorf("conteúdo=%d bytes tipo=%q", len(conteudo), tipo)
	}
	if remoto.opens != 1 {
		t.Errorf("opens = %d, want 1 (cache em disco)", remoto.opens)
	}
}

func TestEnsureFile_ReusesCacheBySHA(t *testing.T) {
	dir := t.TempDir()
	remoto := &fakeRemote{dados: epubDeTeste(t)}
	servico := New(remoto, newMemoryChapters(), dir)
	livro := models.Book{ID: 1, RemoteKey: "Cat/A/Livro.epub", SHA1: "abc123", Format: "epub"}

	caminho, err := servico.EnsureFile(context.Background(), livro)
	if err != nil {
		t.Fatalf("EnsureFile erro = %v", err)
	}
	if filepath.Dir(caminho) != dir {
		t.Errorf("arquivo em %q, want dentro de %q", caminho, dir)
	}
	if _, err := os.Stat(caminho); err != nil {
		t.Errorf("arquivo não existe no cache: %v", err)
	}

	outro := New(remoto, newMemoryChapters(), dir) // processo novo, mesmo cache
	if _, err := outro.EnsureFile(context.Background(), livro); err != nil {
		t.Fatalf("segundo EnsureFile erro = %v", err)
	}
	if remoto.opens != 1 {
		t.Errorf("opens = %d, want 1 (cache por SHA1)", remoto.opens)
	}
}
