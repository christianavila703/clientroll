package handlers

import (
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"clientroll/internal/database"
	"clientroll/internal/events"
	"clientroll/internal/middleware"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// InboundWebhookKeyHandler manages tenant inbound webhook API keys.
type InboundWebhookKeyHandler struct {
	keyRepo   *repository.InboundWebhookKeyRepo
	encryptor *database.Encryptor
}

// NewInboundWebhookKeyHandler creates a new inbound webhook key handler.
func NewInboundWebhookKeyHandler(keyRepo *repository.InboundWebhookKeyRepo, encryptor *database.Encryptor) *InboundWebhookKeyHandler {
	return &InboundWebhookKeyHandler{keyRepo: keyRepo, encryptor: encryptor}
}

// RegisterRoutes registers tenant-scoped inbound webhook key routes.
func (h *InboundWebhookKeyHandler) RegisterRoutes(r chi.Router) {
	r.Get("/webhooks/inbound/keys", h.ListKeys)
	r.Post("/webhooks/inbound/keys", h.CreateKey)
	r.Post("/webhooks/inbound/keys/{keyID}/rotate", h.RotateKey)
	r.Delete("/webhooks/inbound/keys/{keyID}", h.RevokeKey)
}

// ListKeys returns all inbound webhook keys for the tenant (without secrets).
func (h *InboundWebhookKeyHandler) ListKeys(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusForbidden, "Tenant context required")
		return
	}

	keys, err := h.keyRepo.ListByTenant(r.Context(), tenantID)
	if err != nil {
		log.Printf("inbound webhook keys: list failed tenant=%s err=%v", tenantID, err)
		writeError(w, http.StatusInternalServerError, "Failed to list inbound webhook keys")
		return
	}

	type item struct {
		KeyID      string  `json:"key_id"`
		Label      *string `json:"label,omitempty"`
		IsActive   bool    `json:"is_active"`
		LastUsedAt *string `json:"last_used_at,omitempty"`
		CreatedAt  string  `json:"created_at"`
		RevokedAt  *string `json:"revoked_at,omitempty"`
	}

	out := make([]item, len(keys))
	for i, key := range keys {
		var lastUsedAt *string
		if key.LastUsedAt != nil {
			s := key.LastUsedAt.UTC().Format(time.RFC3339)
			lastUsedAt = &s
		}
		var revokedAt *string
		if key.RevokedAt != nil {
			s := key.RevokedAt.UTC().Format(time.RFC3339)
			revokedAt = &s
		}
		out[i] = item{
			KeyID:      key.KeyID,
			Label:      key.Label,
			IsActive:   key.IsActive,
			LastUsedAt: lastUsedAt,
			CreatedAt:  key.CreatedAt.UTC().Format(time.RFC3339),
			RevokedAt:  revokedAt,
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"keys":  out,
		"count": len(out),
	})
}

// CreateKey creates a new inbound webhook key and returns its secret once.
func (h *InboundWebhookKeyHandler) CreateKey(w http.ResponseWriter, r *http.Request) {
	if middleware.GetRole(r.Context()) != "tenant_admin" {
		writeError(w, http.StatusForbidden, "Only admins can manage inbound webhook keys")
		return
	}

	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusForbidden, "Tenant context required")
		return
	}

	var input struct {
		Label *string `json:"label"`
	}
	if err := decodeJSON(r, &input); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	keyID := events.GenerateInboundKeyID()
	secret := events.GenerateInboundSigningSecret()
	encSecret, err := h.encryptor.Encrypt(secret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create key")
		return
	}

	key, err := h.keyRepo.Create(r.Context(), tenantID, keyID, encSecret, input.Label)
	if err != nil {
		log.Printf("inbound webhook keys: create failed tenant=%s err=%v", tenantID, err)
		writeError(w, http.StatusInternalServerError, "Failed to create inbound webhook key")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"key": map[string]interface{}{
			"key_id":     key.KeyID,
			"label":      key.Label,
			"is_active":  key.IsActive,
			"created_at": key.CreatedAt.UTC().Format(time.RFC3339),
		},
		"api_key":        key.KeyID + "." + secret,
		"key_id":         key.KeyID,
		"signing_secret": secret,
	})
}

// RevokeKey deactivates an inbound webhook key.
func (h *InboundWebhookKeyHandler) RevokeKey(w http.ResponseWriter, r *http.Request) {
	if middleware.GetRole(r.Context()) != "tenant_admin" {
		writeError(w, http.StatusForbidden, "Only admins can manage inbound webhook keys")
		return
	}

	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusForbidden, "Tenant context required")
		return
	}
	keyID := chi.URLParam(r, "keyID")
	if keyID == "" {
		writeError(w, http.StatusBadRequest, "keyID is required")
		return
	}

	if err := h.keyRepo.RevokeByKeyID(r.Context(), tenantID, keyID); err != nil {
		log.Printf("inbound webhook keys: revoke failed tenant=%s key_id=%s err=%v", tenantID, keyID, err)
		writeError(w, http.StatusInternalServerError, "Failed to revoke inbound webhook key")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Inbound webhook key revoked"})
}

// RotateKey revokes an existing key and creates a replacement.
func (h *InboundWebhookKeyHandler) RotateKey(w http.ResponseWriter, r *http.Request) {
	if middleware.GetRole(r.Context()) != "tenant_admin" {
		writeError(w, http.StatusForbidden, "Only admins can manage inbound webhook keys")
		return
	}

	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusForbidden, "Tenant context required")
		return
	}
	oldKeyID := chi.URLParam(r, "keyID")
	if oldKeyID == "" {
		writeError(w, http.StatusBadRequest, "keyID is required")
		return
	}

	existing, err := h.keyRepo.FindByTenantAndKeyID(r.Context(), tenantID, oldKeyID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to rotate inbound webhook key")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "Inbound webhook key not found")
		return
	}

	var input struct {
		Label *string `json:"label"`
	}
	if err := decodeJSON(r, &input); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.keyRepo.RevokeByKeyID(r.Context(), tenantID, oldKeyID); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to rotate inbound webhook key")
		return
	}

	newKeyID := events.GenerateInboundKeyID()
	secret := events.GenerateInboundSigningSecret()
	encSecret, err := h.encryptor.Encrypt(secret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to rotate inbound webhook key")
		return
	}

	label := input.Label
	if label == nil {
		label = existing.Label
	}
	created, err := h.keyRepo.Create(r.Context(), tenantID, newKeyID, encSecret, label)
	if err != nil {
		log.Printf("inbound webhook keys: rotate create failed tenant=%s old_key=%s err=%v", tenantID, oldKeyID, err)
		writeError(w, http.StatusInternalServerError, "Failed to rotate inbound webhook key")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":    "Inbound webhook key rotated",
		"old_key_id": oldKeyID,
		"key": map[string]interface{}{
			"key_id":     created.KeyID,
			"label":      created.Label,
			"is_active":  created.IsActive,
			"created_at": created.CreatedAt.UTC().Format(time.RFC3339),
		},
		"api_key":        created.KeyID + "." + secret,
		"key_id":         created.KeyID,
		"signing_secret": secret,
	})
}
