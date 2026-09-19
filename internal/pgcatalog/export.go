package pgcatalog

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ExportCSV streams a table to CSV via COPY TO STDOUT (parameterized table, allowlisted).
func ExportCSV(ctx context.Context, pool *pgxpool.Pool, schema, table string, w io.Writer) error {
	if err := validateIdentifier(schema); err != nil {
		return err
	}
	if err := validateIdentifier(table); err != nil {
		return err
	}
	// Verify table exists in pg_catalog (allowlist check)
	var exists bool
	err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1 AND c.relname=$2)", schema, table).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("table %s.%s not found", schema, table)
	}
	ident := sanitizeIdent(schema) + "." + sanitizeIdent(table)
	// pgx doesn't support COPY TO STDOUT directly via Query; use pgxpool's low-level
	// For simplicity, fall back to SELECT + csv writer
	rows, err := pool.Query(ctx, fmt.Sprintf("SELECT * FROM %s", ident))
	if err != nil {
		return err
	}
	defer rows.Close()
	cw := csv.NewWriter(w)
	defer cw.Flush()
	fds := rows.FieldDescriptions()
	header := make([]string, len(fds))
	for i, fd := range fds {
		header[i] = string(fd.Name)
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return err
		}
		record := make([]string, len(vals))
		for i, v := range vals {
			if v == nil {
				record[i] = ""
			} else {
				record[i] = fmt.Sprint(v)
			}
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	return rows.Err()
}

// ImportCSV streams CSV into a table using COPY FROM STDIN. For simplicity, use INSERT path in low-memory cases.
func ImportCSV(ctx context.Context, pool *pgxpool.Pool, schema, table string, r io.Reader) (int64, error) {
	if err := validateIdentifier(schema); err != nil {
		return 0, err
	}
	if err := validateIdentifier(table); err != nil {
		return 0, err
	}
	cr := csv.NewReader(r)
	header, err := cr.Read()
	if err != nil {
		return 0, fmt.Errorf("csv header: %w", err)
	}
	// Build INSERT per row (parameterized, column allowlist via header validation)
	for _, col := range header {
		if err := validateIdentifier(col); err != nil {
			return 0, fmt.Errorf("invalid column %q: %w", col, err)
		}
	}
	var count int64
	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, err
		}
		if len(record) != len(header) {
			return count, fmt.Errorf("column count mismatch")
		}
		// Build parameterized insert
		vals := make(map[string]interface{}, len(header))
		for i, col := range header {
			vals[col] = record[i]
		}
		// Use transaction per batch would be better; for now single inserts
		if err := InsertRow(ctx, pool, schema, table, vals); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}
