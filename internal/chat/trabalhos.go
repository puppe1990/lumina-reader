package chat

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Estados de um trabalho de chat.
const (
	EstadoRodando = "rodando"
	EstadoPronto  = "pronto"
	EstadoErro    = "erro"
)

// vidaDoTrabalho é quanto tempo a resposta fica disponível para o polling.
const vidaDoTrabalho = 10 * time.Minute

// tempoDoProvedor é o teto da chamada ao LLM (modelos de raciocínio passam de
// 15s com folga; o proxy na frente do app corta requisições longas, por isso o
// trabalho roda em background e o navegador busca o resultado).
const tempoDoProvedor = 150 * time.Second

// Trabalho é uma pergunta em andamento ou já respondida.
type Trabalho struct {
	ID       string
	Estado   string
	Resposta string
	Erro     string
	CriadoEm time.Time
}

// Trabalhos guarda as perguntas em memória: o POST responde um id na hora e o
// GET devolve o estado até a resposta chegar. Respondedor nil desliga o chat.
type Trabalhos struct {
	mu          sync.Mutex
	itens       map[string]Trabalho
	agora       func() time.Time
	timeout     time.Duration
	respondedor Respondedor
}

// NovosTrabalhos cria o registro com o provedor que responde as perguntas.
func NovosTrabalhos(respondedor Respondedor) *Trabalhos {
	return &Trabalhos{itens: map[string]Trabalho{}, agora: time.Now, timeout: tempoDoProvedor, respondedor: respondedor}
}

// Ligado diz se há provedor configurado.
func (t *Trabalhos) Ligado() bool {
	return t.respondedor != nil
}

// Criar dispara a pergunta em background e devolve o id para o polling.
func (t *Trabalhos) Criar(mensagens []Mensagem) string {
	id := novoID()
	trabalho := Trabalho{ID: id, Estado: EstadoRodando, CriadoEm: t.agora()}
	t.mu.Lock()
	t.limpar()
	t.itens[id] = trabalho
	respondedor := t.respondedor
	t.mu.Unlock()
	if respondedor == nil {
		return id
	}

	go func() {
		ctx, cancelar := context.WithTimeout(context.Background(), t.timeout)
		defer cancelar()
		resposta, err := respondedor.Responder(ctx, mensagens)
		t.mu.Lock()
		defer t.mu.Unlock()
		atual, ok := t.itens[id]
		if !ok {
			return
		}
		if err != nil {
			atual.Estado = EstadoErro
			atual.Erro = MensagemAmigavel(err)
		} else {
			atual.Estado = EstadoPronto
			atual.Resposta = resposta
		}
		t.itens[id] = atual
	}()
	return id
}

// Buscar devolve o trabalho e se ele ainda existe (expirado conta como sumido).
func (t *Trabalhos) Buscar(id string) (Trabalho, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.limpar()
	trabalho, ok := t.itens[id]
	return trabalho, ok
}

// limpar remove trabalhos antigos (chamado com o mutex preso).
func (t *Trabalhos) limpar() {
	limite := t.agora().Add(-vidaDoTrabalho)
	for id, trabalho := range t.itens {
		if trabalho.CriadoEm.Before(limite) {
			delete(t.itens, id)
		}
	}
}

func novoID() string {
	var bruto [9]byte
	if _, err := rand.Read(bruto[:]); err != nil {
		return "job-" + time.Now().Format("150405.000000000")
	}
	return hex.EncodeToString(bruto[:])
}

// MensagemAmigavel traduz a falha do provedor sem vazar chave nem stack.
func MensagemAmigavel(err error) string {
	var provedor *ErroProvedor
	if errors.As(err, &provedor) {
		switch provedor.Status {
		case http.StatusUnauthorized, http.StatusForbidden:
			return "O provedor recusou a chave configurada no servidor."
		case http.StatusNotFound:
			return "O modelo configurado não está disponível para esta conta."
		case http.StatusTooManyRequests:
			return "O provedor está limitando as requisições; tente de novo em instantes."
		}
		return "O provedor respondeu com erro " + itoa(provedor.Status) + "."
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "O provedor demorou demais para responder; tente de novo."
	}
	return "Não consegui falar com o provedor agora: " + strings.TrimSpace(err.Error())
}

func itoa(valor int) string {
	if valor == 0 {
		return "0"
	}
	negativo := valor < 0
	if negativo {
		valor = -valor
	}
	var digitos []byte
	for valor > 0 {
		digitos = append([]byte{byte('0' + valor%10)}, digitos...)
		valor /= 10
	}
	if negativo {
		return "-" + string(digitos)
	}
	return string(digitos)
}
