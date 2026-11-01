package chat

import (
	"strconv"
	"strings"
	"testing"
)

func TestMontarMensagens_SistemaFalaDoLivroInteiro(t *testing.T) {
	mensagens, err := MontarMensagens(Pedido{
		Livro:    "Enshittification",
		Autor:    "Cory Doctorow",
		Pergunta: "Qual é a ideia central do livro?",
	})
	if err != nil {
		t.Fatalf("MontarMensagens erro = %v", err)
	}
	if len(mensagens) != 2 {
		t.Fatalf("mensagens = %d, want 2", len(mensagens))
	}
	sistema := mensagens[0]
	for _, alvo := range []string{"Enshittification", "Cory Doctorow", "livro inteiro", "português"} {
		if !strings.Contains(sistema.Texto, alvo) {
			t.Errorf("system não menciona %q", alvo)
		}
	}
}

func TestMontarMensagens_JuntaSumarioTrechosEContexto(t *testing.T) {
	mensagens, err := MontarMensagens(Pedido{
		Livro:    "Livro",
		Posicao:  "página 125 de 225",
		Sumario:  []string{"capítulo 1 — Abertura", "capítulo 2 — O método"},
		Trechos:  []Trecho{{Rotulo: "capítulo 2 — O método", Texto: "passagem relevante sobre o método"}},
		Contexto: "texto da página atual",
		Pergunta: "o que o autor defende?",
	})
	if err != nil {
		t.Fatal(err)
	}
	corpo := mensagens[len(mensagens)-1].Texto
	for _, alvo := range []string{
		"Sumário do livro", "capítulo 1 — Abertura", "capítulo 2 — O método",
		"Trechos do livro que podem responder", "passagem relevante sobre o método",
		"Trecho em que o leitor está agora (página 125 de 225)", "texto da página atual",
		"Pergunta do leitor: o que o autor defende?",
	} {
		if !strings.Contains(corpo, alvo) {
			t.Errorf("mensagem sem %q:\n%s", alvo, corpo)
		}
	}
}

func TestMontarMensagens_SemTrechosAvisaOModelo(t *testing.T) {
	mensagens, err := MontarMensagens(Pedido{Livro: "Livro", Pergunta: "resuma"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mensagens[len(mensagens)-1].Texto, "não conseguiu ler o texto deste livro") {
		t.Errorf("faltou o aviso de texto indisponível: %q", mensagens[len(mensagens)-1].Texto)
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
	var trechos []Trecho
	for i := 0; i < 30; i++ {
		trechos = append(trechos, Trecho{Rotulo: "página " + strconv.Itoa(i), Texto: strings.Repeat("z", MaxTrechoCaracteres)})
	}
	mensagens, err := MontarMensagens(Pedido{
		Livro:     "Livro",
		Sumario:   []string{strings.Repeat("título ", 500)},
		Trechos:   trechos,
		Contexto:  strings.Repeat("c", LimiteContextoAtual*2),
		Pergunta:  strings.Repeat("p", LimitePergunta*3),
		Historico: historico,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(mensagens) != 1+LimiteHistorico+1 {
		t.Fatalf("mensagens = %d, want %d", len(mensagens), 1+LimiteHistorico+1)
	}
	if !strings.Contains(mensagens[1].Texto, "mensagem-22") {
		t.Errorf("histórico deveria manter as últimas: %q", mensagens[1].Texto)
	}
	corpo := mensagens[len(mensagens)-1].Texto
	if len(corpo) > 20000 {
		t.Errorf("mensagem final grande demais: %d", len(corpo))
	}
	if !strings.Contains(corpo, "truncado") {
		t.Errorf("faltou aviso de truncamento")
	}
}

func TestMontarMensagens_PapelInvalidoViraUser(t *testing.T) {
	mensagens, err := MontarMensagens(Pedido{
		Livro: "L", Pergunta: "p",
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
	if _, err := MontarMensagens(Pedido{Livro: "L", Pergunta: "   "}); err == nil {
		t.Fatal("pergunta vazia deveria dar erro")
	}
}
