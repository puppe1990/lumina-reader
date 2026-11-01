package chat

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeRespondedor struct {
	resposta string
	erro     error
	demora   time.Duration
}

func (f *fakeRespondedor) Responder(ctx context.Context, mensagens []Mensagem) (string, error) {
	if f.demora > 0 {
		select {
		case <-time.After(f.demora):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return f.resposta, f.erro
}

func esperarEstado(t *testing.T, trabalhos *Trabalhos, id, estado string) Trabalho {
	t.Helper()
	prazo := time.Now().Add(2 * time.Second)
	for time.Now().Before(prazo) {
		trabalho, ok := trabalhos.Buscar(id)
		if !ok {
			t.Fatalf("trabalho %q sumiu", id)
		}
		if trabalho.Estado == estado {
			return trabalho
		}
		time.Sleep(5 * time.Millisecond)
	}
	trabalho, _ := trabalhos.Buscar(id)
	t.Fatalf("estado = %q, want %q", trabalho.Estado, estado)
	return Trabalho{}
}

func TestTrabalhos_RespostaChegaNoPolling(t *testing.T) {
	trabalhos := NovosTrabalhos(&fakeRespondedor{resposta: "É a degradação das plataformas.", demora: 20 * time.Millisecond})
	id := trabalhos.Criar([]Mensagem{{Papel: "user", Texto: "oi"}})
	if id == "" {
		t.Fatal("id vazio")
	}
	trabalho := esperarEstado(t, trabalhos, id, EstadoPronto)
	if trabalho.Resposta != "É a degradação das plataformas." || trabalho.Erro != "" {
		t.Errorf("trabalho = %+v", trabalho)
	}
}

func TestTrabalhos_ErroDoProvedorViraMensagemAmigavel(t *testing.T) {
	trabalhos := NovosTrabalhos(&fakeRespondedor{erro: &ErroProvedor{Status: 429}})
	id := trabalhos.Criar(nil)
	trabalho := esperarEstado(t, trabalhos, id, EstadoErro)
	if !strings.Contains(trabalho.Erro, "limitando") {
		t.Errorf("erro = %q, want menção ao limite", trabalho.Erro)
	}
}

func TestTrabalhos_TimeoutCancelaOProvedor(t *testing.T) {
	trabalhos := NovosTrabalhos(&fakeRespondedor{demora: time.Second})
	trabalhos.timeout = 10 * time.Millisecond
	id := trabalhos.Criar(nil)
	trabalho := esperarEstado(t, trabalhos, id, EstadoErro)
	if !strings.Contains(trabalho.Erro, "demorou") {
		t.Errorf("erro = %q, want timeout amigável", trabalho.Erro)
	}
}

func TestTrabalhos_Expira(t *testing.T) {
	trabalhos := NovosTrabalhos(&fakeRespondedor{resposta: "ok"})
	base := time.Now()
	trabalhos.agora = func() time.Time { return base }
	id := trabalhos.Criar(nil)
	esperarEstado(t, trabalhos, id, EstadoPronto)
	trabalhos.agora = func() time.Time { return base.Add(vidaDoTrabalho + time.Minute) }
	if _, ok := trabalhos.Buscar(id); ok {
		t.Fatal("trabalho velho deveria expirar")
	}
}

func TestTrabalhos_ConcorrenciaAguentaOPolling(t *testing.T) {
	respondedor := &fakeRespondedor{resposta: "resposta", demora: 5 * time.Millisecond}
	trabalhos := NovosTrabalhos(respondedor)
	var wg sync.WaitGroup
	ids := make([]string, 0, 50)
	var trava sync.Mutex
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := trabalhos.Criar(nil)
			trava.Lock()
			ids = append(ids, id)
			trava.Unlock()
			trabalhos.Buscar(id)
		}()
	}
	wg.Wait()
	if len(ids) != 50 {
		t.Fatalf("ids = %d", len(ids))
	}
	for _, id := range ids {
		esperarEstado(t, trabalhos, id, EstadoPronto)
	}
}

func TestMensagemAmigavel_Casos(t *testing.T) {
	casos := map[string]error{
		"recusou":   &ErroProvedor{Status: 401},
		"modelo":    &ErroProvedor{Status: 404},
		"limitando": &ErroProvedor{Status: 429},
		"demorou":   context.DeadlineExceeded,
		"falar":     errors.New("rede fora"),
	}
	for trecho, err := range casos {
		if msg := MensagemAmigavel(err); msg == "" || !strings.Contains(strings.ToLower(msg), trecho) {
			t.Errorf("MensagemAmigavel(%v) = %q, want %q", err, msg, trecho)
		}
	}
}
