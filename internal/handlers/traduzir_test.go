package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/puppe1990/amarra-cais/pkg/cais"

	"github.com/puppe1990/lumina-reader/internal/chat"
)

type fakeCache struct {
	itens map[string]string
}

func (f *fakeCache) TraducaoCacheada(fp string) (string, bool, error) {
	valor, ok := f.itens[fp]
	return valor, ok, nil
}

func (f *fakeCache) GuardarTraducao(fp, traducao string) error {
	if f.itens == nil {
		f.itens = map[string]string{}
	}
	f.itens[fp] = traducao
	return nil
}

func newTranslateHandler(t *testing.T, conversador ConversadorRLM) *TranslateHandler {
	t.Helper()
	return NewTranslateHandler(cais.Config{Env: "test"}, conversador, nil)
}

func newTranslateHandlerComCache(t *testing.T, conversador ConversadorRLM, cache TraducaoCache) *TranslateHandler {
	t.Helper()
	return NewTranslateHandler(cais.Config{Env: "test"}, conversador, cache)
}

func pedirTraducao(h *TranslateHandler, corpo string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/traduzir", strings.NewReader(corpo))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Post(rr, req)
	return rr
}

type traducaoPost struct {
	OK       bool   `json:"ok"`
	ID       string `json:"id"`
	Estado   string `json:"estado"`
	Traducao string `json:"traducao"`
	Cache    bool   `json:"cache"`
	Erro     string `json:"erro"`
}

type traducaoGet struct {
	Estado   string `json:"estado"`
	Traducao string `json:"traducao"`
	Erro     string `json:"erro"`
}

func aguardarTraducao(t *testing.T, h *TranslateHandler, id string) traducaoGet {
	t.Helper()
	for i := 0; i < 400; i++ {
		req := httptest.NewRequest(http.MethodGet, "/traduzir/"+id, nil)
		rr := httptest.NewRecorder()
		h.Get(rr, req, id)
		if rr.Code != http.StatusOK {
			t.Fatalf("GET status = %d, body %s", rr.Code, rr.Body.String())
		}
		var corpo traducaoGet
		if err := json.Unmarshal(rr.Body.Bytes(), &corpo); err != nil {
			t.Fatal(err)
		}
		if corpo.Estado != chat.EstadoRodando {
			return corpo
		}
	}
	t.Fatalf("tradução %q não terminou", id)
	return traducaoGet{}
}

func TestTraduzir_PostCriaTrabalhoEGetDevolveTraducao(t *testing.T) {
	conversador := &fakeConversador{resposta: "  enshitificação (degradação deliberada de uma plataforma)  "}
	h := newTranslateHandler(t, conversador)

	rr := pedirTraducao(h, `{"texto":"enshittification","livro":12}`)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, corpo %s", rr.Code, rr.Body.String())
	}
	var criado traducaoPost
	if err := json.Unmarshal(rr.Body.Bytes(), &criado); err != nil {
		t.Fatal(err)
	}
	if !criado.OK || criado.ID == "" {
		t.Fatalf("corpo = %+v", criado)
	}

	final := aguardarTraducao(t, h, criado.ID)
	if final.Estado != chat.EstadoPronto {
		t.Fatalf("estado = %q", final.Estado)
	}
	if final.Traducao != "enshitificação (degradação deliberada de uma plataforma)" {
		t.Errorf("tradução = %q (deveria vir sem espaços sobrando)", final.Traducao)
	}
	if len(conversador.recebidas) != 2 {
		t.Fatalf("mensagens = %d, want 2", len(conversador.recebidas))
	}
	if !strings.Contains(conversador.recebidas[0].Texto, "português do Brasil") {
		t.Errorf("system = %q", conversador.recebidas[0].Texto)
	}
	if conversador.recebidas[1].Texto != "enshittification" {
		t.Errorf("trecho enviado = %q", conversador.recebidas[1].Texto)
	}
}

func TestTraduzir_TextoVazio(t *testing.T) {
	h := newTranslateHandler(t, &fakeConversador{})
	rr := pedirTraducao(h, `{"texto":"   "}`)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rr.Code)
	}
}

func TestTraduzir_SemProvedorConfigurado(t *testing.T) {
	h := newTranslateHandler(t, nil)
	rr := pedirTraducao(h, `{"texto":"hello"}`)
	if rr.Code != http.StatusFailedDependency {
		t.Fatalf("status = %d, want 424", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "LUMINA_CHAT_API_KEY") {
		t.Errorf("mensagem deveria explicar a configuração: %s", rr.Body.String())
	}
}

func TestTraduzir_PollingMostraErro(t *testing.T) {
	conversador := &fakeConversador{erro: &chat.ErroProvedor{Status: 429}}
	h := newTranslateHandler(t, conversador)
	rr := pedirTraducao(h, `{"texto":"hello"}`)
	var criado traducaoPost
	if err := json.Unmarshal(rr.Body.Bytes(), &criado); err != nil {
		t.Fatal(err)
	}
	final := aguardarTraducao(t, h, criado.ID)
	if final.Estado != chat.EstadoErro || !strings.Contains(final.Erro, "limitando") {
		t.Errorf("final = %+v", final)
	}
}

func TestTraduzir_GetDesconhecido(t *testing.T) {
	h := newTranslateHandler(t, &fakeConversador{})
	req := httptest.NewRequest(http.MethodGet, "/traduzir/nada", nil)
	rr := httptest.NewRecorder()
	h.Get(rr, req, "nada")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestTraduzir_CacheDevolveNaHoraEGuarda(t *testing.T) {
	conversador := &fakeConversador{resposta: "degradação deliberada de plataformas"}
	cache := &fakeCache{}
	h := newTranslateHandlerComCache(t, conversador, cache)

	// primeira vez: cria trabalho e guarda no cache ao terminar
	rr := pedirTraducao(h, `{"texto":"enshittification"}`)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d", rr.Code)
	}
	var criado traducaoPost
	if err := json.Unmarshal(rr.Body.Bytes(), &criado); err != nil {
		t.Fatal(err)
	}
	aguardarTraducao(t, h, criado.ID)
	marca := fingerprint("enshittification")
	if cache.itens[marca] != "degradação deliberada de plataformas" {
		t.Fatalf("cache não guardou: %+v", cache.itens)
	}

	// segunda vez (mesmo texto, outra caixa/espaço): volta direto, sem trabalho
	rr = pedirTraducao(h, `{"texto":"  Enshittification "}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 do cache", rr.Code)
	}
	var direto traducaoPost
	if err := json.Unmarshal(rr.Body.Bytes(), &direto); err != nil {
		t.Fatal(err)
	}
	if !direto.Cache || direto.Estado != chat.EstadoPronto || direto.Traducao != "degradação deliberada de plataformas" {
		t.Errorf("resposta do cache = %+v", direto)
	}
	if len(conversador.recebidas) != 2 {
		t.Errorf("o provedor foi chamado de novo: %d mensagens (uma tradução só)", len(conversador.recebidas))
	}
}

func TestFingerprint_IgnoraCaixaEEspacos(t *testing.T) {
	if fingerprint("  Olá   Mundo ") != fingerprint("olá mundo") {
		t.Fatal("fingerprint deveria normalizar caixa e espaços")
	}
	if fingerprint("palavra") == fingerprint("outra") {
		t.Fatal("textos diferentes precisam de fingerprints diferentes")
	}
}
