<!-- Thanks for contributing to TableForge! -->
## What / Why
<!-- Fixes # -->
## How to test
- [ ] `go vet ./...` / `go test -race` pass
- [ ] 8 tabs always clickable (no disabled/#), fallback to `tables?schema=public` etc.
- [ ] Error pages: `/nonexistent` → 404 HTML, `Accept: json` → JSON `{"code":404}`
- [ ] `web/templates` + `internal/api/templates` both updated
## Screenshots
## Checklist
- [ ] `s.handleError` used (no raw `http.Error`)
- [ ] `pg_dump` allowlisted, `PGPASSWORD` env
