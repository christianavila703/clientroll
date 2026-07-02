package middleware

import (
	"net/http"

	"clientroll/internal/database"
)

// TenantMiddleware sets the PostgreSQL RLS session variable for tenant isolation.
type TenantMiddleware struct {
	db *database.Postgres
}

// NewTenantMiddleware creates a new tenant middleware.
func NewTenantMiddleware(db *database.Postgres) *TenantMiddleware {
	return &TenantMiddleware{db: db}
}

// SetTenantContext sets the RLS tenant context on the database connection.
// This MUST run after AuthMiddleware so that CtxTenantID is populated.
func (t *TenantMiddleware) SetTenantContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := GetTenantID(r.Context())
		if tenantID == "" {
			// Platform admins don't have a tenant_id -- skip RLS
			next.ServeHTTP(w, r)
			return
		}

		// Set the RLS session variable for this request
		if err := t.db.SetTenantContext(r.Context(), tenantID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Failed to set tenant context",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireTenant rejects requests that don't have a tenant context.
func (t *TenantMiddleware) RequireTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := GetTenantID(r.Context())
		if tenantID == "" {
			writeJSON(w, http.StatusForbidden, map[string]string{
				"error": "Tenant context required",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}
