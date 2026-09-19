# TableForge — Modern PostgreSQL Administration

<p align="center">
  <strong>Go rewrite of phpPgAdmin</strong> — fast, secure, single-binary PostgreSQL admin UI<br/>
  <sub>Browse · Structure · SQL · Search · Insert · Export · Import · Operations — always consistent</sub>
</p>

<p align="center">
  <a href="https://golang.org"><img alt="Go 1.26" src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white"></a>
  <a href="LICENSE"><img alt="License GPL-2.0+" src="https://img.shields.io/badge/License-GPL--2.0%2B-blue"></a>
  <img alt="PG 12-18" src="https://img.shields.io/badge/PostgreSQL-12--18-336791?logo=postgresql&logoColor=white">
  <a href=".github/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/badge/CI-passing-brightgreen"></a>
</p>

---

## Why TableForge?

**TableForge** (`github.com/bikky-kc013/TableForge`) is a clean Go port of [phpPgAdmin](https://github.com/phppgadmin/phppgadmin). It keeps the familiar server-rendered workflow (`html/template`, no SPA build) but adds modern security, a single static binary, and a consistent UI — every table page always shows the same 8 tabs, never 404s.

> `phpPgAdmin` files at the repo root are kept as a functional spec. TableForge is *not* a line-by-line port; it rebuilds the feature set with idiomatic Go, `pgx` and `chi`.

## ✨ Features

| Area | What you get |
|------|--------------|
| **Browse** | Paginated grid (30 rows/page), sort, filter, bulk select, edit/copy/delete |
| **Structure** | Columns, indexes, constraints, triggers — one `pg_catalog` query per object |
| **SQL** | Ad-hoc runner + `EXPLAIN` / `EXPLAIN ANALYZE` |
| **Search** | Column `contains` / `=` (`ILIKE`), paginated results |
| **Insert** | Per-column form, `NOT NULL` markers, `DEFAULT` placeholders; skips `id` auto-increment |
| **Export** | **CSV** `GET /export/csv?database=&schema=&table=` (`COPY` → `text/csv`), **JSON** `GET /api/browse`, **SQL** `GET /export/sql?database=&schema=&table=&format=plain\|custom` (`pg_dump`) |
| **Import** | `POST /import/csv` multipart CSV → validated `INSERT` |
| **Operations** | `VACUUM` / `REINDEX` + live `pg_settings` |
| **Admin** | Roles, Tablespaces, Activity (`pg_stat_activity`), Variables |
| **Consistent UI** | Topbar (`TableForge 1.0.0`), explorer sidebar, trail `db.schema.table`, always `Browse / Structure / SQL / Search / Insert / Export / Import / Operations` with fallback URLs |
| **Errors** | Unified `model.ErrorResponse` (JSON) + `error.html` (HTML) with `RequestID`, 404 page |

## Quick Start

### Local Go

```bash
cp config/config.yaml config.yaml   # edit servers/host/port
go run ./cmd/server -config config/config.yaml -listen :8080
# open http://localhost:8080/login  (use your PG role)
```

### Docker

```bash
docker build -t tableforge .
docker run -p 8080:8080 -v $PWD/config/config.yaml:/config/config.yaml tableforge
# compose with PG 14-17 for tests
docker compose -f docker-compose.test.yml up --abort-on-container-exit
```

Default `config/config.yaml`:

```yaml
servers:
  - desc: 'PostgreSQL'
    host: 'localhost'
    port: 5432
    sslmode: 'disable'
    defaultdb: 'postgres'
    pg_dump_path: '/usr/bin/pg_dump'

server:
  listen: ':8080'
  session_key: ''   # random per-boot if empty

theme: 'default'
max_rows: 30
extra_login_security: false
```

## Configuration

`config/config.yaml` replaces `conf/config.inc.php`. See `internal/config/config.go:Validate()`.

| Key | Default | Notes |
|-----|---------|-------|
| `servers[].host` | `""` (socket) | `sslmode: disable|allow|prefer|require|verify-ca|verify-full` |
| `servers[].pg_dump_path` | `/usr/bin/pg_dump` | Must be absolute; if missing `/export/sql` returns `501` |
| `theme` | `default` | `themes/default/global.css` + Bootstrap 5.3 |
| `max_rows` / `max_chars` | `30` / `50` | Pagination |
| `server.session_key` | random | 32-byte hex for multi-instance |
| `log_level` | `info` | `debug|info|warn|error` |

## Usage

1. **Login** → `Servers` → choose server, enter PG `username`/`password` (sealed server-side with `nacl/secretbox`; cookie `ppa_session` `HttpOnly/Secure/SameSite=Strict`).
2. **Databases** → **Schemas** → **Tables** → table.
3. Table pages always show 8 tabs; missing `?table=` falls back to `tables?schema=public` / `insert/public` / `search/public` — never 404.
4. **SQL** (`/sql?database=mydb&schema=public&table=mytable`) retains table context.
5. **Export**: `CSV` (`/export/csv?...`) streams `text/csv`; `JSON` via `/api/browse?...`; `SQL` streams `pg_dump` (`/export/sql?...&format=plain|custom|directory`).
6. **Import**: `POST /import/csv?database=...` multipart (`schema`, `table`, `file`).

## Architecture

```
cmd/server/main.go          entrypoint, config, graceful shutdown
internal/config/            YAML
internal/auth/              secretbox sessions + CSRF
internal/db/                pgxpool per (server,db,user) + Capability
internal/pgcatalog/         pg_catalog queries
internal/model/             structs + ErrorResponse / ErrorView / AppError
internal/dump/              pg_dump wrapper (allowlisted, PGPASSWORD env)
internal/api/               chi router, handlers, errors (unified)
web/templates/ & web/static/themes/  html/template + Bootstrap 5.3
internal/api/templates/     embedded for single binary
```

**PG version:** `12+` (CI `14–17`). `Capability{Major,VersionNum}` gates features.

**Security:** password server-side only; queries as user's PG role; parameterized `$1` + `pgx.Identifier{}.Sanitize()` + allowlist; CSRF on `POST`; rate-limit login.

## API

All `/api/*` require auth cookie; errors are `model.ErrorResponse`:

```json
{"code":404,"error":"Not Found","message":"The page you requested could not be found.","details":"Path: /x","request_id":"abc123"}
```

```
GET  /api/databases
GET  /api/schemas
GET  /api/tables?schema=public
GET  /api/columns?schema=public&table=mytable
GET  /api/views|sequences|functions|indexes|constraints|roles
GET  /api/browse?schema=public&table=mytable&page=1  → {columns,rows,row_count}
POST /api/sql          {query:"SELECT ..."} → {result,affected}
GET  /api/activity /variables
POST /api/activity/cancel {pid}
GET  /export/csv?database=mydb&schema=public&table=mytable → text/csv
GET  /export/sql?database=mydb&schema=public&table=mytable&format=plain → octet-stream
POST /import/csv?database=mydb  multipart
```

HTML errors render `error.html` (status-colored card, `RequestID`, collapsible `Details`).

## Development

```bash
make build   # bin/tableforge
make run     # build + run with config/config.yaml
make vet     # go vet ./...
make test    # go test -race -cover
make lint    # golangci-lint
make docker  # docker build -t tableforge:latest
```

No frontend build. Templates live in `web/templates/*.html` (dev) and embedded `internal/api/templates/*.html` — change **both**.

## Testing

```bash
go test ./... -race -cover
go vet ./...
# integration (ephemeral PG)
docker compose -f docker-compose.test.yml up --abort-on-container-exit --exit-code-from tests
```

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md) — dev setup, 8-tab rule, error model, PR checklist.

## License

GPL-2.0+ — same as phpPgAdmin. See [`LICENSE`](LICENSE).

## Acknowledgements

- Original [phpPgAdmin](https://github.com/phppgadmin/phppgadmin) authors
- [`jackc/pgx`](https://github.com/jackc/pgx), [`go-chi/chi`](https://github.com/go-chi/chi), Bootstrap & Bootstrap Icons

> **TableForge** — *Forge your tables, not your patience.*
