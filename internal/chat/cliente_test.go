package chat

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCliente_Responder_EnviaModeloMensagensEChave(t *testing.T) {
	var recebido requisicaoChat
	var autorizacao string
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		autorizacao = r.Header.Get("Authorization")
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&recebido); err != nil {
			t.Errorf("corpo inválido: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"  Enshittification é a degradação deliberada de plataformas.  ","reasoning_content":"pensando..."}}]}`))
	}))
	defer servidor.Close()

	cliente := NovoCliente(servidor.URL, "chave-secreta", "minimaxai/minimax-m3.1-flash")
	resposta, err := cliente.Responder(context.Background(), []Mensagem{{Papel: "system", Texto: "oi"}, {Papel: "user", Texto: "pergunta"}})
	if err != nil {
		t.Fatalf("Responder erro = %v", err)
	}
	if resposta != "Enshittification é a degradação deliberada de plataformas." {
		t.Errorf("resposta = %q (esperava trim do content, sem reasoning)", resposta)
	}
	if autorizacao != "Bearer chave-secreta" {
		t.Errorf("Authorization = %q", autorizacao)
	}
	if recebido.Model != "minimaxai/minimax-m3.1-flash" {
		t.Errorf("model = %q", recebido.Model)
	}
	if len(recebido.Messages) != 2 || recebido.Messages[1].Content != "pergunta" {
		t.Errorf("messages = %+v", recebido.Messages)
	}
	if recebido.MaxTokens < 1000 {
		t.Errorf("max_tokens = %d; modelos de raciocínio precisam de folga", recebido.MaxTokens)
	}
}

func TestCliente_Responder_ErroDoProvedor(t *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail":"invalid key"}`))
	}))
	defer servidor.Close()

	_, err := NovoCliente(servidor.URL, "chave-errada", "modelo").Responder(context.Background(), []Mensagem{{Papel: "user", Texto: "oi"}})
	var erro *ErroProvedor
	if !errors.As(err, &erro) {
		t.Fatalf("erro = %v, want ErroProvedor", err)
	}
	if erro.Status != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", erro.Status)
	}
	if !strings.Contains(erro.Detalhe, "invalid key") {
		t.Errorf("detalhe = %q", erro.Detalhe)
	}
}

func TestCliente_Responder_ConteudoVazio(t *testing.T) {
	casos := []struct {
		nome     string
		corpo    string
		esperado string
	}{
		{"sem choices", `{"choices":[]}`, "vazia"},
		{"length sem content", `{"choices":[{"finish_reason":"length","message":{"content":""}}]}`, "tokens"},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(caso.corpo))
			}))
			defer servidor.Close()
			_, err := NovoCliente(servidor.URL, "k", "m").Responder(context.Background(), nil)
			if err == nil || !strings.Contains(err.Error(), caso.esperado) {
				t.Errorf("erro = %v, want menção a %q", err, caso.esperado)
			}
		})
	}
}

func TestCliente_Responder_TentaDeNovoDepoisDeErro5xx(t *testing.T) {
	chamadas := 0
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chamadas++
		if chamadas == 1 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"detail":"instável"}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"deu certo"}}]}`))
	}))
	defer servidor.Close()

	resposta, err := NovoCliente(servidor.URL, "k", "m").Responder(context.Background(), []Mensagem{{Papel: "user", Texto: "oi"}})
	if err != nil {
		t.Fatalf("Responder com retry erro = %v", err)
	}
	if resposta != "deu certo" || chamadas != 2 {
		t.Errorf("resposta = %q, chamadas = %d", resposta, chamadas)
	}
}

func TestCliente_Responder_NaoTentaDeNovoEmErro4xx(t *testing.T) {
	chamadas := 0
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chamadas++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer servidor.Close()

	if _, err := NovoCliente(servidor.URL, "k", "m").Responder(context.Background(), nil); err == nil {
		t.Fatal("esperava erro")
	}
	if chamadas != 1 {
		t.Errorf("chamadas = %d, want 1 (4xx não repete)", chamadas)
	}
}

func TestCliente_Responder_SemChaveNemChamaAAPI(t *testing.T) {
	chamou := false
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { chamou = true }))
	defer servidor.Close()
	if _, err := NovoCliente(servidor.URL, "", "m").Responder(context.Background(), nil); err == nil {
		t.Fatal("esperava erro sem chave")
	}
	if chamou {
		t.Fatal("não deveria chamar a API sem chave")
	}
}

func TestCliente_Responder_ContextoCancelado(t *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer servidor.Close()
	ctx, cancelar := context.WithCancel(context.Background())
	cancelar()
	if _, err := NovoCliente(servidor.URL, "k", "m").Responder(ctx, nil); err == nil {
		t.Fatal("esperava erro com contexto cancelado")
	}
}
