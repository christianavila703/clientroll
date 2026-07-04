package database

import "github.com/google/uuid"

// TenantRLSScope returns bind arguments for queries that set RLS tenant context via:
//
//	WITH _ AS (SELECT set_config('app.current_tenant_id', $1, true))
//
// and reference the tenant UUID as $2. Using separate parameters avoids PostgreSQL
// inferring $1 as text when set_config casts it, which breaks uuid column comparisons.
func TenantRLSScope(tenantID uuid.UUID) (rlsTenantID string, tenantUUID uuid.UUID) {
	return tenantID.String(), tenantID
}
