package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/httpx"

	"github.com/puppe1990/lumina-reader/internal/chat"
	"github.com/puppe1990/lumina-reader/internal/models"
	"github.com/puppe1990/lumina-reader/internal/reader"
	"github.com/puppe1990/lumina-reader/internal/store"
)

// ConversadorRLM é o "conversar com o livro": recebe a conversa montada e
// devolve a resposta do provedor (nil = recurso desligado por falta de chave).
type ConversadorRLM interface {
	Responder(ctx context.Context, mensagens []chat.Mensagem) (string, error)
}

// TextosDoLivro entrega o livro inteiro em unidades pesquisáveis (capítulo no
// EPUB, página no PDF) para o chat responder sobre a obra toda.
type TextosDoLivro interface {
	TextoDoLivro(ctx context.Context, book models.Book) ([]reader.Unidade, error)
}

// ChatHandler responde a pergunta em background: o POST devolve um id na hora
// (o proxy na frente do app corta requisições longas) e o GET devolve o estado
// até a resposta do LLM chegar.
// inicioDoLivroCaracteres é quanto do começo do livro vai junto da pergunta.
const inicioDoLivroCaracteres = 8000

type ChatHandler struct {
	store     store.Store
	cfg       cais.Config
	trabalhos *chat.Trabalhos
	textos    TextosDoLivro
}

func NewChatHandler(s store.Store, cfg cais.Config, respondedor ConversadorRLM, textos TextosDoLivro) *ChatHandler {
	return &ChatHandler{store: s, cfg: cfg, trabalhos: chat.NovosTrabalhos(respondedor), textos: textos}
}

// Post valida a pergunta, dispara o trabalho e devolve o id do polling.
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
	if !h.trabalhos.Ligado() {
		respondJSON(w, http.StatusFailedDependency, map[string]any{
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

	sumario, trechos := h.recortarLivro(r.Context(), livro, pergunta)
	contexto := strings.TrimSpace(r.FormValue("contexto"))
	if len(sumario) == 0 && len(trechos) == 0 && contexto == "" {
		// PDF digitalizado (páginas como imagem): não há texto no servidor nem
		// na página que o leitor está vendo. Dizer isso é melhor que deixar o
		// modelo responder que "não achou os trechos".
		respondJSON(w, http.StatusFailedDependency, map[string]any{
			"ok":   false,
			"erro": "Este livro não tem texto que eu consiga ler (PDF digitalizado, com as páginas como imagem). O chat funciona em livros com texto.",
		})
		return
	}
	mensagens, err := chat.MontarMensagens(chat.Pedido{
		Livro:     livro.Title,
		Autor:     livro.Author,
		Posicao:   r.FormValue("posicao"),
		Sumario:   sumario,
		Trechos:   trechos,
		Contexto:  contexto,
		Pergunta:  pergunta,
		Historico: lerHistorico(r.FormValue("historico")),
	})
	if err != nil {
		respondJSON(w, http.StatusUnprocessableEntity, map[string]any{"ok": false, "erro": err.Error()})
		return
	}

	id := h.trabalhos.Criar(mensagens)
	respondJSON(w, http.StatusAccepted, map[string]any{"ok": true, "id": id})
}

// Get devolve o estado da pergunta: rodando, pronta (com resposta) ou erro.
func (h *ChatHandler) Get(w http.ResponseWriter, r *http.Request, bookID int64, trabalhoID string) {
	_ = bookID
	trabalho, ok := h.trabalhos.Buscar(trabalhoID)
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]any{"ok": false, "erro": "pergunta não encontrada (expirou?)"})
		return
	}
	corpo := map[string]any{"ok": true, "estado": trabalho.Estado}
	if trabalho.Resposta != "" {
		corpo["resposta"] = trabalho.Resposta
	}
	if trabalho.Erro != "" {
		corpo["erro"] = trabalho.Erro
	}
	respondJSON(w, http.StatusOK, corpo)
}

// recortarLivro lê o livro inteiro e separa o sumário (só EPUB) dos trechos
// que respondem à pergunta. Poucos acertos ganham a abertura do livro (a
// introdução costuma ter a tese e as listas do autor). Formato sem texto
// (mobi/azw3) devolve vazio e o modelo cai no trecho atual do navegador.
func (h *ChatHandler) recortarLivro(ctx context.Context, livro models.Book, pergunta string) ([]string, []chat.Trecho) {
	if h.textos == nil {
		return nil, nil
	}
	unidades, err := h.textos.TextoDoLivro(ctx, livro)
	if err != nil || len(unidades) == 0 {
		return nil, nil
	}
	todos := make([]chat.Trecho, 0, len(unidades))
	for _, unidade := range unidades {
		todos = append(todos, chat.Trecho{Rotulo: unidade.Rotulo, Texto: unidade.Texto})
	}
	var sumario []string
	if livro.Format == "epub" {
		sumario = make([]string, 0, len(unidades))
		for _, unidade := range unidades {
			sumario = append(sumario, unidade.Rotulo)
		}
	}
	trechos := chat.SelecionarTrechos(todos, pergunta, chat.MaxTrechos)
	trechos = append(trechos, inicioDoLivro(todos)...)
	return sumario, trechos
}

// inicioDoLivro manda os primeiros pedaços do livro (ficha, sumário e
// introdução): é onde ficam a tese e as listas do autor, que uma pergunta
// geral costuma querer. Fatia em trechos do tamanho que o prompt aceita.
func inicioDoLivro(unidades []chat.Trecho) []chat.Trecho {
	var construtor strings.Builder
	for _, unidade := range unidades {
		texto := strings.TrimSpace(unidade.Texto)
		if len([]rune(texto)) < 80 {
			continue
		}
		construtor.WriteString("[" + unidade.Rotulo + "] " + texto + "\n")
		if construtor.Len() >= inicioDoLivroCaracteres {
			break
		}
	}
	conteudo := []rune(strings.TrimSpace(construtor.String()))
	if len(conteudo) == 0 {
		return nil
	}
	var fatias []chat.Trecho
	for indice := 0; indice < len(conteudo); indice += chat.MaxTrechoCaracteres {
		fim := indice + chat.MaxTrechoCaracteres
		if fim > len(conteudo) {
			fim = len(conteudo)
		}
		fatias = append(fatias, chat.Trecho{
			Rotulo: fmt.Sprintf("início do livro %d/%d", len(fatias)+1, (len(conteudo)+chat.MaxTrechoCaracteres-1)/chat.MaxTrechoCaracteres),
			Texto:  string(conteudo[indice:fim]),
		})
	}
	return fatias
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
