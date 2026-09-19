# migrations

phpPgAdmin is stateless against the target PG servers — no app metadata DB required.
If you add saved-query history, per-user settings, or audit log, store them here.

Example (if you choose a Postgres metadata store):

```bash
go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
migrate create -ext sql -dir migrations -seq init_meta
```

For ephemeral/demo deployments, SQLite is sufficient:

```sql
-- 001_init_meta.up.sql
CREATE TABLE query_history (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT NOT NULL,
  database_name TEXT NOT NULL,
  query TEXT NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

Leave this directory empty for stateless operation (the default).
