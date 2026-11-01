package handlers

import (
	"net/http"
	"strings"

	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/httpx"

	"github.com/puppe1990/lumina-reader/internal/chat"
	"github.com/puppe1990/lumina-reader/internal/translate"
)

// TranslateHandler traduz o trecho selecionado em background (mesmo padrão do
// chat: POST devolve id na hora, GET entrega o resultado).
type TranslateHandler struct {
	cfg       cais.Config
	trabalhos *chat.Trabalhos
}

func NewTranslateHandler(cfg cais.Config, respondedor ConversadorRLM) *TranslateHandler {
	return &TranslateHandler{cfg: cfg, trabalhos: chat.NovosTrabalhos(respondedor)}
}

// Post valida o trecho e dispara a tradução.
func (h *TranslateHandler) Post(w http.ResponseWriter, r *http.Request) {
	if err := httpx.ParseFormOrJSON(r); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "erro": "corpo inválido"})
		return
	}
	if !h.trabalhos.Ligado() {
		respondJSON(w, http.StatusFailedDependency, map[string]any{
			"ok":   false,
			"erro": "A tradução ainda não está configurada no servidor (falta LUMINA_CHAT_API_KEY).",
		})
		return
	}
	mensagens, err := translate.MontarMensagens(r.FormValue("texto"))
	if err != nil {
		respondJSON(w, http.StatusUnprocessableEntity, map[string]any{"ok": false, "erro": err.Error()})
		return
	}
	id := h.trabalhos.Criar([]chat.Mensagem{
		{Papel: "system", Texto: mensagens[0]},
		{Papel: "user", Texto: mensagens[1]},
	})
	respondJSON(w, http.StatusAccepted, map[string]any{"ok": true, "id": id})
}

// Get devolve o estado da tradução.
func (h *TranslateHandler) Get(w http.ResponseWriter, r *http.Request, trabalhoID string) {
	trabalho, ok := h.trabalhos.Buscar(trabalhoID)
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]any{"ok": false, "erro": "tradução não encontrada (expirou?)"})
		return
	}
	corpo := map[string]any{"ok": true, "estado": trabalho.Estado}
	if trabalho.Resposta != "" {
		corpo["traducao"] = strings.TrimSpace(trabalho.Resposta)
	}
	if trabalho.Erro != "" {
		corpo["erro"] = trabalho.Erro
	}
	respondJSON(w, http.StatusOK, corpo)
}
