package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Respondedor é quem conversa com o provedor (interface para testes).
type Respondedor interface {
	Responder(ctx context.Context, mensagens []Mensagem) (string, error)
}

// ErroProvedor carrega o status HTTP do provedor para a camada de cima montar
// uma mensagem amigável.
type ErroProvedor struct {
	Status  int
	Detalhe string
}

func (e *ErroProvedor) Error() string {
	return fmt.Sprintf("provedor respondeu %d: %s", e.Status, e.Detalhe)
}

// Cliente fala com qualquer endpoint OpenAI-compatível (/chat/completions).
type Cliente struct {
	baseURL string
	chave   string
	modelo  string
	http    *http.Client
}

// NovoCliente monta o cliente com o timeout folgado que o raciocínio dos
// modelos NIM exige.
func NovoCliente(baseURL, chave, modelo string) *Cliente {
	return &Cliente{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		chave:   strings.TrimSpace(chave),
		modelo:  strings.TrimSpace(modelo),
		http:    &http.Client{Timeout: 150 * time.Second},
	}
}

type requisicaoChat struct {
	Model       string     `json:"model"`
	Messages    []mensagem `json:"messages"`
	Temperature float64    `json:"temperature"`
	MaxTokens   int        `json:"max_tokens"`
}

type mensagem struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type respostaChat struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Responder manda a conversa e devolve só o texto da resposta (o
// reasoning_content dos modelos de raciocínio fica de fora).
func (c *Cliente) Responder(ctx context.Context, mensagens []Mensagem) (string, error) {
	if c.chave == "" {
		return "", errors.New("chave do provedor não configurada")
	}
	corpo := requisicaoChat{Model: c.modelo, Temperature: 0.3, MaxTokens: 2000}
	for _, m := range mensagens {
		corpo.Messages = append(corpo.Messages, mensagem{Role: m.Papel, Content: m.Texto})
	}
	dados, err := json.Marshal(corpo)
	if err != nil {
		return "", fmt.Errorf("montar requisição: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", strings.NewReader(string(dados)))
	if err != nil {
		return "", fmt.Errorf("criar requisição: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.chave)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("chamar provedor: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	texto, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", &ErroProvedor{Status: resp.StatusCode, Detalhe: string(texto[:min(len(texto), 300)])}
	}

	var saida respostaChat
	if err := json.Unmarshal(texto, &saida); err != nil {
		return "", fmt.Errorf("resposta do provedor ilegível: %w", err)
	}
	if len(saida.Choices) == 0 {
		return "", errors.New("provedor devolveu resposta vazia")
	}
	escolha := saida.Choices[0]
	resposta := strings.TrimSpace(escolha.Message.Content)
	if resposta == "" {
		if escolha.FinishReason == "length" {
			// Motivo comum nos modelos de raciocínio: o teto de tokens acabou
			// antes da resposta começar.
			return "", errors.New("o provedor gastou o limite de tokens sem responder; pergunte de novo")
		}
		return "", errors.New("provedor devolveu resposta vazia")
	}
	return resposta, nil
}
