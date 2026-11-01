package translate

import (
	"strings"
	"testing"
)

func TestMontarMensagens_MontaSistemaETrecho(t *testing.T) {
	mensagens, err := MontarMensagens("  enshittification  ")
	if err != nil {
		t.Fatalf("MontarMensagens erro = %v", err)
	}
	if len(mensagens) != 2 {
		t.Fatalf("mensagens = %d, want 2", len(mensagens))
	}
	if !strings.Contains(mensagens[0], "português do Brasil") || !strings.Contains(mensagens[0], "explicação") {
		t.Errorf("sistema = %q", mensagens[0])
	}
	if mensagens[1] != "enshittification" {
		t.Errorf("trecho = %q (deveria vir liso)", mensagens[1])
	}
}

func TestMontarMensagens_TruncaTrechoLongo(t *testing.T) {
	mensagens, err := MontarMensagens(strings.Repeat("a", LimiteTexto*2))
	if err != nil {
		t.Fatal(err)
	}
	if len(mensagens[1]) > LimiteTexto+3 {
		t.Errorf("trecho = %d caracteres, want <= %d", len(mensagens[1]), LimiteTexto)
	}
	if !strings.HasSuffix(mensagens[1], "…") {
		t.Errorf("truncado sem marca: %q", mensagens[1][len(mensagens[1])-5:])
	}
}

func TestMontarMensagens_Vazio(t *testing.T) {
	if _, err := MontarMensagens("   "); err != ErrTextoVazio {
		t.Fatalf("erro = %v, want ErrTextoVazio", err)
	}
}
