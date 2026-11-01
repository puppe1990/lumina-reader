// Package chat monta as mensagens do "conversar com o livro" e fala com um
// provedor OpenAI-compatível (NVIDIA NIM por padrão).
package chat

import (
	"errors"
	"fmt"
	"strings"
)

const (
	// LimiteContexto é o teto de caracteres do trecho enviado ao modelo.
	LimiteContexto = 12000
	// LimitePergunta é o teto da pergunta do leitor.
	LimitePergunta = 2000
	// LimiteHistorico é quantas mensagens anteriores seguem na conversa.
	LimiteHistorico = 8
	// LimiteMensagemHistorico é o teto de cada mensagem do histórico.
	LimiteMensagemHistorico = 1000
)

var ErrPerguntaVazia = errors.New("a pergunta está vazia")

// Mensagem é uma fala no formato do provedor (system | user | assistant).
type Mensagem struct {
	Papel string
	Texto string
}

// Pedido é o que o leitor manda: onde ele está e o que quer saber.
type Pedido struct {
	Livro     string
	Autor     string
	Posicao   string
	Contexto  string
	Pergunta  string
	Historico []Mensagem
}

const instrucoes = `Você é o Lumina, assistente de leitura do livro %q%s. Você conversa com o leitor sobre o trecho atual do livro%s.
Responda em português do Brasil, direto e útil, em até 3 parágrafos curtos.
Baseie a resposta no trecho atual do livro quando for possível; se a resposta não estiver nele, diga que não encontrou no trecho e responda com conhecimento geral, deixando claro que vem de fora do livro.
Não invente citações nem repita o trecho inteiro de volta.`

// MontarMensagens transforma o pedido na conversa que vai ao provedor:
// sistema (livro + posição) → histórico recente → trecho atual + pergunta.
func MontarMensagens(p Pedido) ([]Mensagem, error) {
	pergunta := strings.TrimSpace(p.Pergunta)
	if pergunta == "" {
		return nil, ErrPerguntaVazia
	}

	autor := ""
	if strings.TrimSpace(p.Autor) != "" {
		autor = " de " + strings.TrimSpace(p.Autor)
	}
	posicao := ""
	if strings.TrimSpace(p.Posicao) != "" {
		posicao = " — " + strings.TrimSpace(p.Posicao)
	}
	mensagens := []Mensagem{{
		Papel: "system",
		Texto: fmt.Sprintf(instrucoes, strings.TrimSpace(p.Livro), autor, posicao),
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

	trecho := truncar(strings.TrimSpace(p.Contexto), LimiteContexto)
	cabecalho := "Trecho atual do livro"
	if strings.TrimSpace(p.Posicao) != "" {
		cabecalho += " (" + strings.TrimSpace(p.Posicao) + ")"
	}
	corpo := cabecalho + ":\n\n" + trecho + "\n\nPergunta do leitor: " + truncar(pergunta, LimitePergunta)
	mensagens = append(mensagens, Mensagem{Papel: "user", Texto: corpo})
	return mensagens, nil
}

func truncar(texto string, limite int) string {
	if len(texto) <= limite {
		return texto
	}
	return strings.TrimSpace(texto[:limite]) + " […] (trecho truncado)"
}
