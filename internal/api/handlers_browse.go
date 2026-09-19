package api

import (
	"fmt"
	"log"
	"os/exec"
	"strings"

	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/bikky-kc013/TableForge/internal/dump"
	"github.com/bikky-kc013/TableForge/internal/pgcatalog"
)

func (s *Server) handleDatabasesPage(w http.ResponseWriter, r *http.Request) {
	pool, sess, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	dbs, err := pgcatalog.ListDatabases(r.Context(), pool)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to list databases.", err)
		return
	}
	s.renderWithTheme(w, r, "databases.html", map[string]interface{}{
		"Databases": dbs,
		"Session":   sess,
		"CSRFToken": sess.CSRFToken,
		"Database":  r.URL.Query().Get("database"),
	})
}

func (s *Server) handleCreateDatabase(w http.ResponseWriter, r *http.Request) {
	pool, _, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	name := r.FormValue("name")
	encoding := r.FormValue("encoding")
	if name == "" {
		s.handleError(w, r, http.StatusBadRequest, "Database name is required.", nil)
		return
	}
	if err := pgcatalog.CreateDatabase(r.Context(), pool, name, encoding, "", "", ""); err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to create database.", err)
		return
	}
	http.Redirect(w, r, "/databases?database="+r.URL.Query().Get("database"), http.StatusFound)
}

func (s *Server) handleDropDatabase(w http.ResponseWriter, r *http.Request) {
	pool, _, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	name := r.FormValue("name")
	if name == "" {
		s.handleError(w, r, http.StatusBadRequest, "Database name is required.", nil)
		return
	}
	if err := pgcatalog.DropDatabase(r.Context(), pool, name); err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to drop database.", err)
		return
	}
	http.Redirect(w, r, "/databases", http.StatusFound)
}

func (s *Server) handleSchemasPage(w http.ResponseWriter, r *http.Request) {
	pool, sess, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	schemas, err := pgcatalog.ListSchemas(r.Context(), pool, s.cfg.ShowSystem)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to list schemas.", err)
		return
	}
	s.renderWithTheme(w, r, "schemas.html", map[string]interface{}{
		"Schemas":   schemas,
		"Session":   sess,
		"CSRFToken": sess.CSRFToken,
		"Database":  r.URL.Query().Get("database"),
	})
}

func (s *Server) handleTablesPage(w http.ResponseWriter, r *http.Request) {
	pool, sess, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	schema := r.URL.Query().Get("schema")
	if schema == "" {
		schema = "public"
	}
	tables, err := pgcatalog.ListTables(r.Context(), pool, schema)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to list tables.", err)
		return
	}
	views, _ := pgcatalog.ListViews(r.Context(), pool, schema)
	seqs, _ := pgcatalog.ListSequences(r.Context(), pool, schema)
	s.renderWithTheme(w, r, "tables.html", map[string]interface{}{
		"Tables":    tables,
		"Views":     views,
		"Sequences": seqs,
		"Schema":    schema,
		"Session":   sess,
		"CSRFToken": sess.CSRFToken,
		"Database":  r.URL.Query().Get("database"),
	})
}

func (s *Server) handleTableDetail(w http.ResponseWriter, r *http.Request) {
	pool, sess, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	schema := r.URL.Query().Get("schema")
	if schema == "" {
		schema = "public"
	}
	table := chi.URLParam(r, "table")
	cols, err := pgcatalog.ListColumns(r.Context(), pool, schema, table)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to load table structure.", err)
		return
	}
	idxs, _ := pgcatalog.ListIndexes(r.Context(), pool, schema, table)
	cons, _ := pgcatalog.ListConstraints(r.Context(), pool, schema, table)
	trigs, _ := pgcatalog.ListTriggers(r.Context(), pool, schema, table)
	s.renderWithTheme(w, r, "table_detail.html", map[string]interface{}{
		"Table":       table,
		"Schema":      schema,
		"Columns":     cols,
		"Indexes":     idxs,
		"Constraints": cons,
		"Triggers":    trigs,
		"Session":     sess,
		"CSRFToken":   sess.CSRFToken,
		"Database":    r.URL.Query().Get("database"),
	})
}

func (s *Server) handleBrowsePage(w http.ResponseWriter, r *http.Request) {
	pool, sess, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	schema := chi.URLParam(r, "schema")
	table := chi.URLParam(r, "table")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page == 0 {
		page = 1
	}
	sortCol := r.URL.Query().Get("sort")
	sortDir := r.URL.Query().Get("dir")
	result, err := pgcatalog.BrowsePage(r.Context(), pool, schema, table, page, s.cfg.MaxRows, sortCol, sortDir, nil)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to browse table.", err)
		return
	}
	pkcol, _ := pgcatalog.GetPrimaryKeyColumn(r.Context(), pool, schema, table)
	if pkcol == "" && len(result.Columns) > 0 {
		pkcol = result.Columns[0]
	}
	s.renderWithTheme(w, r, "browse.html", map[string]interface{}{
		"Schema":    schema,
		"Table":     table,
		"Result":    result,
		"Session":   sess,
		"CSRFToken": sess.CSRFToken,
		"Database":  r.URL.Query().Get("database"),
		"Page":      page,
		"PKCol":     pkcol,
	})
}

func (s *Server) handleSQLPage(w http.ResponseWriter, r *http.Request) {
	sess, _ := getSession(r)
	schema := chi.URLParam(r, "schema")
	if schema == "" {
		schema = r.URL.Query().Get("schema")
	}
	table := chi.URLParam(r, "table")
	if table == "" {
		table = r.URL.Query().Get("table")
	}
	s.renderWithTheme(w, r, "sql.html", map[string]interface{}{
		"Session":   sess,
		"CSRFToken": sess.CSRFToken,
		"Database":  r.URL.Query().Get("database"),
		"Schema":    schema,
		"Table":     table,
	})
}

func (s *Server) handleSQLExec(w http.ResponseWriter, r *http.Request) {
	pool, sess, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	query := r.FormValue("query")
	if query == "" {
		s.handleError(w, r, http.StatusBadRequest, "SQL query is required.", nil)
		return
	}
	result, rowsAffected, err := pgcatalog.RunSQL(r.Context(), pool, query)
	if err != nil {
		s.renderWithTheme(w, r, "sql.html", map[string]interface{}{
			"Error": err.Error(), "Query": query, "CSRFToken": r.FormValue("csrf_token"),
			"Session": sess, "Database": r.URL.Query().Get("database"),
			"Schema": r.URL.Query().Get("schema"), "Table": r.URL.Query().Get("table"),
		})
		return
	}
	s.renderWithTheme(w, r, "sql_result.html", map[string]interface{}{
		"Result": result, "RowsAffected": rowsAffected, "Query": query,
		"Session": sess, "CSRFToken": r.FormValue("csrf_token"),
		"Database": r.URL.Query().Get("database"),
		"Schema": r.URL.Query().Get("schema"), "Table": r.URL.Query().Get("table"),
	})
}

func (s *Server) handleExplain(w http.ResponseWriter, r *http.Request) {
	pool, sess, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	query := r.FormValue("query")
	analyze := r.FormValue("analyze") == "on"
	res, err := pgcatalog.Explain(r.Context(), pool, query, analyze)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to explain query.", err)
		return
	}
	s.renderWithTheme(w, r, "explain.html", map[string]interface{}{
		"Rows": res.Rows, "Query": query,
		"Session": sess, "CSRFToken": sess.CSRFToken,
		"Database": r.URL.Query().Get("database"),
		"Schema": r.URL.Query().Get("schema"), "Table": r.URL.Query().Get("table"),
	})
}

func (s *Server) handleRolesPage(w http.ResponseWriter, r *http.Request) {
	pool, sess, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	roles, err := pgcatalog.ListRoles(r.Context(), pool)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to list roles.", err)
		return
	}
	s.renderWithTheme(w, r, "roles.html", map[string]interface{}{"Roles": roles, "Session": sess, "CSRFToken": sess.CSRFToken})
}

func (s *Server) handleTablespacesPage(w http.ResponseWriter, r *http.Request) {
	pool, sess, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	tss, err := pgcatalog.ListTablespaces(r.Context(), pool)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to list tablespaces.", err)
		return
	}
	s.renderWithTheme(w, r, "tablespaces.html", map[string]interface{}{"Tablespaces": tss, "Session": sess, "CSRFToken": sess.CSRFToken})
}

func (s *Server) handleAdminPage(w http.ResponseWriter, r *http.Request) {
	sess, _ := getSession(r)
	pool, _, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	vars, _ := pgcatalog.GetVariables(r.Context(), pool)
	schema := chi.URLParam(r, "schema")
	if schema == "" {
		schema = r.URL.Query().Get("schema")
	}
	table := chi.URLParam(r, "table")
	if table == "" {
		table = r.URL.Query().Get("table")
	}
	s.renderWithTheme(w, r, "admin.html", map[string]interface{}{
		"Variables": vars, "Session": sess, "CSRFToken": sess.CSRFToken,
		"Database": r.URL.Query().Get("database"), "Schema": schema, "Table": table,
	})
}

func (s *Server) handleVacuum(w http.ResponseWriter, r *http.Request) {
	pool, _, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	schema := r.FormValue("schema")
	table := r.FormValue("table")
	full := r.FormValue("full") == "on"
	if err := pgcatalog.Vacuum(r.Context(), pool, schema, table, full, false, false); err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to vacuum.", err)
		return
	}
	http.Redirect(w, r, "/admin?database="+r.URL.Query().Get("database"), http.StatusFound)
}

func (s *Server) handleReindex(w http.ResponseWriter, r *http.Request) {
	pool, _, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	schema := r.FormValue("schema")
	table := r.FormValue("table")
	sess, _ := getSession(r)
	var versionNum int
	if sess != nil && sess.ServerIdx != nil {
		if pw, err2 := s.sessions.GetPassword(sess); err2 == nil {
			if c, err2 := s.dbMgr.GetOrCreate(r.Context(), *sess.ServerIdx, sess.Database, sess.Username, pw); err2 == nil {
				versionNum = c.Capability.VersionNum
			}
		}
	}
	cap := deriveCap(versionNum)
	if err := pgcatalog.Reindex(r.Context(), pool, cap, schema, table, "", false); err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to reindex.", err)
		return
	}
	http.Redirect(w, r, "/admin?database="+r.URL.Query().Get("database"), http.StatusFound)
}

func (s *Server) handleExportPage(w http.ResponseWriter, r *http.Request) {
	sess, _ := getSession(r)
	schema := chi.URLParam(r, "schema")
	if schema == "" {
		schema = r.URL.Query().Get("schema")
	}
	table := chi.URLParam(r, "table")
	if table == "" {
		table = r.URL.Query().Get("table")
	}
	isImport := strings.HasPrefix(r.URL.Path, "/import")
	s.renderWithTheme(w, r, "export.html", map[string]interface{}{
		"Session": sess, "CSRFToken": sess.CSRFToken,
		"Database": r.URL.Query().Get("database"), "Schema": schema, "Table": table,
		"IsImport": isImport,
	})
}

func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	pool, _, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	database := r.URL.Query().Get("database")
	schema := r.URL.Query().Get("schema")
	if schema == "" {
		schema = "public"
	}
	table := r.URL.Query().Get("table")
	if table == "" {
		s.handleError(w, r, http.StatusBadRequest, "Table is required for CSV export.", nil)
		return
	}
	// Validate identifiers via ExportCSV
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	filename := fmt.Sprintf("%s_%s_%s.csv", database, schema, table)
	if database == "" {
		filename = fmt.Sprintf("%s_%s.csv", schema, table)
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	if err := pgcatalog.ExportCSV(r.Context(), pool, schema, table, w); err != nil {
		// Headers already sent – log and abort, cannot render error page
		log.Printf("[export csv] failed %s.%s: %v", schema, table, err)
		return
	}
}

func (s *Server) handleExportSQL(w http.ResponseWriter, r *http.Request) {
	_, sess, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	if sess.ServerIdx == nil {
		s.handleError(w, r, http.StatusBadRequest, "Server not selected.", nil)
		return
	}
	srv := s.cfg.Servers[*sess.ServerIdx]
	if !dump.IsDumpEnabled(srv, false) {
		s.handleError(w, r, http.StatusNotImplemented, "pg_dump is not enabled.", fmt.Errorf("pg_dump_path not configured for server %d", *sess.ServerIdx))
		return
	}
	password, err := s.sessions.GetPassword(sess)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to retrieve credentials.", err)
		return
	}
	database := r.URL.Query().Get("database")
	if database == "" {
		database = sess.Database
		if database == "" {
			database = srv.DefaultDB
		}
	}
	schema := r.URL.Query().Get("schema")
	table := r.URL.Query().Get("table")
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "plain"
	}
	opts := dump.Options{
		Database: database,
		Schema:   schema,
		Table:    table,
		Format:   format,
	}
	args, err := dump.BuildArgs(srv, sess.Username, opts)
	if err != nil {
		s.handleError(w, r, http.StatusBadRequest, "Invalid dump options.", err)
		return
	}
	bin := srv.PgDumpPath
	if bin == "" {
		bin = "/usr/bin/pg_dump"
	}
	cmd := exec.CommandContext(r.Context(), bin, args...)
	cmd.Env = []string{
		"PGPASSWORD=" + password,
		"PGSSLMODE=" + srv.SSLMode,
	}
	// Stream dump to client
	filename := database + ".sql"
	if table != "" {
		filename = fmt.Sprintf("%s_%s_%s.sql", database, schema, table)
	} else if schema != "" {
		filename = fmt.Sprintf("%s_%s.sql", database, schema)
	}
	if format == "custom" {
		filename += ".dump"
	} else if format == "tar" {
		filename += ".tar"
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	cmd.Stdout = w
	cmd.Stderr = w
	if err := cmd.Run(); err != nil {
		log.Printf("[export sql] pg_dump failed for %s: %v", database, err)
		return
	}
}

func (s *Server) handleImportCSV(w http.ResponseWriter, r *http.Request) {
	pool, sess, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		s.handleError(w, r, http.StatusBadRequest, "Invalid form submission.", err)
		return
	}
	schema := r.FormValue("schema")
	if schema == "" {
		schema = chi.URLParam(r, "schema")
	}
	if schema == "" {
		schema = r.URL.Query().Get("schema")
	}
	table := r.FormValue("table")
	if table == "" {
		table = chi.URLParam(r, "table")
	}
	if table == "" {
		table = r.URL.Query().Get("table")
	}
	if schema == "" {
		schema = "public"
	}
	if table == "" {
		s.renderWithTheme(w, r, "export.html", map[string]interface{}{
			"Session": sess, "CSRFToken": sess.CSRFToken,
			"Database": r.URL.Query().Get("database"), "Schema": schema, "Table": table,
			"IsImport": true, "Error": "Schema and table are required for import.",
		})
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		s.renderWithTheme(w, r, "export.html", map[string]interface{}{
			"Session": sess, "CSRFToken": sess.CSRFToken,
			"Database": r.URL.Query().Get("database"), "Schema": schema, "Table": table,
			"IsImport": true, "Error": "File is required: " + err.Error(),
		})
		return
	}
	defer file.Close()
	count, err := pgcatalog.ImportCSV(r.Context(), pool, schema, table, file)
	if err != nil {
		s.renderWithTheme(w, r, "export.html", map[string]interface{}{
			"Session": sess, "CSRFToken": sess.CSRFToken,
			"Database": r.URL.Query().Get("database"), "Schema": schema, "Table": table,
			"IsImport": true, "Error": err.Error(),
		})
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/browse/%s/%s?database=%s", schema, table, r.URL.Query().Get("database")), http.StatusFound)
	_ = count
	_ = sess
}

func (s *Server) handleRowDelete(w http.ResponseWriter, r *http.Request) {
	pool, _, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	schema := chi.URLParam(r, "schema")
	table := chi.URLParam(r, "table")
	if err := r.ParseForm(); err != nil {
		s.handleError(w, r, http.StatusBadRequest, "Invalid form submission.", nil)
		return
	}
	pkcol := r.FormValue("pkcol")
	pkval := r.FormValue("pkval")
	if pkcol == "" || pkval == "" {
		s.handleError(w, r, http.StatusBadRequest, "Primary key is required.", nil)
		return
	}
	q := fmt.Sprintf("DELETE FROM %s WHERE %s = $1", pgx.Identifier{schema, table}.Sanitize(), pgx.Identifier{pkcol}.Sanitize())
	tag, err := pool.Exec(r.Context(), q, pkval)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to delete row.", err)
		return
	}
	if tag.RowsAffected() == 0 {
		s.handleError(w, r, http.StatusNotFound, "No rows were deleted.", nil)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/browse/%s/%s?database=%s", schema, table, r.URL.Query().Get("database")), http.StatusFound)
}

func (s *Server) handleRowEditPage(w http.ResponseWriter, r *http.Request) {
	pool, sess, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	schema := chi.URLParam(r, "schema")
	table := chi.URLParam(r, "table")
	pkcol := r.URL.Query().Get("pkcol")
	pkval := r.URL.Query().Get("pkval")
	if pkcol == "" {
		if c, err := pgcatalog.GetPrimaryKeyColumn(r.Context(), pool, schema, table); err == nil {
			pkcol = c
		} else {
			pkcol = "id"
		}
	}
	q := fmt.Sprintf("SELECT * FROM %s WHERE %s = $1 LIMIT 1", pgx.Identifier{schema, table}.Sanitize(), pgx.Identifier{pkcol}.Sanitize())
	rows, err := pool.Query(r.Context(), q, pkval)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to load row.", err)
		return
	}
	defer rows.Close()
	if !rows.Next() {
		s.handleError(w, r, http.StatusNotFound, "Row not found.", nil)
		return
	}
	vals, _ := rows.Values()
	// format values
	for i, v := range vals {
		vals[i] = fmt.Sprint(v)
	}
	fds := rows.FieldDescriptions()
	cols := make([]string, len(fds))
	for i, fd := range fds {
		cols[i] = string(fd.Name)
	}
	colInfos, _ := pgcatalog.ListColumns(r.Context(), pool, schema, table)
	s.renderWithTheme(w, r, "edit.html", map[string]interface{}{
		"Schema":    schema,
		"Table":     table,
		"Columns":   colInfos,
		"ColNames":  cols,
		"Values":    vals,
		"PKCol":     pkcol,
		"PKVal":     pkval,
		"Session":   sess,
		"CSRFToken": sess.CSRFToken,
		"Database":  r.URL.Query().Get("database"),
	})
}

func (s *Server) handleInsertPage(w http.ResponseWriter, r *http.Request) {
	pool, sess, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	schema := chi.URLParam(r, "schema")
	table := chi.URLParam(r, "table")
	// handle schema-only or db-only insert (no table) gracefully
	if table == "" {
		// try to get table from query param or show generic page
		table = r.URL.Query().Get("table")
		if table == "" {
			// for db-level, just show a message or redirect to tables
			s.renderWithTheme(w, r, "insert.html", map[string]interface{}{
				"Schema": schema, "Table": "", "Columns": []interface{}{}, "Session": sess, "CSRFToken": sess.CSRFToken, "Database": r.URL.Query().Get("database"),
				"Error": "Select a table first to insert a row.",
			})
			return
		}
	}
	if schema == "" {
		schema = "public"
	}
	cols, err := pgcatalog.ListColumns(r.Context(), pool, schema, table)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to load table columns.", err)
		return
	}
	s.renderWithTheme(w, r, "insert.html", map[string]interface{}{
		"Schema": schema, "Table": table, "Columns": cols, "Session": sess, "CSRFToken": sess.CSRFToken, "Database": r.URL.Query().Get("database"),
	})
}
func (s *Server) handleInsertPost(w http.ResponseWriter, r *http.Request) {
	pool, _, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	schema := chi.URLParam(r, "schema")
	table := chi.URLParam(r, "table")
	if err := r.ParseForm(); err != nil {
		s.handleError(w, r, http.StatusBadRequest, "Invalid form submission.", nil)
		return
	}
	cols, _ := pgcatalog.ListColumns(r.Context(), pool, schema, table)
	vals := map[string]interface{}{}
	for _, col := range cols {
		if col.Name == "id" && r.FormValue("col_"+col.Name) == "" {
			continue // skip auto-increment
		}
		v := r.FormValue("col_" + col.Name)
		if v == "" && !col.NotNull {
			vals[col.Name] = nil
		} else {
			vals[col.Name] = v
		}
	}
	if err := pgcatalog.InsertRow(r.Context(), pool, schema, table, vals); err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to insert row.", err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/browse/%s/%s?database=%s", schema, table, r.URL.Query().Get("database")), http.StatusFound)
}
func (s *Server) handleSearchPage(w http.ResponseWriter, r *http.Request) {
	pool, sess, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	schema := chi.URLParam(r, "schema")
	if schema == "" {
		schema = r.URL.Query().Get("schema")
		if schema == "" {
			schema = "public"
		}
	}
	table := chi.URLParam(r, "table")
	if table == "" {
		table = r.URL.Query().Get("table")
	}
	var cols []interface{}
	if table != "" {
		if c, err := pgcatalog.ListColumns(r.Context(), pool, schema, table); err == nil {
			// convert to interface slice for template
			for _, cc := range c {
				cols = append(cols, cc)
			}
		}
	} else {
		// for db/schema level search, get tables to let user choose, or just show empty
		if tables, err := pgcatalog.ListTables(r.Context(), pool, schema); err == nil {
			for _, t := range tables {
				cols = append(cols, t)
			}
		}
	}
	s.renderWithTheme(w, r, "search.html", map[string]interface{}{"Schema": schema, "Table": table, "Columns": cols, "Session": sess, "CSRFToken": sess.CSRFToken, "Database": r.URL.Query().Get("database")})
}
func (s *Server) handleSearchPost(w http.ResponseWriter, r *http.Request) {
	pool, sess, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	schema := chi.URLParam(r, "schema")
	table := chi.URLParam(r, "table")
	if err := r.ParseForm(); err != nil {
		s.handleError(w, r, http.StatusBadRequest, "Invalid form submission.", nil)
		return
	}
	col := r.FormValue("col")
	op := r.FormValue("op")
	val := r.FormValue("val")
	filters := map[string]string{}
	if col != "" && val != "" {
		// map op to ILIKE or = etc - for now use ILIKE for contains
		if op == "contains" || op == "like" {
			filters[col] = val
		} else {
			filters[col] = val
		}
	}
	result, err := pgcatalog.BrowsePage(r.Context(), pool, schema, table, 1, s.cfg.MaxRows, "", "ASC", filters)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Search failed.", err)
		return
	}
	s.renderWithTheme(w, r, "search.html", map[string]interface{}{"Schema": schema, "Table": table, "Result": result, "Session": sess, "CSRFToken": sess.CSRFToken, "Database": r.URL.Query().Get("database"), "SearchCol": col, "SearchVal": val})
}

func (s *Server) handleRowEditPost(w http.ResponseWriter, r *http.Request) {
	pool, _, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	schema := chi.URLParam(r, "schema")
	table := chi.URLParam(r, "table")
	if err := r.ParseForm(); err != nil {
		s.handleError(w, r, http.StatusBadRequest, "Invalid form submission.", nil)
		return
	}
	pkcol := r.FormValue("pkcol")
	pkval := r.FormValue("pkval")
	cols, _ := pgcatalog.ListColumns(r.Context(), pool, schema, table)
	sets := []string{}
	args := []interface{}{}
	idx := 1
	for _, col := range cols {
		if col.Name == pkcol {
			continue
		}
		v := r.FormValue("col_" + col.Name)
		// Normalize value per column type for production
		var arg interface{} = v
		colType := strings.ToLower(col.Type)
		isJSON := strings.Contains(colType, "json")
		isBool := colType == "boolean" || colType == "bool"
		if v == "" {
			// empty string -> NULL if column allows null, else keep empty for text
			if !col.NotNull {
				arg = nil
			} else if isJSON {
				arg = "null"
			}
		} else if isJSON {
			// ensure valid JSON; if not valid, wrap as JSON string
			if !json.Valid([]byte(v)) {
				// try to marshal as JSON string value
				if b, err := json.Marshal(v); err == nil {
					// use the marshaled string (quoted)
					arg = string(b)
				} else {
					arg = v
				}
			} else {
				arg = v
			}
		} else if isBool {
			// normalize boolean
			lv := strings.ToLower(strings.TrimSpace(v))
			if lv == "true" || lv == "t" || lv == "1" || lv == "yes" {
				arg = true
			} else if lv == "false" || lv == "f" || lv == "0" || lv == "no" || lv == "" {
				if !col.NotNull && v == "" {
					arg = nil
				} else {
					arg = false
				}
			}
		}
		sets = append(sets, fmt.Sprintf("%s = $%d", pgx.Identifier{col.Name}.Sanitize(), idx))
		args = append(args, arg)
		idx++
	}
	if len(sets) == 0 {
		s.handleError(w, r, http.StatusBadRequest, "No fields to update.", nil)
		return
	}
	args = append(args, pkval)
	q := fmt.Sprintf("UPDATE %s SET %s WHERE %s = $%d", pgx.Identifier{schema, table}.Sanitize(), strings.Join(sets, ", "), pgx.Identifier{pkcol}.Sanitize(), idx)
	_, err = pool.Exec(r.Context(), q, args...)
	if err != nil {
		// production: show user-friendly message with detail collapsed
		s.renderWithTheme(w, r, "edit.html", map[string]interface{}{
			"Schema":  schema,
			"Table":   table,
			"Columns": cols,
			"Error":   "Save failed — please check the values. The database returned an error.",
			"ErrorDetail": err.Error(),
			"PKCol":   pkcol,
			"PKVal":   pkval,
			"Database": r.URL.Query().Get("database"),
		})
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/browse/%s/%s?database=%s", schema, table, r.URL.Query().Get("database")), http.StatusFound)
}

// API JSON handlers


func (s *Server) apiListDatabases(w http.ResponseWriter, r *http.Request) {
	pool, _, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	dbs, err := pgcatalog.ListDatabases(r.Context(), pool)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to list databases.", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dbs)
}

func (s *Server) apiListSchemas(w http.ResponseWriter, r *http.Request) {
	pool, _, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	schemas, err := pgcatalog.ListSchemas(r.Context(), pool, true)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to list schemas.", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(schemas)
}

func (s *Server) apiListTables(w http.ResponseWriter, r *http.Request) {
	pool, _, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	schema := r.URL.Query().Get("schema")
	if schema == "" {
		schema = "public"
	}
	tables, err := pgcatalog.ListTables(r.Context(), pool, schema)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to list tables.", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tables)
}

func (s *Server) apiListColumns(w http.ResponseWriter, r *http.Request) {
	pool, _, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	schema := r.URL.Query().Get("schema")
	table := r.URL.Query().Get("table")
	cols, err := pgcatalog.ListColumns(r.Context(), pool, schema, table)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to list columns.", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cols)
}

func (s *Server) apiListViews(w http.ResponseWriter, r *http.Request) {
	pool, _, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	schema := r.URL.Query().Get("schema")
	if schema == "" {
		schema = "public"
	}
	views, err := pgcatalog.ListViews(r.Context(), pool, schema)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to list views.", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(views)
}

func (s *Server) apiListSequences(w http.ResponseWriter, r *http.Request) {
	pool, _, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	schema := r.URL.Query().Get("schema")
	if schema == "" {
		schema = "public"
	}
	seqs, err := pgcatalog.ListSequences(r.Context(), pool, schema)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to list sequences.", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(seqs)
}

func (s *Server) apiListFunctions(w http.ResponseWriter, r *http.Request) {
	pool, _, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	schema := r.URL.Query().Get("schema")
	if schema == "" {
		schema = "public"
	}
	funcs, err := pgcatalog.ListFunctions(r.Context(), pool, schema)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to list functions.", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(funcs)
}

func (s *Server) apiListIndexes(w http.ResponseWriter, r *http.Request) {
	pool, _, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	schema := r.URL.Query().Get("schema")
	table := r.URL.Query().Get("table")
	idxs, err := pgcatalog.ListIndexes(r.Context(), pool, schema, table)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to list indexes.", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(idxs)
}

func (s *Server) apiListConstraints(w http.ResponseWriter, r *http.Request) {
	pool, _, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	schema := r.URL.Query().Get("schema")
	table := r.URL.Query().Get("table")
	cons, err := pgcatalog.ListConstraints(r.Context(), pool, schema, table)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to list constraints.", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cons)
}

func (s *Server) apiListRoles(w http.ResponseWriter, r *http.Request) {
	pool, _, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	roles, err := pgcatalog.ListRoles(r.Context(), pool)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to list roles.", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(roles)
}

func (s *Server) apiBrowse(w http.ResponseWriter, r *http.Request) {
	pool, _, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	schema := r.URL.Query().Get("schema")
	table := r.URL.Query().Get("table")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	sortCol := r.URL.Query().Get("sort")
	sortDir := r.URL.Query().Get("dir")
	res, err := pgcatalog.BrowsePage(r.Context(), pool, schema, table, page, s.cfg.MaxRows, sortCol, sortDir, nil)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to browse table.", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func (s *Server) apiRunSQL(w http.ResponseWriter, r *http.Request) {
	pool, _, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	var body struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.handleError(w, r, http.StatusBadRequest, "Invalid JSON payload.", nil)
		return
	}
	result, affected, err := pgcatalog.RunSQL(r.Context(), pool, body.Query)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to execute query.", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"result": result, "affected": affected})
}

func (s *Server) apiActivity(w http.ResponseWriter, r *http.Request) {
	pool, _, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	sess, _ := getSession(r)
	var versionNum int
	if sess != nil && sess.ServerIdx != nil {
		if pw, err2 := s.sessions.GetPassword(sess); err2 == nil {
			if c, err2 := s.dbMgr.GetOrCreate(r.Context(), *sess.ServerIdx, sess.Database, sess.Username, pw); err2 == nil {
				versionNum = c.Capability.VersionNum
			}
		}
	}
	act, err := pgcatalog.ListActivity(r.Context(), pool, versionNum)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to list activity.", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(act)
}

func (s *Server) apiCancelBackend(w http.ResponseWriter, r *http.Request) {
	pool, _, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	var body struct{ PID int32 `json:"pid"` }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.handleError(w, r, http.StatusBadRequest, "Invalid JSON payload.", nil)
		return
	}
	ok, err := pgcatalog.CancelBackend(r.Context(), pool, body.PID)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to cancel backend.", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": ok})
}

func (s *Server) apiVariables(w http.ResponseWriter, r *http.Request) {
	pool, _, err := s.mustPoolTyped(r)
	if err != nil {
		s.handleError(w, r, http.StatusUnauthorized, "Not connected — please log in again.", err)
		return
	}
	vars, err := pgcatalog.GetVariables(r.Context(), pool)
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to fetch variables.", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vars)
}