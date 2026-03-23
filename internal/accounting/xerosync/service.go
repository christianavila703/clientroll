package xerosync

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"clientroll/internal/accounting/xero"
	"clientroll/internal/config"
	"clientroll/internal/database"
	"clientroll/internal/models"
	"clientroll/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Service struct {
	cfg            *config.Config
	encryptor      *database.Encryptor
	authClient     *xero.AuthClient
	connectionRepo *repository.XeroConnectionRepo
	syncRepo       *repository.XeroSyncRepo
	clientRepo     *repository.ClientRepo
}

func NewService(
	cfg *config.Config,
	encryptor *database.Encryptor,
	connectionRepo *repository.XeroConnectionRepo,
	syncRepo *repository.XeroSyncRepo,
	clientRepo *repository.ClientRepo,
) *Service {
	return &Service{
		cfg:            cfg,
		encryptor:      encryptor,
		authClient:     xero.NewAuthClient(nil),
		connectionRepo: connectionRepo,
		syncRepo:       syncRepo,
		clientRepo:     clientRepo,
	}
}

func (s *Service) IsConfigured() bool {
	return s.cfg != nil &&
		s.cfg.XeroSyncEnabled &&
		strings.TrimSpace(s.cfg.XeroClientID) != "" &&
		strings.TrimSpace(s.cfg.XeroClientSecret) != ""
}

func (s *Service) IsTenantActive(ctx context.Context, tenantID uuid.UUID) bool {
	if !s.IsConfigured() || s.connectionRepo == nil {
		return false
	}
	c, err := s.connectionRepo.GetByTenant(ctx, tenantID)
	return err == nil && c != nil && c.IsActive && !c.NeedsReauth && c.AccessTokenEncrypted != ""
}

func (s *Service) EnsureContactForClient(ctx context.Context, tenantID, clientID uuid.UUID) (string, error) {
	if !s.IsTenantActive(ctx, tenantID) {
		return "", nil
	}

	m, findErr := s.syncRepo.FindMapping(ctx, tenantID, repository.XeroEntityContact, clientID)
	if findErr != nil && !errors.Is(findErr, pgx.ErrNoRows) {
		return "", fmt.Errorf("check existing xero contact mapping: %w", findErr)
	}
	if m != nil {
		return m.XeroObjectID, nil
	}

	client, err := s.clientRepo.FindByID(ctx, clientID, tenantID)
	if err != nil {
		return "", fmt.Errorf("load client for xero contact: %w", err)
	}

	call := func(c *xero.Client) (string, error) {
		in := xero.ContactInput{Name: client.Name}
		if client.Email != nil && strings.TrimSpace(*client.Email) != "" {
			email := strings.TrimSpace(*client.Email)
			in.EmailAddress = &email
		}
		contact, err := c.UpsertContact(ctx, in)
		if err != nil {
			return "", err
		}
		return contact.ContactID, nil
	}

	contactID, err := s.withTenantClient(ctx, tenantID, call)
	if err != nil {
		s.recordFailure(ctx, tenantID, repository.XeroEntityContact, clientID, "ensure_contact", err)
		return "", err
	}
	_, mapErr := s.syncRepo.UpsertMapping(ctx, tenantID, repository.UpsertXeroSyncMappingInput{
		EntityType:   repository.XeroEntityContact,
		EntityID:     clientID,
		XeroObjectID: contactID,
		SyncState:    "synced",
	})
	if mapErr != nil {
		log.Printf("xero.sync contact mapping failed tenant=%s client=%s err=%v", tenantID, clientID, mapErr)
	}
	_ = s.syncRepo.AppendEvent(ctx, tenantID, repository.XeroEntityContact, clientID, "ensure_contact", "success", nil, nil)
	return contactID, nil
}

func (s *Service) withTenantClient(ctx context.Context, tenantID uuid.UUID, call func(c *xero.Client) (string, error)) (string, error) {
	if s.connectionRepo == nil || s.encryptor == nil {
		return "", fmt.Errorf("xero service unavailable")
	}
	conn, err := s.connectionRepo.GetByTenant(ctx, tenantID)
	if err != nil {
		return "", err
	}
	if !conn.IsActive || conn.NeedsReauth {
		return "", nil
	}
	accessToken, err := s.encryptor.Decrypt(conn.AccessTokenEncrypted)
	if err != nil {
		return "", err
	}
	client := xero.NewClient(nil, accessToken, conn.XeroTenantID)
	result, err := call(client)
	if err == nil {
		return result, nil
	}
	apiErr, ok := err.(*xero.APIError)
	if !ok || apiErr.StatusCode != 401 {
		return "", err
	}
	if strings.TrimSpace(conn.RefreshTokenEncrypted) == "" || !s.IsConfigured() {
		_ = s.connectionRepo.SetNeedsReauth(ctx, tenantID, true)
		return "", err
	}
	refreshToken, decErr := s.encryptor.Decrypt(conn.RefreshTokenEncrypted)
	if decErr != nil {
		return "", decErr
	}
	tokenResp, refreshErr := s.authClient.RefreshToken(ctx, s.cfg.XeroClientID, s.cfg.XeroClientSecret, refreshToken)
	if refreshErr != nil {
		_ = s.connectionRepo.SetNeedsReauth(ctx, tenantID, true)
		return "", refreshErr
	}
	newAccess, err := s.encryptor.Encrypt(tokenResp.AccessToken)
	if err != nil {
		return "", err
	}
	newRefresh, err := s.encryptor.Encrypt(tokenResp.RefreshToken)
	if err != nil {
		return "", err
	}
	_, err = s.connectionRepo.UpsertByTenant(ctx, tenantID, repository.UpsertXeroConnectionInput{
		AccessTokenEncrypted:      newAccess,
		RefreshTokenEncrypted:     newRefresh,
		TokenExpiry:               tokenResp.Expiry(time.Now().UTC()),
		XeroTenantID:              conn.XeroTenantID,
		XeroTenantName:            conn.XeroTenantName,
		DefaultRevenueAccountCode: conn.DefaultRevenueAccountCode,
		DefaultPaymentAccountCode: conn.DefaultPaymentAccountCode,
		IsActive:                  true,
		NeedsReauth:               false,
	})
	if err != nil {
		return "", err
	}
	client = xero.NewClient(nil, tokenResp.AccessToken, conn.XeroTenantID)
	return call(client)
}

func dayString(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

func shortRef(prefix string, id uuid.UUID) string {
	s := strings.ReplaceAll(id.String(), "-", "")
	if len(s) > 8 {
		s = s[:8]
	}
	return prefix + s
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}

func (s *Service) recordFailure(ctx context.Context, tenantID uuid.UUID, entityType string, entityID uuid.UUID, action string, err error) {
	if s.syncRepo == nil {
		return
	}
	body := err.Error()
	code := "xero_error"
	_ = s.syncRepo.AppendEvent(ctx, tenantID, entityType, entityID, action, "failed", &code, &body)
}

func clientName(client *models.Client) string {
	if client == nil {
		return "ClientRoll Charge"
	}
	name := strings.TrimSpace(client.Name)
	if name != "" {
		return name
	}
	return "ClientRoll Charge"
}
