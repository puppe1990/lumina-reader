// Package chat monta as mensagens do "conversar com o livro" (perguntas sobre a
// obra inteira, com trechos achados no texto) e fala com um provedor
// OpenAI-compatível (NVIDIA NIM por padrão).
package chat

import (
	"errors"
	"fmt"
	"strings"
)

const (
	// LimiteContexto é o teto de caracteres dos trechos enviados ao modelo.
	LimiteContexto = 12000
	// LimiteContextoAtual é o teto do trecho que está na tela do leitor.
	LimiteContextoAtual = 3000
	// LimiteSumario é o teto do sumário (títulos dos capítulos).
	LimiteSumario = 2500
	// LimitePergunta é o teto da pergunta do leitor.
	LimitePergunta = 2000
	// LimiteHistorico é quantas mensagens anteriores seguem na conversa.
	LimiteHistorico = 8
	// LimiteMensagemHistorico é o teto de cada mensagem do histórico.
	LimiteMensagemHistorico = 1000
	// maxCapitulosNoSumario evita um sumário gigante no prompt.
	maxCapitulosNoSumario = 40
)

var ErrPerguntaVazia = errors.New("a pergunta está vazia")

// Mensagem é uma fala no formato do provedor (system | user | assistant).
type Mensagem struct {
	Papel string
	Texto string
}

// Pedido é o que o leitor manda: a pergunta e o que o servidor sabe do livro.
type Pedido struct {
	Livro     string
	Autor     string
	Posicao   string
	Sumario   []string
	Trechos   []Trecho
	Contexto  string
	Pergunta  string
	Historico []Mensagem
}

const instrucoes = `Você é o Lumina, assistente do livro %q%s. O leitor conversa sobre o livro inteiro: pode perguntar a ideia central, o que o autor defende, resumo, comparações e detalhes de qualquer parte da obra.
Responda em português do Brasil, direto e útil, em até 3 parágrafos curtos.
Use primeiro os trechos do livro fornecidos; se a resposta não estiver neles, diga que não encontrou no livro e responda com conhecimento geral, deixando claro que vem de fora da obra.
Nunca invente citações, listas, nomes ou números de página: se os trechos não trouxerem a lista, diga que os trechos disponíveis não a detalham em vez de preencher de memória. Não repita os trechos inteiros de volta.`

// MontarMensagens transforma o pedido na conversa que vai ao provedor:
// sistema (livro) → histórico recente → sumário + trechos + pergunta.
func MontarMensagens(p Pedido) ([]Mensagem, error) {
	pergunta := strings.TrimSpace(p.Pergunta)
	if pergunta == "" {
		return nil, ErrPerguntaVazia
	}

	autor := ""
	if strings.TrimSpace(p.Autor) != "" {
		autor = " de " + strings.TrimSpace(p.Autor)
	}
	mensagens := []Mensagem{{
		Papel: "system",
		Texto: fmt.Sprintf(instrucoes, strings.TrimSpace(p.Livro), autor),
	}}

	historico := p.Historico
	if len(historico) > LimiteHistorico {
		historico = historico[len(historico)-LimiteHistorico:]
	}
	for _, fala := range historico {
		texto := truncar(strings.TrimSpace(fala.Texto), LimiteMensagemHistorico)
		if texto == "" {
			continue
		}
		papel := fala.Papel
		if papel != "assistant" {
			papel = "user"
		}
		mensagens = append(mensagens, Mensagem{Papel: papel, Texto: texto})
	}

	var partes []string
	if sumario := montarSumario(p.Sumario); sumario != "" {
		partes = append(partes, "Sumário do livro:\n"+sumario)
	}
	if trechos := montarTrechos(p.Trechos); trechos != "" {
		partes = append(partes, "Trechos do livro que podem responder:\n"+trechos)
	}
	if atual := truncar(strings.TrimSpace(p.Contexto), LimiteContextoAtual); atual != "" {
		cabecalho := "Trecho em que o leitor está agora"
		if strings.TrimSpace(p.Posicao) != "" {
			cabecalho += " (" + strings.TrimSpace(p.Posicao) + ")"
		}
		partes = append(partes, cabecalho+":\n"+atual)
	}
	if len(partes) == 0 {
		partes = append(partes, "O servidor não conseguiu ler o texto deste livro; responda com o que souber sobre a obra.")
	}
	partes = append(partes, "Pergunta do leitor: "+truncar(pergunta, LimitePergunta))
	mensagens = append(mensagens, Mensagem{Papel: "user", Texto: strings.Join(partes, "\n\n")})
	return mensagens, nil
}

func montarSumario(titulos []string) string {
	if len(titulos) == 0 {
		return ""
	}
	if len(titulos) > maxCapitulosNoSumario {
		titulos = titulos[:maxCapitulosNoSumario]
	}
	limpos := make([]string, 0, len(titulos))
	for _, titulo := range titulos {
		if titulo = strings.TrimSpace(titulo); titulo != "" {
			limpos = append(limpos, titulo)
		}
	}
	return truncar(strings.Join(limpos, "; "), LimiteSumario)
}

func montarTrechos(trechos []Trecho) string {
	if len(trechos) == 0 {
		return ""
	}
	var construtor strings.Builder
	for indice, trecho := range trechos {
		rotulo := strings.TrimSpace(trecho.Rotulo)
		texto := truncar(strings.TrimSpace(trecho.Texto), MaxTrechoCaracteres)
		if texto == "" {
			continue
		}
		if indice > 0 {
			construtor.WriteString("\n\n")
		}
		if rotulo != "" {
			construtor.WriteString("[" + rotulo + "] ")
		}
		construtor.WriteString(texto)
		if construtor.Len() >= LimiteContexto {
			break
		}
	}
	return truncar(strings.TrimSpace(construtor.String()), LimiteContexto)
}

func truncar(texto string, limite int) string {
	if len(texto) <= limite {
		return texto
	}
	return strings.TrimSpace(texto[:limite]) + " […] (trecho truncado)"
}
