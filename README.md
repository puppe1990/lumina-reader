# Lumina

Leitor editorial do acervo pessoal em [Amarra](https://github.com/puppe1990/amarra-cais): a estante espelha um balde privado do Backblaze B2, os EPUBs são baixados e convertidos em capítulos limpos na primeira abertura, e a leitura acontece na página — temas (claro/sépia/crepúsculo/noite), tipografia ajustável, progresso salvo, destaques e caderno de notas.

Live: https://lumina.apps.gestaobem.com

## O que ele faz

- **Biblioteca** (`/`) — sincroniza o balde B2 para o SQLite, capa tipográfica por livro (tom determinístico), chips de filtro, hero "lendo agora" e sequência de leitura (dias + minutos do dia).
- **Leitura** (`/livros/{id}/cap/{n}`) — capítulos sanitizados do EPUB (bluemonday), imagens servidas do próprio arquivo (`/livros/{id}/recurso/...`), capitular, sumário, marcador, posição e pulso de tempo salvos no servidor.
- **Destaques** — seleção de texto abre a barra com 4 cores; o trecho vira `mark` no texto ao reabrir e aparece no **Caderno** (`/notas`) com "ir para o capítulo", edição e remoção.
- **PDF** — abre inline no navegador (`/livros/{id}/arquivo`) com download ao lado; o leitor web renderiza EPUB.
- **Coleções e busca** — navegação por categoria e busca por título/autor.

A biblioteca é privada: login por sessão (sem cadastro público). `Documentos Pessoais/` do balde fica fora da estante.

## Stack

- Go 1.26 (net/http stdlib) + Amarra Views + Drive
- Tailwind CSS 3.x (paleta do design "Lumina Editorial Reader")
- SQLite (`modernc.org/sqlite`, sem CGO)
- Backblaze B2 (API nativa: autorizar, listar, baixar) e parser EPUB próprio
- `github.com/microcosm-cc/bluemonday` para sanitizar o HTML dos capítulos

## Configuração

| Env                                                          | Para quê                                                              |
| ------------------------------------------------------------ | --------------------------------------------------------------------- |
| `B2_APPLICATION_KEY_ID` / `B2_APPLICATION_KEY` / `B2_BUCKET` | acervo no Backblaze (sem eles a estante não sincroniza)               |
| `OWNER_EMAIL` / `OWNER_PASSWORD`                             | cria o usuário inicial no primeiro boot em produção (não há cadastro) |
| `DB_PATH`                                                    | SQLite (o cache dos livros fica em `{dir do DB}/cache`)               |
| `APP_URL`, `ADMIN_TOKEN`, `ENV=production`                   | exigidos em produção pelo `cais.Validate()`                           |
| `CSP_STYLE_SRC` / `CSP_FONT_SRC`                             | Google Fonts (Literata, Inter, Material Symbols)                      |

Desenvolvimento usa o seed `demo@example.com` / `password`.

## Comandos

```bash
amarra-cais install   # npm install + go mod tidy
amarra-cais dev       # http://localhost:8080 (porta muda se ocupada)
make ci               # test + lint + format-check (gate completo)
amarra-cais test      # go test ./...
amarra-cais css       # Tailwind → web/static/css/styles.css
amarra-cais build     # bin/server
amarra-cais doctor    # checa amarra.js, PWA, paleta e assets
```

## Arquitetura

```
internal/b2/        cliente da API nativa do B2 (autoriza, lista, baixa) — testes com httptest
internal/library/   metadados derivados do caminho remoto + sincronização do balde
internal/epub/      parser EPUB (OPF/spine/nav) + sanitização e reescrita de imagens
internal/reader/    download para cache por SHA1 + capítulos no SQLite (parse na 1ª abertura)
internal/store/     SQLite: books, book_chapters, reading_progress, book_notes, reading_days
internal/handlers/  biblioteca, livro, leitor, caderno, coleções, busca, sincronizar, auth
web/templates/      layout + pages (Drive morphs #amarra-main) + partial "capa"
web/static/js/      amarra.js (framework) + lumina.js (leitor: tema, progresso, seleção)
```

O leitor usa navegação completa (`data-amarra-skip`), então o `lumina.js` hidrata a página no load: restaura tema/tamanho (localStorage), posição (servidor) e realça os destaques salvos casando o texto do trecho.

## Deploy

Cleat PaaS (runtime `golang`): `npm run build` (Tailwind) + `go build ./cmd/server`, assets em `web/static`, dados em `CLEAT_DATA_DIR` (`DB_PATH=/opt/lumina/data/app.db`). Segredos do B2 e o usuário inicial vivem nas envs do app no painel.

## Testes

TDD com `go test ./...` — domínio (`library`, `b2`, `epub`, `reader`), store em SQLite `:memory:` e handlers com `httptest`. Fixtures de livros/usuários usam `pkg/cais/fakedata` (gofakeit com seed): `fakedata.Seed(42)` deixa a sequência reprodutível.
