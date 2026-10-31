package reader

import (
	"strings"
	"testing"

	"github.com/puppe1990/lumina-reader/internal/models"
)

func capitulosDeTeste(textos ...string) []models.Chapter {
	capitulos := make([]models.Chapter, len(textos))
	for i, texto := range textos {
		capitulos[i] = models.Chapter{Ordinal: i, Title: "Cap " + string(rune('A'+i)), HTML: "<p>" + texto + "</p>"}
	}
	return capitulos
}

func TestBuscarNoTexto_EncontraComSnippet(t *testing.T) {
	enchimento := strings.Repeat("palavras de contexto antes e depois do trecho. ", 10)
	caps := capitulosDeTeste(
		"A carruagem rodava compassada pelos paralelepípedos úmidos do Catete.",
		enchimento+"Pensei em Virgília, pensei no rigor dos velhos cânones sociais. "+enchimento,
	)
	hits := BuscarNoTexto("virgília", caps, 3, 10)
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1", len(hits))
	}
	if hits[0].ChapterOrdinal != 1 || !strings.Contains(hits[0].Snippet, "Virgília") {
		t.Errorf("hit = %+v", hits[0])
	}
	if !strings.HasPrefix(hits[0].Snippet, "…") || !strings.HasSuffix(hits[0].Snippet, "…") {
		t.Errorf("snippet deveria ter reticências nas duas pontas: %q", hits[0].Snippet)
	}
}

func TestBuscarNoTexto_CaseInsensitiveComAcento(t *testing.T) {
	caps := capitulosDeTeste("MEMÓRIA póstuma de um defunto autor.")
	hits := BuscarNoTexto("memória", caps, 3, 10)
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1 (busca deveria ignorar caixa)", len(hits))
	}
}

func TestBuscarNoTexto_LimitaPorCapituloETotal(t *testing.T) {
	texto := strings.Repeat("palavra e mais palavra. ", 20)
	caps := capitulosDeTeste(texto, texto, texto)
	hits := BuscarNoTexto("palavra", caps, 2, 5)
	if len(hits) != 5 {
		t.Fatalf("hits = %d, want 5 (maxHits)", len(hits))
	}
	porCapitulo := map[int]int{}
	for _, hit := range hits {
		porCapitulo[hit.ChapterOrdinal]++
	}
	for ordinal, total := range porCapitulo {
		if total > 2 {
			t.Errorf("capítulo %d tem %d hits, want <= 2", ordinal, total)
		}
	}
}

func TestBuscarNoTexto_TermoCurtoOuAusente(t *testing.T) {
	caps := capitulosDeTeste("texto qualquer")
	if hits := BuscarNoTexto("a", caps, 3, 10); hits != nil {
		t.Errorf("termo de 1 letra não busca: %+v", hits)
	}
	if hits := BuscarNoTexto("inexistente", caps, 3, 10); hits != nil {
		t.Errorf("sem match deveria ser nil: %+v", hits)
	}
}

func TestBuscarNoTexto_IgnoraMarkup(t *testing.T) {
	caps := []models.Chapter{{Ordinal: 0, Title: "Cap", HTML: `<p>com <em>ênfase</em> e <a href="#">link</a></p>`}}
	hits := BuscarNoTexto("ênfase e link", caps, 3, 10)
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1 — o texto plano deve atravessar markup inline", len(hits))
	}
}
