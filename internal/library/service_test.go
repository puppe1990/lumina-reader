package library

import (
	"context"
	"errors"
	"testing"

	"github.com/puppe1990/amarra-cais/pkg/cais/fakedata"

	"github.com/puppe1990/lumina-reader/internal/models"
)

type fakeLister struct {
	files []RemoteFile
	err   error
}

func (f *fakeLister) List(ctx context.Context) ([]RemoteFile, error) {
	return f.files, f.err
}

type memoryBooks struct {
	byKey  map[string]models.Book
	nextID int64
}

func newMemoryBooks() *memoryBooks {
	return &memoryBooks{byKey: map[string]models.Book{}}
}

func (m *memoryBooks) UpsertBook(book models.Book, vivas map[string]bool) (bool, error) {
	existente, ok := m.byKey[book.RemoteKey]
	if !ok {
		if book.SHA1 != "" && len(vivas) > 0 {
			for chave, antigo := range m.byKey {
				if antigo.SHA1 == book.SHA1 && !vivas[chave] {
					delete(m.byKey, chave)
					book.ID = antigo.ID
					m.byKey[book.RemoteKey] = book
					return false, nil
				}
			}
		}
		m.nextID++
		book.ID = m.nextID
		m.byKey[book.RemoteKey] = book
		return true, nil
	}
	book.ID = existente.ID
	m.byKey[book.RemoteKey] = book
	return false, nil
}

func (m *memoryBooks) PruneBooks(vivas map[string]bool) (int, error) {
	removidos := 0
	for chave := range m.byKey {
		if !vivas[chave] {
			delete(m.byKey, chave)
			removidos++
		}
	}
	return removidos, nil
}

func (m *memoryBooks) CountBooks() (int, error) { return len(m.byKey), nil }

func TestSync_AddsNewBooks(t *testing.T) {
	remoto := &fakeLister{files: []RemoteFile{
		{Key: "Ciencia e Natureza/Autora Um/Livro Um/Livro Um.epub", Size: 10, SHA1: "aaa"},
		{Key: "Filosofia e Ciencia/Livro Dois.pdf", Size: 20, SHA1: "bbb"},
	}}
	store := newMemoryBooks()
	servico := NewService(remoto, store)

	report, err := servico.Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync erro = %v", err)
	}
	if report.New != 2 || report.Total != 2 {
		t.Errorf("report = %+v, want Total 2 / New 2", report)
	}
	if len(store.byKey) != 2 {
		t.Fatalf("store tem %d livros, want 2", len(store.byKey))
	}
	salvo := store.byKey["Ciencia e Natureza/Autora Um/Livro Um/Livro Um.epub"]
	if salvo.Title != "Livro Um" || salvo.Author != "Autora Um" || salvo.Format != "epub" || salvo.SHA1 != "aaa" {
		t.Errorf("livro salvo = %+v", salvo)
	}
}

func TestSync_RenomeacaoConservaOLivroPeloSHA1(t *testing.T) {
	store := newMemoryBooks()
	remoto := &fakeLister{files: []RemoteFile{
		{Key: "Ciencia e Natureza/Autora Um/Livro Um/Livro Um.epub", Size: 10, SHA1: "sha-igual"},
	}}
	servico := NewService(remoto, store)
	if _, err := servico.Sync(context.Background()); err != nil {
		t.Fatalf("Sync erro = %v", err)
	}
	original := store.byKey["Ciencia e Natureza/Autora Um/Livro Um/Livro Um.epub"]

	// O acervo foi padronizado: mesma obra, chave nova e o mesmo SHA-1.
	remoto.files = []RemoteFile{
		{Key: "Ciencia e Natureza/Autora Um/Livro Um - Edicao Nova/Livro Um - Edicao Nova.epub", Size: 10, SHA1: "sha-igual"},
	}
	report, err := servico.Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync após renomear erro = %v", err)
	}
	if len(store.byKey) != 1 {
		t.Fatalf("store tem %d livros, want 1 (renomeação não pode duplicar)", len(store.byKey))
	}
	if report.New != 0 || report.Removed != 0 {
		t.Errorf("report = %+v, want New 0 / Removed 0", report)
	}
	novo, ok := store.byKey["Ciencia e Natureza/Autora Um/Livro Um - Edicao Nova/Livro Um - Edicao Nova.epub"]
	if !ok {
		t.Fatalf("livro não está na chave nova: %v", store.byKey)
	}
	if novo.ID != original.ID {
		t.Errorf("id mudou na renomeação: %d -> %d (progresso/favoritos seriam perdidos)", original.ID, novo.ID)
	}
}

func TestSync_PodaLivroQueSaiuDoBalde(t *testing.T) {
	store := newMemoryBooks()
	remoto := &fakeLister{files: []RemoteFile{
		{Key: "Cat/A/Fica.pdf", Size: 1, SHA1: "fica"},
		{Key: "Cat/A/Sai.pdf", Size: 1, SHA1: "sai"},
	}}
	servico := NewService(remoto, store)
	if _, err := servico.Sync(context.Background()); err != nil {
		t.Fatalf("Sync erro = %v", err)
	}
	if len(store.byKey) != 2 {
		t.Fatalf("store tem %d livros, want 2", len(store.byKey))
	}

	remoto.files = []RemoteFile{{Key: "Cat/A/Fica.pdf", Size: 1, SHA1: "fica"}}
	report, err := servico.Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync após remoção erro = %v", err)
	}
	if report.Removed != 1 {
		t.Errorf("report.Removed = %d, want 1", report.Removed)
	}
	if _, ok := store.byKey["Cat/A/Sai.pdf"]; ok {
		t.Errorf("livro removido do balde continua na estante")
	}
	if _, ok := store.byKey["Cat/A/Fica.pdf"]; !ok {
		t.Errorf("livro que continua no balde sumiu da estante")
	}
}

func TestSync_SkipsPersonalDocsAndUnsupported(t *testing.T) {
	remoto := &fakeLister{files: []RemoteFile{
		{Key: "Documentos Pessoais/laudo.pdf", Size: 1, SHA1: "x"},
		{Key: "Psicologia e Terapia/conversion-report.txt", Size: 1, SHA1: "y"},
		{Key: "Psicologia e Terapia/Livro Bom.pdf", Size: 1, SHA1: "z"},
	}}
	store := newMemoryBooks()
	servico := NewService(remoto, store)

	report, err := servico.Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync erro = %v", err)
	}
	if report.New != 1 || report.Skipped != 2 {
		t.Errorf("report = %+v, want New 1 / Skipped 2", report)
	}
	if _, ok := store.byKey["Documentos Pessoais/laudo.pdf"]; ok {
		t.Errorf("Documentos Pessoais não deveria entrar na biblioteca")
	}
}

func TestSync_ReportsUpdates(t *testing.T) {
	remoto := &fakeLister{files: []RemoteFile{
		{Key: "Cat/A/Livro.pdf", Size: 10, SHA1: "v1"},
	}}
	store := newMemoryBooks()
	servico := NewService(remoto, store)
	if _, err := servico.Sync(context.Background()); err != nil {
		t.Fatalf("primeiro Sync erro = %v", err)
	}

	if _, err := servico.Sync(context.Background()); err != nil {
		t.Fatalf("segundo Sync erro = %v", err)
	}
	if len(store.byKey) != 1 {
		t.Fatalf("store tem %d livros, want 1", len(store.byKey))
	}

	remoto.files[0].SHA1 = "v2"
	report, err := servico.Sync(context.Background())
	if err != nil {
		t.Fatalf("terceiro Sync erro = %v", err)
	}
	if report.New != 0 {
		t.Errorf("report.New = %d, want 0 numa nova sincronização sem livros novos", report.New)
	}
}

func TestSync_PropagatesListError(t *testing.T) {
	remoto := &fakeLister{err: errors.New("boom")}
	servico := NewService(remoto, newMemoryBooks())

	if _, err := servico.Sync(context.Background()); err == nil {
		t.Fatal("Sync deveria propagar o erro do B2")
	}
}

func TestSync_UsesFakedataLibrary(t *testing.T) {
	fakedata.Seed(7)
	remoto := &fakeLister{}
	for i := 0; i < 20; i++ {
		remoto.files = append(remoto.files, RemoteFile{
			Key:  "Sociedade e Cultura/" + fakedata.Name() + "/" + fakedata.Title() + ".epub",
			Size: int64(i * 100),
			SHA1: fakedata.Sentence(),
		})
	}
	store := newMemoryBooks()
	report, err := NewService(remoto, store).Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync erro = %v", err)
	}
	if report.New != 20 {
		t.Errorf("report.New = %d, want 20", report.New)
	}
	for _, livro := range store.byKey {
		if livro.Title == "" || livro.Author == "" {
			t.Errorf("livro sem metadados: %+v", livro)
		}
	}
}
