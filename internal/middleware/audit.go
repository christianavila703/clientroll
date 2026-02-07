package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"
)

// AuditMiddleware logs write operations for the audit trail.
// It captures the request body for POST/PUT/PATCH/DELETE requests
// and passes it downstream for the handler to record in the audit_log table.
type AuditMiddleware struct{}

// NewAuditMiddleware creates a new audit middleware.
func NewAuditMiddleware() *AuditMiddleware {
	return &AuditMiddleware{}
}

// LogWrites captures request bodies for write operations.
func (a *AuditMiddleware) LogWrites(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only capture write operations
		if !isWriteMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		// Read and restore the body so downstream handlers can read it too
		if r.Body != nil {
			body, err := io.ReadAll(r.Body)
			if err == nil {
				r.Body = io.NopCloser(bytes.NewBuffer(body))
			}
		}

		next.ServeHTTP(w, r)
	})
}

func isWriteMethod(method string) bool {
	switch strings.ToUpper(method) {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}
