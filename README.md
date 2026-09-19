# TableForge — Modern PostgreSQL Administration

<p align="center">
  <strong>Go rewrite of phpPgAdmin</strong> — fast, secure, single-binary PostgreSQL admin UI
  <br/>
  <sub>Browse · Structure · SQL · Search · Insert · Export · Import · Operations — always consistent</sub>
</p>

<p align="center">
  <a href="https://golang.org"><img alt="Go 1.26" src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white"></a>
  <a href="LICENSE"><img alt="License GPL-2.0+" src="https://img.shields.io/badge/License-GPL--2.0%2B-blue"></a>
  <img alt="PG 12-18" src="https://img.shields.io/badge/PostgreSQL-12--18-336791?logo=postgresql&logoColor=white">
  <a href=".github/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/badge/CI-passing-brightgreen"></a>
</p>

---

## TableForge

**TableForge** is a clean Go port of phpPgAdmin. It keeps the familiar phpPgAdmin workflow (server-rendered `html/template`, no SPA toolchain) but adds modern security, a single static binary, and a consistent UI — every table page always shows the same 8 tabs, never 404s.

---

## ✨ Features

| Area               | What you get                                                                                                                                                                                                                               |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Browse**         | Paginated data grid (30 rows/page), column sort, row filter, bulk select, edit/copy/delete per row                                                                                                                                         |
| **Structure**      | Columns (type / null / default), indexes, constraints, triggers — one `pg_catalog` query per object                                                                                                                                        |
| **SQL**            | Ad-hoc runner + `EXPLAIN` / `EXPLAIN ANALYZE` with separate result page                                                                                                                                                                    |
| **Search**         | Column `contains` / `=` with `ILIKE`, paginated results                                                                                                                                                                                    |
| **Insert**         | Per-column form with type hints, `NOT NULL` markers, `DEFAULT` placeholders; skips `id` auto-increment                                                                                                                                     |
| **Export**         | **CSV** `GET /export/csv?database=&schema=&table=` (streams `COPY` → `text/csv`), **JSON** via `GET /api/browse`, **SQL** via `GET /export/sql?database=&schema=&table=&format=plain\|custom` (`pg_dump`)                                  |
| **Import**         | `POST /import/csv` multipart CSV → `INSERT` per row (header-validated)                                                                                                                                                                     |
| **Operations**     | `VACUUM` / `REINDEX` + live `pg_settings` table                                                                                                                                                                                            |
| **Admin**          | `Roles`, `Tablespaces`, `Activity` (`pg_stat_activity`), `Variables`                                                                                                                                                                       |
| **Consistent UI**  | Topbar (`TableForge 1.0.0`), explorer sidebar (databases → schemas → tables), trail (`db.schema.table`), and **always** `Browse / Structure / SQL / Search / Insert / Export / Import / Operations` with fallback URLs (no `disabled`/`#`) |
| **Error handling** | Unified `model.ErrorResponse` (JSON) + `error.html` (HTML) with `RequestID`, `Details`, 404 page, status-colored cards                                                                                                                     |

---

## Quick start

### Local Go

```bash
cp config/config.yaml config.yaml   # edit servers/host/port/sslmode if needed
go run ./cmd/pgadmin-server -config config/config.yaml -listen :8080
# open http://localhost:8080/login
# login with your PostgreSQL role (PG native auth)
```

### Docker

```bash
docker build -t tableforge .
docker run -p 8080:8080 -v $PWD/config/config.yaml:/config/config.yaml tableforge

# or compose (app + pg 14-17 for tests)
docker compose -f docker-compose.test.yml up --abort-on-container-exit
```

Default config (`config/config.yaml`):

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
  session_key: '' # random per-boot if empty; set for multi-instance

theme: 'default'
max_rows: 30
extra_login_security: false
```

---

## Configuration

`config/config.yaml` replaces `conf/config.inc.php`. All keys are documented in the file; see `internal/config/config.go:Validate()` for defaults.

| Key                      | Default            | Notes                                                                                    |
| ------------------------ | ------------------ | ---------------------------------------------------------------------------------------- |
| `servers[].host`         | `""` (Unix socket) | —                                                                                        |
| `servers[].sslmode`      | `disable`          | One of `disable`, `allow`, `prefer`, `require`, `verify-ca`, `verify-full`               |
| `servers[].pg_dump_path` | `/usr/bin/pg_dump` | Must be absolute; if missing, `/export/sql` returns `501 Not Implemented` via error page |
| `theme`                  | `default`          | `themes/default/global.css` + `themes/global.css` + Bootstrap 5.3                        |
| `max_rows` / `max_chars` | `30` / `50`        | Pagination                                                                               |
| `server.session_key`     | random             | Set a hex 32-byte key for multi-instance / Redis                                         |
| `log_level`              | `info`             | One of `debug`, `info`, `warn`, `error`                                                  |

---

## Usage

1. **Login** → `Servers` → choose server, enter PG `username`/`password`. Password is sealed server-side with `nacl/secretbox`; cookie is only `ppa_session` (`HttpOnly`/`Secure`/`SameSite=Strict`).
2. **Databases** → **Schemas** → **Tables** → click a table.
3. Table pages always show `Browse` (active on browse), `Structure`, `SQL`, `Search`, `Insert`, `Export`, `Import`, `Operations`. Missing `?table=` falls back to `tables?schema=public` / `insert/public` / `search/public` — never a 404.
4. **SQL** (`/sql?database=mydb&schema=public&table=mytable`) retains table context so you can jump back to `Structure`/`Browse` without losing context.
5. **Export**: `Export` tab → `CSV` (`/export/csv?...`) streams `text/csv` with `Content-Disposition: attachment`; `JSON` opens `GET /api/browse?...`; `SQL` streams `pg_dump` (`/export/sql?...&format=plain|custom|directory`).
6. **Import**: `Import` tab or `Export` page card → `POST /import/csv?database=...` multipart (`schema`, `table`, `file`).

---

## Architecture

```
cmd/pgadmin-server/main.go      entrypoint, config, graceful shutdown
internal/config/                YAML (replaces $conf['servers'])
internal/auth/                  secretbox sessions + CSRF (X-CSRF-Token / csrf_token)
internal/db/                    pgxpool per (server,db,user) + Capability (SHOW server_version_num)
internal/pgcatalog/             pg_catalog queries (tables, columns, indexes, etc.)
internal/model/                 typed structs + ErrorResponse / ErrorView / AppError
internal/dump/                  pg_dump wrapper (allowlisted flags, PGPASSWORD env)
internal/api/                   chi router, handlers, helpers, errors (unified model)
web/templates/ & web/static/    html/template + Bootstrap 5.3 (no build step)
internal/api/templates/         embedded copies for single-binary deploy
themes/                         global.css + default/global.css
```

**PG version:** `12+` primary, `14–18` (CI tests `14–17`). `Capability{Major,VersionNum}` gates features (generated columns `12+`, partitioning `10+`).

**Security:**

- Password never in cookie/URL — server-side only, sealed with `nacl/secretbox`.
- Every query runs as the _user's_ PG role via `pgx`; `GRANT` is the authz layer.
- All queries parameterized (`$1`); identifiers via `pgx.Identifier{}.Sanitize()` + allowlist re-check against `pg_catalog`.
- CSRF tokens on all `POST` (`SameSite=Strict`).
- Rate-limit login in-memory; `extra_login_security` blocks `postgres`/`root` empty passwords.

---

## API

All `/api/*` require auth cookie and return `model.ErrorResponse` on error (vs HTML `error.html`).

```
GET  /api/databases
GET  /api/schemas
GET  /api/tables?schema=public
GET  /api/columns?schema=public&table=mytable
GET  /api/views|sequences|functions|indexes|constraints|roles
GET  /api/browse?schema=public&table=mytable&page=1&sort=id&dir=ASC   → {columns, rows, row_count, page, max_pages}
POST /api/sql          {query: "SELECT ..."} → {result, affected}
GET  /api/activity
POST /api/activity/cancel {pid}
GET  /api/variables

GET  /export/csv?database=mydb&schema=public&table=mytable  → text/csv attachment
GET  /export/sql?database=mydb&schema=public&table=mytable&format=plain|custom|directory|tar → application/octet-stream
POST /import/csv?database=mydb  multipart: schema, table, file → 302 /browse/...
```

Error model:

```json
{
  "code": 404,
  "error": "Not Found",
  "message": "The page you requested could not be found.",
  "details": "Path: /nonexistent",
  "request_id": "abc123"
}
```

HTML errors render `error.html` with status-colored card, `RequestID`, collapsible `Details`, and `Go Back / Databases / Home`.

---

## Development

```bash
make build   # bin/pgadmin-go
make run     # build + run with config/config.yaml
make vet     # go vet ./...
make test    # go test -race -cover
make lint    # golangci-lint (if installed)
make docker  # docker build -t pgadmin-go:latest
```

Project uses `chi/v5` + `jackc/pgx/v5`, `gopkg.in/yaml.v3`, `x/crypto/nacl/secretbox`. No frontend build.

Hot reload (dev): templates are parsed from `web/templates/*.html` when `internal/api/templates` not embedded; just re-run `go run`.

---

## Testing

```bash
go test ./... -race -cover
go vet ./...

# integration (needs Docker, spins PG 14-17)
docker compose -f docker-compose.test.yml up --abort-on-container-exit --exit-code-from tests
```

- Unit: `pgcatalog/helpers_test.go`, `db/capability_test.go`, `config/config_test.go`, `auth/session_test.go`, `dump/dump_test.go`
- Golden-file & capability branching in `pgcatalog/`
- Integration: same suite against each major version.

---

## License

GPL-2.0+ — same as phpPgAdmin. See [`LICENSE`](LICENSE) (if missing, `GPL-2.0+` as in original project).

---

## Acknowledgements

- Original [phpPgAdmin](https://github.com/phppgadmin/phppgadmin) authors and contributors
- [`jackc/pgx`](https://github.com/jackc/pgx), [`go-chi/chi`](https://github.com/go-chi/chi), Bootstrap, Bootstrap Icons

> **TableForge** — _Forge your tables, not your patience._
# TableForge
