package chat

import (
	"strconv"
	"strings"
	"testing"
)

func TestMontarMensagens_MontaSistemaComLivroEAutor(t *testing.T) {
	mensagens, err := MontarMensagens(Pedido{
		Livro:    "Enshittification",
		Autor:    "Cory Doctorow",
		Posicao:  "página 12 de 225",
		Contexto: "A plataforma começa atraindo usuários com uma experiência encantadora.",
		Pergunta: "O que é enshittification?",
	})
	if err != nil {
		t.Fatalf("MontarMensagens erro = %v", err)
	}
	if len(mensagens) != 2 {
		t.Fatalf("mensagens = %d, want 2 (system + pergunta)", len(mensagens))
	}
	sistema := mensagens[0]
	if sistema.Papel != "system" {
		t.Errorf("primeiro papel = %q, want system", sistema.Papel)
	}
	for _, alvo := range []string{"Enshittification", "Cory Doctorow", "página 12 de 225", "português"} {
		if !strings.Contains(sistema.Texto, alvo) {
			t.Errorf("system não menciona %q: %q", alvo, sistema.Texto)
		}
	}
	pergunta := mensagens[1]
	if pergunta.Papel != "user" {
		t.Errorf("papel da pergunta = %q, want user", pergunta.Papel)
	}
	if !strings.Contains(pergunta.Texto, "experiência encantadora") {
		t.Errorf("pergunta sem contexto: %q", pergunta.Texto)
	}
	if !strings.Contains(pergunta.Texto, "O que é enshittification?") {
		t.Errorf("pergunta sem a pergunte: %q", pergunta.Texto)
	}
}

func TestMontarMensagens_LivroSemAutorNaoQuebra(t *testing.T) {
	mensagens, err := MontarMensagens(Pedido{Livro: "Apostila", Contexto: "trecho", Pergunta: "resuma"})
	if err != nil {
		t.Fatalf("MontarMensagens erro = %v", err)
	}
	if strings.Contains(mensagens[0].Texto, " de  ") {
		t.Errorf("system com autor vazio: %q", mensagens[0].Texto)
	}
}

func TestMontarMensagens_TruncaContextoPerguntaEHistorico(t *testing.T) {
	historico := make([]Mensagem, 0, 30)
	for i := 0; i < 30; i++ {
		historico = append(historico, Mensagem{Papel: "user", Texto: "mensagem-" + strconv.Itoa(i)})
	}
	mensagens, err := MontarMensagens(Pedido{
		Livro:     "Livro",
		Contexto:  strings.Repeat("c", LimiteContexto*2),
		Pergunta:  strings.Repeat("p", LimitePergunta*3),
		Historico: historico,
	})
	if err != nil {
		t.Fatalf("MontarMensagens erro = %v", err)
	}
	if len(mensagens) != 1+LimiteHistorico+1 {
		t.Fatalf("mensagens = %d, want %d", len(mensagens), 1+LimiteHistorico+1)
	}
	if !strings.Contains(mensagens[1].Texto, "mensagem-22") {
		t.Errorf("histórico deveria manter as últimas: %q", mensagens[1].Texto)
	}
	if strings.Contains(strings.Join(textos(mensagens), " "), "mensagem-0 ") {
		t.Errorf("histórico antigo deveria ser descartado")
	}
	ultima := mensagens[len(mensagens)-1]
	if len(ultima.Texto) > LimiteContexto+LimitePergunta+200 {
		t.Errorf("mensagem final grande demais: %d", len(ultima.Texto))
	}
	if !strings.Contains(ultima.Texto, "truncado") {
		t.Errorf("contexto truncado sem aviso: %q", ultima.Texto[:80])
	}
}

func TestMontarMensagens_PapelInvalidoViraUser(t *testing.T) {
	mensagens, err := MontarMensagens(Pedido{
		Livro: "L", Contexto: "t", Pergunta: "p",
		Historico: []Mensagem{{Papel: "hacker", Texto: "oi"}, {Papel: "assistant", Texto: "olá"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if mensagens[1].Papel != "user" || mensagens[2].Papel != "assistant" {
		t.Errorf("papéis = %q, %q", mensagens[1].Papel, mensagens[2].Papel)
	}
}

func TestMontarMensagens_PerguntaVazia(t *testing.T) {
	if _, err := MontarMensagens(Pedido{Livro: "L", Contexto: "t", Pergunta: "   "}); err == nil {
		t.Fatal("pergunta vazia deveria dar erro")
	}
}

func textos(mensagens []Mensagem) []string {
	saida := make([]string, 0, len(mensagens))
	for _, m := range mensagens {
		saida = append(saida, m.Texto)
	}
	return saida
}
