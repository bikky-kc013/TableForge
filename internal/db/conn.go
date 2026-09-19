package db

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/bikky-kc013/TableForge/internal/config"
)

// Conn wraps a pgxpool.Pool tied to a specific (server, database, user) tuple
// and stores version capability derived once at connect.
type Conn struct {
	Pool       *pgxpool.Pool
	ServerIdx  int
	Server     config.Server
	Database   string
	Username   string
	Capability Capability
	VersionStr string
}

// Manager maintains pools per (server, database, user). One pool per session-opened combo.
type Manager struct {
	mu    sync.RWMutex
	pools map[string]*Conn
	cfg   *config.Config
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		pools: make(map[string]*Conn),
		cfg:   cfg,
	}
}

func poolKey(serverIdx int, database, username string) string {
	return fmt.Sprintf("%d|%s|%s", serverIdx, database, username)
}

// GetOrCreate returns an existing Conn or creates a new pgxpool, probing server_version_num.
func (m *Manager) GetOrCreate(ctx context.Context, serverIdx int, database, username, password string) (*Conn, error) {
	if serverIdx < 0 || serverIdx >= len(m.cfg.Servers) {
		return nil, fmt.Errorf("invalid server idx %d", serverIdx)
	}
	key := poolKey(serverIdx, database, username)
	m.mu.RLock()
	if c, ok := m.pools[key]; ok {
		m.mu.RUnlock()
		// verify pool still healthy
		if err := c.Pool.Ping(ctx); err == nil {
			return c, nil
		}
		// unhealthy: drop and recreate below
		m.mu.Lock()
		delete(m.pools, key)
		m.mu.Unlock()
	} else {
		m.mu.RUnlock()
	}

	srv := m.cfg.Servers[serverIdx]
	dsn := buildDSN(srv, database, username, password)

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	poolCfg.MaxConns = 5
	poolCfg.MinConns = 0

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}

	// Probe version once.
	var versionNum int
	var versionStr string
	err = pool.QueryRow(ctx, "SHOW server_version_num").Scan(&versionStr)
	if err != nil {
		// fallback: try SELECT current_setting
		var s string
		if err2 := pool.QueryRow(ctx, "SELECT current_setting('server_version_num')").Scan(&s); err2 == nil {
			versionStr = s
		} else {
			pool.Close()
			return nil, fmt.Errorf("detect version: %w", err)
		}
	}
	// versionStr is like "140005"
	fmt.Sscanf(versionStr, "%d", &versionNum)
	if versionNum == 0 {
		// try second query for full string "14.5"
		var ver string
		_ = pool.QueryRow(ctx, "SHOW server_version").Scan(&ver)
		versionStr = ver
		// parse "14.5" -> 140005 approx
		versionNum = parseVersionStr(ver)
	} else {
		// also fetch human string
		_ = pool.QueryRow(ctx, "SHOW server_version").Scan(&versionStr)
		if versionNum == 0 {
			versionNum = parseVersionStr(versionStr)
		}
	}

	cap := Derive(versionNum)

	conn := &Conn{
		Pool:       pool,
		ServerIdx:  serverIdx,
		Server:     srv,
		Database:   database,
		Username:   username,
		Capability: cap,
		VersionStr: versionStr,
	}

	m.mu.Lock()
	m.pools[key] = conn
	m.mu.Unlock()

	return conn, nil
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, c := range m.pools {
		c.Pool.Close()
		delete(m.pools, k)
	}
}

// buildDSN constructs a pgx connection string with proper escaping.
// We use keyword=value format to avoid URL encoding edge cases with passwords containing @:/? .
func buildDSN(srv config.Server, database, username, password string) string {
	if database == "" {
		database = srv.DefaultDB
	}
	parts := []string{}
	if srv.Host != "" {
		parts = append(parts, fmt.Sprintf("host=%s", quote(srv.Host)))
	}
	if srv.Port != 0 {
		parts = append(parts, fmt.Sprintf("port=%d", srv.Port))
	}
	parts = append(parts, fmt.Sprintf("dbname=%s", quote(database)))
	parts = append(parts, fmt.Sprintf("user=%s", quote(username)))
	if password != "" {
		parts = append(parts, fmt.Sprintf("password=%s", quote(password)))
	}
	sslmode := srv.SSLMode
	if sslmode == "" {
		sslmode = "allow"
	}
	parts = append(parts, fmt.Sprintf("sslmode=%s", quote(sslmode)))
	parts = append(parts, "application_name=pgadmin-go")
	return strings.Join(parts, " ")
}

func quote(s string) string {
	// quote for libpq keyword=value: single-quote and escape ' and \
	if strings.ContainsAny(s, " \t\n'\\") {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `'`, `\'`)
		return "'" + s + "'"
	}
	return s
}

func parseVersionStr(v string) int {
	// "14.5" -> 140005, "10.3" -> 100003, "9.6.12" -> 90612
	var major, minor, patch int
	n, _ := fmt.Sscanf(v, "%d.%d.%d", &major, &minor, &patch)
	if n == 2 {
		if major >= 10 {
			// PG 10+: version is MAJOR.minor
			return major*10000 + minor
		}
		return major*10000 + minor*100 + patch
	}
	if n == 3 {
		return major*10000 + minor*100 + patch
	}
	if n == 1 {
		return major * 10000
	}
	var m int
	if _, err := fmt.Sscanf(v, "%d", &m); err == nil {
		return m
	}
	return 0
}

// SanitizeIdentifier safely quotes an identifier using pgx.Identifier.
// Use for table/column names where interpolation is unavoidable, but
// always re-check against pg_catalog allowlist where possible.
func SanitizeIdentifier(ident ...string) string {
	return pgx.Identifier(ident).Sanitize()
}

// Query helpers that enforce parameterized queries.

func QueryRow(ctx context.Context, pool *pgxpool.Pool, sql string, args ...interface{}) pgx.Row {
	return pool.QueryRow(ctx, sql, args...)
}
