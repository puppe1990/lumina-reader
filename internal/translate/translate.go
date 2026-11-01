// Package translate traduz o trecho que o leitor selecionou (palavra, frase ou
// parágrafo) para português do Brasil usando o mesmo provedor do chat.
package translate

import (
	"errors"
	"strings"
)

const (
	// LimiteTexto é o teto do trecho selecionado (um parágrafo já é bastante).
	LimiteTexto = 1200
	// IdiomaAlvo é o destino fixo do app.
	IdiomaAlvo = "português do Brasil"
)

var ErrTextoVazio = errors.New("selecione um trecho para traduzir")

// MontarMensagens monta a conversa de tradução: instruções curtas + o trecho.
// O modelo devolve só a tradução (e uma explicação de uma linha quando for um
// termo isolado), sem repetir o texto original.
func MontarMensagens(texto string) ([]string, error) {
	limpo := strings.TrimSpace(texto)
	if limpo == "" {
		return nil, ErrTextoVazio
	}
	limpo = truncar(limpo, LimiteTexto)
	sistema := "Você é um tradutor do Lumina. Traduza o trecho do leitor para " + IdiomaAlvo + ".\n" +
		"Responda apenas com a tradução, sem repetir o original e sem comentários.\n" +
		"Se for uma palavra ou expressão curta, traduza e acrescente entre parênteses uma explicação de até 12 palavras quando isso ajudar a entender o termo."
	return []string{sistema, limpo}, nil
}

func truncar(texto string, limite int) string {
	if len(texto) <= limite {
		return texto
	}
	return strings.TrimSpace(texto[:limite]) + "…"
}
