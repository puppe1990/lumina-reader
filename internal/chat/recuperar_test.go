package chat

import (
	"strings"
	"testing"
)

func TestSelecionarTrechos_EscolheOsQueFalamDaPergunta(t *testing.T) {
	unidades := []Trecho{
		{Rotulo: "página 1", Texto: "Introdução sobre café e atendimento."},
		{Rotulo: "página 2", Texto: "O autor discute a degradação das plataformas digitais e a extração de valor dos usuários."},
		{Rotulo: "página 3", Texto: "Receitas de bolo."},
	}
	trechos := SelecionarTrechos(unidades, "o que ele diz sobre degradação das plataformas?", 6)
	if len(trechos) != 1 {
		t.Fatalf("trechos = %d, want 1: %+v", len(trechos), trechos)
	}
	if trechos[0].Rotulo != "página 2" {
		t.Errorf("rótulo = %q", trechos[0].Rotulo)
	}
}

func TestSelecionarTrechos_IgnoraAcentoECaixa(t *testing.T) {
	unidades := []Trecho{
		{Rotulo: "capítulo 1", Texto: "A DEGRADAÇÃO das plataformas é o tema central."},
	}
	trechos := SelecionarTrechos(unidades, "degradacao", 6)
	if len(trechos) != 1 {
		t.Fatalf("trechos = %d, want 1", len(trechos))
	}
	if !strings.Contains(strings.ToLower(trechos[0].Texto), "plataformas") {
		t.Errorf("trecho = %q", trechos[0].Texto)
	}
}

func TestSelecionarTrechos_RespeitaOLimiteEOrdemPorPontos(t *testing.T) {
	unidades := []Trecho{
		{Rotulo: "p1", Texto: "starbucks uma vez"},
		{Rotulo: "p2", Texto: "starbucks starbucks starbucks starbucks"},
		{Rotulo: "p3", Texto: "starbucks duas vezes starbucks"},
		{Rotulo: "p4", Texto: "nada a ver"},
	}
	trechos := SelecionarTrechos(unidades, "starbucks", 2)
	if len(trechos) != 2 {
		t.Fatalf("trechos = %d, want 2", len(trechos))
	}
	if trechos[0].Rotulo != "p2" || trechos[1].Rotulo != "p3" {
		t.Errorf("ordem = %q, %q (want p2, p3)", trechos[0].Rotulo, trechos[1].Rotulo)
	}
}

func TestSelecionarTrechos_SemAcertoDevolveNada(t *testing.T) {
	unidades := []Trecho{{Rotulo: "página 1", Texto: "assunto completamente diferente"}}
	if trechos := SelecionarTrechos(unidades, "xilofone", 6); len(trechos) != 0 {
		t.Errorf("trechos = %+v, want vazio", trechos)
	}
	if trechos := SelecionarTrechos(unidades, "que como para", 6); len(trechos) != 0 {
		t.Errorf("pergunta só com palavras paradas deveria devolver vazio: %+v", trechos)
	}
}

func TestSelecionarTrechos_JanelaEmVoltaDoAcerto(t *testing.T) {
	enchimento := strings.Repeat("palavra ", 400)
	texto := enchimento + " ENSHITTIFICATION é o conceito " + enchimento
	unidades := []Trecho{{Rotulo: "capítulo 9", Texto: texto}}

	trechos := SelecionarTrechos(unidades, "enshittification", 1)
	if len(trechos) != 1 {
		t.Fatalf("trechos = %d", len(trechos))
	}
	recorte := trechos[0].Texto
	if len([]rune(recorte)) > MaxTrechoCaracteres+2 {
		t.Errorf("recorte grande demais: %d", len([]rune(recorte)))
	}
	if !strings.Contains(strings.ToLower(recorte), "enshittification") {
		t.Errorf("recorte sem o termo: %q", recorte)
	}
	if !strings.HasPrefix(recorte, "…") || !strings.HasSuffix(recorte, "…") {
		t.Errorf("recorte deveria marcar os cortes: %q…%q", recorte[:20], recorte[len(recorte)-20:])
	}
	if strings.HasPrefix(recorte, "…palavra ") == false && strings.HasPrefix(recorte, "… ") == false {
		t.Errorf("corte no meio de palavra: %q", recorte[:40])
	}
}
