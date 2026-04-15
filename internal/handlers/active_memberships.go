package handlers

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"clientroll/internal/accounting/xerosync"
	"clientroll/internal/database"
	"clientroll/internal/email"
	"clientroll/internal/events"
	"clientroll/internal/middleware"
	"clientroll/internal/models"
	"clientroll/internal/payment"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/invoice"
	"github.com/stripe/stripe-go/v82/price"
	"github.com/stripe/stripe-go/v82/product"
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
	xeroSync      *xerosync.Service

	emailSvc          email.Provider
	emailTemplateRepo *repository.TenantEmailTemplateRepo
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
	xeroSync *xerosync.Service,
) *ActiveMembershipsHandler {
	return &ActiveMembershipsHandler{
		db: db, tenantRepo: tenantR, memberRepo: mr, mtRepo: mtRepo, clientRepo: cr, paymentRepo: pr, credRepo: credR, encryptor: enc,
		agreementRepo: agreementRepo, settingsRepo: settingsRepo, termsRepo: termsRepo,
		xeroSync: xeroSync,
	}
}

// SetDispatcher sets optional outbound webhook event dispatcher.
func (h *ActiveMembershipsHandler) SetDispatcher(dispatcher *events.Dispatcher) {
	h.dispatcher = dispatcher
}

// SetEmailDeps injects dependencies needed for client-facing emails.
func (h *ActiveMembershipsHandler) SetEmailDeps(emailSvc email.Provider, emailTemplateRepo *repository.TenantEmailTemplateRepo) {
	h.emailSvc = emailSvc
	h.emailTemplateRepo = emailTemplateRepo
}

// sendMembershipConfirmationEmail sends a confirmation email to the client after staff creates a membership.
func (h *ActiveMembershipsHandler) sendMembershipConfirmationEmail(tenantID uuid.UUID, client *models.Client, mt *models.MembershipType, tenant *models.Tenant, paymentAmount float64) {
	if h.emailSvc == nil || client == nil || client.Email == nil || *client.Email == "" {
		return
	}
	tenantName := ""
	if tenant != nil {
		tenantName = tenant.Name
	}
	mtTitle := "Membership"
	if mt != nil && strings.TrimSpace(mt.Title) != "" {
		mtTitle = mt.Title
	}
	clientEmail := *client.Email
	clientName := client.Name
	tid := tenantID

	go func() {
		templateKey := "membership_confirmation"
		def, ok := email.OperationalTemplateByKey(templateKey)
		if !ok {
			return
		}
		enabled := def.DefaultEnabled
		subject := def.Subject
		body := def.Body
		if h.emailTemplateRepo != nil {
			override, err := h.emailTemplateRepo.GetByKey(context.Background(), tid, templateKey)
			if err != nil {
				log.Printf("membership.email template lookup failed tenant=%s template=%s err=%v", tid, templateKey, err)
			} else if override != nil {
				enabled = override.Enabled
				if override.SubjectOverride != nil && strings.TrimSpace(*override.SubjectOverride) != "" {
					subject = *override.SubjectOverride
				}
				if override.BodyOverride != nil && strings.TrimSpace(*override.BodyOverride) != "" {
					body = *override.BodyOverride
				}
			}
		}
		if !enabled {
			return
		}
		money := func(v float64) string { return fmt.Sprintf("%.2f", v) }
		vars := map[string]string{
			"client_name":      clientName,
			"membership_title": mtTitle,
			"term_months":      "0",
			"total_fee":        money(paymentAmount),
			"company_name":     tenantName,
		}
		render := func(input string) string {
			out := input
			for k, v := range vars {
				out = strings.ReplaceAll(out, "{{"+k+"}}", v)
			}
			return out
		}
		input := email.SendInput{
			To:      clientEmail,
			Subject: render(subject),
			HTML:    email.WrapHTML(render(body)),
			Text:    render(subject),
		}
		if _, err := h.emailSvc.Send(context.Background(), input); err != nil {
			log.Printf("membership.confirmation email send failed to=%s err=%v", clientEmail, err)
		} else {
			log.Printf("membership.confirmation email sent to=%s", clientEmail)
		}
	}()
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
	tenant, tenantErr := h.tenantRepo.FindByID(r.Context(), tenantID)
	if tenantErr != nil || tenant == nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant context")
		return
	}
	if err := validateMinimumFutureStartDate(startDate, tenant.Timezone, minFutureMembershipStartDays); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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
	dup, dupErr := h.memberRepo.FindUnpaidDuplicateCandidate(
		r.Context(),
		tenantID, clientID, mtID, toID,
		startDate, input.PaymentMethod, input.PaymentFrequency, input.PaymentAmount,
		func() *string {
			if input.PaymentIntervalUnit == "" {
				return nil
			}
			return &input.PaymentIntervalUnit
		}(),
		input.PaymentIntervalCount,
	)
	if dupErr != nil {
		log.Printf("membership duplicate check failed tenant=%s client=%s err=%v", tenantID, clientID, dupErr)
		writeError(w, http.StatusInternalServerError, "Failed to validate membership request")
		return
	}
	if dup != nil {
		writeJSON(w, http.StatusConflict, map[string]interface{}{
			"error":               "A matching membership was created in the last 5 minutes and is still awaiting first payment. Please wait 5 minutes before trying again.",
			"existing_membership": dup,
		})
		return
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

	// --- Stripe: create subscription for recurring card payments ---
	if strings.EqualFold(input.PaymentMethod, "card") && input.PaymentMethodID != "" && input.PaymentFrequency != "upfront" {
		if secretKey, keyErr := h.getStripeSecretKey(r.Context(), tenantID); keyErr == nil {
			stripe.Key = secretKey
			client, _ := h.clientRepo.FindByID(r.Context(), clientID, tenantID)
			if client != nil {
				stripeCustomerID := ""
				if client.StripeCustomerID != nil {
					stripeCustomerID = strings.TrimSpace(*client.StripeCustomerID)
				}
				if stripeCustomerID != "" {
					applySalesTax := mt.ApplySalesTax
					if err := h.createStripeSubscription(r.Context(), tenant, client, m, stripeCustomerID, input.PaymentMethodID,
						membershipCurrency, input.PaymentFrequency, input.PaymentIntervalUnit, input.PaymentIntervalCount,
						input.PaymentAmount, applySalesTax, term.ContinueAfterTerm, m.MinTermEndDate, startDate); err != nil {
						log.Printf("membership.create stripe_subscription_failed tenant=%s membership=%s err=%v", tenantID, m.ID, err)
					}
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
			if cl, _ := h.clientRepo.FindByID(r.Context(), clientID, tenantID); cl != nil {
				h.sendMembershipConfirmationEmail(tenantID, cl, mt, tenant, input.PaymentAmount)
			}
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
	if cl, _ := h.clientRepo.FindByID(r.Context(), clientID, tenantID); cl != nil {
		h.sendMembershipConfirmationEmail(tenantID, cl, mt, tenant, input.PaymentAmount)
	}
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
	// Cancel GoCardless subscription + mandate if exists
	if (m.GoCardlessSubscriptionID != nil && *m.GoCardlessSubscriptionID != "") ||
		(m.GoCardlessMandateID != nil && *m.GoCardlessMandateID != "") {
		prov, provErr := h.getGoCardlessProvider(r.Context(), tenantID)
		if provErr != nil || prov == nil {
			log.Printf("membership.cancel gocardless_provider_unavailable tenant=%s membership=%s err=%v", tenantID, membershipID, provErr)
			writeError(w, http.StatusBadGateway, "GoCardless is not configured but membership has GoCardless references")
			return
		}
		if m.GoCardlessSubscriptionID != nil && *m.GoCardlessSubscriptionID != "" {
			if err := prov.CancelSubscription(r.Context(), *m.GoCardlessSubscriptionID); err != nil {
				log.Printf("membership.cancel gc_subscription_cancel_failed tenant=%s membership=%s subscription=%s err=%v", tenantID, membershipID, *m.GoCardlessSubscriptionID, err)
				writeError(w, http.StatusBadGateway, "Failed to cancel GoCardless subscription")
				return
			}
		}
		if m.GoCardlessMandateID != nil && *m.GoCardlessMandateID != "" {
			if err := prov.CancelMandate(r.Context(), *m.GoCardlessMandateID); err != nil {
				log.Printf("membership.cancel gc_mandate_cancel_failed tenant=%s membership=%s mandate=%s err=%v", tenantID, membershipID, *m.GoCardlessMandateID, err)
				writeError(w, http.StatusBadGateway, "Failed to cancel GoCardless mandate")
				return
			}
		}
	}

	// Cancel Stripe subscription if exists
	if stripeSubID, _ := h.paymentRepo.FindLatestStripeSubscriptionIDByMembership(r.Context(), tenantID, membershipID); stripeSubID != "" {
		if secretKey, keyErr := h.getStripeSecretKey(r.Context(), tenantID); keyErr == nil {
			prevKey := stripe.Key
			stripe.Key = secretKey
			if _, err := subscription.Cancel(stripeSubID, nil); err != nil {
				log.Printf("membership.cancel stripe_subscription_cancel_failed tenant=%s membership=%s subscription=%s err=%v", tenantID, membershipID, stripeSubID, err)
			} else {
				log.Printf("membership.cancel stripe_subscription_cancelled tenant=%s membership=%s subscription=%s", tenantID, membershipID, stripeSubID)
			}
			stripe.Key = prevKey
		}
	}

	// Update membership status
	if err := h.memberRepo.UpdateStatus(r.Context(), membershipID, tenantID, "cancelled"); err != nil {
		log.Printf("Error cancelling membership: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to cancel membership")
		return
	}
	if h.xeroSync != nil {
		if openPayment, pErr := h.paymentRepo.FindLatestOpenByMembership(r.Context(), tenantID, membershipID); pErr == nil && openPayment != nil {
			_ = h.xeroSync.VoidInvoiceForPaymentIfUnpaid(r.Context(), openPayment, "Membership cancelled")
		}
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
		// Clear scheduled cancellation. Do not send cancel_at_period_end together with cancel_at,
		// Stripe rejects requests that include both parameters in a single update call.
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

// createStripeSubscription creates a Stripe subscription for a staff-created card membership.
func (h *ActiveMembershipsHandler) createStripeSubscription(
	ctx context.Context,
	tenant *models.Tenant,
	client *models.Client,
	membership *models.Membership,
	stripeCustomerID, pmID string,
	currencyCode, frequency, intervalUnit string,
	intervalCount int,
	amount float64,
	applySalesTax, continueAfterTerm bool,
	termEndTime time.Time,
	membershipStart time.Time,
) error {
	cur := strings.ToLower(strings.TrimSpace(currencyCode))
	if cur == "" {
		cur = "aud"
	}
	unitAmount := int64(math.Round(amount * 100))
	if unitAmount <= 0 {
		unitAmount = 1
	}
	feeCents := stripeProcessingFee(unitAmount, "card")
	baseAmount, taxAmount := splitAmountByTaxMode(amount, tenantSalesTaxPercent(tenant), applySalesTax)
	totalAmount := float64(unitAmount+feeCents) / 100.0

	subMeta := map[string]string{
		"tenant_id":             tenant.ID.String(),
		"tenant_slug":           tenant.Slug,
		"client_id":             client.ID.String(),
		"membership_id":         membership.ID.String(),
		"payment_amount":        strconv.FormatFloat(totalAmount, 'f', 2, 64),
		"base_amount":           strconv.FormatFloat(baseAmount, 'f', 2, 64),
		"tax_amount":            strconv.FormatFloat(taxAmount, 'f', 2, 64),
		"processing_fee_amount": strconv.FormatFloat(float64(feeCents)/100, 'f', 2, 64),
		"currency_code":         strings.ToUpper(cur),
		"payment_freq":          frequency,
		"source":                "staff",
	}
	if !continueAfterTerm && !termEndTime.IsZero() {
		subMeta["continue_after_term"] = "false"
		subMeta["cancel_at_timestamp"] = strconv.FormatInt(termEndTime.Unix(), 10)
	} else {
		subMeta["continue_after_term"] = "true"
	}

	prod, err := product.New(&stripe.ProductParams{
		Name: stripe.String("Membership — " + tenant.Name),
		Metadata: map[string]string{
			"tenant_id":     tenant.ID.String(),
			"membership_id": membership.ID.String(),
			"clientroll":    "staff_subscription",
		},
	})
	if err != nil {
		return fmt.Errorf("stripe product: %w", err)
	}

	var iu *string
	if intervalUnit != "" {
		iu = &intervalUnit
	}
	interval, count := stripeRecurringInterval(frequency, iu, intervalCount)
	recurring := &stripe.PriceRecurringParams{
		Interval:      stripe.String(interval),
		IntervalCount: stripe.Int64(count),
	}
	taxExcl := string(stripe.PriceTaxBehaviorExclusive)
	prMain, err := price.New(&stripe.PriceParams{
		Currency:    stripe.String(cur),
		Product:     stripe.String(prod.ID),
		UnitAmount:  stripe.Int64(unitAmount),
		Recurring:   recurring,
		TaxBehavior: stripe.String(taxExcl),
		Nickname:    stripe.String("Membership"),
	})
	if err != nil {
		return fmt.Errorf("stripe price (membership): %w", err)
	}
	items := []*stripe.SubscriptionItemsParams{
		{Price: stripe.String(prMain.ID), Quantity: stripe.Int64(1)},
	}
	if feeCents > 0 {
		prFee, pErr := price.New(&stripe.PriceParams{
			Currency:    stripe.String(cur),
			Product:     stripe.String(prod.ID),
			UnitAmount:  stripe.Int64(feeCents),
			Recurring:   recurring,
			TaxBehavior: stripe.String(taxExcl),
			Nickname:    stripe.String("Processing fee"),
		})
		if pErr != nil {
			return fmt.Errorf("stripe price (fee): %w", pErr)
		}
		items = append(items, &stripe.SubscriptionItemsParams{Price: stripe.String(prFee.ID), Quantity: stripe.Int64(1)})
	}

	var trialEnd *int64
	nowUTC := time.Now().UTC()
	if !membershipStart.IsZero() {
		startAt := tenantDateStartUTC(membershipStart, tenant.Timezone).Truncate(time.Second)
		if startAt.After(nowUTC) {
			v := startAt.Unix()
			trialEnd = &v
		}
	}

	subParams := &stripe.SubscriptionParams{
		Customer:             stripe.String(stripeCustomerID),
		DefaultPaymentMethod: stripe.String(pmID),
		Items:                items,
		Metadata:             subMeta,
		PaymentBehavior:      stripe.String("error_if_incomplete"),
		OffSession:           stripe.Bool(true),
	}
	if trialEnd != nil {
		if time.Unix(*trialEnd, 0).UTC().Sub(nowUTC) >= 2*time.Minute {
			subParams.TrialEnd = trialEnd
		} else {
			subParams.BillingCycleAnchor = trialEnd
			subParams.ProrationBehavior = stripe.String("none")
		}
	}
	subParams.AddExpand("latest_invoice")
	sub, err := subscription.New(subParams)
	if err != nil {
		return fmt.Errorf("stripe subscription: %w", err)
	}
	if sub == nil || strings.TrimSpace(sub.ID) == "" {
		return fmt.Errorf("stripe subscription missing id")
	}
	log.Printf("membership.create stripe_subscription tenant=%s membership=%s subscription=%s", tenant.ID, membership.ID, sub.ID)

	if sub.LatestInvoice != nil && strings.TrimSpace(sub.LatestInvoice.ID) != "" {
		if _, invErr := invoice.Update(sub.LatestInvoice.ID, &stripe.InvoiceParams{Metadata: subMeta}); invErr != nil {
			log.Printf("membership.create stripe_invoice_metadata tenant=%s invoice=%s err=%v", tenant.ID, sub.LatestInvoice.ID, invErr)
		}
	}

	if !continueAfterTerm && !termEndTime.IsZero() && termEndTime.Unix() > 0 {
		if _, caErr := subscription.Update(sub.ID, &stripe.SubscriptionParams{
			CancelAt:          stripe.Int64(termEndTime.Unix()),
			CancelAtPeriodEnd: stripe.Bool(false),
		}); caErr != nil {
			return fmt.Errorf("set subscription cancel_at: %w", caErr)
		}
	}
	return nil
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
