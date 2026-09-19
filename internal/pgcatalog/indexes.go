package pgcatalog

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/bikky-kc013/TableForge/internal/model"
)

func ListIndexes(ctx context.Context, pool *pgxpool.Pool, schema, table string) ([]model.Index, error) {
	q := `
SELECT i.relname, n.nspname, c.relname, pg_get_indexdef(i.oid),
       ix.indisprimary, ix.indisunique, ix.indisvalid,
       COALESCE(ts.spcname,'')
FROM pg_catalog.pg_index ix
JOIN pg_catalog.pg_class i ON i.oid=ix.indexrelid
JOIN pg_catalog.pg_class c ON c.oid=ix.indrelid
JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
LEFT JOIN pg_catalog.pg_tablespace ts ON ts.oid=i.reltablespace
WHERE n.nspname=$1 AND c.relname=$2
ORDER BY i.relname`
	rows, err := pool.Query(ctx, q, schema, table)
	if err != nil {
		return nil, fmt.Errorf("list indexes: %w", err)
	}
	defer rows.Close()
	var out []model.Index
	for rows.Next() {
		var idx model.Index
		if err := rows.Scan(&idx.Name, &idx.Schema, &idx.Table, &idx.Definition, &idx.IsPrimary, &idx.IsUnique, &idx.IsValid, &idx.Tablespace); err != nil {
			return nil, err
		}
		out = append(out, idx)
	}
	return out, rows.Err()
}

func ListConstraints(ctx context.Context, pool *pgxpool.Pool, schema, table string) ([]model.Constraint, error) {
	q := `
SELECT con.conname, con.contype, pg_get_constraintdef(con.oid, true), c.relname, n.nspname
FROM pg_catalog.pg_constraint con
JOIN pg_catalog.pg_class c ON c.oid=con.conrelid
JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
WHERE n.nspname=$1 AND c.relname=$2
ORDER BY con.conname`
	rows, err := pool.Query(ctx, q, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Constraint
	for rows.Next() {
		var cs model.Constraint
		if err := rows.Scan(&cs.Name, &cs.Type, &cs.Definition, &cs.Table, &cs.Schema); err != nil {
			return nil, err
		}
		out = append(out, cs)
	}
	return out, rows.Err()
}

func ListTriggers(ctx context.Context, pool *pgxpool.Pool, schema, table string) ([]model.Trigger, error) {
	q := `
SELECT t.tgname, c.relname, n.nspname, pg_get_triggerdef(t.oid, true), t.tgenabled
FROM pg_catalog.pg_trigger t
JOIN pg_catalog.pg_class c ON c.oid=t.tgrelid
JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
WHERE n.nspname=$1 AND c.relname=$2 AND NOT t.tgisinternal
ORDER BY t.tgname`
	rows, err := pool.Query(ctx, q, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Trigger
	for rows.Next() {
		var tr model.Trigger
		if err := rows.Scan(&tr.Name, &tr.Table, &tr.Schema, &tr.Definition, &tr.Enabled); err != nil {
			return nil, err
		}
		out = append(out, tr)
	}
	return out, rows.Err()
}
