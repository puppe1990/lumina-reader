package epub

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/puppe1990/amarra-cais/pkg/cais/fakedata"
)

// buildTestEPUB escreve um EPUB mínimo válido com metadados, nav e capítulos
// gerados por fakedata — o mesmo formato que o acervo real usa.
type epubFixture struct {
	titulo    string
	autor     string
	caps      []string
	comScript bool
	comImagem bool
}

func (f epubFixture) escrever(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zipw := zip.NewWriter(&buf)

	adicionar := func(nome, conteudo string) {
		w, err := zipw.Create(nome)
		if err != nil {
			t.Fatalf("zip create %s: %v", nome, err)
		}
		if _, err := w.Write([]byte(conteudo)); err != nil {
			t.Fatalf("zip write %s: %v", nome, err)
		}
	}

	adicionar("mimetype", "application/epub+zip")
	adicionar("META-INF/container.xml", `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`)

	var navItens, spineItens, manifestItens strings.Builder
	for i := range f.caps {
		n := i + 1
		arquivo := "cap" + itoa(n) + ".xhtml"
		corpo := f.caps[i]
		if f.comScript && i == 0 {
			corpo = `<script>alert("xss")</script>` + corpo
		}
		if f.comImagem && i == 0 {
			corpo = `<p>Texto com figura <img src="images/fig.png" alt="figura"/></p>` + corpo
		}
		adicionar("OEBPS/"+arquivo, `<?xml version="1.0" encoding="utf-8"?>
<html xmlns="http://www.w3.org/1999/xhtml"><head><title>cap</title></head>
<body><h1>Título `+itoa(n)+`</h1><p id="p`+itoa(n)+`">`+corpo+`</p></body></html>`)
		navItens.WriteString(`<li><a href="` + arquivo + `">Capítulo ` + itoa(n) + ` do fixture</a></li>`)
		spineItens.WriteString(`<itemref idref="cap` + itoa(n) + `"/>`)
		manifestItens.WriteString(`<item id="cap` + itoa(n) + `" href="cap` + itoa(n) + `.xhtml" media-type="application/xhtml+xml"/>`)
	}
	adicionar("OEBPS/nav.xhtml", `<?xml version="1.0" encoding="utf-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head><title>nav</title></head><body><nav epub:type="toc"><ol>`+navItens.String()+`</ol></nav></body></html>`)

	manifestItens.WriteString(`<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>`)

	adicionar("OEBPS/content.opf", `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>`+f.titulo+`</dc:title>
    <dc:creator>`+f.autor+`</dc:creator>
    <dc:language>pt-BR</dc:language>
  </metadata>
  <manifest>`+manifestItens.String()+`</manifest>
  <spine>`+spineItens.String()+`</spine>
</package>`)

	if f.comImagem {
		png, err := zipw.Create("OEBPS/images/fig.png")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = png.Write([]byte("\x89PNG\r\n\x1a\nfake"))
	}

	if err := zipw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func itoa(n int) string { return string(rune('0' + n)) }

func abrirFixture(t *testing.T, fixture epubFixture) (*Book, error) {
	t.Helper()
	dados := fixture.escrever(t)
	return Parse(bytes.NewReader(dados), int64(len(dados)), ParseOptions{
		ResourceURL: func(caminho string) string { return "/livros/7/recurso/" + caminho },
	})
}

func TestParse_ReadsMetadataAndSpineFromFakedata(t *testing.T) {
	fakedata.Seed(11)
	fixture := epubFixture{
		titulo: fakedata.Title(),
		autor:  fakedata.Name(),
		caps:   []string{fakedata.Sentence(), fakedata.Sentence(), fakedata.Sentence()},
	}
	livro, err := abrirFixture(t, fixture)
	if err != nil {
		t.Fatalf("Parse erro = %v", err)
	}
	if livro.Title != fixture.titulo {
		t.Errorf("Title = %q, want %q", livro.Title, fixture.titulo)
	}
	if livro.Author != fixture.autor {
		t.Errorf("Author = %q, want %q", livro.Author, fixture.autor)
	}
	if len(livro.Chapters) != 3 {
		t.Fatalf("capítulos = %d, want 3", len(livro.Chapters))
	}
	if livro.Chapters[1].Title != "Capítulo 2 do fixture" {
		t.Errorf("título do capítulo 2 = %q (deveria vir da nav)", livro.Chapters[1].Title)
	}
	if !strings.Contains(livro.Chapters[0].HTML, fixture.caps[0][:20]) {
		t.Errorf("capítulo 1 sem o texto gerado: %q", livro.Chapters[0].HTML)
	}
}

func TestParse_SanitizesScriptAndKeepsIds(t *testing.T) {
	fixture := epubFixture{
		titulo: "Título", autor: "Autora", caps: []string{"Um parágrafo limpo."},
		comScript: true,
	}
	livro, err := abrirFixture(t, fixture)
	if err != nil {
		t.Fatalf("Parse erro = %v", err)
	}
	html := livro.Chapters[0].HTML
	if strings.Contains(html, "<script") || strings.Contains(html, "alert") {
		t.Errorf("script deveria ser removido do capítulo: %q", html)
	}
	if !strings.Contains(html, `id="p1"`) {
		t.Errorf("âncora id do parágrafo deveria sobreviver: %q", html)
	}
}

func TestParse_RewritesImageSources(t *testing.T) {
	fixture := epubFixture{
		titulo: "Título", autor: "Autora", caps: []string{"Capítulo com figura."},
		comImagem: true,
	}
	livro, err := abrirFixture(t, fixture)
	if err != nil {
		t.Fatalf("Parse erro = %v", err)
	}
	html := livro.Chapters[0].HTML
	if !strings.Contains(html, `src="/livros/7/recurso/OEBPS/images/fig.png"`) {
		t.Errorf("src da imagem não foi reescrito: %q", html)
	}
}

func TestParse_FallsBackToHeadingForChapterTitle(t *testing.T) {
	livro, err := abrirFixture(t, epubFixture{titulo: "T", autor: "A", caps: []string{"texto"}})
	if err != nil {
		t.Fatalf("Parse erro = %v", err)
	}
	if livro.Chapters[0].Title != "Capítulo 1 do fixture" {
		t.Errorf("Title = %q", livro.Chapters[0].Title)
	}
}

func TestOpenResource_ExtractsBytes(t *testing.T) {
	dados := epubFixture{titulo: "T", autor: "A", caps: []string{"x"}, comImagem: true}.escrever(t)
	conteudo, tipo, err := OpenResource(bytes.NewReader(dados), int64(len(dados)), "OEBPS/images/fig.png")
	if err != nil {
		t.Fatalf("OpenResource erro = %v", err)
	}
	if !bytes.HasPrefix(conteudo, []byte("\x89PNG")) {
		t.Errorf("conteúdo extraído = %q", conteudo[:10])
	}
	if tipo != "image/png" {
		t.Errorf("content-type = %q, want image/png", tipo)
	}
}

func TestOpenResource_MissingReturnsError(t *testing.T) {
	dados := epubFixture{titulo: "T", autor: "A", caps: []string{"x"}}.escrever(t)
	if _, _, err := OpenResource(bytes.NewReader(dados), int64(len(dados)), "OEBPS/nao-existe.png"); err == nil {
		t.Fatal("OpenResource deveria falhar para entrada inexistente")
	}
}

func TestParse_RejectsGarbage(t *testing.T) {
	if _, err := Parse(strings.NewReader("nao é zip"), 9, ParseOptions{}); err == nil {
		t.Fatal("Parse deveria falhar para bytes que não são um EPUB")
	}
}

var _ = io.Discard
