# Security Policy

## Supported Versions
| Version | Supported |
|---------|-----------|
| `main` (TableForge 1.x) | ✅ |
| phpPgAdmin legacy | ❌ |

PostgreSQL 12–18 tested; older not supported.

## Reporting a Vulnerability
**Do not open a public issue** for credential leak, RCE via `pg_dump`, CSRF/session bypass, privilege escalation.

Use **GitHub Security → Report a vulnerability** (private) or email owners. Include: PG version, `config.yaml` (redacted), steps, RequestID, impact. Acknowledgement in 48h, fix aim 14 days.

## What We Do
- Passwords server-side only, `nacl/secretbox` (key `server.session_key`), never logged.
- All queries `pgx` parameterized `$1` + `pgx.Identifier{}.Sanitize()` + allowlist.
- `pg_dump` wrapper (`internal/dump`) allowlists flags; password via `PGPASSWORD` env.
- CSRF (`X-CSRF-Token`/`csrf_token`, `SameSite=Strict`), rate-limit login.

## Disclosure
After fix, GHSA/CVE published, reporter credited unless anonymous.

## Questions
Open a `question` issue for non-sensitive topics.
