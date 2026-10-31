package epub

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
)

// capituloPolicy é o contrato de segurança do HTML que vem do EPUB: estrutura
// editorial sim, scripts/estilos/atributos de classe não. O CSS do app manda no
// visual; classes do livro são ruído que quebra o tema de leitura.
var capituloPolicy = func() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements(
		"p", "br", "hr", "em", "i", "strong", "b", "u", "s", "strike", "sub", "sup",
		"blockquote", "code", "pre", "h1", "h2", "h3", "h4", "h5", "h6",
		"ul", "ol", "li", "dl", "dt", "dd",
		"figure", "figcaption", "img", "a",
		"table", "thead", "tbody", "tfoot", "tr", "td", "th",
		"div", "span", "section", "article", "header", "footer", "aside", "nav",
		"abbr", "cite", "q", "small", "mark", "time",
	)
	p.AllowAttrs("id").Globally()
	p.AllowAttrs("lang", "dir").Globally()
	p.AllowAttrs("href").Matching(regexp.MustCompile(`^(?:#|https?://|mailto:)`)).OnElements("a")
	p.AllowAttrs("src", "alt", "title", "width", "height").
		Matching(regexp.MustCompile(`^(?:/livros/\d+/recurso/|data:image/)`)).
		OnElements("img")
	p.AllowAttrs("colspan", "rowspan").OnElements("td", "th")
	return p
}()

func sanitizeCapitulo(bruto string) string {
	return capituloPolicy.Sanitize(bruto)
}

func buildChapter(zipPath string, bruto []byte, toc map[string]string, opts ParseOptions) (Chapter, error) {
	doc, err := html.Parse(bytes.NewReader(bruto))
	if err != nil {
		return Chapter{}, fmt.Errorf("ler capítulo %q: %w", zipPath, err)
	}
	corpo := encontrarBody(doc)
	if corpo == nil {
		return Chapter{}, fmt.Errorf("capítulo %q sem <body>", zipPath)
	}

	rewriteRefs(corpo, zipPath, opts)

	var sb strings.Builder
	for filho := corpo.FirstChild; filho != nil; filho = filho.NextSibling {
		if err := html.Render(&sb, filho); err != nil {
			return Chapter{}, fmt.Errorf("serializar capítulo %q: %w", zipPath, err)
		}
	}
	limpo := sanitizeCapitulo(sb.String())

	titulo := toc[zipPath]
	if titulo == "" {
		titulo = tituloDoHeading(doc)
	}
	return Chapter{Title: titulo, HTML: limpo, Words: contarPalavras(limpo)}, nil
}

// rewriteRefs troca imagens relativas pela rota de recursos do app e rebaixa
// links internos para âncora (mesmo documento) ou texto puro — o leitor navega
// por capítulos, não por hrefs do EPUB.
func rewriteRefs(n *html.Node, base string, opts ParseOptions) {
	if n.Type == html.ElementNode {
		switch n.Data {
		case "img":
			src := atributo(n, "src")
			if src != "" && opts.ResourceURL != nil && !strings.HasPrefix(src, "http") && !strings.HasPrefix(src, "data:") {
				caminho, _ := resolveHref(base, src)
				definirAtributo(n, "src", opts.ResourceURL(caminho))
			}
		case "a":
			href := atributo(n, "href")
			switch {
			case href == "" || strings.HasPrefix(href, "#"):
			case strings.HasPrefix(href, "http://"), strings.HasPrefix(href, "https://"), strings.HasPrefix(href, "mailto:"):
			default:
				_, fragmento := resolveHref(base, href)
				if fragmento == "" {
					removerAtributo(n, "href")
				} else {
					definirAtributo(n, "href", "#"+fragmento)
				}
			}
		}
	}
	for filho := n.FirstChild; filho != nil; filho = filho.NextSibling {
		rewriteRefs(filho, base, opts)
	}
}

func encontrarBody(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.Data == "body" {
		return n
	}
	for filho := n.FirstChild; filho != nil; filho = filho.NextSibling {
		if achado := encontrarBody(filho); achado != nil {
			return achado
		}
	}
	return nil
}

func tituloDoHeading(doc *html.Node) string {
	var achado string
	var percorrer func(*html.Node)
	percorrer = func(n *html.Node) {
		if achado != "" {
			return
		}
		if n.Type == html.ElementNode && (n.Data == "h1" || n.Data == "h2" || n.Data == "h3") {
			achado = strings.Join(strings.Fields(textoDe(n)), " ")
			return
		}
		for filho := n.FirstChild; filho != nil; filho = filho.NextSibling {
			percorrer(filho)
		}
	}
	percorrer(doc)
	return achado
}

func contarPalavras(limpo string) int {
	doc, err := html.Parse(strings.NewReader(limpo))
	if err != nil {
		return 0
	}
	return len(strings.Fields(textoDe(doc)))
}

func definirAtributo(n *html.Node, chave, valor string) {
	for i := range n.Attr {
		if n.Attr[i].Key == chave {
			n.Attr[i].Val = valor
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: chave, Val: valor})
}

func removerAtributo(n *html.Node, chave string) {
	filtrados := n.Attr[:0]
	for _, attr := range n.Attr {
		if attr.Key != chave {
			filtrados = append(filtrados, attr)
		}
	}
	n.Attr = filtrados
}
