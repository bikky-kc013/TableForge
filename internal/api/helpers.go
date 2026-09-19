package api

import (
	"context"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/phppgadmin/phppgadmin-go/internal/auth"
	"github.com/phppgadmin/phppgadmin-go/internal/db"
	"github.com/phppgadmin/phppgadmin-go/internal/model"
	"github.com/phppgadmin/phppgadmin-go/internal/pgcatalog"
)

var availableThemes = []string{"default"}

func (s *Server) getTheme(r *http.Request) string {
	// 1. ?theme= param (like phpPgAdmin lib.inc.php:102)
	if t := r.URL.Query().Get("theme"); t != "" {
		for _, avail := range availableThemes {
			if t == avail {
				// check file exists
				if _, err := os.Stat("themes/" + t + "/global.css"); err == nil {
					return t
				}
			}
		}
	}
	// 2. cookie ppaTheme
	if c, err := r.Cookie("ppaTheme"); err == nil {
		for _, avail := range availableThemes {
			if c.Value == avail {
				if _, err := os.Stat("themes/" + c.Value + "/global.css"); err == nil {
					return c.Value
				}
			}
		}
	}
	// 3. session theme
	if sess, ok := getSession(r); ok && sess.Extra != nil {
		if t, ok := sess.Extra["theme"]; ok {
			for _, avail := range availableThemes {
				if t == avail {
					return t
				}
			}
		}
	}
	// 4. config default
	if s.cfg.Theme != "" {
		return s.cfg.Theme
	}
	return "default"
}

func (s *Server) themeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// handle ?theme= set
		if t := r.URL.Query().Get("theme"); t != "" {
			for _, avail := range availableThemes {
				if t == avail {
					if _, err := os.Stat("themes/" + t + "/global.css"); err == nil {
						http.SetCookie(w, &http.Cookie{
							Name:   "ppaTheme",
							Value:  t,
							Path:   "/",
							MaxAge: 31536000,
						})
						// also store in session if exists
						if cookieID, ok := auth.GetCookie(r); ok {
							if sess, ok := s.sessions.Get(cookieID); ok {
								if sess.Extra == nil {
									sess.Extra = make(map[string]string)
								}
								sess.Extra["theme"] = t
							}
						}
						break
					}
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) mustPoolTyped(r *http.Request) (*pgxpool.Pool, *auth.Session, error) {
	sess, ok := getSession(r)
	if !ok || sess.ServerIdx == nil {
		return nil, nil, http.ErrNoCookie
	}
	pw, err := s.sessions.GetPassword(sess)
	if err != nil {
		return nil, nil, err
	}
	dbName := r.URL.Query().Get("database")
	if dbName == "" {
		dbName = sess.Database
	}
	if dbName == "" {
		dbName = s.cfg.Servers[*sess.ServerIdx].DefaultDB
	}
	conn, err := s.dbMgr.GetOrCreate(r.Context(), *sess.ServerIdx, dbName, sess.Username, pw)
	if err != nil {
		return nil, nil, err
	}
	return conn.Pool, sess, nil
}

func toPgxPool(v interface{}) *pgxpool.Pool {
	if p, ok := v.(*pgxpool.Pool); ok {
		return p
	}
	return nil
}

func deriveCap(versionNum int) db.Capability {
	return db.Derive(versionNum)
}

func (s *Server) render(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if s.tmpl == nil {
		http.Error(w, "templates not loaded", http.StatusInternalServerError)
		return
	}
	// Try named template; fallback to first
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		// try without extension or with fallback
		if err2 := s.tmpl.Execute(w, data); err2 != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func (s *Server) renderWithTheme(w http.ResponseWriter, r *http.Request, name string, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["Theme"] = s.getTheme(r)
	data["AvailableThemes"] = availableThemes
	data["AppName"] = "TableForge"
	data["AppVersion"] = "1.0.0"
	// populate server info for topbar
	if sess, ok := getSession(r); ok && sess.ServerIdx != nil && *sess.ServerIdx >= 0 && *sess.ServerIdx < len(s.cfg.Servers) {
		srv := s.cfg.Servers[*sess.ServerIdx]
		if _, ok := data["ServerDesc"]; !ok {
			data["ServerDesc"] = srv.Desc
		}
		if _, ok := data["ServerPort"]; !ok {
			data["ServerPort"] = srv.Port
		}
		if _, ok := data["ServerHost"]; !ok {
			if srv.Host == "" {
				data["ServerHost"] = "socket"
			} else {
				data["ServerHost"] = srv.Host
			}
		}
		if _, ok := data["Session"]; !ok {
			data["Session"] = sess
		}
	}
	if _, ok := data["Database"]; !ok {
		if v := r.URL.Query().Get("database"); v != "" {
			data["Database"] = v
		} else if sess, ok := getSession(r); ok {
			data["Database"] = sess.Database
		}
	}
	if _, ok := data["Schema"]; !ok {
		if v := r.URL.Query().Get("schema"); v != "" {
			data["Schema"] = v
		}
	}
	if _, ok := data["Table"]; !ok {
		if v := r.URL.Query().Get("table"); v != "" {
			data["Table"] = v
		}
	}
	// sidebar: try to populate databases and tables for current DB (phpMyAdmin-like explorer)
	// only for authenticated pages
	if sess, ok := getSession(r); ok && sess.ServerIdx != nil {
		// avoid doing this for login page to keep it light
		if name != "login.html" {
			if _, ok := data["SidebarDatabases"]; !ok {
				// try to get pool without requiring specific database (use default)
				pw, err := s.sessions.GetPassword(sess)
				if err == nil {
					ctx, cancel := context.WithTimeout(r.Context(), 1500*1_000_000) // 1.5s
					defer cancel()
					// use session database or default
					dbName := sess.Database
					if dbName == "" {
						dbName = s.cfg.Servers[*sess.ServerIdx].DefaultDB
					}
					if conn, err := s.dbMgr.GetOrCreate(ctx, *sess.ServerIdx, dbName, sess.Username, pw); err == nil {
						if dbs, err := pgcatalog.ListDatabases(ctx, conn.Pool); err == nil {
							data["SidebarDatabases"] = dbs
							// if a database is selected, also load its schemas/tables for sidebar tree
							curDB, _ := data["Database"].(string)
							if curDB != "" {
								// load schemas then tables for public schema as example, plus current schema
								// we load tables for the current schema or public
								curSchema, _ := data["Schema"].(string)
								if curSchema == "" {
									curSchema = "public"
								}
								if tables, err := pgcatalog.ListTables(ctx, conn.Pool, curSchema); err == nil {
									// store as map[string][]model.Table keyed by schema
									data["SidebarTables"] = tables
									data["SidebarSchema"] = curSchema
								}
								// also try to get schemas list
								if schemas, err := pgcatalog.ListSchemas(ctx, conn.Pool, false); err == nil {
									// filter to first 20 for sidebar
									if len(schemas) > 20 {
										schemas = schemas[:20]
									}
									data["SidebarSchemas"] = schemas
								}
							}
						}
					}
				}
			}
			// also ensure SidebarDatabases type is []model.Database for template
			if dbs, ok := data["SidebarDatabases"]; ok {
				if dbsTyped, ok := dbs.([]model.Database); ok {
					_ = dbsTyped
				}
			}
		}
	}
	s.render(w, name, data)
}
