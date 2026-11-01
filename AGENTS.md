# lumina — AI Conventions

Primary reader is often an LLM agent. Prefer small greps, small modules, and headless tests.

## App-specific (Lumina)

Leitor editorial do acervo B2 (`ebooks-macbook-air-puppe`). Mapa curto:

| Path                               | Papel                                                                                                                                                                                                                                                         |
| ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/b2/`                     | API nativa do B2 (autorizar/listar/baixar); testes usam `httptest`, nunca a rede real                                                                                                                                                                         |
| `internal/library/`                | `Describe` deriva título/autor/categoria da key; `Service.Sync` espelha o balde (ignora `Documentos Pessoais/`), re-chaveia renomeações pelo SHA-1 (progresso/favoritos/capas sobrevivem) e poda o que saiu do balde                                          |
| `internal/epub/`                   | parser EPUB (container/OPF/spine/nav) + bluemonday; imagens viram `/livros/{id}/recurso/...`                                                                                                                                                                  |
| `internal/chat/`                   | "conversar com o livro": monta a conversa (system com livro/posição + trecho atual + histórico curto, com truncamentos) e fala com provedor OpenAI-compatível (`LUMINA_CHAT_*`, NVIDIA NIM por padrão); sem chave responde 503 amigável                       |
| `internal/reader/`                 | cache em disco por SHA1 + capítulos no SQLite; parse só na primeira abertura                                                                                                                                                                                  |
| `internal/store/`                  | tabelas `books`, `book_chapters`, `reading_progress`, `book_notes`, `reading_days`                                                                                                                                                                            |
| `internal/handlers/`               | biblioteca, livro (detalhe/arquivo/recurso), leitor (capítulo/progresso/pulso/marcador), caderno, coleções, busca, sincronizar                                                                                                                                |
| `web/static/js/lumina.js`          | hidrata o leitor: tema/tamanho (localStorage), posição, pulso, seleção → destaque, realce de notas                                                                                                                                                            |
| `web/static/js/chat-livro.js`      | diálogo "conversar com o livro" (partial `chat.html`): manda pergunta + trecho atual (`.leitor-texto` no EPUB, `window.luminaPdf.contexto()` no PDF) e mantém o histórico da sessão                                                                           |
| `web/static/js/pdf-reader.js`      | leitor de PDF (pdf.js vendorizado em `web/static/vendor/pdfjs/`, Apache-2.0): páginas na horizontal (uma por tela, transform no trilho; swipe, toque nas bordas, setas e ←/→), tema (Noite inverte o canvas), progresso por página, marcador e busca no texto |
| `web/templates/pages/pdf.html`     | página do leitor de PDF (`data-pdf-root`); o progresso grava `chapter_ordinal = página-1` e `books.pdf_pages` via `SaveProgressPages`                                                                                                                         |
| `web/templates/partials/capa.html` | capa do livro: arte real (`/livros/{id}/capa`, cacheada em `book_covers`) sobre a capa tipográfica; sem capa o JS remove a `<img>` e o tom + título aparecem. Classes `capa-tom-*` e `nota-*` são montadas no runtime e vivem no `safelist` do Tailwind       |

Capas: `epub.Cover` acha a arte (properties=cover-image → meta name=cover → href "cover" → 1ª imagem da spine); `reader.Cover` normaliza para JPEG ≤720×1080 (arte de 4 MB vira ~80 KB) e cacheia; PDF usa `pdftoppm` (poppler-utils instalado pelo `release_command` do `.cleat_deploy/deploy.json`, porque o builder golang do Cleat ignora `runtime_apt_packages`). Sem a ferramenta o 404 não é persistido como "missing".

Regras que não se quebram: o texto do capítulo chega por `template.HTML`/`template.JS` (sanitizado antes), o leitor navega com `data-amarra-skip` (página completa), e o dia de leitura é sempre `hojeBRT` (America/São Paulo fixo em -3). Credenciais B2 só via env (`B2_APPLICATION_KEY_ID`/`B2_APPLICATION_KEY`/`B2_BUCKET`) — nunca commitar.

## Rule #1: TDD is mandatory

Before writing production code:

1. Write the test (`*_test.go`)
2. Run focused: `go test ./... -v -run TestName`
3. Confirm it **fails** for the right reason
4. Write the **minimal** code to pass
5. Run `amarra-cais test`

## Clean code for agents

| Priority | Rule                                                                                                   |
| -------- | ------------------------------------------------------------------------------------------------------ |
| 1        | **Small units** — functions ~4–20 lines; files target 200–300 lines, hard cap ~500                     |
| 2        | **SRP** — one reason to change per file/package                                                        |
| 3        | **Greppable names** — unique domain nouns; avoid `data`, `handler`, `Manager`, `util` as primary names |
| 4        | **Comments = WHY** — security, SQLite, CSRF/cookie, Drive vs full HTML. No narrating WHAT              |
| 5        | **Inject deps** — handlers take `Store`, `*view.Renderer`, `cais.Config` via constructor               |
| 6        | **Early returns** — max ~2 nesting levels                                                              |
| 7        | **Errors with values** — `fmt.Errorf("...: %w", err)`                                                  |
| 8        | **Headless tests** — SQLite `:memory:`; no manual seed for unit tests                                  |

## Demo data

Seeds and fixtures come from `pkg/cais/fakedata` (`Name`, `Title`, `Email`, `Sentence`, `URL`, `Date`) — `fakedata.Seed(42)` pins a reproducible sequence for tests.

## Layout

| Path                        | Responsibility                 |
| --------------------------- | ------------------------------ |
| `cmd/server/`               | Entry point                    |
| `internal/app/`             | Bootstrap, `registerRoutes`    |
| `internal/handlers/`        | HTTP handlers (`view.Write`)   |
| `internal/store/`           | SQLite + migrations            |
| `internal/models/`          | Domain structs                 |
| `web/templates/layouts/`    | Amarra layout (`#amarra-main`) |
| `web/templates/pages/`      | HTML pages                     |
| `web/templates/components/` | App component overrides        |
| `web/static/`               | CSS, `amarra.js`, PWA          |

Patch markers (do not remove): `registerRoutes`, `Close() error`, `<!-- cais:nav -->`, `// cais:live-views`.

## Amarra HTML

Handlers render HTML via `view.Write`:

```go
view.Write(w, r, h.views, view.Page{
  Layout: "app",
  Name:   "dashboard",
  Data: map[string]any{
    "Title":     h.catalog.T("dashboard.title"),
    "Site":      meta.ForRequest(h.site, r),
    "CSRFToken": csrf.TokenFromRequest(r),
    "Flash":     flashMsg,
  },
}, h.cfg)

// Validation — same page, status 422, `.Errors` on inputs
// Flash on redirect — cais cookie API only
flash.Set(w, "notice", "Saved!", cfg.CookieSecure())
http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
```

Pages define a content block plus kit tags `<.form>` / `<.input>` / `<.button>` / `<.flash />` / `<.locale-toggle />`.
Kit attributes interpolate: `<.stat label="Potência" value="{{ .Power }} kWp" />` renders `5 kWp`; a control action in an attribute value fails at boot.
Templates load once at boot (`view.Load`): `layouts/*.html`; `pages/*.html` plus `pages/*/*.html` — `pages/blog/post.html` is `view.Page{Name: "blog/post"}`; `partials/*.html` and `components/*.html` are **flat only** (a nested partial never loads), and an unknown `<.x>` fails at boot.
Designed 404: register `r.NotFound(handler)` in `internal/app/routes.go` — it also serves path params that fail to parse (`IntParam`, `StringParam`); render with `writeView(..., http.StatusNotFound)`.
Drive morphs `#amarra-main` by default — plain links/forms and `linkTo` need no attribute; opt out with `data-amarra-skip` (#31). Do not check `HX-Request`.
Session-state forms (login, signup, logout) must change the `data-amarra-shell` marker on `<body>` — the layout emits `auth`/`app` from `.Site.LoggedIn`. Drive compares it with the response and does a full navigation instead of leaving the signed-in page inside the anonymous shell (#249).
Fullbleed pages (own header/footer/sidebar, e.g. marketing landing): Drive only swaps `#amarra-main`, so page-owned chrome outside it goes stale on Drive navigation — keep that chrome **inside** `#amarra-main` (wrap the whole page; the layout emits no chrome for those pages via `ActiveNav`/a second layout, #66). Escape hatch: `data-amarra-skip` (#31).
Inline page scripts run on a full page load only — they do **not** re-run on a Drive morph into that page (idiomorph does not execute injected `<script>`, and the per-request CSP nonce makes re-running unsafe, #325). Put behavior that must survive Drive navigation in a static script loaded by the layout (`web/static/js/*.js`). A node with the same `id` in both responses is the same DOM node after a morph: JS listeners and expando properties survive, while `value`/attributes come from the new HTML and JS-only `data-*` attributes are removed. Do not use `dataset.ready` as an init guard — the morph deletes the attribute. Prefer event delegation on `document` registered once behind a `window` flag, or tear down before re-init (`removeEventListener`, `disconnect`, `cloneNode`); never re-run `addEventListener` on the reused node, and reinit on `amarra:morphed` only after teardown (#369).
Password fields: `<.password name="password" />` kit (eye toggle overlaid inside the field via `cais-password-wrap`; wired to `amarra-hook="password"`; `fieldPassword` stays as the Go form-builder path).
Shipped hooks: `bulk` (select-all: `amarra-hook="bulk"` + `data-amarra-bulk-all`/`-row`/`-bar`), `clipboard`, `dialog` (native `<dialog>` via `amarra-hook="dialog"` + `data-amarra-dialog-open`/`-target`/`-close`; `<.modal>` renders the target), `dropdown` (row actions: `amarra-hook="dropdown"` + `data-amarra-dropdown-button`/`-menu`), `nav` (re-sync active link after Drive morph: `amarra-hook="nav"` + `data-amarra-nav-on`/`-off` on the container), `password`, `reveal` (client show/hide, no Drive round-trip), `select` (search-by-default combobox: the native `<select>` stays for value/`required`/iOS but is clipped, and a trigger + filtered listbox is injected, writing the choice back and firing `change`; kit `<.select>`/`fieldSelect` emit `amarra-hook="select"` and bare `<select>` is auto-enhanced, while an explicit `amarra-hook` keeps it native; coarse pointers keep the OS picker; opt out with `data-amarra-select-search="false"`), `sidebar` (`amarra-hook="sidebar"` on the drawer button + `data-amarra-sidebar-target="#amarra-nav"`: toggles `data-amarra-sidebar-open` on the panel, `aria-expanded`, Esc and outside-click close), `theme` (`html.light` + `localStorage["amarra-theme"]`, per-element via `data-amarra-theme-key` / `-class` / `-color` / `-on-label` / `-off-label`).
The scaffold ships the toggle in the sidebar and the palette in CSS variables (`input.css`: `--amarra-ink`/`-foam`/`-copper`/`-tide`, `:root` dark + `html.light`), read by `tailwind.config.js` as `rgb(var(--amarra-*) / <alpha-value>)` — re-theme through the variables so every kit class follows both themes (#258). `tailwind.config.js` also scans the shipped kit inside the Go module (`shippedKitContent()` via `go list -m`); keep that entry or Tailwind prunes the kit-only utilities and input/button/flash render unstyled (#372).
Theme FOUC snippet belongs in the layout `<head>` before CSS:

```html
<script nonce="{{ .CSPNonce }}">
  try {
    if (localStorage.getItem("amarra-theme") === "light")
      document.documentElement.classList.add("light");
  } catch (e) {}
</script>
```

`view.Write` injects `.CSPNonce` into `map[string]any` data. Inline `<script>` needs `nonce="{{ .CSPNonce }}"`; unsigned inline is `CSP_SCRIPT_SRC='unsafe-inline'` (#263).

Parse bodies with `httpx.ParseFormOrJSON`.

## Auth, CSRF, flash

- Session middleware: `LoadSession` + `Flash` + `CSRF(cfg)`
- Protect routes: `middleware.RequireAuth("/login")` / `RequireAuthFunc`
- CSRF: double-submit cookie `cais_csrf` + form field or `X-CSRF-Token`
- Flash: **only** `flash.Set` + read via `flash.MessageFromRequest`
- Dev demo user (when seeded): `demo@example.com` / `password`

## New page / resource

```bash
amarra-cais g handler settings     # handler + test + web/templates/pages/settings.html + route
amarra-cais g page about           # HTML page only
amarra-cais g component input      # override a kit component with its shipped markup (--list shows them)
amarra-cais g resource bookmark --fields title:string,url:url,notes:text?
amarra-cais g model tag --fields name:string
amarra-cais g migration add_notes
amarra-cais g live counter         # WebSocket Live view + /live/counter
amarra-cais g stream chat --live   # chat with Live WS (SSE is the default)
amarra-cais g auth                 # if app was --blank/--minimal
amarra-cais db migrate
```

Or by hand:

1. Go test in `internal/handlers/`
2. HTML page in `web/templates/pages/`
3. Handler + route in `internal/app/routes.go`

## Commands

```bash
amarra-cais install          # npm + go mod tidy (+ Tailwind build)
amarra-cais dev              # air + tailwind watch
amarra-cais test             # go test ./...
make ci                     # test + lint + format-check
amarra-cais doctor [--mobile]
amarra-cais routes
amarra-cais db migrate | status | rollback | seed
amarra-cais jobs work | status
```

`GET /jobs` — localhost queue dashboard (heartbeats, retry/discard, prune, `?kind=`). Production: SSH tunnel.

## Do not

- Parse templates per request (`view.Load` once at boot)
- Use inline CSS (Tailwind classes)
- Mock the database (use SQLite `:memory:`)
- Grow files past ~500 lines without splitting
- Ship features without a headless test
