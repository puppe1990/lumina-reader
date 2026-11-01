package handlers

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"net/http"
	"regexp"
	"strings"

	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/httpx"

	"github.com/puppe1990/lumina-reader/internal/chat"
	"github.com/puppe1990/lumina-reader/internal/translate"
)

// TraducaoCache guarda as traduções já feitas (palavra repetida volta na hora).
type TraducaoCache interface {
	TraducaoCacheada(fingerprint string) (string, bool, error)
	GuardarTraducao(fingerprint, traducao string) error
}

// TranslateHandler traduz o trecho selecionado em background (mesmo padrão do
// chat: POST devolve id na hora, GET entrega o resultado). Tradução já feita
// volta direto, sem chamar o provedor.
type TranslateHandler struct {
	cfg       cais.Config
	trabalhos *chat.Trabalhos
	cache     TraducaoCache
	tradutor  ConversadorRLM
}

func NewTranslateHandler(cfg cais.Config, respondedor ConversadorRLM, cache TraducaoCache) *TranslateHandler {
	handler := &TranslateHandler{cfg: cfg, trabalhos: chat.NovosTrabalhos(respondedor), cache: cache}
	if respondedor != nil {
		handler.tradutor = respondedor
	}
	return handler
}

// tradutorComCache salva a tradução quando o provedor responde bem.
type tradutorComCache struct {
	proximo     ConversadorRLM
	cache       TraducaoCache
	fingerprint string
}

func (t tradutorComCache) Responder(ctx context.Context, mensagens []chat.Mensagem) (string, error) {
	resposta, err := t.proximo.Responder(ctx, mensagens)
	if err == nil && strings.TrimSpace(resposta) != "" && t.cache != nil {
		_ = t.cache.GuardarTraducao(t.fingerprint, strings.TrimSpace(resposta))
	}
	return resposta, err
}

var espacosTraducao = regexp.MustCompile(`\s+`)

// fingerprint identifica o trecho para o cache (sem caixa nem espaços extras).
func fingerprint(texto string) string {
	limpo := strings.ToLower(strings.TrimSpace(espacosTraducao.ReplaceAllString(texto, " ")))
	resumo := sha1.Sum([]byte(limpo))
	return hex.EncodeToString(resumo[:])
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
	texto := r.FormValue("texto")
	mensagens, err := translate.MontarMensagens(texto)
	if err != nil {
		respondJSON(w, http.StatusUnprocessableEntity, map[string]any{"ok": false, "erro": err.Error()})
		return
	}
	marca := fingerprint(texto)
	if h.cache != nil {
		if traducao, encontrada, err := h.cache.TraducaoCacheada(marca); err == nil && encontrada {
			respondJSON(w, http.StatusOK, map[string]any{"ok": true, "estado": chat.EstadoPronto, "traducao": traducao, "cache": true})
			return
		}
	}
	provedor := ConversadorRLM(tradutorComCache{proximo: h.tradutor, cache: h.cache, fingerprint: marca})
	id := h.trabalhos.CriarCom(provedor, []chat.Mensagem{
		{Papel: "system", Texto: mensagens[0]},
		{Papel: "user", Texto: mensagens[1]},
	})
	respondJSON(w, http.StatusAccepted, map[string]any{"ok": true, "id": id, "cache": false})
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
