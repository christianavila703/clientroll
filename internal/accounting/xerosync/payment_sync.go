package xerosync

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"clientroll/internal/accounting/xero"
	"clientroll/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) EnsureInvoiceForPayment(ctx context.Context, payment *repository.Payment) (string, error) {
	if payment == nil || !s.IsTenantActive(ctx, payment.TenantID) {
		return "", nil
	}
	m, findErr := s.syncRepo.FindMapping(ctx, payment.TenantID, repository.XeroEntityMembershipPayment, payment.ID)
	if findErr != nil && !errors.Is(findErr, pgx.ErrNoRows) {
		return "", fmt.Errorf("check existing xero mapping: %w", findErr)
	}
	if m != nil {
		return m.XeroObjectID, nil
	}
	contactID, err := s.EnsureContactForClient(ctx, payment.TenantID, payment.ClientID)
	if err != nil {
		return "", err
	}
	call := func(c *xero.Client) (string, error) {
		taxType := "OUTPUT"
		total := payment.BaseAmount + payment.TaxAmount
		if payment.TaxAmount <= 0 {
			taxType = "NONE"
			total = payment.Amount
		}
		invoice, err := c.CreateInvoice(ctx, xero.InvoiceInput{
			Type:            "ACCREC",
			ContactID:       contactID,
			Status:          "AUTHORISED",
			LineAmountTypes: "Inclusive",
			LineItems: []xero.InvoiceLine{
				{
					Description: "Membership payment",
					Quantity:    1,
					UnitAmount:  total,
					AccountCode: s.defaultRevenueAccountCode(ctx, payment.TenantID),
					TaxType:     taxType,
				},
			},
			CurrencyCode:  payment.CurrencyCode,
			DueDateString: dayString(payment.DueDate),
			Reference:     shortRef("CR-", payment.ID),
		})
		if err != nil {
			return "", err
		}
		ref := invoice.InvoiceNumber
		if _, mapErr := s.syncRepo.UpsertMapping(ctx, payment.TenantID, repository.UpsertXeroSyncMappingInput{
			EntityType:    repository.XeroEntityMembershipPayment,
			EntityID:      payment.ID,
			XeroObjectID:  invoice.InvoiceID,
			XeroObjectRef: &ref,
			SyncState:     "invoice_created",
		}); mapErr != nil {
			log.Printf("xero.sync upsert mapping failed tenant=%s payment=%s err=%v", payment.TenantID, payment.ID, mapErr)
		}
		return invoice.InvoiceID, nil
	}
	invoiceID, err := s.withTenantClient(ctx, payment.TenantID, call)
	if err != nil {
		s.recordFailure(ctx, payment.TenantID, repository.XeroEntityMembershipPayment, payment.ID, "ensure_invoice", err)
		return "", err
	}
	_ = s.syncRepo.AppendEvent(ctx, payment.TenantID, repository.XeroEntityMembershipPayment, payment.ID, "ensure_invoice", "success", nil, nil)
	return invoiceID, nil
}

func (s *Service) ApplyPaymentForPayment(ctx context.Context, payment *repository.Payment) error {
	if payment == nil || payment.Status != "succeeded" || payment.PaidDate == nil || !s.IsTenantActive(ctx, payment.TenantID) {
		return nil
	}
	invoiceID, err := s.EnsureInvoiceForPayment(ctx, payment)
	if err != nil {
		return err
	}
	if invoiceID == "" {
		return nil
	}
	if existing, findErr := s.syncRepo.FindMapping(ctx, payment.TenantID, repository.XeroEntityMembershipPayment, payment.ID); findErr == nil && existing != nil && existing.SyncState == "payment_applied" {
		return nil
	}
	call := func(c *xero.Client) (string, error) {
		p, err := c.CreatePayment(ctx, xero.PaymentInput{
			InvoiceID:   invoiceID,
			AccountCode: s.defaultPaymentAccountCode(ctx, payment.TenantID),
			Amount:      payment.Amount,
			DateString:  dayString(*payment.PaidDate),
		})
		if err != nil {
			return "", err
		}
		if _, mapErr := s.syncRepo.UpsertMapping(ctx, payment.TenantID, repository.UpsertXeroSyncMappingInput{
			EntityType:     repository.XeroEntityMembershipPayment,
			EntityID:       payment.ID,
			XeroObjectID:   invoiceID,
			XeroObjectRef:  &p.PaymentID,
			SyncState:      "payment_applied",
			PaidDateAtSync: payment.PaidDate,
		}); mapErr != nil {
			log.Printf("xero.sync upsert mapping failed tenant=%s payment=%s err=%v", payment.TenantID, payment.ID, mapErr)
		}
		return p.PaymentID, nil
	}
	_, err = s.withTenantClient(ctx, payment.TenantID, call)
	if err != nil {
		s.recordFailure(ctx, payment.TenantID, repository.XeroEntityMembershipPayment, payment.ID, "apply_payment", err)
		return err
	}
	_ = s.syncRepo.AppendEvent(ctx, payment.TenantID, repository.XeroEntityMembershipPayment, payment.ID, "apply_payment", "success", nil, nil)
	return nil
}

func (s *Service) EnsureInvoiceForServiceCharge(ctx context.Context, charge *repository.ServiceCharge) (string, error) {
	if charge == nil || !s.IsTenantActive(ctx, charge.TenantID) {
		return "", nil
	}
	m, findErr := s.syncRepo.FindMapping(ctx, charge.TenantID, repository.XeroEntityServiceCharge, charge.ID)
	if findErr != nil && !errors.Is(findErr, pgx.ErrNoRows) {
		return "", fmt.Errorf("check existing xero mapping: %w", findErr)
	}
	if m != nil {
		return m.XeroObjectID, nil
	}
	contactID, err := s.EnsureContactForClient(ctx, charge.TenantID, charge.ClientID)
	if err != nil {
		return "", err
	}
	call := func(c *xero.Client) (string, error) {
		total := charge.Amount + charge.TaxAmount
		taxType := "OUTPUT"
		if charge.TaxAmount <= 0 {
			total = charge.Amount
			taxType = "NONE"
		}
		invoice, err := c.CreateInvoice(ctx, xero.InvoiceInput{
			Type:            "ACCREC",
			ContactID:       contactID,
			Status:          "AUTHORISED",
			LineAmountTypes: "Inclusive",
			LineItems: []xero.InvoiceLine{
				{
					Description: charge.Description,
					Quantity:    1,
					UnitAmount:  total,
					AccountCode: s.defaultRevenueAccountCode(ctx, charge.TenantID),
					TaxType:     taxType,
				},
			},
			CurrencyCode: firstNonEmpty(charge.CurrencyCode, "AUD"),
			Reference:    shortRef("CR-SC-", charge.ID),
		})
		if err != nil {
			return "", err
		}
		ref := invoice.InvoiceNumber
		if _, mapErr := s.syncRepo.UpsertMapping(ctx, charge.TenantID, repository.UpsertXeroSyncMappingInput{
			EntityType:    repository.XeroEntityServiceCharge,
			EntityID:      charge.ID,
			XeroObjectID:  invoice.InvoiceID,
			XeroObjectRef: &ref,
			SyncState:     "invoice_created",
		}); mapErr != nil {
			log.Printf("xero.sync upsert mapping failed tenant=%s charge=%s err=%v", charge.TenantID, charge.ID, mapErr)
		}
		return invoice.InvoiceID, nil
	}
	invoiceID, err := s.withTenantClient(ctx, charge.TenantID, call)
	if err != nil {
		s.recordFailure(ctx, charge.TenantID, repository.XeroEntityServiceCharge, charge.ID, "ensure_invoice", err)
		return "", err
	}
	_ = s.syncRepo.AppendEvent(ctx, charge.TenantID, repository.XeroEntityServiceCharge, charge.ID, "ensure_invoice", "success", nil, nil)
	return invoiceID, nil
}

func (s *Service) ApplyPaymentForServiceCharge(ctx context.Context, charge *repository.ServiceCharge) error {
	if charge == nil || charge.Status != "succeeded" || charge.PaidDate == nil || !s.IsTenantActive(ctx, charge.TenantID) {
		return nil
	}
	invoiceID, err := s.EnsureInvoiceForServiceCharge(ctx, charge)
	if err != nil || invoiceID == "" {
		return err
	}
	if existing, findErr := s.syncRepo.FindMapping(ctx, charge.TenantID, repository.XeroEntityServiceCharge, charge.ID); findErr == nil && existing != nil && existing.SyncState == "payment_applied" {
		return nil
	}
	total := charge.Amount + charge.TaxAmount
	call := func(c *xero.Client) (string, error) {
		p, err := c.CreatePayment(ctx, xero.PaymentInput{
			InvoiceID:   invoiceID,
			AccountCode: s.defaultPaymentAccountCode(ctx, charge.TenantID),
			Amount:      total,
			DateString:  dayString(*charge.PaidDate),
		})
		if err != nil {
			return "", err
		}
		if _, mapErr := s.syncRepo.UpsertMapping(ctx, charge.TenantID, repository.UpsertXeroSyncMappingInput{
			EntityType:     repository.XeroEntityServiceCharge,
			EntityID:       charge.ID,
			XeroObjectID:   invoiceID,
			XeroObjectRef:  &p.PaymentID,
			SyncState:      "payment_applied",
			PaidDateAtSync: charge.PaidDate,
		}); mapErr != nil {
			log.Printf("xero.sync upsert mapping failed tenant=%s charge=%s err=%v", charge.TenantID, charge.ID, mapErr)
		}
		return p.PaymentID, nil
	}
	_, err = s.withTenantClient(ctx, charge.TenantID, call)
	if err != nil {
		s.recordFailure(ctx, charge.TenantID, repository.XeroEntityServiceCharge, charge.ID, "apply_payment", err)
		return err
	}
	_ = s.syncRepo.AppendEvent(ctx, charge.TenantID, repository.XeroEntityServiceCharge, charge.ID, "apply_payment", "success", nil, nil)
	return nil
}

func (s *Service) defaultRevenueAccountCode(ctx context.Context, tenantID uuid.UUID) string {
	conn, err := s.connectionRepo.GetByTenant(ctx, tenantID)
	if err != nil || conn == nil || conn.DefaultRevenueAccountCode == "" {
		return "200"
	}
	return conn.DefaultRevenueAccountCode
}

func (s *Service) defaultPaymentAccountCode(ctx context.Context, tenantID uuid.UUID) string {
	conn, err := s.connectionRepo.GetByTenant(ctx, tenantID)
	if err != nil || conn == nil || conn.DefaultPaymentAccountCode == "" {
		return "090"
	}
	return conn.DefaultPaymentAccountCode
}

func (s *Service) CreateCreditNoteForRefund(ctx context.Context, tenantID uuid.UUID, entityType string, entityID uuid.UUID, contactID string, amount float64, reason string) error {
	if s.cfg != nil && !s.cfg.XeroCreditNotesEnabled {
		return nil
	}
	if amount <= 0 || !s.IsTenantActive(ctx, tenantID) {
		return nil
	}
	call := func(c *xero.Client) (string, error) {
		note, err := c.CreateCreditNote(ctx, xero.CreditNoteInput{
			Type:            "ACCRECCREDIT",
			ContactID:       contactID,
			Status:          "AUTHORISED",
			LineAmountTypes: "Inclusive",
			LineItems: []xero.CreditNoteLine{
				{
					Description: reason,
					Quantity:    1,
					UnitAmount:  amount,
					AccountCode: s.defaultRevenueAccountCode(ctx, tenantID),
					TaxType:     "OUTPUT",
				},
			},
			DateString: dayString(time.Now().UTC()),
			Reference:  shortRef("CR-CN-", entityID),
		})
		if err != nil {
			return "", err
		}
		ref := note.Number
		if _, mapErr := s.syncRepo.UpsertMapping(ctx, tenantID, repository.UpsertXeroSyncMappingInput{
			EntityType:    repository.XeroEntityCreditNote,
			EntityID:      entityID,
			XeroObjectID:  note.CreditNoteID,
			XeroObjectRef: &ref,
			SyncState:     "credit_note_applied",
		}); mapErr != nil {
			log.Printf("xero.sync upsert mapping failed tenant=%s entity=%s err=%v", tenantID, entityID, mapErr)
		}
		return note.CreditNoteID, nil
	}
	_, err := s.withTenantClient(ctx, tenantID, call)
	if err != nil {
		s.recordFailure(ctx, tenantID, entityType, entityID, "create_credit_note", err)
		return err
	}
	_ = s.syncRepo.AppendEvent(ctx, tenantID, entityType, entityID, "create_credit_note", "success", nil, nil)
	return nil
}

func (s *Service) CreateCreditNoteForMembershipPayment(ctx context.Context, payment *repository.Payment, amount float64, reason string) error {
	if payment == nil {
		return nil
	}
	contactID, err := s.EnsureContactForClient(ctx, payment.TenantID, payment.ClientID)
	if err != nil {
		return err
	}
	return s.CreateCreditNoteForRefund(ctx, payment.TenantID, repository.XeroEntityMembershipPayment, payment.ID, contactID, amount, reason)
}

func (s *Service) CreateCreditNoteForServiceCharge(ctx context.Context, charge *repository.ServiceCharge, amount float64, reason string) error {
	if charge == nil {
		return nil
	}
	contactID, err := s.EnsureContactForClient(ctx, charge.TenantID, charge.ClientID)
	if err != nil {
		return err
	}
	return s.CreateCreditNoteForRefund(ctx, charge.TenantID, repository.XeroEntityServiceCharge, charge.ID, contactID, amount, reason)
}

func (s *Service) VoidInvoiceForPaymentIfUnpaid(ctx context.Context, payment *repository.Payment, reason string) error {
	if payment == nil || !s.IsTenantActive(ctx, payment.TenantID) {
		return nil
	}
	mapping, err := s.syncRepo.FindMapping(ctx, payment.TenantID, repository.XeroEntityMembershipPayment, payment.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("check existing xero mapping for void: %w", err)
	}
	if mapping == nil || strings.TrimSpace(mapping.XeroObjectID) == "" {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(payment.Status)) {
	case "succeeded", "refunded", "disputed":
		return nil
	}
	call := func(c *xero.Client) (string, error) {
		updated, err := c.UpdateInvoiceStatus(ctx, mapping.XeroObjectID, "VOIDED")
		if err != nil {
			return "", err
		}
		ref := firstNonEmpty(updated.InvoiceNumber, reason)
		if _, mapErr := s.syncRepo.UpsertMapping(ctx, payment.TenantID, repository.UpsertXeroSyncMappingInput{
			EntityType:    repository.XeroEntityMembershipPayment,
			EntityID:      payment.ID,
			XeroObjectID:  mapping.XeroObjectID,
			XeroObjectRef: &ref,
			SyncState:     "synced",
		}); mapErr != nil {
			log.Printf("xero.sync upsert mapping failed tenant=%s payment=%s err=%v", payment.TenantID, payment.ID, mapErr)
		}
		return updated.InvoiceID, nil
	}
	_, err = s.withTenantClient(ctx, payment.TenantID, call)
	if err != nil {
		s.recordFailure(ctx, payment.TenantID, repository.XeroEntityMembershipPayment, payment.ID, "void_invoice", err)
		return err
	}
	_ = s.syncRepo.AppendEvent(ctx, payment.TenantID, repository.XeroEntityMembershipPayment, payment.ID, "void_invoice", "success", nil, nil)
	return nil
}

