package chat

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Trecho é um pedaço do livro com rótulo (capítulo ou página).
type Trecho struct {
	Rotulo string
	Texto  string
}

const (
	// MaxTrechoCaracteres é o teto de cada trecho enviado ao modelo.
	MaxTrechoCaracteres = 900
	// MaxTrechos por pergunta.
	MaxTrechos = 6
	// janelaTrecho é quanto contexto se abre antes do acerto.
	janelaTrecho = 400
)

var minusculas = map[rune]rune{
	'á': 'a', 'à': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'Á': 'a', 'À': 'a', 'Â': 'a', 'Ã': 'a', 'Ä': 'a',
	'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e', 'É': 'e', 'È': 'e', 'Ê': 'e', 'Ë': 'e',
	'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i', 'Í': 'i', 'Ì': 'i', 'Î': 'i', 'Ï': 'i',
	'ó': 'o', 'ò': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o', 'Ó': 'o', 'Ò': 'o', 'Ô': 'o', 'Õ': 'o', 'Ö': 'o',
	'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u', 'Ú': 'u', 'Ù': 'u', 'Û': 'u', 'Ü': 'u',
	'ç': 'c', 'Ç': 'c', 'ñ': 'n', 'Ñ': 'n',
}

// paradas são as palavras que não ajudam a achar o assunto no livro.
var paradas = map[string]bool{
	"para": true, "como": true, "qual": true, "que": true, "com": true, "uma": true,
	"dos": true, "das": true, "por": true, "sobre": true, "este": true, "esta": true,
	"isso": true, "esse": true, "essa": true, "pelo": true, "pela": true, "mais": true,
	"menos": true, "aqui": true, "aquilo": true, "tem": true, "são": true, "the": true,
	"and": true, "what": true, "this": true, "that": true, "from": true, "with": true,
	"about": true, "book": true, "livro": true, "autor": true, "autora": true,
}

// normalizarRune deixa a comparação sem acento e em minúscula, um rune por rune
// (o índice continua valendo no texto original).
func normalizarRune(caractere rune) rune {
	if trocado, existe := minusculas[caractere]; existe {
		return trocado
	}
	return unicode.ToLower(caractere)
}

// normalizarTextinho converte um texto para busca, preservando a contagem de
// runes (só troca acento por letra e caixa).
func normalizarTexto(texto string) string {
	var construtor strings.Builder
	construtor.Grow(len(texto))
	for _, caractere := range texto {
		construtor.WriteRune(normalizarRune(caractere))
	}
	return construtor.String()
}

// termosDe extrai as palavras úteis da pergunta.
func termosDe(pergunta string) []string {
	limpa := normalizarTexto(pergunta)
	brutos := strings.FieldsFunc(limpa, func(caractere rune) bool {
		return !unicode.IsLetter(caractere) && !unicode.IsDigit(caractere)
	})
	vistos := map[string]bool{}
	termos := make([]string, 0, len(brutos))
	for _, bruto := range brutos {
		if utf8.RuneCountInString(bruto) < 4 || paradas[bruto] || vistos[bruto] {
			continue
		}
		vistos[bruto] = true
		termos = append(termos, bruto)
	}
	return termos
}

// SelecionarTrechos escolhe os pedaços do livro que respondem à pergunta e abre
// uma janela de contexto em volta do primeiro acerto de cada um.
func SelecionarTrechos(unidades []Trecho, pergunta string, limite int) []Trecho {
	termos := termosDe(pergunta)
	if len(termos) == 0 {
		return nil
	}
	if limite <= 0 || limite > MaxTrechos {
		limite = MaxTrechos
	}

	type achado struct {
		indice int
		pontos int
	}
	candidatos := make([]achado, 0, len(unidades))
	for indice, unidade := range unidades {
		texto := normalizarTexto(unidade.Texto)
		pontos := 0
		for _, termo := range termos {
			ocorrencias := strings.Count(texto, termo)
			if ocorrencias > 3 {
				ocorrencias = 3
			}
			pontos += ocorrencias
		}
		if pontos > 0 {
			candidatos = append(candidatos, achado{indice: indice, pontos: pontos})
		}
	}
	if len(candidatos) == 0 {
		return nil
	}

	// ordena por pontos (desc), mantendo a ordem do livro no empate
	for i := 1; i < len(candidatos); i++ {
		for j := i; j > 0 && candidatos[j].pontos > candidatos[j-1].pontos; j-- {
			candidatos[j], candidatos[j-1] = candidatos[j-1], candidatos[j]
		}
	}
	if len(candidatos) > limite {
		candidatos = candidatos[:limite]
	}

	trechos := make([]Trecho, 0, len(candidatos))
	for _, achado := range candidatos {
		unidade := unidades[achado.indice]
		trechos = append(trechos, Trecho{Rotulo: unidade.Rotulo, Texto: janela(unidade.Texto, termos)})
	}
	return trechos
}

// janela recorta o pedaço em volta do primeiro termo encontrado no texto.
func janela(texto string, termos []string) string {
	original := []rune(strings.TrimSpace(texto))
	if len(original) <= MaxTrechoCaracteres {
		return string(original)
	}

	normalizado := []rune(normalizarTexto(string(original)))
	centro := -1
	for _, termo := range termos {
		indice := strings.Index(string(normalizado), termo)
		if indice < 0 {
			continue
		}
		posicao := utf8.RuneCountInString(string(normalizado)[:indice])
		if posicao < len(original) {
			centro = posicao
			break
		}
	}
	if centro < 0 {
		centro = 0
	}

	inicio := centro - janelaTrecho
	if inicio < 0 {
		inicio = 0
	}
	fim := inicio + MaxTrechoCaracteres
	if fim > len(original) {
		fim = len(original)
		inicio = fim - MaxTrechoCaracteres
		if inicio < 0 {
			inicio = 0
		}
	}
	for inicio > 0 && inicio < len(original) && !unicode.IsSpace(original[inicio-1]) {
		inicio++
	}
	for fim > inicio && fim < len(original) && !unicode.IsSpace(original[fim-1]) {
		fim--
	}
	recorte := strings.TrimSpace(string(original[inicio:fim]))
	if inicio > 0 {
		recorte = "…" + recorte
	}
	if fim < len(original) {
		recorte += "…"
	}
	return recorte
}
