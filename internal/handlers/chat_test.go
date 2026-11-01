package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/puppe1990/amarra-cais/pkg/cais"

	"github.com/puppe1990/lumina-reader/internal/chat"
	"github.com/puppe1990/lumina-reader/internal/store"
)

type fakeConversador struct {
	resposta  string
	erro      error
	recebidas []chat.Mensagem
}

func (f *fakeConversador) Responder(ctx context.Context, mensagens []chat.Mensagem) (string, error) {
	f.recebidas = mensagens
	return f.resposta, f.erro
}

func newChatHandler(t *testing.T, s store.Store, conversador ConversadorRLM) *ChatHandler {
	t.Helper()
	return NewChatHandler(s, cais.Config{Env: "test"}, conversador)
}

func pedirChat(h *ChatHandler, id int64, corpo string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/livros/"+itoa64(id)+"/chat", strings.NewReader(corpo))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Post(rr, req, id)
	return rr
}

func TestChat_RespondeUsandoLivroDoAcervo(t *testing.T) {
	s := setupTestStore(t)
	id := seedLivro(t, s, "Sociedade e Cultura/Cory Doctorow/Enshittification/Enshittification.pdf", "Enshittification", "Cory Doctorow", "Sociedade e Cultura", "pdf")
	conversador := &fakeConversador{resposta: "É a degradação das plataformas."}
	h := newChatHandler(t, s, conversador)

	rr := pedirChat(h, id, `{"pergunta":"O que é?","contexto":"trecho do livro","posicao":"página 3 de 200"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, corpo %s", rr.Code, rr.Body.String())
	}
	var corpo map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &corpo); err != nil {
		t.Fatal(err)
	}
	if corpo["ok"] != true || corpo["resposta"] != "É a degradação das plataformas." {
		t.Errorf("corpo = %v", corpo)
	}
	if len(conversador.recebidas) != 2 {
		t.Fatalf("mensagens = %d, want 2", len(conversador.recebidas))
	}
	if !strings.Contains(conversador.recebidas[0].Texto, "Enshittification") {
		t.Errorf("system sem o título do livro: %q", conversador.recebidas[0].Texto)
	}
}

func TestChat_SemProvedorConfigurado(t *testing.T) {
	s := setupTestStore(t)
	id := seedLivro(t, s, "Cat/A/Livro.pdf", "Livro", "", "Cat", "pdf")
	rr := pedirChat(newChatHandler(t, s, nil), id, `{"pergunta":"oi"}`)
	if rr.Code != http.StatusFailedDependency {
		t.Fatalf("status = %d, want 424", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "LUMINA_CHAT_API_KEY") {
		t.Errorf("mensagem deveria explicar a configuração: %s", rr.Body.String())
	}
}

func TestChat_PerguntaVazia(t *testing.T) {
	s := setupTestStore(t)
	id := seedLivro(t, s, "Cat/A/Livro.pdf", "Livro", "", "Cat", "pdf")
	rr := pedirChat(newChatHandler(t, s, &fakeConversador{}), id, `{"pergunta":"   "}`)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rr.Code)
	}
}

func TestChat_LivroInexistente(t *testing.T) {
	s := setupTestStore(t)
	rr := pedirChat(newChatHandler(t, s, &fakeConversador{}), 999, `{"pergunta":"oi"}`)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestChat_ErroDoProvedorViraMensagemAmigavel(t *testing.T) {
	casos := []struct {
		erro   error
		status int
		trecho string
	}{
		{&chat.ErroProvedor{Status: 401}, http.StatusFailedDependency, "recusou a chave"},
		{&chat.ErroProvedor{Status: 429}, http.StatusFailedDependency, "limitando"},
		{context.DeadlineExceeded, http.StatusFailedDependency, "demorou"},
		{errors.New("rede fora"), http.StatusFailedDependency, "Não consegui falar"},
	}
	for _, caso := range casos {
		s := setupTestStore(t)
		id := seedLivro(t, s, "Cat/A/Livro.pdf", "Livro", "", "Cat", "pdf")
		h := newChatHandler(t, s, &fakeConversador{erro: caso.erro})
		rr := pedirChat(h, id, `{"pergunta":"oi","contexto":"t"}`)
		if rr.Code != caso.status {
			t.Errorf("erro %v: status = %d, want %d", caso.erro, rr.Code, caso.status)
		}
		if !strings.Contains(rr.Body.String(), caso.trecho) {
			t.Errorf("erro %v: corpo = %s, quer %q", caso.erro, rr.Body.String(), caso.trecho)
		}
	}
}

func TestChat_HistoricoInvalidoNaoQuebra(t *testing.T) {
	s := setupTestStore(t)
	id := seedLivro(t, s, "Cat/A/Livro.pdf", "Livro", "", "Cat", "pdf")
	conversador := &fakeConversador{resposta: "ok"}
	h := newChatHandler(t, s, conversador)
	rr := pedirChat(h, id, `{"pergunta":"oi","historico":"não é json"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	if len(conversador.recebidas) != 2 {
		t.Errorf("histórico inválido deveria ser ignorado: %d mensagens", len(conversador.recebidas))
	}
}
