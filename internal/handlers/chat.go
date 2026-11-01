package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/httpx"

	"github.com/puppe1990/lumina-reader/internal/chat"
	"github.com/puppe1990/lumina-reader/internal/store"
)

// ConversadorRLM é o "conversar com o livro": recebe a conversa montada e
// devolve a resposta do provedor (nil = recurso desligado por falta de chave).
type ConversadorRLM interface {
	Responder(ctx context.Context, mensagens []chat.Mensagem) (string, error)
}

type ChatHandler struct {
	store       store.Store
	cfg         cais.Config
	respondedor ConversadorRLM
}

func NewChatHandler(s store.Store, cfg cais.Config, respondedor ConversadorRLM) *ChatHandler {
	return &ChatHandler{store: s, cfg: cfg, respondedor: respondedor}
}

// Post responde a pergunta do leitor com base no trecho atual do livro.
func (h *ChatHandler) Post(w http.ResponseWriter, r *http.Request, bookID int64) {
	if err := httpx.ParseFormOrJSON(r); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "erro": "corpo inválido"})
		return
	}
	pergunta := strings.TrimSpace(r.FormValue("pergunta"))
	if pergunta == "" {
		respondJSON(w, http.StatusUnprocessableEntity, map[string]any{"ok": false, "erro": "escreva uma pergunta"})
		return
	}
	if h.respondedor == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]any{
			"ok":   false,
			"erro": "O chat ainda não está configurado no servidor (falta LUMINA_CHAT_API_KEY).",
		})
		return
	}
	livro, err := h.store.GetBook(bookID)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]any{"ok": false, "erro": "livro não encontrado"})
		return
	}

	mensagens, err := chat.MontarMensagens(chat.Pedido{
		Livro:     livro.Title,
		Autor:     livro.Author,
		Posicao:   r.FormValue("posicao"),
		Contexto:  r.FormValue("contexto"),
		Pergunta:  pergunta,
		Historico: lerHistorico(r.FormValue("historico")),
	})
	if err != nil {
		respondJSON(w, http.StatusUnprocessableEntity, map[string]any{"ok": false, "erro": err.Error()})
		return
	}

	ctx, cancelar := context.WithTimeout(r.Context(), 130*time.Second)
	defer cancelar()
	resposta, err := h.respondedor.Responder(ctx, mensagens)
	if err != nil {
		status, mensagem := erroChatAmigavel(err)
		respondJSON(w, status, map[string]any{"ok": false, "erro": mensagem})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true, "resposta": resposta})
}

// erroChatAmigavel traduz a falha do provedor sem vazar chave nem stack.
func erroChatAmigavel(err error) (int, string) {
	var provedor *chat.ErroProvedor
	if errors.As(err, &provedor) {
		switch provedor.Status {
		case http.StatusUnauthorized, http.StatusForbidden:
			return http.StatusBadGateway, "O provedor recusou a chave configurada no servidor."
		case http.StatusNotFound:
			return http.StatusBadGateway, "O modelo configurado não está disponível para esta conta."
		case http.StatusTooManyRequests:
			return http.StatusServiceUnavailable, "O provedor está limitando as requisições; tente de novo em instantes."
		}
		return http.StatusBadGateway, "O provedor respondeu com erro " + strconv.Itoa(provedor.Status) + "."
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusGatewayTimeout, "O provedor demorou demais para responder; tente de novo."
	}
	return http.StatusBadGateway, "Não consegui falar com o provedor agora: " + err.Error()
}

// lerHistorico aceita o JSON do navegador e ignora o que não fizer sentido.
func lerHistorico(bruto string) []chat.Mensagem {
	bruto = strings.TrimSpace(bruto)
	if bruto == "" || !strings.HasPrefix(bruto, "[") {
		return nil
	}
	var itens []struct {
		Papel string `json:"papel"`
		Texto string `json:"texto"`
	}
	if err := json.Unmarshal([]byte(bruto), &itens); err != nil {
		return nil
	}
	saida := make([]chat.Mensagem, 0, len(itens))
	for _, item := range itens {
		if strings.TrimSpace(item.Texto) == "" {
			continue
		}
		saida = append(saida, chat.Mensagem{Papel: item.Papel, Texto: item.Texto})
	}
	return saida
}
