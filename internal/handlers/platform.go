package handlers

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"clientroll/internal/auth"
	"clientroll/internal/database"
	"clientroll/internal/models"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// PlatformHandler handles super-admin tenant management.
type PlatformHandler struct {
	tenantRepo *repository.TenantRepo
	userRepo   *repository.UserRepo
	memberRepo *repository.MembershipRepo
	db         *database.Postgres
}

// NewPlatformHandler creates a new platform handler.
func NewPlatformHandler(db *database.Postgres, tr *repository.TenantRepo, ur *repository.UserRepo, mr *repository.MembershipRepo) *PlatformHandler {
	return &PlatformHandler{tenantRepo: tr, userRepo: ur, memberRepo: mr, db: db}
}

// RegisterRoutes registers platform routes.
func (h *PlatformHandler) RegisterRoutes(r chi.Router) {
	r.Get("/tenants", h.ListTenants)
	r.Post("/tenants", h.CreateTenant)
	r.Get("/tenants/{tenantID}", h.GetTenant)
	r.Put("/tenants/{tenantID}", h.UpdateTenant)
	r.Get("/ops/membership-status-drift", h.GetMembershipStatusDrift)
	r.Post("/ops/membership-status-reconcile", h.ReconcileMembershipStatuses)
	r.Get("/ops/health-snapshot", h.GetOpsHealthSnapshot)
	r.Post("/ops/reconcile-runbook", h.RunOpsReconcileRunbook)
}

// ListTenants returns all tenants.
func (h *PlatformHandler) ListTenants(w http.ResponseWriter, r *http.Request) {
	tenants, err := h.tenantRepo.List(r.Context())
	if err != nil {
		log.Printf("Error listing tenants: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list tenants")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tenants": tenants,
		"count":   len(tenants),
	})
}

// CreateTenant creates a new tenant with an initial admin user.
func (h *PlatformHandler) CreateTenant(w http.ResponseWriter, r *http.Request) {
	var input struct {
		models.CreateTenantInput
		AdminEmail     string `json:"admin_email"`
		AdminPassword  string `json:"admin_password"`
		AdminFirstName string `json:"admin_first_name"`
		AdminLastName  string `json:"admin_last_name"`
	}

	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if input.Name == "" || input.Slug == "" {
		writeError(w, http.StatusBadRequest, "Name and slug are required")
		return
	}
	if input.Timezone == "" {
		input.Timezone = "Australia/Brisbane"
	}
	if input.AdminEmail == "" || input.AdminPassword == "" {
		writeError(w, http.StatusBadRequest, "Admin email and password are required")
		return
	}

	// Check slug uniqueness
	existing, _ := h.tenantRepo.FindBySlug(r.Context(), input.Slug)
	if existing != nil {
		writeError(w, http.StatusConflict, "Tenant slug already taken")
		return
	}

	// Validate password strength
	if err := auth.ValidatePasswordStrength(input.AdminPassword); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Create tenant
	tenant, err := h.tenantRepo.Create(r.Context(), input.CreateTenantInput)
	if err != nil {
		log.Printf("Error creating tenant: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create tenant")
		return
	}

	// Hash password and create admin user
	hash, err := auth.HashPassword(input.AdminPassword)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create admin user")
		return
	}

	user, err := h.userRepo.Create(r.Context(), tenant.ID,
		input.AdminEmail, hash, input.AdminFirstName, input.AdminLastName,
		models.RoleTenantAdmin,
	)
	if err != nil {
		log.Printf("Error creating admin user: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create admin user")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"tenant": tenant,
		"admin":  user.ToResponse(),
	})
}

// GetTenant returns a single tenant by ID.
func (h *PlatformHandler) GetTenant(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "tenantID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant ID")
		return
	}

	tenant, err := h.tenantRepo.FindByID(r.Context(), id)
	if err != nil {
		log.Printf("Error fetching tenant: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to fetch tenant")
		return
	}
	if tenant == nil {
		writeError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	writeJSON(w, http.StatusOK, tenant)
}

// UpdateTenant updates a tenant.
func (h *PlatformHandler) UpdateTenant(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "tenantID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant ID")
		return
	}

	var input models.UpdateTenantInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	tenant, err := h.tenantRepo.Update(r.Context(), id, input)
	if err != nil {
		log.Printf("Error updating tenant: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update tenant")
		return
	}

	writeJSON(w, http.StatusOK, tenant)
}

func (h *PlatformHandler) GetMembershipStatusDrift(w http.ResponseWriter, r *http.Request) {
	if h.memberRepo == nil {
		writeError(w, http.StatusInternalServerError, "Membership repository unavailable")
		return
	}
	tenantID, err := parseOptionalTenantID(r.URL.Query().Get("tenant_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant_id")
		return
	}
	count, err := h.memberRepo.CountLifecycleStatusDrift(r.Context(), tenantID)
	if err != nil {
		log.Printf("platform ops drift count failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to count membership status drift")
		return
	}
	resp := map[string]interface{}{
		"drift_count": count,
	}
	if tenantID != nil {
		resp["tenant_id"] = *tenantID
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *PlatformHandler) ReconcileMembershipStatuses(w http.ResponseWriter, r *http.Request) {
	if h.memberRepo == nil {
		writeError(w, http.StatusInternalServerError, "Membership repository unavailable")
		return
	}
	var input struct {
		TenantID string `json:"tenant_id"`
	}
	_ = decodeJSON(r, &input) // optional
	tenantID, err := parseOptionalTenantID(input.TenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant_id")
		return
	}
	updated, err := h.memberRepo.ReconcileLifecycleStatuses(r.Context(), tenantID)
	if err != nil {
		log.Printf("platform ops reconcile failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to reconcile membership statuses")
		return
	}
	resp := map[string]interface{}{
		"status":  "ok",
		"updated": updated,
	}
	if tenantID != nil {
		resp["tenant_id"] = *tenantID
	}
	writeJSON(w, http.StatusOK, resp)
}

func parseOptionalTenantID(raw string) (*uuid.UUID, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	id, err := uuid.Parse(trimmed)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func (h *PlatformHandler) GetOpsHealthSnapshot(w http.ResponseWriter, r *http.Request) {
	if h.db == nil || h.memberRepo == nil {
		writeError(w, http.StatusInternalServerError, "Operational dependencies unavailable")
		return
	}
	snapshot, err := h.buildOpsSnapshot(r.Context(), nil)
	if err != nil {
		log.Printf("platform ops health snapshot failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to load ops health snapshot")
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (h *PlatformHandler) RunOpsReconcileRunbook(w http.ResponseWriter, r *http.Request) {
	if h.memberRepo == nil {
		writeError(w, http.StatusInternalServerError, "Membership repository unavailable")
		return
	}
	var input struct {
		TenantID string `json:"tenant_id"`
	}
	_ = decodeJSON(r, &input) // optional
	tenantID, err := parseOptionalTenantID(input.TenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant_id")
		return
	}
	before, err := h.memberRepo.CountLifecycleStatusDrift(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to count pre-run drift")
		return
	}
	activated, err := h.memberRepo.ActivateScheduledDue(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to activate scheduled memberships")
		return
	}
	reconciled, err := h.memberRepo.ReconcileLifecycleStatuses(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to reconcile membership statuses")
		return
	}
	after, err := h.memberRepo.CountLifecycleStatusDrift(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to count post-run drift")
		return
	}
	resp := map[string]interface{}{
		"status": "ok",
		"run": map[string]interface{}{
			"ran_at":                          time.Now().UTC().Format(time.RFC3339),
			"activated_scheduled_memberships": activated,
			"reconciled_memberships":          reconciled,
			"drift_before":                    before,
			"drift_after":                     after,
		},
	}
	if tenantID != nil {
		resp["tenant_id"] = *tenantID
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *PlatformHandler) buildOpsSnapshot(ctx context.Context, tenantID *uuid.UUID) (map[string]interface{}, error) {
	var retryQueueDepth int
	var failedDeliveries15m int
	var stuckRetries int
	var serviceMismatch24h int
	var scheduledDue int
	var failedStripe int
	var failedGoCardless int
	var failedInbound int

	if err := h.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM webhook_deliveries
		WHERE status = 'retrying'
	`).Scan(&retryQueueDepth); err != nil {
		return nil, err
	}
	if err := h.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM webhook_deliveries
		WHERE status = 'failed'
		  AND created_at >= NOW() - INTERVAL '15 minutes'
	`).Scan(&failedDeliveries15m); err != nil {
		return nil, err
	}
	if err := h.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM webhook_deliveries
		WHERE status = 'retrying'
		  AND next_retry_at IS NOT NULL
		  AND next_retry_at < NOW() - INTERVAL '10 minutes'
	`).Scan(&stuckRetries); err != nil {
		return nil, err
	}
	if err := h.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM service_charges sc
		WHERE sc.created_at >= NOW() - INTERVAL '24 hours'
		  AND NOT EXISTS (
		    SELECT 1 FROM payments p
		    WHERE p.tenant_id = sc.tenant_id
		      AND p.provider_reference = ('charge-' || sc.id::text)
		  )
	`).Scan(&serviceMismatch24h); err != nil {
		return nil, err
	}
	if err := h.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM memberships m
		JOIN tenants t ON t.id = m.tenant_id
		WHERE m.status = 'scheduled'
		  AND m.start_date <= ((NOW() AT TIME ZONE COALESCE(NULLIF(t.timezone, ''), 'UTC'))::date)
		  AND ($1::uuid IS NULL OR m.tenant_id = $1::uuid)
	`, tenantID).Scan(&scheduledDue); err != nil {
		return nil, err
	}
	if err := h.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM inbound_webhook_events
		WHERE provider = 'stripe' AND status = 'failed'
	`).Scan(&failedStripe); err != nil {
		return nil, err
	}
	if err := h.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM inbound_webhook_events
		WHERE provider = 'gocardless' AND status = 'failed'
	`).Scan(&failedGoCardless); err != nil {
		return nil, err
	}
	if err := h.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM inbound_webhook_events
		WHERE provider = 'inbound_webhook' AND status = 'failed'
	`).Scan(&failedInbound); err != nil {
		return nil, err
	}
	drift, err := h.memberRepo.CountLifecycleStatusDrift(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := map[string]interface{}{
		"as_of": time.Now().UTC().Format(time.RFC3339),
		"webhooks": map[string]interface{}{
			"retry_queue_depth":     retryQueueDepth,
			"failed_deliveries_15m": failedDeliveries15m,
			"stuck_retries":         stuckRetries,
			"failed_inbound_by_provider": map[string]int{
				"stripe":          failedStripe,
				"gocardless":      failedGoCardless,
				"inbound_webhook": failedInbound,
			},
		},
		"memberships": map[string]interface{}{
			"status_drift":        drift,
			"scheduled_due_count": scheduledDue,
		},
		"ledger": map[string]interface{}{
			"service_payment_mismatch_24h": serviceMismatch24h,
		},
	}
	if tenantID != nil {
		out["tenant_id"] = *tenantID
	}
	return out, nil
}
