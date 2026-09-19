package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/phppgadmin/phppgadmin-go/internal/model"
)

// isAPIRequest decides whether to respond with JSON (for /api/* or JSON Accept).
func isAPIRequest(r *http.Request) bool {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		return true
	}
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		return true
	}
	// htmx / fetch
	if r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
		return true
	}
	return false
}

func requestID(r *http.Request) string {
	if id := middleware.GetReqID(r.Context()); id != "" {
		return id
	}
	if id := r.Header.Get("X-Request-ID"); id != "" {
		return id
	}
	return ""
}

// writeAPIError sends a JSON error using the canonical model.ErrorResponse.
func (s *Server) writeAPIError(w http.ResponseWriter, r *http.Request, status int, message, details string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	resp := model.NewErrorResponse(status, message, details, requestID(r))
	_ = json.NewEncoder(w).Encode(resp)
}

// renderErrorPage renders the HTML error page (error.html) using the error view model.
// For API requests it automatically falls back to JSON.
func (s *Server) renderErrorPage(w http.ResponseWriter, r *http.Request, status int, title, message, details string) {
	if isAPIRequest(r) {
		s.writeAPIError(w, r, status, message, details)
		return
	}
	// Log server errors
	if status >= 500 {
		log.Printf("[error] %d %s %s: %s | details: %s | reqID=%s", status, r.Method, r.URL.Path, message, details, requestID(r))
	}
	// Build view data for error.html
	data := map[string]interface{}{
		"Status":    status,
		"Title":     title,
		"Message":   message,
		"Details":   details,
		"RequestID": requestID(r),
		"ShowLogin": status == http.StatusUnauthorized,
	}
	// renderWithTheme will enrich with AppName, Theme, Sidebar, Database etc.
	// We need to set status code before rendering.
	w.WriteHeader(status)
	s.renderWithTheme(w, r, "error.html", data)
}

// handleError is the central helper for handler errors.
// message is user-facing; err provides technical details (may be nil).
func (s *Server) handleError(w http.ResponseWriter, r *http.Request, status int, message string, err error) {
	details := ""
	if err != nil {
		details = err.Error()
	}
	title := model.HTTPStatusText(status)
	s.renderErrorPage(w, r, status, title, message, details)
}

// handleNotFound is registered as chi NotFound handler.
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	s.renderErrorPage(w, r, http.StatusNotFound, "Not Found",
		"The page you requested could not be found.",
		"Path: "+r.URL.Path,
	)
}

// handleMethodNotAllowed is registered as chi MethodNotAllowed handler.
func (s *Server) handleMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	s.renderErrorPage(w, r, http.StatusMethodNotAllowed, "Method Not Allowed",
		"The method "+r.Method+" is not allowed for "+r.URL.Path+".",
		"",
	)
}
