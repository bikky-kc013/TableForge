package auth

import (
	"crypto/subtle"
	"net/http"
)

const CSRFHeader = "X-CSRF-Token"
const CSRFFormField = "csrf_token"

// VerifyCSRF checks token from header or form against session.
func VerifyCSRF(r *http.Request, sess *Session) bool {
	if sess == nil {
		return false
	}
	token := r.Header.Get(CSRFHeader)
	if token == "" {
		token = r.FormValue(CSRFFormField)
	}
	if token == "" {
		return false
	}
	// constant-time compare
	return subtle.ConstantTimeCompare([]byte(token), []byte(sess.CSRFToken)) == 1
}

// CSRFMiddleware enforces token on state-changing methods.
func CSRFMiddleware(store *Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		cookieID, ok := GetCookie(r)
		if !ok {
			http.Error(w, "missing session", http.StatusUnauthorized)
			return
		}
		sess, ok := store.Get(cookieID)
		if !ok {
			http.Error(w, "invalid session", http.StatusUnauthorized)
			return
		}
		if !VerifyCSRF(r, sess) {
			http.Error(w, "invalid CSRF token", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
