package handlers

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"clientroll/internal/database"
	"clientroll/internal/events"
	"clientroll/internal/middleware"
	"clientroll/internal/models"
	"clientroll/internal/payment"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/subscription"
)

// ActiveMembershipsHandler serves the active memberships view.
type ActiveMembershipsHandler struct {
	db            *database.Postgres
	tenantRepo    *repository.TenantRepo
	memberRepo    *repository.MembershipRepo
	mtRepo        *repository.MembershipTypeRepo
	clientRepo    *repository.ClientRepo
	paymentRepo   *repository.PaymentRepo
	credRepo      *repository.TenantCredentialRepo
	encryptor     *database.Encryptor
	agreementRepo *repository.AgreementSnapshotRepo
	settingsRepo  *repository.SettingsRepo
	termsRepo     *repository.TermsRepo
	dispatcher    *events.Dispatcher
}

// NewActiveMembershipsHandler creates a new handler.
func NewActiveMembershipsHandler(
	db *database.Postgres,
	tenantR *repository.TenantRepo,
	mr *repository.MembershipRepo,
	mtRepo *repository.MembershipTypeRepo,
	cr *repository.ClientRepo,
	pr *repository.PaymentRepo,
	credR *repository.TenantCredentialRepo,
	enc *database.Encryptor,
	agreementRepo *repository.AgreementSnapshotRepo,
	settingsRepo *repository.SettingsRepo,
	termsRepo *repository.TermsRepo,
) *ActiveMembershipsHandler {
	return &ActiveMembershipsHandler{
		db: db, tenantRepo: tenantR, memberRepo: mr, mtRepo: mtRepo, clientRepo: cr, paymentRepo: pr, credRepo: credR, encryptor: enc,
		agreementRepo: agreementRepo, settingsRepo: settingsRepo, termsRepo: termsRepo,
	}
}

// SetDispatcher sets optional outbound webhook event dispatcher.
func (h *ActiveMembershipsHandler) SetDispatcher(dispatcher *events.Dispatcher) {
	h.dispatcher = dispatcher
}

// RegisterRoutes registers routes.
func (h *ActiveMembershipsHandler) RegisterRoutes(r chi.Router) {
	r.Get("/active-memberships", h.List)
	r.Post("/memberships", h.CreateMembership)
	r.Post("/memberships/{membershipID}/cancel", h.CancelMembership)
	r.Put("/memberships/{membershipID}/continue-after-term", h.UpdateContinueAfterTerm)
}

// CreateMembership creates a membership for a client (staff-initiated).
// If bank details (bsb + account_number) are provided and GoCardless is configured,
// it also creates a GoCardless customer + mandate and links them.
func (h *ActiveMembershipsHandler) CreateMembership(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	userID, _ := uuid.Parse(middleware.GetUserID(r.Context()))

	var input struct {
		ClientID             string  `json:"client_id"`
		MembershipTypeID     string  `json:"membership_type_id"`
		TermOptionID         string  `json:"term_option_id"`
		StartDate            string  `json:"start_date"`
		PaymentMethod        string  `json:"payment_method"`
		PaymentFrequency     string  `json:"payment_frequency"`
		PaymentIntervalUnit  string  `json:"payment_interval_unit,omitempty"`
		PaymentIntervalCount int     `json:"payment_interval_count,omitempty"`
		PaymentAmount        float64 `json:"payment_amount"`
		NumPayments          *int    `json:"num_payments,omitempty"`
		PaymentMethodID      string  `json:"payment_method_id,omitempty"`
		GoCardlessMandateID  string  `json:"gocardless_mandate_id,omitempty"`
		// Bank details for GoCardless mandate setup
		AccountHolderName string `json:"account_holder_name,omitempty"`
		AccountNumber     string `json:"account_number,omitempty"`
		BSB               string `json:"bsb,omitempty"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	input.ClientID = strings.TrimSpace(input.ClientID)
	input.MembershipTypeID = strings.TrimSpace(input.MembershipTypeID)
	input.TermOptionID = strings.TrimSpace(input.TermOptionID)
	input.StartDate = strings.TrimSpace(input.StartDate)
	input.PaymentMethod = strings.TrimSpace(input.PaymentMethod)
	input.PaymentFrequency = strings.TrimSpace(strings.ToLower(input.PaymentFrequency))
	input.PaymentIntervalUnit = strings.TrimSpace(strings.ToLower(input.PaymentIntervalUnit))
	input.PaymentMethodID = strings.TrimSpace(input.PaymentMethodID)
	input.GoCardlessMandateID = strings.TrimSpace(input.GoCardlessMandateID)
	if input.PaymentIntervalCount <= 0 {
		input.PaymentIntervalCount = 1
	}
	if input.PaymentIntervalUnit != "" && input.PaymentIntervalUnit != "week" && input.PaymentIntervalUnit != "month" {
		writeError(w, http.StatusBadRequest, "payment_interval_unit must be 'week' or 'month'")
		return
	}

	if input.ClientID == "" || input.MembershipTypeID == "" || input.TermOptionID == "" || input.StartDate == "" {
		writeError(w, http.StatusBadRequest, "Missing required fields")
		return
	}
	if input.PaymentMethod == "" || input.PaymentFrequency == "" || input.PaymentAmount <= 0 {
		writeError(w, http.StatusBadRequest, "Invalid payment details")
		return
	}

	clientID, err := uuid.Parse(input.ClientID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid client_id")
		return
	}
	mtID, err := uuid.Parse(input.MembershipTypeID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid membership_type_id")
		return
	}
	toID, err := uuid.Parse(input.TermOptionID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid term_option_id")
		return
	}
	startDate, err := time.Parse("2006-01-02", input.StartDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid start_date")
		return
	}

	mt, err := h.mtRepo.FindByID(r.Context(), mtID, tenantID)
	if err != nil || mt == nil {
		writeError(w, http.StatusNotFound, "Membership type not found")
		return
	}
	term, err := h.mtRepo.GetTermOption(r.Context(), toID, tenantID)
	if err != nil || term == nil {
		writeError(w, http.StatusBadRequest, "Term option not found")
		return
	}
	if term.MembershipTypeID != mtID {
		writeError(w, http.StatusBadRequest, "term_option_id does not belong to membership_type_id")
		return
	}
	canonical, err := deriveCanonicalMembershipPricing(term, input.PaymentFrequency)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if math.Abs(input.PaymentAmount-canonical.Amount) > 0.01 {
		writeError(w, http.StatusBadRequest, "payment_amount does not match selected term option")
		return
	}
	if input.NumPayments != nil && canonical.NumPayments != nil && *input.NumPayments != *canonical.NumPayments {
		writeError(w, http.StatusBadRequest, "num_payments does not match selected term option")
		return
	}
	if input.PaymentIntervalUnit != "" && canonical.IntervalUnit != nil && input.PaymentIntervalUnit != *canonical.IntervalUnit {
		writeError(w, http.StatusBadRequest, "payment_interval_unit does not match selected term option")
		return
	}
	if input.PaymentIntervalCount > 0 && canonical.IntervalCount > 0 && input.PaymentIntervalCount != canonical.IntervalCount {
		writeError(w, http.StatusBadRequest, "payment_interval_count does not match selected term option")
		return
	}
	input.PaymentFrequency = canonical.Frequency
	input.PaymentAmount = canonical.Amount
	input.PaymentIntervalCount = canonical.IntervalCount
	input.NumPayments = canonical.NumPayments
	if canonical.IntervalUnit != nil {
		input.PaymentIntervalUnit = *canonical.IntervalUnit
	} else {
		input.PaymentIntervalUnit = ""
	}
	membershipCurrency := strings.ToUpper(strings.TrimSpace(mt.CurrencyCode))
	if membershipCurrency == "" {
		membershipCurrency = "AUD"
	}
	m, err := h.memberRepo.Create(r.Context(), tenantID, clientID, mtID, toID,
		startDate, input.PaymentMethod, input.PaymentFrequency, membershipCurrency, input.PaymentAmount,
		func() *string {
			if input.PaymentIntervalUnit == "" {
				return nil
			}
			return &input.PaymentIntervalUnit
		}(), input.PaymentIntervalCount, input.NumPayments, term.ContinueAfterTerm,
		"", &userID, "")
	if err != nil {
		log.Printf("Error creating membership: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create membership")
		return
	}
	if h.dispatcher != nil {
		h.dispatcher.Dispatch(r.Context(), events.NewEvent(events.MembershipCreated, tenantID, map[string]interface{}{
			"membership_id":      m.ID,
			"client_id":          m.ClientID,
			"membership_type_id": m.MembershipTypeID,
			"term_option_id":     m.TermOptionID,
			"status":             m.Status,
			"source":             "staff",
		}))
	}
	if (strings.EqualFold(input.PaymentMethod, "card") || strings.EqualFold(input.PaymentMethod, "direct_debit")) && input.PaymentMethodID != "" {
		if secretKey, keyErr := h.getStripeSecretKey(r.Context(), tenantID); keyErr == nil {
			stripe.Key = secretKey
			client, _ := h.clientRepo.FindByID(r.Context(), clientID, tenantID)
			if client != nil {
				stripeCustomerID := ""
				if client.StripeCustomerID != nil {
					stripeCustomerID = strings.TrimSpace(*client.StripeCustomerID)
				}
				if stripeCustomerID == "" {
					if fromHistory, _ := h.clientRepo.FindLatestStripeCustomerIDFromHistory(r.Context(), clientID, tenantID); strings.TrimSpace(fromHistory) != "" {
						stripeCustomerID = strings.TrimSpace(fromHistory)
						_ = h.clientRepo.SetStripeCustomerID(r.Context(), clientID, tenantID, stripeCustomerID)
					}
				}
				if stripeCustomerID == "" {
					cparams := &stripe.CustomerParams{
						Name: stripe.String(client.Name),
						Metadata: map[string]string{
							"tenant_id": tenantID.String(),
							"client_id": clientID.String(),
						},
					}
					if client.Email != nil && strings.TrimSpace(*client.Email) != "" {
						cparams.Email = stripe.String(strings.TrimSpace(*client.Email))
					}
					if cust, cErr := customer.New(cparams); cErr == nil && cust != nil && strings.TrimSpace(cust.ID) != "" {
						stripeCustomerID = strings.TrimSpace(cust.ID)
						_ = h.clientRepo.SetStripeCustomerID(r.Context(), clientID, tenantID, stripeCustomerID)
					}
				}
				if stripeCustomerID != "" {
					_, _ = customer.Update(stripeCustomerID, &stripe.CustomerParams{
						InvoiceSettings: &stripe.CustomerInvoiceSettingsParams{
							DefaultPaymentMethod: stripe.String(input.PaymentMethodID),
						},
					})
				}
			}
		}
	}

	// --- GoCardless: create customer + mandate if bank details provided ---
	if input.BSB != "" && input.AccountNumber != "" {
		prov, provErr := h.getGoCardlessProvider(r.Context(), tenantID)
		if provErr == nil && prov != nil {
			client, _ := h.clientRepo.FindByID(r.Context(), clientID, tenantID)
			if client != nil {
				gcCustomerID := ""
				if client.GoCardlessCustomerID != nil && *client.GoCardlessCustomerID != "" {
					gcCustomerID = *client.GoCardlessCustomerID
				} else {
					clientEmail := ""
					if client.Email != nil {
						clientEmail = *client.Email
					}
					clientPhone := ""
					if client.Phone != nil {
						clientPhone = *client.Phone
					}
					cid, err := prov.CreateCustomer(r.Context(), payment.CreateCustomerInput{
						Email:     clientEmail,
						Name:      client.Name,
						Phone:     clientPhone,
						Reference: client.ID.String(),
					})
					if err != nil {
						log.Printf("GoCardless CreateCustomer (staff signup): %v", err)
					} else {
						gcCustomerID = cid
						_ = h.clientRepo.SetGoCardlessCustomerID(r.Context(), client.ID, tenantID, cid)
					}
				}
				if gcCustomerID != "" {
					holderName := input.AccountHolderName
					if holderName == "" {
						holderName = client.Name
					}
					mandateID, err := prov.SetupDirectDebit(r.Context(), payment.SetupDirectDebitInput{
						CustomerID:        gcCustomerID,
						AccountHolderName: holderName,
						AccountNumber:     input.AccountNumber,
						BSB:               input.BSB,
						Reference:         m.ID.String(),
					})
					if err != nil {
						log.Printf("GoCardless SetupDirectDebit (staff signup): %v", err)
					} else {
						_ = h.memberRepo.SetGoCardlessMandateID(r.Context(), m.ID, tenantID, mandateID)
						log.Printf("GoCardless mandate %s created for membership %s", mandateID, m.ID)

						// Create subscription for recurring payments (pass num_payments from term option if set)
						numPay := m.NumPayments
						continueAfterTerm := false
						if to, _ := h.mtRepo.GetTermOption(r.Context(), toID, tenantID); to != nil {
							if numPay == nil && to.NumPayments != nil {
								numPay = to.NumPayments
							}
							continueAfterTerm = to.ContinueAfterTerm
						}
						if continueAfterTerm {
							numPay = nil
						}
						createGoCardlessSubscription(r.Context(), prov, h.memberRepo, mandateID, m,
							tenantID, input.PaymentFrequency, input.PaymentAmount,
							m.PaymentIntervalUnit, m.PaymentIntervalCount, "Membership", m.StartDate, m.MinTermEndDate, numPay, continueAfterTerm)
					}
				}
			}
		}
	}
	// If no bank details were supplied, reuse an existing client mandate when possible.
	if input.BSB == "" && input.AccountNumber == "" && strings.EqualFold(input.PaymentMethod, "direct_debit") {
		if input.GoCardlessMandateID != "" {
			if err := h.memberRepo.SetGoCardlessMandateID(r.Context(), m.ID, tenantID, input.GoCardlessMandateID); err != nil {
				log.Printf("membership.create explicit mandate set failed tenant=%s membership=%s mandate=%s err=%v", tenantID, m.ID, input.GoCardlessMandateID, err)
			}
			h.saveStaffAgreementSnapshot(r, tenantID, clientID, m, mt, term)
			writeJSON(w, http.StatusCreated, m)
			return
		}
		prov, provErr := h.getGoCardlessProvider(r.Context(), tenantID)
		if provErr == nil && prov != nil {
			if existingMandateID := h.findReusableMandateForClient(r.Context(), tenantID, clientID); existingMandateID != "" {
				if err := h.memberRepo.SetGoCardlessMandateID(r.Context(), m.ID, tenantID, existingMandateID); err != nil {
					log.Printf("membership.create reuse mandate set failed tenant=%s membership=%s mandate=%s err=%v", tenantID, m.ID, existingMandateID, err)
				} else {
					// Create GoCardless subscription for recurring frequencies so future debits run automatically.
					if input.PaymentFrequency != "upfront" {
						numPay := m.NumPayments
						continueAfterTerm := false
						if to, _ := h.mtRepo.GetTermOption(r.Context(), toID, tenantID); to != nil {
							if numPay == nil && to.NumPayments != nil {
								numPay = to.NumPayments
							}
							continueAfterTerm = to.ContinueAfterTerm
						}
						if continueAfterTerm {
							numPay = nil
						}
						createGoCardlessSubscription(r.Context(), prov, h.memberRepo, existingMandateID, m,
							tenantID, input.PaymentFrequency, input.PaymentAmount,
							m.PaymentIntervalUnit, m.PaymentIntervalCount, "Membership", m.StartDate, m.MinTermEndDate, numPay, continueAfterTerm)
					}
				}
			}
		}
	}

	h.saveStaffAgreementSnapshot(r, tenantID, clientID, m, mt, term)
	writeJSON(w, http.StatusCreated, m)
}

func (h *ActiveMembershipsHandler) saveStaffAgreementSnapshot(
	r *http.Request,
	tenantID, clientID uuid.UUID,
	m *models.Membership,
	mt *models.MembershipType,
	term *models.TermOption,
) {
	if h.agreementRepo == nil || h.tenantRepo == nil {
		return
	}
	tenant, err := h.tenantRepo.FindByID(r.Context(), tenantID)
	if err != nil || tenant == nil {
		log.Printf("saveStaffAgreementSnapshot: tenant load failed tenant=%s err=%v", tenantID, err)
		return
	}
	client, err := h.clientRepo.FindByID(r.Context(), clientID, tenantID)
	if err != nil || client == nil {
		log.Printf("saveStaffAgreementSnapshot: client load failed client=%s err=%v", clientID, err)
		return
	}
	SaveAgreementSnapshotIfMissing(r.Context(), r, tenant, client, m, mt, term, h.agreementRepo, h.settingsRepo, h.termsRepo)
}

func (h *ActiveMembershipsHandler) findReusableMandateForClient(ctx context.Context, tenantID, clientID uuid.UUID) string {
	memberships, err := h.clientRepo.GetMemberships(ctx, clientID, tenantID)
	if err != nil {
		return ""
	}
	// Prefer active mandate first.
	for _, m := range memberships {
		if m.Status == "active" && m.GoCardlessMandateID != nil && *m.GoCardlessMandateID != "" {
			return *m.GoCardlessMandateID
		}
	}
	// Fallback: any stored mandate reference.
	for _, m := range memberships {
		if m.GoCardlessMandateID != nil && *m.GoCardlessMandateID != "" {
			return *m.GoCardlessMandateID
		}
	}
	return ""
}

// CancelMembership cancels a membership and its GoCardless subscription+mandate.
func (h *ActiveMembershipsHandler) CancelMembership(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	membershipID, err := uuid.Parse(chi.URLParam(r, "membershipID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid membership ID")
		return
	}

	m, err := h.memberRepo.FindByID(r.Context(), membershipID, tenantID)
	if err != nil || m == nil {
		writeError(w, http.StatusNotFound, "Membership not found")
		return
	}
	prov, provErr := h.getGoCardlessProvider(r.Context(), tenantID)
	if provErr != nil {
		log.Printf("membership.cancel provider_error tenant=%s membership=%s err=%v", tenantID, membershipID, provErr)
		writeError(w, http.StatusBadGateway, "Failed to initialise payment provider for cancellation")
		return
	}

	// Cancel GoCardless subscription if exists
	if m.GoCardlessSubscriptionID != nil && *m.GoCardlessSubscriptionID != "" {
		if prov == nil {
			writeError(w, http.StatusBadGateway, "GoCardless is not configured for this tenant")
			return
		}
		if err := prov.CancelSubscription(r.Context(), *m.GoCardlessSubscriptionID); err != nil {
			log.Printf("membership.cancel subscription_cancel_failed tenant=%s membership=%s subscription=%s err=%v", tenantID, membershipID, *m.GoCardlessSubscriptionID, err)
			writeError(w, http.StatusBadGateway, "Failed to cancel GoCardless subscription")
			return
		}
	}

	// Cancel GoCardless mandate if exists
	if m.GoCardlessMandateID != nil && *m.GoCardlessMandateID != "" {
		if prov == nil {
			writeError(w, http.StatusBadGateway, "GoCardless is not configured for this tenant")
			return
		}
		if err := prov.CancelMandate(r.Context(), *m.GoCardlessMandateID); err != nil {
			log.Printf("membership.cancel mandate_cancel_failed tenant=%s membership=%s mandate=%s err=%v", tenantID, membershipID, *m.GoCardlessMandateID, err)
			writeError(w, http.StatusBadGateway, "Failed to cancel GoCardless mandate")
			return
		}
	}

	// Update membership status
	if err := h.memberRepo.UpdateStatus(r.Context(), membershipID, tenantID, "cancelled"); err != nil {
		log.Printf("Error cancelling membership: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to cancel membership")
		return
	}
	if h.dispatcher != nil {
		h.dispatcher.Dispatch(r.Context(), events.NewEvent(events.MembershipCancelled, tenantID, map[string]interface{}{
			"membership_id": membershipID,
			"client_id":     m.ClientID,
			"status":        "cancelled",
			"source":        "staff",
		}))
	}

	log.Printf("membership.cancel success tenant=%s membership=%s", tenantID, membershipID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// UpdateContinueAfterTerm updates Stripe cancel_at for a single membership instance.
func (h *ActiveMembershipsHandler) UpdateContinueAfterTerm(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	membershipID, err := uuid.Parse(chi.URLParam(r, "membershipID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid membership ID")
		return
	}
	var input struct {
		ContinueAfterTerm bool `json:"continue_after_term"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	m, err := h.memberRepo.FindByID(r.Context(), membershipID, tenantID)
	if err != nil || m == nil {
		writeError(w, http.StatusNotFound, "Membership not found")
		return
	}
	if input.ContinueAfterTerm && m.PaymentFrequency != nil && strings.EqualFold(strings.TrimSpace(*m.PaymentFrequency), "custom_schedule") {
		writeError(w, http.StatusBadRequest, "Custom schedule memberships cannot continue billing after term")
		return
	}

	subID, err := h.paymentRepo.FindLatestStripeSubscriptionIDByMembership(r.Context(), tenantID, membershipID)
	if err != nil {
		log.Printf("membership.continue_after_term lookup_failed tenant=%s membership=%s err=%v", tenantID, membershipID, err)
		writeError(w, http.StatusInternalServerError, "Failed to resolve Stripe subscription")
		return
	}
	if strings.TrimSpace(subID) == "" {
		writeError(w, http.StatusBadRequest, "No Stripe subscription found for this membership")
		return
	}

	secretKey, err := h.getStripeSecretKey(r.Context(), tenantID)
	if err != nil {
		log.Printf("membership.continue_after_term stripe_key tenant=%s membership=%s err=%v", tenantID, membershipID, err)
		writeError(w, http.StatusBadGateway, "Stripe is not configured for this tenant")
		return
	}

	previousStripeKey := stripe.Key
	stripe.Key = secretKey
	defer func() { stripe.Key = previousStripeKey }()

	params := &stripe.SubscriptionParams{}
	if input.ContinueAfterTerm {
		// Clear scheduled cancellation; must not send cancel_at together with cancel_at_period_end.
		params.CancelAtPeriodEnd = stripe.Bool(false)
		params.AddExtra("cancel_at", "")
	} else {
		cancelAt := time.Date(
			m.MinTermEndDate.Year(),
			m.MinTermEndDate.Month(),
			m.MinTermEndDate.Day(),
			0, 0, 0, 0,
			time.UTC,
		).Unix()
		if cancelAt <= 0 {
			writeError(w, http.StatusBadRequest, "Membership term end date is invalid")
			return
		}
		params.CancelAtPeriodEnd = stripe.Bool(false)
		params.CancelAt = stripe.Int64(cancelAt)
	}
	if _, err := subscription.Update(subID, params); err != nil {
		log.Printf("membership.continue_after_term stripe_update tenant=%s membership=%s subscription=%s err=%v", tenantID, membershipID, subID, err)
		writeError(w, http.StatusBadGateway, "Failed to update Stripe subscription")
		return
	}
	if err := h.memberRepo.UpdateContinueAfterTerm(r.Context(), membershipID, tenantID, input.ContinueAfterTerm); err != nil {
		log.Printf("membership.continue_after_term persist_failed tenant=%s membership=%s err=%v", tenantID, membershipID, err)
		writeError(w, http.StatusInternalServerError, "Stripe updated but failed to save membership setting")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"membership_id":          membershipID.String(),
		"continue_after_term":    input.ContinueAfterTerm,
		"stripe_subscription_id": subID,
	})
}

// getGoCardlessProvider returns a GoCardless provider for the tenant (if configured).
func (h *ActiveMembershipsHandler) getGoCardlessProvider(ctx context.Context, tenantID uuid.UUID) (payment.GoCardlessExtended, error) {
	if h.credRepo == nil || h.encryptor == nil {
		return nil, nil
	}
	creds, err := h.credRepo.GetByProvider(ctx, tenantID, "gocardless")
	if err != nil || creds == nil || creds.APIKeyEncrypted == "" {
		return nil, err
	}
	apiKey, err := h.encryptor.Decrypt(creds.APIKeyEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt gocardless key: %w", err)
	}
	env := creds.Environment
	if env == "" {
		env = "sandbox"
	}
	return payment.NewGoCardlessProvider(apiKey, "", env), nil
}

func (h *ActiveMembershipsHandler) getStripeSecretKey(ctx context.Context, tenantID uuid.UUID) (string, error) {
	if h.credRepo == nil || h.encryptor == nil {
		return "", fmt.Errorf("stripe integration unavailable")
	}
	creds, err := h.credRepo.GetByProvider(ctx, tenantID, "stripe")
	if err != nil || creds == nil || creds.APIKeyEncrypted == "" {
		return "", fmt.Errorf("stripe not configured")
	}
	secretKey, err := h.encryptor.Decrypt(creds.APIKeyEncrypted)
	if err != nil || strings.TrimSpace(secretKey) == "" {
		return "", fmt.Errorf("stripe credentials invalid")
	}
	return secretKey, nil
}

// ActiveMembershipRow is the joined data for the active memberships view.
type ActiveMembershipRow struct {
	MembershipID      string   `json:"membership_id"`
	ClientID          string   `json:"client_id"`
	ClientName        string   `json:"client_name"`
	ClientEmail       *string  `json:"client_email"`
	ClientPhone       *string  `json:"client_phone"`
	MembershipType    string   `json:"membership_type"`
	TermMonths        int      `json:"term_months"`
	PaymentFrequency  *string  `json:"payment_frequency"`
	PaymentAmount     *float64 `json:"payment_amount"`
	PaymentMethod     *string  `json:"payment_method"`
	CurrencyCode      string   `json:"currency_code"`
	Status            string   `json:"status"`
	StartDate         string   `json:"start_date"`
	EndDate           string   `json:"end_date"`
	MandateID         *string  `json:"mandate_id"`
	SubscriptionID    *string  `json:"subscription_id"`
	LastPaymentDate   *string  `json:"last_payment_date"`
	LastPaymentAmt    *float64 `json:"last_payment_amount"`
	LastPaymentStatus *string  `json:"last_payment_status"`
	TotalPaid         float64  `json:"total_paid"`
	TotalExpected     float64  `json:"total_expected"`
	FailedCount       int      `json:"failed_count"`
}

// List returns all memberships with payment summary.
func (h *ActiveMembershipsHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	statusFilter := r.URL.Query().Get("status")
	if statusFilter == "" {
		statusFilter = "active"
	}

	query := `
		SELECT
			m.id::text,
			c.id::text,
			c.name,
			c.email,
			c.phone,
			mt.title,
			mto.duration_months,
			m.payment_frequency,
			m.payment_amount,
			m.payment_method,
			m.currency_code,
			m.status,
			TO_CHAR(m.start_date, 'YYYY-MM-DD'),
			TO_CHAR(m.min_term_end_date, 'YYYY-MM-DD'),
			m.gocardless_mandate_id,
			m.gocardless_subscription_id,
			(SELECT TO_CHAR(p.paid_date, 'YYYY-MM-DD') FROM payments p WHERE p.membership_id = m.id AND p.status = 'succeeded' ORDER BY p.paid_date DESC LIMIT 1),
			(SELECT p.amount FROM payments p WHERE p.membership_id = m.id AND p.status = 'succeeded' ORDER BY p.paid_date DESC LIMIT 1),
			(SELECT p.status FROM payments p WHERE p.membership_id = m.id ORDER BY p.created_at DESC LIMIT 1),
			COALESCE((SELECT SUM(p.amount) FROM payments p WHERE p.membership_id = m.id AND p.status = 'succeeded'), 0),
			membership_expected_total(
				COALESCE(m.payment_amount, 0),
				m.num_payments,
				m.payment_frequency,
				m.payment_interval_unit,
				m.payment_interval_count,
				m.start_date,
				m.min_term_end_date,
				mto.duration_months
			),
			(SELECT COUNT(*) FROM payments p WHERE p.membership_id = m.id AND p.status = 'failed')
		FROM memberships m
		JOIN clients c ON c.id = m.client_id
		JOIN membership_types mt ON mt.id = m.membership_type_id
		JOIN membership_term_options mto ON mto.id = m.term_option_id
		WHERE m.tenant_id = $1`

	args := []interface{}{tenantID}
	if statusFilter != "all" {
		query += " AND m.status = $2"
		args = append(args, statusFilter)
	}
	query += " ORDER BY m.start_date DESC"

	rows, err := h.db.Pool.Query(r.Context(), query, args...)
	if err != nil {
		log.Printf("Error listing active memberships: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list memberships")
		return
	}
	defer rows.Close()

	var results []ActiveMembershipRow
	for rows.Next() {
		var row ActiveMembershipRow
		if err := rows.Scan(
			&row.MembershipID, &row.ClientID, &row.ClientName, &row.ClientEmail, &row.ClientPhone,
			&row.MembershipType, &row.TermMonths, &row.PaymentFrequency,
			&row.PaymentAmount, &row.PaymentMethod, &row.CurrencyCode, &row.Status, &row.StartDate, &row.EndDate,
			&row.MandateID, &row.SubscriptionID,
			&row.LastPaymentDate, &row.LastPaymentAmt, &row.LastPaymentStatus,
			&row.TotalPaid, &row.TotalExpected, &row.FailedCount,
		); err != nil {
			log.Printf("Error scanning active membership: %v", err)
			continue
		}
		results = append(results, row)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"memberships": results,
		"count":       len(results),
	})
}
