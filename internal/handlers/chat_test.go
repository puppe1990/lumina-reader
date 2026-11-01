package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/puppe1990/amarra-cais/pkg/cais"

	"github.com/puppe1990/lumina-reader/internal/chat"
	"github.com/puppe1990/lumina-reader/internal/store"
)

type fakeConversador struct {
	resposta  string
	erro      error
	demora    time.Duration
	recebidas []chat.Mensagem
}

func (f *fakeConversador) Responder(ctx context.Context, mensagens []chat.Mensagem) (string, error) {
	f.recebidas = mensagens
	if f.demora > 0 {
		select {
		case <-time.After(f.demora):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
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

type respostaPost struct {
	OK   bool   `json:"ok"`
	ID   string `json:"id"`
	Erro string `json:"erro"`
}

type respostaGet struct {
	OK       bool   `json:"ok"`
	Estado   string `json:"estado"`
	Resposta string `json:"resposta"`
	Erro     string `json:"erro"`
}

// aguardarResposta faz o papel do polling do navegador.
func aguardarResposta(t *testing.T, h *ChatHandler, id int64, trabalhoID string) respostaGet {
	t.Helper()
	prazo := time.Now().Add(2 * time.Second)
	var corpo respostaGet
	for time.Now().Before(prazo) {
		req := httptest.NewRequest(http.MethodGet, "/livros/"+itoa64(id)+"/chat/"+trabalhoID, nil)
		rr := httptest.NewRecorder()
		h.Get(rr, req, id, trabalhoID)
		if rr.Code != http.StatusOK {
			t.Fatalf("GET status = %d, body %s", rr.Code, rr.Body.String())
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &corpo); err != nil {
			t.Fatal(err)
		}
		if corpo.Estado != chat.EstadoRodando {
			return corpo
		}
		time.Sleep(3 * time.Millisecond)
	}
	t.Fatalf("trabalho %q não terminou: %+v", trabalhoID, corpo)
	return corpo
}

func TestChat_PostCriaTrabalhoEGetDevolveResposta(t *testing.T) {
	s := setupTestStore(t)
	id := seedLivro(t, s, "Sociedade e Cultura/Cory Doctorow/Enshittification/Enshittification.pdf", "Enshittification", "Cory Doctorow", "Sociedade e Cultura", "pdf")
	conversador := &fakeConversador{resposta: "É a degradação das plataformas."}
	h := newChatHandler(t, s, conversador)

	rr := pedirChat(h, id, `{"pergunta":"O que é?","contexto":"trecho do livro","posicao":"página 3 de 200"}`)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("POST status = %d, corpo %s", rr.Code, rr.Body.String())
	}
	var criado respostaPost
	if err := json.Unmarshal(rr.Body.Bytes(), &criado); err != nil {
		t.Fatal(err)
	}
	if !criado.OK || criado.ID == "" {
		t.Fatalf("POST corpo = %+v", criado)
	}

	final := aguardarResposta(t, h, id, criado.ID)
	if final.Estado != chat.EstadoPronto || final.Resposta != "É a degradação das plataformas." {
		t.Errorf("GET final = %+v", final)
	}
	if len(conversador.recebidas) != 2 {
		t.Fatalf("mensagens = %d, want 2", len(conversador.recebidas))
	}
	if !strings.Contains(conversador.recebidas[0].Texto, "Enshittification") {
		t.Errorf("system sem o título do livro: %q", conversador.recebidas[0].Texto)
	}
}

func TestChat_GetTrabalhoDesconhecido(t *testing.T) {
	s := setupTestStore(t)
	h := newChatHandler(t, s, &fakeConversador{})
	req := httptest.NewRequest(http.MethodGet, "/livros/1/chat/nao-existe", nil)
	rr := httptest.NewRecorder()
	h.Get(rr, req, 1, "nao-existe")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
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

func TestChat_PollingMostraErroDoProvedor(t *testing.T) {
	s := setupTestStore(t)
	id := seedLivro(t, s, "Cat/A/Livro.pdf", "Livro", "", "Cat", "pdf")
	h := newChatHandler(t, s, &fakeConversador{erro: &chat.ErroProvedor{Status: 429}})

	rr := pedirChat(h, id, `{"pergunta":"oi","contexto":"t"}`)
	var criado respostaPost
	if err := json.Unmarshal(rr.Body.Bytes(), &criado); err != nil {
		t.Fatal(err)
	}
	final := aguardarResposta(t, h, id, criado.ID)
	if final.Estado != chat.EstadoErro || !strings.Contains(final.Erro, "limitando") {
		t.Errorf("GET final = %+v", final)
	}
}

func TestChat_HistoricoInvalidoNaoQuebra(t *testing.T) {
	s := setupTestStore(t)
	id := seedLivro(t, s, "Cat/A/Livro.pdf", "Livro", "", "Cat", "pdf")
	conversador := &fakeConversador{resposta: "ok"}
	h := newChatHandler(t, s, conversador)

	rr := pedirChat(h, id, `{"pergunta":"oi","historico":"não é json"}`)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	var criado respostaPost
	if err := json.Unmarshal(rr.Body.Bytes(), &criado); err != nil {
		t.Fatal(err)
	}
	aguardarResposta(t, h, id, criado.ID)
	if len(conversador.recebidas) != 2 {
		t.Errorf("histórico inválido deveria ser ignorado: %d mensagens", len(conversador.recebidas))
	}
}
