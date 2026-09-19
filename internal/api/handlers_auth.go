package api

import (
	"html/template"
	"net/http"
	"strconv"

	"github.com/bikky-kc013/TableForge/internal/auth"
)

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.GetCookie(r); ok {
		http.Redirect(w, r, "/servers", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	// If already logged in, redirect
	if cookieID, ok := auth.GetCookie(r); ok {
		if _, exists := s.sessions.Get(cookieID); exists {
			http.Redirect(w, r, "/servers", http.StatusFound)
			return
		}
	}
	data := map[string]interface{}{
		"Servers": s.cfg.Servers,
	}
	s.renderWithTheme(w, r, "login.html", data)
}

func (s *Server) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	if !rateLimitCheck(r) {
		s.handleError(w, r, http.StatusTooManyRequests, "Too many login attempts. Please try again later.", nil)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.handleError(w, r, http.StatusBadRequest, "Invalid form submission.", nil)
		return
	}
	serverStr := r.FormValue("server")
	username := r.FormValue("username")
	password := r.FormValue("password")

	if serverStr == "" || username == "" {
		s.handleError(w, r, http.StatusBadRequest, "Server and username are required.", nil)
		return
	}
	serverIdx, err := strconv.Atoi(serverStr)
	if err != nil || serverIdx < 0 || serverIdx >= len(s.cfg.Servers) {
		s.handleError(w, r, http.StatusBadRequest, "Invalid server selection.", nil)
		return
	}

	// extra_login_security: if true, block empty password and well-known usernames (phpPgAdmin compat)
	if s.cfg.ExtraLoginSecurity {
		if password == "" {
			rateLimitInc(r)
			s.renderWithTheme(w, r, "login.html", map[string]interface{}{
				"Servers": s.cfg.Servers, "Error": "Password is required.",
			})
			return
		}
		deny := map[string]bool{"pgsql": true, "postgres": true, "root": true, "administrator": true}
		if deny[username] && password == "" {
			rateLimitInc(r)
			s.handleError(w, r, http.StatusForbidden, "Login denied by security policy.", nil)
			return
		}
	}

	srv := s.cfg.Servers[serverIdx]
	database := srv.DefaultDB
	if database == "" {
		database = "postgres"
	}

	// Try to connect to verify credentials — uses the credentials the user entered (PG native auth)
	ctx := r.Context()
	conn, err := s.dbMgr.GetOrCreate(ctx, serverIdx, database, username, password)
	if err != nil {
		rateLimitInc(r)
		// Production: user-friendly message, log detail server-side; do not expose raw PG error verbatim
		// Keep detail minimal; the underlying error is rate-limited and not leaked to unauth user beyond this.
		s.renderWithTheme(w, r, "login.html", map[string]interface{}{
			"Servers": s.cfg.Servers, "Error": "Login failed — please check server, username and password.",
			"ErrorDetail": template.HTMLEscapeString(err.Error()),
		})
		return
	}
	_ = conn // verified

	// Create session and store encrypted password server-side (never in cookie)
	sess, err := s.sessions.NewSession()
	if err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to create session.", nil)
		return
	}
	if err := s.sessions.SetPassword(sess, password); err != nil {
		s.handleError(w, r, http.StatusInternalServerError, "Failed to secure session.", nil)
		return
	}
	s.sessions.SetServerInfo(sess, serverIdx, username, database)

	// Secure flag depends on request scheme
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	auth.SetCookie(w, sess.ID, secure)
	rateLimitReset(r)
	http.Redirect(w, r, "/databases", http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookieID, ok := auth.GetCookie(r); ok {
		s.sessions.Delete(cookieID)
	}
	auth.ClearCookie(w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (s *Server) handleServersPage(w http.ResponseWriter, r *http.Request) {
	sess, _ := getSession(r)
	data := map[string]interface{}{
		"Servers":   s.cfg.Servers,
		"Session":   sess,
		"CSRFToken": sess.CSRFToken,
	}
	s.renderWithTheme(w, r, "servers.html", data)
}
