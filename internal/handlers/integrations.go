package handlers

import (
	"log"
	"net/http"

	"clientroll/internal/database"
	"clientroll/internal/middleware"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// IntegrationsHandler handles per-tenant integration credential management.
type IntegrationsHandler struct {
	credRepo  *repository.TenantCredentialRepo
	encryptor *database.Encryptor
}

// NewIntegrationsHandler creates a new integrations handler.
func NewIntegrationsHandler(cr *repository.TenantCredentialRepo, enc *database.Encryptor) *IntegrationsHandler {
	return &IntegrationsHandler{credRepo: cr, encryptor: enc}
}

// RegisterRoutes registers integration routes.
func (h *IntegrationsHandler) RegisterRoutes(r chi.Router) {
	r.Get("/integrations", h.List)
	r.Put("/integrations/{provider}", h.Upsert)
	r.Delete("/integrations/{provider}", h.Delete)
}

// List returns all configured integrations (keys masked).
func (h *IntegrationsHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))

	creds, err := h.credRepo.ListByTenant(r.Context(), tenantID)
	if err != nil {
		log.Printf("Error listing integrations: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list integrations")
		return
	}

	// Mask keys — only show last 4 chars
	type SafeCred struct {
		Provider    string `json:"provider"`
		Environment string `json:"environment"`
		IsActive    bool   `json:"is_active"`
		HasKey      bool   `json:"has_key"`
		HasSecret   bool   `json:"has_secret"`
		KeyHint     string `json:"key_hint"`
	}

	safe := make([]SafeCred, len(creds))
	for i, c := range creds {
		keyHint := ""
		if c.APIKeyEncrypted != "" {
			decrypted, err := h.encryptor.Decrypt(c.APIKeyEncrypted)
			if err == nil && len(decrypted) > 4 {
				keyHint = "..." + decrypted[len(decrypted)-4:]
			}
		}
		sc := SafeCred{
			Provider:    c.ProviderName,
			Environment: c.Environment,
			IsActive:    c.IsActive,
			HasKey:      c.APIKeyEncrypted != "",
			HasSecret:   c.APISecretEncrypted != "",
			KeyHint:     keyHint,
		}
		safe[i] = sc
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"integrations": safe})
}

// Upsert creates or updates integration credentials.
func (h *IntegrationsHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	provider := chi.URLParam(r, "provider")

	role := middleware.GetRole(r.Context())
	if role != "tenant_admin" {
		writeError(w, http.StatusForbidden, "Only admins can manage integrations")
		return
	}

	var input struct {
		APIKey      string `json:"api_key"`
		APISecret   string `json:"api_secret"`
		Environment string `json:"environment"`
		IsActive    bool   `json:"is_active"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Encrypt credentials
	var encKey, encSecret string
	var err error
	if input.APIKey != "" {
		encKey, err = h.encryptor.Encrypt(input.APIKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Encryption error")
			return
		}
	}
	if input.APISecret != "" {
		encSecret, err = h.encryptor.Encrypt(input.APISecret)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Encryption error")
			return
		}
	}

	// GoCardless: API key only. Allow empty api_key on update (keep existing).
	if provider == "gocardless" {
		env := input.Environment
		if env != "live" && env != "sandbox" {
			env = "sandbox"
		}
		existing, _ := h.credRepo.GetByProvider(r.Context(), tenantID, provider)
		if input.APIKey == "" {
			if existing != nil && existing.APIKeyEncrypted != "" {
				encKey = existing.APIKeyEncrypted
			} else {
				// Allow saving inactive state even before credentials are configured.
				if input.IsActive {
					writeError(w, http.StatusBadRequest, "API key is required to activate GoCardless")
					return
				}
				encKey = ""
			}
		}
		// If currently active but no key was ever configured, prevent impossible state.
		if input.IsActive && encKey == "" {
			if existing == nil || existing.APIKeyEncrypted == "" {
				writeError(w, http.StatusBadRequest, "API key is required to activate GoCardless")
				return
			}
		}
		err = h.credRepo.Upsert(r.Context(), tenantID, provider, encKey, "", env, input.IsActive)
		if err != nil {
			log.Printf("Error saving GoCardless API key: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to save integration")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "Integration saved"})
		return
	}

	err = h.credRepo.Upsert(r.Context(), tenantID, provider, encKey, encSecret, input.Environment, input.IsActive)
	if err != nil {
		log.Printf("Error saving integration: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to save integration")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Integration saved"})
}

// Delete removes integration credentials.
func (h *IntegrationsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	provider := chi.URLParam(r, "provider")

	if err := h.credRepo.Delete(r.Context(), tenantID, provider); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to remove integration")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Integration removed"})
}
