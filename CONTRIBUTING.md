# Contributing to TableForge

Thank you for contributing! TableForge (`github.com/bikky-kc013/TableForge`) is a Go port of phpPgAdmin — secure single-binary admin with zero frontend toolchain.

## Table of Contents
- [Getting Started](#getting-started)
- [Workflow](#workflow)
- [Structure](#structure)
- [Coding Guidelines](#coding-guidelines)
- [HTML / Templates](#html--templates)
- [Error Handling](#error-handling)
- [Testing](#testing)
- [Commits & PRs](#commits--prs)
- [Issues](#issues)
- [Security](#security)

## Getting Started

**Prereqs:** Go 1.26+, PostgreSQL 12–18 (CI 14–17), `pg_dump` optional.

```bash
git clone https://github.com/bikky-kc013/TableForge.git
cd TableForge
cp config/config.yaml config.yaml
go run ./cmd/server -config config/config.yaml -listen :8080
# http://localhost:8080/login
```

Docker:
```bash
docker build -t tableforge .
docker run -p 8080:8080 -v $PWD/config/config.yaml:/config/config.yaml tableforge
```

`config/config.yaml` replaces `config.inc.php` — see `internal/config/config.go`.

## Workflow

```bash
make build   # bin/tableforge
make run
make vet
make test    # -race -cover
make lint    # golangci-lint if installed
```

Templates live in `web/templates/*.html` (dev) **and** `internal/api/templates/*.html` (embedded). Change **both**:

```bash
cp web/templates/my.html internal/api/templates/my.html
go vet ./... && go test ./...
```

## Structure

```
cmd/server/main.go
internal/config/      YAML
internal/auth/        secretbox sessions + CSRF
internal/db/          pgxpool per (server,db,user) + Capability
internal/pgcatalog/   pg_catalog queries
internal/model/       structs + errors.go (ErrorResponse)
internal/dump/        pg_dump wrapper
internal/api/         chi router, handlers, helpers, errors
web/templates/ & web/static/
internal/api/templates/ embedded
themes/
```

## Coding Guidelines

- **Go:** `gofmt`/`go vet` clean, no `fmt.Println` in handlers.
- **SQL:** Always `$1` param; identifiers via `pgx.Identifier{}.Sanitize()` + allowlist vs `pg_catalog`. Never `fmt.Sprintf` raw input.
- **Auth:** Never read password from cookie; use `s.sessions.GetPassword(sess)`; queries via `mustPoolTyped` (user’s PG role).
- **Errors:** Never `http.Error(w, err.Error(), code)`. Use:

  ```go
  s.handleError(w, r, http.StatusInternalServerError, "Failed to list databases.", err)
  // API → JSON model.ErrorResponse, HTML → error.html with RequestID
  ```
  See `internal/model/errors.go`, `internal/api/errors.go`.

- **Naming:** `handleXPage` / `apiList*`; `internal/pgcatalog` stays `net/http`-free.

## HTML / Templates

**8 tabs required, always clickable, in order:**

`Browse | Structure | SQL | Search | Insert | Export | Import | Operations`

Never `disabled` + `href="#"`. Use fallback URLs when `{{.Table}}` empty:

```html
href="{{if .Table}}/browse/{{.Schema}}/{{.Table}}?database={{.Database}}{{else}}/tables?schema={{if .Schema}}{{.Schema}}{{else}}public{{end}}&database={{.Database}}{{end}}"
href="{{if .Table}}/search/{{.Schema}}/{{.Table}}?database={{.Database}}{{else}}{{if .Schema}}/search/{{.Schema}}?database={{.Database}}{{else}}/search?database={{.Database}}{{end}}{{end}}"
href="{{if .Table}}/insert/{{.Schema}}/{{.Table}}?database={{.Database}}{{else}}/insert/{{if .Schema}}{{.Schema}}{{else}}public{{end}}?database={{.Database}}{{end}}"
```

`SQL` must retain context: `href="/sql?database={{.Database}}{{if .Schema}}&schema={{.Schema}}{{end}}{{if .Table}}&table={{.Table}}{{end}}"`.

`Export`/`Import`: `{{if .Table}}href="/export/{{.Schema}}/{{.Table}}?database={{.Database}}"{{else}}href="/export?database={{.Database}}"{{end}}`.

Reference is `browse.html` — copy its `topbar/trail/pma-sidebar/pma-main/nav-tabs`.

Theme: `themes/global.css` + `themes/{{.Theme}}/global.css`, Bootstrap 5.3 via CDN; only `filterTree()` JS.

## Error Handling

- `isAPIRequest(r)` (`/api/*` or `Accept: application/json`) → JSON `ErrorResponse{code,error,message,details,request_id}`.
- HTML → `error.html` status-colored card, collapsible `Details`, `Go Back / Databases / Home`.
- `r.NotFound` / `r.MethodNotAllowed` set to `handleNotFound` / `handleMethodNotAllowed` in `router.go`.
- New handlers must use `s.handleError`.

## Testing

```bash
go test ./... -race -cover
go vet ./...
# integration PG 14-17
docker compose -f docker-compose.test.yml up --abort-on-container-exit --exit-code-from tests
```

Add test for new 8-tab fallback and `handleError` JSON/HTML branching.

## Commits & PRs

- Branch: `feat/<x>`, `fix/<x>`, `docs/<x>`
- Commits: `feat:`, `fix:`, `docs:`, `chore:` e.g., `fix: export csv streams with Content-Disposition`
- PR template: what/why, `go vet`/`go test` output, screenshots for UI, link issue. One feature per PR. Update both template trees and `config.yaml` docs if needed.

## Issues

- **Bug:** PG version, `config.yaml` (redacted), steps, expected vs actual, `RequestID`, logs.
- **Feature:** Use case + proposed UI/`pg_catalog` query.
- Search existing first.

## Security

See [`SECURITY.md`](SECURITY.md). **Do not** file public issues for RCE/`pg_dump` injection, credential leak, bypass — use GitHub Security Advisories privately.

## License

By contributing you agree **GPL-2.0+** (see `LICENSE`).
