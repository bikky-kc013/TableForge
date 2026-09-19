package api

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/bikky-kc013/TableForge/internal/auth"
	"github.com/bikky-kc013/TableForge/internal/config"
	"github.com/bikky-kc013/TableForge/internal/db"
)

//go:embed templates/*.html
var templateFS embed.FS

type Server struct {
	cfg      *config.Config
	sessions *auth.Store
	dbMgr    *db.Manager
	tmpl     *template.Template
	router   *chi.Mux
}

func New(cfg *config.Config, sessions *auth.Store, dbMgr *db.Manager) (*Server, error) {
	s := &Server{
		cfg:      cfg,
		sessions: sessions,
		dbMgr:    dbMgr,
	}
	if err := s.parseTemplates(); err != nil {
		return nil, err
	}
	s.router = s.buildRouter()
	return s, nil
}

func (s *Server) parseTemplates() error {
	// Parse all templates; use html/template auto-escaping like phpPgAdmin's Misc.php html escaping
	tmpl := template.New("").Funcs(template.FuncMap{
		"safe":     func(s string) template.HTML { return template.HTML(s) },
		"add":      func(a, b int) int { return a + b },
		"subtract": func(a, b int) int { return a - b },
		"colAlign": func(col string) string {
			switch col {
			case "id", "template_id", "version", "count", "rows", "size", "oid", "pid":
				return "num"
			case "is_active", "active", "enabled", "bool", "boolean":
				return "center"
			default:
				return ""
			}
		},
		"colWidth": func(col string) string {
			switch col {
			case "id", "version", "count":
				return "width:70px"
			case "template_id", "is_active", "channel", "created_at", "updated_at":
				return "width:110px"
			case "name", "subject", "title":
				return "width:180px"
			case "body", "variables", "definition":
				return "width:280px"
			default:
				return ""
			}
		},
	})
	// Walk embedded FS
	entries, err := fs.ReadDir(templateFS, "templates")
	if err != nil {
		// fallback to disk for dev (web/templates)
		tmpl, err = template.ParseGlob(filepath.Join("web", "templates", "*.html"))
		if err != nil {
			// still no templates — create minimal empty
			s.tmpl = template.New("empty")
			return nil
		}
		s.tmpl = tmpl
		return nil
	}
	for _, e := range entries {
		data, err := fs.ReadFile(templateFS, filepath.Join("templates", e.Name()))
		if err != nil {
			continue
		}
		_, err = tmpl.New(e.Name()).Parse(string(data))
		if err != nil {
			return err
		}
	}
	s.tmpl = tmpl
	return nil
}

func (s *Server) Handler() http.Handler { return s.router }

func (s *Server) buildRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(s.themeMiddleware)

	// Static files
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	r.Handle("/themes/*", http.StripPrefix("/themes/", http.FileServer(http.Dir("themes"))))
	r.Handle("/images/*", http.StripPrefix("/images/", http.FileServer(http.Dir("images"))))

	// Public routes
	r.Get("/", s.handleRoot)
	r.Get("/login", s.handleLoginPage)
	r.Post("/login", s.handleLoginPost)
	r.Get("/logout", s.handleLogout)

	// Authenticated HTML pages
	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)
		r.Get("/servers", s.handleServersPage)
		r.Get("/databases", s.handleDatabasesPage)
		r.Post("/databases/create", s.csrfMiddleware(http.HandlerFunc(s.handleCreateDatabase)).ServeHTTP)
		r.Post("/databases/drop", s.csrfMiddleware(http.HandlerFunc(s.handleDropDatabase)).ServeHTTP)

		r.Get("/schemas", s.handleSchemasPage)
		r.Get("/tables", s.handleTablesPage)
		r.Get("/table/{table}", s.handleTableDetail)
		r.Get("/browse/{schema}/{table}", s.handleBrowsePage)
		r.Post("/browse/{schema}/{table}/delete", s.csrfMiddleware(http.HandlerFunc(s.handleRowDelete)).ServeHTTP)
		r.Get("/edit/{schema}/{table}", s.handleRowEditPage)
		r.Post("/edit/{schema}/{table}", s.csrfMiddleware(http.HandlerFunc(s.handleRowEditPost)).ServeHTTP)
		r.Get("/insert/{schema}/{table}", s.handleInsertPage)
		r.Post("/insert/{schema}/{table}", s.csrfMiddleware(http.HandlerFunc(s.handleInsertPost)).ServeHTTP)
		r.Get("/insert/{schema}", s.handleInsertPage)
		r.Post("/insert/{schema}", s.csrfMiddleware(http.HandlerFunc(s.handleInsertPost)).ServeHTTP)
		r.Get("/insert/{schema}/", s.handleInsertPage)
		r.Post("/insert/{schema}/", s.csrfMiddleware(http.HandlerFunc(s.handleInsertPost)).ServeHTTP)
		r.Get("/search/{schema}/{table}", s.handleSearchPage)
		r.Post("/search/{schema}/{table}", s.csrfMiddleware(http.HandlerFunc(s.handleSearchPost)).ServeHTTP)
		r.Get("/search/{schema}", s.handleSearchPage)
		r.Post("/search/{schema}", s.csrfMiddleware(http.HandlerFunc(s.handleSearchPost)).ServeHTTP)
		r.Get("/search/{schema}/", s.handleSearchPage)
		r.Post("/search/{schema}/", s.csrfMiddleware(http.HandlerFunc(s.handleSearchPost)).ServeHTTP)
		r.Get("/search", s.handleSearchPage)
		r.Post("/search", s.csrfMiddleware(http.HandlerFunc(s.handleSearchPost)).ServeHTTP)
		r.Get("/sql", s.handleSQLPage)
		r.Post("/sql/exec", s.csrfMiddleware(http.HandlerFunc(s.handleSQLExec)).ServeHTTP)
		r.Post("/sql/explain", s.csrfMiddleware(http.HandlerFunc(s.handleExplain)).ServeHTTP)
		r.Get("/roles", s.handleRolesPage)
		r.Get("/tablespaces", s.handleTablespacesPage)
		r.Get("/admin", s.handleAdminPage)
		r.Post("/admin/vacuum", s.csrfMiddleware(http.HandlerFunc(s.handleVacuum)).ServeHTTP)
		r.Post("/admin/reindex", s.csrfMiddleware(http.HandlerFunc(s.handleReindex)).ServeHTTP)
		r.Get("/export/csv", s.handleExportCSV)
		r.Get("/export/sql", s.handleExportSQL)
		r.Get("/export/dump", s.handleExportSQL)
		r.Get("/export", s.handleExportPage)
		r.Get("/export/{schema}", s.handleExportPage)
		r.Get("/export/{schema}/", s.handleExportPage)
		r.Get("/export/{schema}/{table}", s.handleExportPage)
		r.Get("/export/{schema}/{table}/", s.handleExportPage)
		r.Get("/import", s.handleExportPage)
		r.Get("/import/{schema}", s.handleExportPage)
		r.Get("/import/{schema}/", s.handleExportPage)
		r.Get("/import/{schema}/{table}", s.handleExportPage)
		r.Get("/import/{schema}/{table}/", s.handleExportPage)
		r.Post("/import/csv", s.csrfMiddleware(http.HandlerFunc(s.handleImportCSV)).ServeHTTP)
	})

	// JSON API (for SPA or htmx partials)
	r.Route("/api", func(r chi.Router) {
		r.Use(s.authMiddleware)
		r.Get("/databases", s.apiListDatabases)
		r.Get("/schemas", s.apiListSchemas)
		r.Get("/tables", s.apiListTables)
		r.Get("/columns", s.apiListColumns)
		r.Get("/views", s.apiListViews)
		r.Get("/sequences", s.apiListSequences)
		r.Get("/functions", s.apiListFunctions)
		r.Get("/indexes", s.apiListIndexes)
		r.Get("/constraints", s.apiListConstraints)
		r.Get("/roles", s.apiListRoles)
		r.Get("/browse", s.apiBrowse)
		r.Post("/sql", s.csrfMiddleware(http.HandlerFunc(s.apiRunSQL)).ServeHTTP)
		r.Get("/activity", s.apiActivity)
		r.Post("/activity/cancel", s.csrfMiddleware(http.HandlerFunc(s.apiCancelBackend)).ServeHTTP)
		r.Get("/variables", s.apiVariables)
	})

	// Health
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	r.NotFound(s.handleNotFound)
	r.MethodNotAllowed(s.handleMethodNotAllowed)
	return r
}
