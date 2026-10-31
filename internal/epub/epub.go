// Package epub lê EPUBs (zip + OPF) em capítulos limpos para o leitor: metadados,
// spine na ordem de leitura, sumário da nav/NCX e HTML sanitizado com as imagens
// apontando para a rota de recursos do app.
package epub

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"mime"
	"net/url"
	"path"
	"strings"

	"golang.org/x/net/html"
)

type ParseOptions struct {
	// ResourceURL monta a URL do app para um caminho dentro do EPUB
	// (ex. "OEBPS/images/fig.png" → "/livros/7/recurso/OEBPS/images/fig.png").
	ResourceURL func(caminho string) string
}

type Book struct {
	Title    string
	Author   string
	Language string
	Chapters []Chapter
}

type Chapter struct {
	Title string
	HTML  string
	Words int
}

type containerXML struct {
	Rootfiles []struct {
		FullPath  string `xml:"full-path,attr"`
		MediaType string `xml:"media-type,attr"`
	} `xml:"rootfiles>rootfile"`
}

type opfPackage struct {
	Metadata struct {
		Title    string `xml:"title"`
		Creator  string `xml:"creator"`
		Language string `xml:"language"`
	} `xml:"metadata"`
	Manifest struct {
		Items []struct {
			ID         string `xml:"id,attr"`
			Href       string `xml:"href,attr"`
			MediaType  string `xml:"media-type,attr"`
			Properties string `xml:"properties,attr"`
		} `xml:"item"`
	} `xml:"manifest"`
	Spine struct {
		Toc      string `xml:"toc,attr"`
		Itemrefs []struct {
			IDRef string `xml:"idref,attr"`
		} `xml:"itemref"`
	} `xml:"spine"`
}

type ncxDoc struct {
	NavMap struct {
		NavPoints []ncxPoint `xml:"navPoint"`
	} `xml:"navMap"`
}

type ncxPoint struct {
	NavLabel struct {
		Text string `xml:"text"`
	} `xml:"navLabel"`
	Content struct {
		Src string `xml:"src,attr"`
	} `xml:"content"`
	NavPoints []ncxPoint `xml:"navPoint"`
}

// Parse lê o EPUB inteiro: metadados, sumário e capítulos limpos.
func Parse(r io.ReaderAt, size int64, opts ParseOptions) (*Book, error) {
	zipr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("abrir EPUB (zip): %w", err)
	}

	opfPath, err := findOPF(zipr)
	if err != nil {
		return nil, err
	}
	opfBytes, err := readZipEntry(zipr, opfPath)
	if err != nil {
		return nil, err
	}
	var pkg opfPackage
	if err := xml.Unmarshal(opfBytes, &pkg); err != nil {
		return nil, fmt.Errorf("ler OPF %q: %w", opfPath, err)
	}

	opfDir := path.Dir(opfPath)
	hrefPorID := map[string]string{}
	navHref := ""
	ncxHref := ""
	for _, item := range pkg.Manifest.Items {
		hrefPorID[item.ID] = item.Href
		if strings.Contains(item.Properties, "nav") {
			navHref = item.Href
		}
		if item.MediaType == "application/x-dtbncx+xml" || (pkg.Spine.Toc != "" && item.ID == pkg.Spine.Toc) {
			ncxHref = item.Href
		}
	}

	toc := parseTOC(zipr, path.Join(opfDir, navHref), path.Join(opfDir, ncxHref))

	book := &Book{
		Title:    strings.TrimSpace(pkg.Metadata.Title),
		Author:   strings.TrimSpace(pkg.Metadata.Creator),
		Language: strings.TrimSpace(pkg.Metadata.Language),
	}
	for _, itemref := range pkg.Spine.Itemrefs {
		href, ok := hrefPorID[itemref.IDRef]
		if !ok {
			continue
		}
		caminho := path.Join(opfDir, href)
		bruto, err := readZipEntry(zipr, caminho)
		if err != nil {
			continue // um item quebrado não derruba a leitura dos demais
		}
		capitulo, err := buildChapter(path.Clean(caminho), bruto, toc, opts)
		if err != nil {
			continue
		}
		book.Chapters = append(book.Chapters, capitulo)
	}
	if len(book.Chapters) == 0 {
		return nil, fmt.Errorf("EPUB %q sem capítulos legíveis na spine", opfPath)
	}
	return book, nil
}

// OpenResource extrai um arquivo interno do EPUB (imagens e afins) com o
// content-type pelo nome.
func OpenResource(r io.ReaderAt, size int64, nome string) ([]byte, string, error) {
	zipr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, "", fmt.Errorf("abrir EPUB (zip): %w", err)
	}
	procurado := strings.TrimPrefix(path.Clean(nome), "/")
	for _, f := range zipr.File {
		if f.Name == procurado {
			conteudo, err := readZipFile(f)
			if err != nil {
				return nil, "", err
			}
			return conteudo, ContentTypeFor(f.Name), nil
		}
	}
	return nil, "", fmt.Errorf("recurso %q não existe no EPUB", nome)
}

// ContentTypeFor mapeia a extensão de um arquivo interno para o Content-Type.
func ContentTypeFor(nome string) string {
	ext := strings.ToLower(path.Ext(nome))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".css":
		return "text/css; charset=utf-8"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".otf":
		return "font/otf"
	case ".xhtml", ".html", ".htm":
		return "application/xhtml+xml; charset=utf-8"
	}
	if tipo := mime.TypeByExtension(ext); tipo != "" {
		return tipo
	}
	return "application/octet-stream"
}

func findOPF(zipr *zip.Reader) (string, error) {
	container, err := readZipEntry(zipr, "META-INF/container.xml")
	if err != nil {
		return "", fmt.Errorf("EPUB sem META-INF/container.xml: %w", err)
	}
	var doc containerXML
	if err := xml.Unmarshal(container, &doc); err != nil {
		return "", fmt.Errorf("ler container.xml: %w", err)
	}
	for _, rootfile := range doc.Rootfiles {
		if strings.Contains(rootfile.MediaType, "oebps-package") || rootfile.MediaType == "" {
			return rootfile.FullPath, nil
		}
	}
	if len(doc.Rootfiles) > 0 {
		return doc.Rootfiles[0].FullPath, nil
	}
	return "", fmt.Errorf("container.xml sem rootfile")
}

func readZipEntry(zipr *zip.Reader, nome string) ([]byte, error) {
	procurado := strings.TrimPrefix(path.Clean(nome), "/")
	for _, f := range zipr.File {
		if f.Name == procurado {
			return readZipFile(f)
		}
	}
	return nil, fmt.Errorf("entrada %q não encontrada no EPUB", nome)
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("abrir %q: %w", f.Name, err)
	}
	defer func() { _ = rc.Close() }()
	conteudo, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("ler %q: %w", f.Name, err)
	}
	return conteudo, nil
}

// parseTOC junta nav (EPUB3) e NCX (EPUB2) num mapa href→título, com o href
// normalizado a partir do diretório do próprio arquivo de navegação.
func parseTOC(zipr *zip.Reader, navPath, ncxPath string) map[string]string {
	toc := map[string]string{}
	if navPath != "" && !strings.HasPrefix(navPath, ".") {
		if bruto, err := readZipEntry(zipr, navPath); err == nil {
			coletarNav(bruto, path.Dir(navPath), toc)
		}
	}
	if ncxPath != "" && !strings.HasPrefix(ncxPath, ".") {
		if bruto, err := readZipEntry(zipr, ncxPath); err == nil {
			var ncx ncxDoc
			if xml.Unmarshal(bruto, &ncx) == nil {
				var coletar func(pontos []ncxPoint)
				coletar = func(pontos []ncxPoint) {
					for _, p := range pontos {
						if p.Content.Src != "" {
							href := strings.Split(p.Content.Src, "#")[0]
							toc[path.Join(path.Dir(ncxPath), href)] = strings.TrimSpace(p.NavLabel.Text)
						}
						coletar(p.NavPoints)
					}
				}
				coletar(ncx.NavMap.NavPoints)
			}
		}
	}
	return toc
}

func coletarNav(bruto []byte, dir string, toc map[string]string) {
	doc, err := html.Parse(bytes.NewReader(bruto))
	if err != nil {
		return
	}
	var percorrer func(*html.Node)
	percorrer = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			href := atributo(n, "href")
			if href != "" && !strings.HasPrefix(href, "http") {
				caminho := strings.Split(href, "#")[0]
				if caminho != "" {
					titulo := strings.Join(strings.Fields(textoDe(n)), " ")
					toc[path.Join(dir, caminho)] = titulo
				}
			}
		}
		for filho := n.FirstChild; filho != nil; filho = filho.NextSibling {
			percorrer(filho)
		}
	}
	percorrer(doc)
}

func textoDe(n *html.Node) string {
	var sb strings.Builder
	var percorrer func(*html.Node)
	percorrer = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for filho := node.FirstChild; filho != nil; filho = filho.NextSibling {
			percorrer(filho)
		}
	}
	percorrer(n)
	return sb.String()
}

func atributo(n *html.Node, chave string) string {
	for _, attr := range n.Attr {
		if attr.Key == chave {
			return attr.Val
		}
	}
	return ""
}

// resolveHref normaliza um href relativo ao arquivo que o contém, devolvendo o
// caminho dentro do zip (sem fragmento) e o fragmento.
func resolveHref(base, href string) (caminho, fragmento string) {
	href = strings.TrimSpace(href)
	if i := strings.Index(href, "#"); i >= 0 {
		fragmento = href[i+1:]
		href = href[:i]
	}
	if href == "" {
		return "", fragmento
	}
	desescapado, err := url.PathUnescape(href)
	if err == nil {
		href = desescapado
	}
	return path.Clean(path.Join(path.Dir(base), href)), fragmento
}
