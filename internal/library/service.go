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
type BookWriter interface {
	UpsertBook(book models.Book) (created bool, err error)
}

type Report struct {
	Total   int
	New     int
	Updated int
	Skipped int
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
// documentos pessoais ficam de fora da estante.
func (s *Service) Sync(ctx context.Context) (Report, error) {
	files, err := s.remote.List(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("listar B2: %w", err)
	}

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
		created, err := s.books.UpsertBook(models.Book{
			RemoteKey: file.Key,
			Title:     meta.Title,
			Author:    meta.Author,
			Category:  meta.Category,
			Format:    meta.Format,
			SizeBytes: file.Size,
			SHA1:      file.SHA1,
		})
		if err != nil {
			return report, fmt.Errorf("salvar %q: %w", file.Key, err)
		}
		if created {
			report.New++
		} else {
			report.Updated++
		}
	}
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
