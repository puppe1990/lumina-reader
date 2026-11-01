package library

import (
	"context"
	"fmt"
	"strings"

	"github.com/puppe1990/lumina-reader/internal/models"
)

// RemoteFile é o recorte do B2 que a sincronização precisa.
type RemoteFile struct {
	Key  string
	Size int64
	SHA1 string
}

// Lister lista o balde remoto (implementado por internal/b2).
type Lister interface {
	List(ctx context.Context) ([]RemoteFile, error)
}

// BookWriter grava/atualiza um livro na estante (implementado por internal/store).
// vivas é o conjunto de chaves presentes na listagem atual: o store usa para
// reconhecer renomeações (mesmo SHA-1, chave antiga fora do balde) e para podar
// o que saiu do acervo.
type BookWriter interface {
	UpsertBook(book models.Book, vivas map[string]bool) (created bool, err error)
	PruneBooks(vivas map[string]bool) (removed int, err error)
}

type Report struct {
	Total   int
	New     int
	Updated int
	Skipped int
	Removed int
}

type Service struct {
	remote       Lister
	books        BookWriter
	skipPrefixes []string
}

func NewService(remote Lister, books BookWriter) *Service {
	return &Service{
		remote:       remote,
		books:        books,
		skipPrefixes: []string{"Documentos Pessoais/"},
	}
}

// Sync espelha o balde no banco local; arquivos fora dos formatos de livro e
// documentos pessoais ficam de fora da estante. Livros que saíram do balde são
// removidos da estante; renomeações conservam a linha (e o progresso) pelo SHA-1.
func (s *Service) Sync(ctx context.Context) (Report, error) {
	files, err := s.remote.List(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("listar B2: %w", err)
	}

	type candidato struct {
		file RemoteFile
		meta Meta
	}
	var visiveis []candidato
	vivas := make(map[string]bool, len(files))
	var report Report
	for _, file := range files {
		report.Total++
		if s.skip(file.Key) {
			report.Skipped++
			continue
		}
		meta, err := Describe(file.Key)
		if err != nil {
			report.Skipped++
			continue
		}
		visiveis = append(visiveis, candidato{file: file, meta: meta})
		vivas[file.Key] = true
	}

	for _, c := range visiveis {
		created, err := s.books.UpsertBook(models.Book{
			RemoteKey: c.file.Key,
			Title:     c.meta.Title,
			Author:    c.meta.Author,
			Category:  c.meta.Category,
			Format:    c.meta.Format,
			SizeBytes: c.file.Size,
			SHA1:      c.file.SHA1,
		}, vivas)
		if err != nil {
			return report, fmt.Errorf("salvar %q: %w", c.file.Key, err)
		}
		if created {
			report.New++
		} else {
			report.Updated++
		}
	}

	removed, err := s.books.PruneBooks(vivas)
	if err != nil {
		return report, fmt.Errorf("podar estante: %w", err)
	}
	report.Removed = removed
	return report, nil
}

func (s *Service) skip(key string) bool {
	for _, prefix := range s.skipPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}
