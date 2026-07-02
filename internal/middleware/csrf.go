package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

// CSRFMiddleware implements double-submit cookie CSRF protection.
type CSRFMiddleware struct{}

// NewCSRFMiddleware creates new CSRF middleware.
func NewCSRFMiddleware() *CSRFMiddleware {
	return &CSRFMiddleware{}
}

const (
	csrfCookieName = "csrf_token"
	csrfHeaderName = "X-CSRF-Token"
)

// Protect validates CSRF tokens on state-changing requests.
func (c *CSRFMiddleware) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read-only methods don't need CSRF
		if isSafeMethod(r.Method) {
			// Set CSRF cookie if not present
			if _, err := r.Cookie(csrfCookieName); err != nil {
				c.setCSRFCookie(w)
			}
			next.ServeHTTP(w, r)
			return
		}

		// Validate: header token must match cookie token
		cookie, err := r.Cookie(csrfCookieName)
		if err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "CSRF cookie missing"})
			return
		}

		headerToken := r.Header.Get(csrfHeaderName)
		if headerToken == "" || headerToken != cookie.Value {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "CSRF token mismatch"})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (c *CSRFMiddleware) setCSRFCookie(w http.ResponseWriter) {
	token := generateCSRFToken()
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false, // Must be readable by JS
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400, // 24 hours
	})
}

func generateCSRFToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func isSafeMethod(method string) bool {
	switch strings.ToUpper(method) {
	case "GET", "HEAD", "OPTIONS":
		return true
	default:
		return false
	}
}
