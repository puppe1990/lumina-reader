// Package library deriva metadados dos livros a partir do caminho no B2 e
// sincroniza o balde com a estante local. Categoria/Autor/Livro/Arquivo.ext é o
// layout do acervo; arquivos soltos na categoria também são aceitos.
package library

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var supportedFormats = map[string]bool{
	"epub": true, "pdf": true, "mobi": true, "azw": true, "azw3": true,
	"djvu": true, "cbz": true, "cbr": true,
}

var (
	numericPrefix = regexp.MustCompile(`^\d{5,}[-_ ]+`)
	ebookPrefix   = regexp.MustCompile(`(?i)^e[\s-]?book[\s\-–:]*`)
	zlibSuffix    = regexp.MustCompile(`\s*\((?:z-library|z-lib)[^)]*\)`)
	dupSuffix     = regexp.MustCompile(`\s*\(\d+\)\s*$`)
	multiSpace    = regexp.MustCompile(`\s{2,}`)
)

type Meta struct {
	Title    string
	Author   string
	Category string
	Format   string
}

// Describe traduz a key remota (ex. "Categoria/Autor/Livro/Livro.epub") nos
// metadados da estante. Formatos fora do acervo (md, html, txt…) retornam erro.
func Describe(remoteKey string) (Meta, error) {
	partes := strings.Split(strings.Trim(remoteKey, "/"), "/")
	arquivo := partes[len(partes)-1]
	ext := filepath.Ext(arquivo)
	format := strings.ToLower(strings.TrimPrefix(ext, "."))
	if !supportedFormats[format] {
		return Meta{}, fmt.Errorf("formato não suportado: %q (key %q)", format, remoteKey)
	}

	meta := Meta{
		Title:    cleanTitle(strings.TrimSuffix(arquivo, ext)),
		Format:   format,
		Category: partes[0],
	}
	switch {
	case len(partes) >= 4:
		meta.Author = strings.TrimSpace(partes[1])
	case len(partes) == 3:
		meta.Author = strings.TrimSpace(partes[1])
	}
	if len(partes) == 1 {
		meta.Category = ""
	}
	return meta, nil
}

func cleanTitle(bruto string) string {
	titulo := strings.ReplaceAll(bruto, "_", " ")
	tinhaPrefixo := numericPrefix.MatchString(titulo)
	titulo = numericPrefix.ReplaceAllString(titulo, "")
	titulo = ebookPrefix.ReplaceAllString(titulo, "")
	// Nomes de download vêm TODO-COM-HIFENS; títulos reais com hífen preservam
	// os espaços ao redor e não passam por aqui.
	if tinhaPrefixo && !strings.Contains(titulo, " ") {
		titulo = strings.ReplaceAll(titulo, "-", " ")
	}
	titulo = zlibSuffix.ReplaceAllString(titulo, "")
	titulo = dupSuffix.ReplaceAllString(titulo, "")
	titulo = multiSpace.ReplaceAllString(titulo, " ")
	titulo = strings.TrimSpace(titulo)
	if titulo == "" {
		return strings.TrimSpace(bruto)
	}
	return titulo
}
