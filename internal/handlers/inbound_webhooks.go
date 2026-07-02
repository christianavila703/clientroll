package handlers

import (
	"crypto/hmac"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"clientroll/internal/database"
	"clientroll/internal/events"
	"clientroll/internal/models"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
)

const (
	inboundWebhookProvider = "inbound_webhook"
	maxInboundBodyBytes    = 1 << 20 // 1 MiB
	maxClockSkewSeconds    = 300
)

// InboundWebhookHandler handles public inbound webhook endpoints.
type InboundWebhookHandler struct {
	clientRepo *repository.ClientRepo
	keyRepo    *repository.InboundWebhookKeyRepo
	eventRepo  *repository.InboundWebhookEventRepo
	encryptor  *database.Encryptor
}

// RegisterPlatformRoutes registers internal platform-admin operational routes.
func (h *InboundWebhookHandler) RegisterPlatformRoutes(r chi.Router) {
	r.Get("/webhooks/inbound/failures", h.ListFailedInboundWebhookEvents)
}

// NewInboundWebhookHandler creates a new inbound webhook handler.
func NewInboundWebhookHandler(
	clientRepo *repository.ClientRepo,
	keyRepo *repository.InboundWebhookKeyRepo,
	eventRepo *repository.InboundWebhookEventRepo,
	encryptor *database.Encryptor,
) *InboundWebhookHandler {
	return &InboundWebhookHandler{
		clientRepo: clientRepo,
		keyRepo:    keyRepo,
		eventRepo:  eventRepo,
		encryptor:  encryptor,
	}
}

// RegisterPublicRoutes registers inbound public webhook routes.
func (h *InboundWebhookHandler) RegisterPublicRoutes(r chi.Router) {
	r.Post("/webhooks/inbound/clients", h.CreateClient)
}

// CreateClient handles inbound "create client" webhook events.
func (h *InboundWebhookHandler) CreateClient(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxInboundBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	keyID := strings.TrimSpace(r.Header.Get("X-Webhook-Key-Id"))
	timestamp := strings.TrimSpace(r.Header.Get("X-Webhook-Timestamp"))
	signature := strings.TrimSpace(r.Header.Get("X-Webhook-Signature"))
	if keyID == "" || timestamp == "" || signature == "" {
		writeError(w, http.StatusUnauthorized, "Missing webhook auth headers")
		return
	}

	if !isFreshTimestamp(timestamp, maxClockSkewSeconds) {
		writeError(w, http.StatusUnauthorized, "Stale webhook timestamp")
		return
	}

	key, err := h.keyRepo.FindActiveByKeyID(r.Context(), keyID)
	if err != nil {
		log.Printf("inbound webhook auth lookup failed key_id=%s err=%v", keyID, err)
		writeError(w, http.StatusUnauthorized, "Invalid webhook key")
		return
	}
	if key == nil {
		writeError(w, http.StatusUnauthorized, "Invalid webhook key")
		return
	}

	secret, err := h.encryptor.Decrypt(key.SecretEncrypted)
	if err != nil || secret == "" {
		log.Printf("inbound webhook secret decrypt failed tenant=%s key_id=%s err=%v", key.TenantID, key.KeyID, err)
		writeError(w, http.StatusUnauthorized, "Invalid webhook key")
		return
	}

	expected := events.SignTimestampedPayload(timestamp, body, secret)
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		writeError(w, http.StatusUnauthorized, "Invalid webhook signature")
		return
	}

	_ = h.keyRepo.TouchLastUsed(r.Context(), key.ID)

	var payload struct {
		EventID   string `json:"event_id"`
		EventType string `json:"event_type"`
		Data      struct {
			Name                  string  `json:"name"`
			Address               *string `json:"address"`
			Suburb                *string `json:"suburb"`
			State                 *string `json:"state"`
			Postcode              *string `json:"postcode"`
			Email                 *string `json:"email"`
			Phone                 *string `json:"phone"`
			BirthDate             *string `json:"birth_date"`
			Gender                *string `json:"gender"`
			EmergencyContactName  *string `json:"emergency_contact_name"`
			EmergencyContactPhone *string `json:"emergency_contact_phone"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid webhook payload")
		return
	}

	if payload.EventID == "" || payload.Data.Name == "" {
		writeError(w, http.StatusBadRequest, "event_id and data.name are required")
		return
	}
	if payload.EventType != "" && payload.EventType != events.ClientCreated && payload.EventType != "client.create" {
		writeError(w, http.StatusBadRequest, "Unsupported event_type")
		return
	}

	if h.eventRepo != nil {
		inserted, err := h.eventRepo.RecordReceived(
			r.Context(),
			key.TenantID,
			inboundWebhookProvider,
			payload.EventID,
			"client",
			payload.Data.Name,
		)
		if err != nil {
			log.Printf("inbound webhook dedupe store failed tenant=%s event=%s err=%v", key.TenantID, payload.EventID, err)
			writeError(w, http.StatusInternalServerError, "Failed to process webhook")
			return
		}
		if !inserted {
			writeJSON(w, http.StatusOK, map[string]string{"status": "duplicate"})
			return
		}
	}

	if err := h.clientRepo.DB().SetTenantContext(r.Context(), key.TenantID.String()); err != nil {
		log.Printf("inbound webhook set tenant context failed tenant=%s err=%v", key.TenantID, err)
		if h.eventRepo != nil {
			_ = h.eventRepo.MarkFailed(r.Context(), key.TenantID, inboundWebhookProvider, payload.EventID, err.Error())
		}
		writeError(w, http.StatusInternalServerError, "Failed to process webhook")
		return
	}

	client := models.Client{
		Name:                  payload.Data.Name,
		Address:               payload.Data.Address,
		Suburb:                payload.Data.Suburb,
		State:                 payload.Data.State,
		Postcode:              payload.Data.Postcode,
		Email:                 payload.Data.Email,
		Phone:                 payload.Data.Phone,
		Gender:                payload.Data.Gender,
		EmergencyContactName:  payload.Data.EmergencyContactName,
		EmergencyContactPhone: payload.Data.EmergencyContactPhone,
	}
	if payload.Data.BirthDate != nil && *payload.Data.BirthDate != "" {
		if t, err := time.Parse("2006-01-02", *payload.Data.BirthDate); err == nil {
			client.BirthDate = &t
		}
	}

	created, err := h.clientRepo.Create(r.Context(), key.TenantID, &client)
	if err != nil {
		log.Printf("inbound webhook create client failed tenant=%s key_id=%s err=%v", key.TenantID, key.KeyID, err)
		if h.eventRepo != nil {
			_ = h.eventRepo.MarkFailed(r.Context(), key.TenantID, inboundWebhookProvider, payload.EventID, err.Error())
		}
		writeError(w, http.StatusInternalServerError, "Failed to create client")
		return
	}

	if h.eventRepo != nil {
		_ = h.eventRepo.MarkProcessed(r.Context(), key.TenantID, inboundWebhookProvider, payload.EventID)
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status":    "processed",
		"event_id":  payload.EventID,
		"tenant_id": key.TenantID,
		"client":    created,
	})
}

// ListFailedInboundWebhookEvents returns recent failed custom inbound webhook processing events.
func (h *InboundWebhookHandler) ListFailedInboundWebhookEvents(w http.ResponseWriter, r *http.Request) {
	if h.eventRepo == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"events": []interface{}{}, "count": 0})
		return
	}
	limit := 50
	if s := strings.TrimSpace(r.URL.Query().Get("limit")); s != "" {
		if parsed, err := strconv.Atoi(s); err == nil {
			limit = parsed
		}
	}
	events, err := h.eventRepo.ListFailed(r.Context(), inboundWebhookProvider, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list failed inbound webhook events")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"events": events,
		"count":  len(events),
	})
}

func isFreshTimestamp(ts string, maxSkewSeconds int64) bool {
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return false
	}
	now := time.Now().Unix()
	diff := now - sec
	if diff < 0 {
		diff = -diff
	}
	return diff <= maxSkewSeconds
}
