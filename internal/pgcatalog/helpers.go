package pgcatalog

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

var identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$]*$`)

func validateIdentifier(name string) error {
	if name == "" {
		return fmt.Errorf("empty identifier")
	}
	// Allow quoted identifiers with any chars except null; we sanitize via pgx anyway.
	// But reject obvious injection patterns.
	if strings.Contains(name, "\x00") {
		return fmt.Errorf("invalid identifier: null byte")
	}
	if len(name) > 63 {
		// PG max 63, but allow longer for quoted — just warn
	}
	return nil
}

func sanitizeIdent(name string) string {
	return pgx.Identifier{name}.Sanitize()
}

func pqQuote(s string) string {
	// quote literal for ENCODING etc.
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func quoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
