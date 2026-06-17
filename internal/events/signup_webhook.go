package events

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"clientroll/internal/database"
	"clientroll/internal/models"
	"clientroll/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SignupWebhookService delivers Ignition-style signup webhooks when memberships complete.
type SignupWebhookService struct {
	db            *database.Postgres
	redis         *database.Redis
	encryptor     *database.Encryptor
	tenantRepo    *repository.TenantRepo
	clientRepo    *repository.ClientRepo
	memberRepo    *repository.MembershipRepo
	mtRepo        *repository.MembershipTypeRepo
	agreementRepo *repository.AgreementSnapshotRepo
}

func NewSignupWebhookService(
	db *database.Postgres,
	redis *database.Redis,
	enc *database.Encryptor,
	tenantRepo *repository.TenantRepo,
	clientRepo *repository.ClientRepo,
	memberRepo *repository.MembershipRepo,
	mtRepo *repository.MembershipTypeRepo,
	agreementRepo *repository.AgreementSnapshotRepo,
) *SignupWebhookService {
	return &SignupWebhookService{
		db:            db,
		redis:         redis,
		encryptor:     enc,
		tenantRepo:    tenantRepo,
		clientRepo:    clientRepo,
		memberRepo:    memberRepo,
		mtRepo:        mtRepo,
		agreementRepo: agreementRepo,
	}
}

// SendTest posts a sample signup.completed payload synchronously (for settings UI verification).
func (s *SignupWebhookService) SendTest(ctx context.Context, tenant *models.Tenant, appBaseURL string) (int, string, error) {
	if s == nil || tenant == nil || s.encryptor == nil {
		return 0, "", fmt.Errorf("signup webhook service unavailable")
	}
	if tenant.SignupWebhookURL == nil || strings.TrimSpace(*tenant.SignupWebhookURL) == "" {
		return 0, "", fmt.Errorf("signup webhook URL is not configured")
	}
	if strings.TrimSpace(tenant.SignupWebhookSecretEncrypted) == "" {
		return 0, "", fmt.Errorf("signup webhook signing secret is missing")
	}

	payload := BuildSignupWebhookTestPayload(tenant, appBaseURL)
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return 0, "", fmt.Errorf("failed to build test payload")
	}

	secret, err := s.encryptor.Decrypt(tenant.SignupWebhookSecretEncrypted)
	if err != nil {
		return 0, "", fmt.Errorf("failed to decrypt signing secret")
	}

	ts := time.Now().UTC().Format(time.RFC3339)
	eventID := "evt_test_" + uuid.New().String()[:8]
	signedPayload := append([]byte(ts+"."), payloadJSON...)
	signature := SignPayload(signedPayload, secret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(*tenant.SignupWebhookURL), bytes.NewReader(payloadJSON))
	if err != nil {
		return 0, "", fmt.Errorf("invalid webhook URL")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)
	req.Header.Set("X-Webhook-Event", SignupCompleted)
	req.Header.Set("X-Webhook-Event-Id", eventID)
	req.Header.Set("X-Webhook-Timestamp", ts)
	req.Header.Set("User-Agent", "ClientRoll-Webhook/1.0")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return resp.StatusCode, string(body), nil
}

// NotifyCompleted sends the signup webhook once per membership when configured on the tenant.
func (s *SignupWebhookService) NotifyCompleted(ctx context.Context, tenantID, membershipID uuid.UUID, appBaseURL string) {
	if s == nil || s.db == nil || s.redis == nil {
		return
	}

	marked, err := s.tryMarkSignupWebhookSent(ctx, tenantID, membershipID)
	if err != nil {
		log.Printf("signup webhook mark sent failed tenant=%s membership=%s err=%v", tenantID, membershipID, err)
		return
	}
	if !marked {
		return
	}

	tenant, err := s.tenantRepo.FindByID(ctx, tenantID)
	if err != nil || tenant == nil {
		return
	}
	if !tenant.SignupWebhookEnabled || tenant.SignupWebhookURL == nil || strings.TrimSpace(*tenant.SignupWebhookURL) == "" {
		return
	}
	if strings.TrimSpace(tenant.SignupWebhookSecretEncrypted) == "" {
		log.Printf("signup webhook skipped tenant=%s membership=%s reason=missing_secret", tenantID, membershipID)
		return
	}

	membership, err := s.memberRepo.FindByID(ctx, membershipID, tenantID)
	if err != nil || membership == nil {
		return
	}
	client, err := s.clientRepo.FindByID(ctx, membership.ClientID, tenantID)
	if err != nil || client == nil {
		return
	}
	mt, err := s.mtRepo.FindByID(ctx, membership.MembershipTypeID, tenantID)
	if err != nil || mt == nil {
		return
	}
	var term *models.TermOption
	if to, err := s.mtRepo.GetTermOption(ctx, membership.TermOptionID, tenantID); err == nil {
		term = to
	}
	var snap *repository.AgreementSnapshot
	if s.agreementRepo != nil {
		snap, _ = s.agreementRepo.FindByMembership(ctx, membershipID, tenantID)
	}

	payload := BuildSignupWebhookPayload(tenant, client, membership, mt, term, snap, appBaseURL)
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		log.Printf("signup webhook marshal failed tenant=%s membership=%s err=%v", tenantID, membershipID, err)
		return
	}

	eventID := "evt_" + uuid.New().String()[:12]
	ts := time.Now().UTC().Format(time.RFC3339)

	var deliveryID uuid.UUID
	err = s.db.Pool.QueryRow(ctx, `
		INSERT INTO signup_webhook_deliveries (tenant_id, membership_id, payload, status, attempt_count)
		VALUES ($1, $2, $3::jsonb, 'pending', 0)
		RETURNING id`, tenantID, membershipID, string(payloadJSON)).Scan(&deliveryID)
	if err != nil {
		log.Printf("signup webhook delivery record failed tenant=%s membership=%s err=%v", tenantID, membershipID, err)
		return
	}

	job := DeliveryJob{
		DeliveryKind:    "signup",
		DeliveryID:      deliveryID.String(),
		EndpointID:      deliveryID.String(),
		TenantID:        tenantID.String(),
		URL:             strings.TrimSpace(*tenant.SignupWebhookURL),
		SecretEncrypted: tenant.SignupWebhookSecretEncrypted,
		Payload:         string(payloadJSON),
		EventType:       SignupCompleted,
		EventID:         eventID,
		Timestamp:       ts,
		AttemptCount:    0,
	}
	jobBytes, _ := json.Marshal(job)
	if err := s.redis.Client.LPush(ctx, s.redis.Key("webhook:queue"), string(jobBytes)).Err(); err != nil {
		log.Printf("signup webhook enqueue failed tenant=%s membership=%s err=%v", tenantID, membershipID, err)
	}
}

func (s *SignupWebhookService) tryMarkSignupWebhookSent(ctx context.Context, tenantID, membershipID uuid.UUID) (bool, error) {
	var sentAt time.Time
	err := s.db.Pool.QueryRow(ctx, `
		UPDATE memberships
		SET signup_webhook_sent_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2 AND signup_webhook_sent_at IS NULL
		RETURNING signup_webhook_sent_at`,
		membershipID, tenantID).Scan(&sentAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
