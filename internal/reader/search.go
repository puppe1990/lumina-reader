package reader

import (
	"regexp"
	"strings"

	"golang.org/x/net/html"

	"github.com/puppe1990/lumina-reader/internal/models"
)

// Hit é um trecho de capítulo que casou com a busca.
type Hit struct {
	ChapterOrdinal int    `json:"ChapterOrdinal"`
	ChapterTitle   string `json:"ChapterTitle"`
	Snippet        string `json:"Snippet"`
}

const (
	buscaContexto    = 64 // caracteres de contexto de cada lado do trecho
	buscaMinimoTermo = 2
)

// BuscarNoTexto procura o termo no texto plano dos capítulos, devolvendo até
// porCapitulo trechos por capítulo e maxHits no total. Case-insensitive via
// strings.ToLower (cobre acentos do pt-BR).
func BuscarNoTexto(termo string, capitulos []models.Chapter, porCapitulo, maxHits int) []Hit {
	termo = strings.TrimSpace(termo)
	if len([]rune(termo)) < buscaMinimoTermo {
		return nil
	}
	alvo := strings.ToLower(termo)

	var hits []Hit
	for _, capitulo := range capitulos {
		texto := textoPlano(capitulo.HTML)
		if texto == "" {
			continue
		}
		minusculo := strings.ToLower(texto)
		achados := 0
		inicio := 0
		for achados < porCapitulo {
			pos := strings.Index(minusculo[inicio:], alvo)
			if pos < 0 {
				break
			}
			pos += inicio
			hits = append(hits, Hit{
				ChapterOrdinal: capitulo.Ordinal,
				ChapterTitle:   capitulo.Title,
				Snippet:        recorte(texto, pos, len(alvo)),
			})
			achados++
			if len(hits) >= maxHits {
				return hits
			}
			inicio = pos + len(alvo)
			if inicio >= len(minusculo) {
				break
			}
		}
	}
	return hits
}

// textoPlano extrai os nós de texto do HTML sanitizado do capítulo e colapsa
// os espaços — é o mesmo texto que o leitor mostra, sem markup. Como cada nó
// entra com um espaço depois, a pontuação colada ganha um passe de limpeza.
func textoPlano(markup string) string {
	doc, err := html.Parse(strings.NewReader(markup))
	if err != nil {
		return ""
	}
	var sb strings.Builder
	var percorrer func(*html.Node)
	percorrer = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
			sb.WriteString(" ")
		}
		for filho := n.FirstChild; filho != nil; filho = filho.NextSibling {
			percorrer(filho)
		}
	}
	percorrer(doc)
	plano := strings.Join(strings.Fields(sb.String()), " ")
	return espacoAntesDePontuacao.ReplaceAllString(plano, "$1")
}

var espacoAntesDePontuacao = regexp.MustCompile(`\s+([.,;:!?)\]»”’])`)

func recorte(texto string, pos, tamanho int) string {
	inicio := pos - buscaContexto
	if inicio < 0 {
		inicio = 0
	}
	fim := pos + tamanho + buscaContexto
	if fim > len(texto) {
		fim = len(texto)
	}
	inicio = ajustarRuna(texto, inicio)
	fim = ajustarRuna(texto, fim)
	trecho := strings.TrimSpace(texto[inicio:fim])
	if inicio > 0 {
		trecho = "…" + trecho
	}
	if fim < len(texto) {
		trecho += "…"
	}
	return trecho
}

// ajustarRuna recua até o começo de um caractere UTF-8 válido.
func ajustarRuna(texto string, pos int) int {
	for pos > 0 && pos < len(texto) && (texto[pos]&0xC0) == 0x80 {
		pos--
	}
	return pos
}
