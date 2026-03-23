package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"clientroll/internal/accounting/xerosync"
	"clientroll/internal/database"
	"clientroll/internal/events"
	"clientroll/internal/middleware"
	"clientroll/internal/payment"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v82"
	checkoutsession "github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/paymentintent"
	"github.com/stripe/stripe-go/v82/paymentmethod"
)

var isoServiceCurrencyRegex = regexp.MustCompile(`^[A-Z]{3}$`)

var allowedServiceCurrencies = map[string]struct{}{
	"AUD": {},
	"USD": {},
	"EUR": {},
	"GBP": {},
	"NZD": {},
	"CAD": {},
	"SGD": {},
}

type ServiceHandler struct {
	serviceRepo *repository.ServiceRepo
	clientRepo  *repository.ClientRepo
	memberRepo  *repository.MembershipRepo
	mtRepo      *repository.MembershipTypeRepo
	paymentRepo *repository.PaymentRepo
	credRepo    *repository.TenantCredentialRepo
	encryptor   *database.Encryptor
	dispatcher  *events.Dispatcher
	xeroSync    *xerosync.Service
}

func NewServiceHandler(sr *repository.ServiceRepo, cr *repository.ClientRepo, mr *repository.MembershipRepo, mtRepo *repository.MembershipTypeRepo, pr *repository.PaymentRepo, credR *repository.TenantCredentialRepo, enc *database.Encryptor, xeroSync *xerosync.Service) *ServiceHandler {
	return &ServiceHandler{serviceRepo: sr, clientRepo: cr, memberRepo: mr, mtRepo: mtRepo, paymentRepo: pr, credRepo: credR, encryptor: enc, xeroSync: xeroSync}
}

func (h *ServiceHandler) SetDispatcher(d *events.Dispatcher) {
	h.dispatcher = d
}

func (h *ServiceHandler) RegisterRoutes(r chi.Router) {
	r.Get("/services", h.List)
	r.Post("/services", h.Create)
	r.Put("/services/{serviceID}", h.Update)
	r.Delete("/services/{serviceID}", h.Delete)
	r.Post("/services/charge", h.TakePayment)
	r.Post("/services/charge-with-bank-details", h.ChargeWithBankDetails)
	r.Post("/services/charge-stripe", h.ChargeStripe)
	r.Get("/services/stripe-payment-methods/{clientID}", h.ListStripePaymentMethods)
	r.Get("/services/charges/{clientID}", h.ListCharges)
}

func (h *ServiceHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	services, err := h.serviceRepo.List(r.Context(), tenantID)
	if err != nil {
		log.Printf("Error listing services: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list services")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"services": services, "count": len(services)})
}

func (h *ServiceHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	var input struct {
		Name         string  `json:"name"`
		Description  *string `json:"description"`
		Price        float64 `json:"price"`
		CurrencyCode string  `json:"currency_code"`
		TaxRate      float64 `json:"tax_rate"`
		SortOrder    int     `json:"sort_order"`
	}
	if err := decodeJSON(r, &input); err != nil || input.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}
	currencyCode := strings.ToUpper(strings.TrimSpace(input.CurrencyCode))
	if currencyCode == "" {
		currencyCode = "AUD"
	}
	if !isoServiceCurrencyRegex.MatchString(currencyCode) {
		writeError(w, http.StatusBadRequest, "currency_code must be ISO-4217 uppercase")
		return
	}
	if _, ok := allowedServiceCurrencies[currencyCode]; !ok {
		writeError(w, http.StatusBadRequest, "unsupported currency_code")
		return
	}
	svc, err := h.serviceRepo.Create(r.Context(), tenantID, input.Name, input.Description, input.Price, currencyCode, input.TaxRate, input.SortOrder)
	if err != nil {
		log.Printf("Error creating service: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create service")
		return
	}
	writeJSON(w, http.StatusCreated, svc)
}

func (h *ServiceHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	svcID, _ := uuid.Parse(chi.URLParam(r, "serviceID"))
	var input struct {
		Name         string  `json:"name"`
		Description  *string `json:"description"`
		Price        float64 `json:"price"`
		CurrencyCode string  `json:"currency_code"`
		TaxRate      float64 `json:"tax_rate"`
		IsActive     bool    `json:"is_active"`
		SortOrder    int     `json:"sort_order"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request")
		return
	}
	currencyCode := strings.ToUpper(strings.TrimSpace(input.CurrencyCode))
	if currencyCode == "" {
		currencyCode = "AUD"
	}
	if !isoServiceCurrencyRegex.MatchString(currencyCode) {
		writeError(w, http.StatusBadRequest, "currency_code must be ISO-4217 uppercase")
		return
	}
	if _, ok := allowedServiceCurrencies[currencyCode]; !ok {
		writeError(w, http.StatusBadRequest, "unsupported currency_code")
		return
	}
	svc, err := h.serviceRepo.Update(r.Context(), svcID, tenantID, input.Name, input.Description, input.Price, currencyCode, input.TaxRate, input.IsActive, input.SortOrder)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update service")
		return
	}
	writeJSON(w, http.StatusOK, svc)
}

func (h *ServiceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	svcID, _ := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err := h.serviceRepo.Delete(r.Context(), svcID, tenantID); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete service")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// TakePayment charges a client for a service/one-off payment.
// If the client has an active membership with a GoCardless mandate, it also creates
// a GoCardless payment (direct debit charge) and records it in the payments table.
func (h *ServiceHandler) TakePayment(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	userID, _ := uuid.Parse(middleware.GetUserID(r.Context()))
	var input struct {
		ClientID     string  `json:"client_id"`
		ServiceID    *string `json:"service_id"`
		Description  string  `json:"description"`
		Amount       float64 `json:"amount"`
		TaxAmount    float64 `json:"tax_amount"`
		Notes        *string `json:"notes"`
		MembershipID *string `json:"membership_id,omitempty"` // optional: use this membership's mandate
	}
	if err := decodeJSON(r, &input); err != nil || input.ClientID == "" || input.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "Client ID and positive amount required")
		return
	}
	clientID, _ := uuid.Parse(input.ClientID)
	var serviceID *uuid.UUID
	if input.ServiceID != nil && *input.ServiceID != "" {
		sid, _ := uuid.Parse(*input.ServiceID)
		serviceID = &sid
	}

	// Record the service charge in our DB
	charge, err := h.serviceRepo.CreateCharge(r.Context(), tenantID, clientID, serviceID, input.Description, input.Amount, input.TaxAmount, input.Notes, &userID)
	if err != nil {
		log.Printf("Error creating service charge: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create charge")
		return
	}
	h.dispatchServiceChargeEvent(r.Context(), events.ServiceChargeCreated, charge)
	h.dispatchServiceChargeEvent(r.Context(), events.ServiceChargeSucceeded, charge)
	h.syncXeroForCharge(r.Context(), charge)

	// --- Try to charge through GoCardless if the client has a mandate ---
	gcPaymentID := ""
	var chargedMembershipID *uuid.UUID
	prov, provErr := h.getGoCardlessProvider(r.Context(), tenantID)
	if provErr == nil && prov != nil {
		mandateID, membershipID := h.findMandateForClient(r.Context(), tenantID, clientID, input.MembershipID)
		if mandateID != "" {
			chargedMembershipID = membershipID
			totalAmount := input.Amount + input.TaxAmount
			amountMinor := int64(totalAmount * 100)
			chargeDate := time.Now().AddDate(0, 0, 3) // 3 days from now (minimum for DD)
			ref := fmt.Sprintf("charge-%s", charge.ID)

			gcID, err := prov.CreatePayment(r.Context(), payment.CreatePaymentInput{
				MandateID:   mandateID,
				Amount:      amountMinor,
				Currency:    "AUD",
				Description: input.Description,
				ChargeDate:  chargeDate,
				Reference:   ref,
			})
			if err != nil {
				log.Printf("GoCardless CreatePayment for charge %s: %v", charge.ID, err)
				// Don't fail the whole request — the service charge is saved
			} else {
				gcPaymentID = gcID
				log.Printf("GoCardless payment %s created for charge %s (mandate %s)", gcID, charge.ID, mandateID)
				// Persist payment ledger row for reconciliation/reporting when we know membership context.
				if chargedMembershipID != nil {
					if _, _, perr := h.paymentRepo.CreateIfNotExists(
						r.Context(),
						tenantID,
						*chargedMembershipID,
						clientID,
						totalAmount,
						"AUD",
						chargeDate,
						gcID,
						&repository.PaymentComponents{
							BaseAmount:          input.Amount,
							TaxAmount:           input.TaxAmount,
							ProcessingFeeAmount: 0,
						},
					); perr != nil {
						log.Printf("service.take_payment ledger_insert_failed tenant=%s charge=%s payment_ref=%s err=%v", tenantID, charge.ID, gcID, perr)
					}
				} else {
					log.Printf("service.take_payment missing_membership_context tenant=%s charge=%s payment_ref=%s", tenantID, charge.ID, gcID)
				}
			}
		}
	}

	resp := map[string]interface{}{"charge": charge}
	if gcPaymentID != "" {
		resp["gocardless_payment_id"] = gcPaymentID
	}
	writeJSON(w, http.StatusCreated, resp)
}

// findMandateForClient looks for a GoCardless mandate on the client's active memberships.
func (h *ServiceHandler) findMandateForClient(ctx context.Context, tenantID, clientID uuid.UUID, membershipIDStr *string) (string, *uuid.UUID) {
	if h.memberRepo == nil {
		return "", nil
	}
	// If a specific membership was provided, use that
	if membershipIDStr != nil && *membershipIDStr != "" {
		mid, _ := uuid.Parse(*membershipIDStr)
		m, err := h.memberRepo.FindByID(ctx, mid, tenantID)
		if err == nil && m != nil && m.GoCardlessMandateID != nil && *m.GoCardlessMandateID != "" {
			return *m.GoCardlessMandateID, &m.ID
		}
	}
	// Otherwise find the client's first active membership with a mandate
	memberships, err := h.clientRepo.GetMemberships(ctx, clientID, tenantID)
	if err != nil {
		return "", nil
	}
	for _, m := range memberships {
		if m.Status == "active" && m.GoCardlessMandateID != nil && *m.GoCardlessMandateID != "" {
			id := m.ID
			return *m.GoCardlessMandateID, &id
		}
	}
	return "", nil
}

// getGoCardlessProvider returns a GoCardless provider for the tenant (if configured).
func (h *ServiceHandler) getGoCardlessProvider(ctx context.Context, tenantID uuid.UUID) (payment.GoCardlessExtended, error) {
	if h.credRepo == nil || h.encryptor == nil {
		return nil, nil
	}
	creds, err := h.credRepo.GetByProvider(ctx, tenantID, "gocardless")
	if err != nil || creds == nil || creds.APIKeyEncrypted == "" {
		return nil, err
	}
	apiKey, err := h.encryptor.Decrypt(creds.APIKeyEncrypted)
	if err != nil {
		return nil, err
	}
	env := creds.Environment
	if env == "" {
		env = "sandbox"
	}
	return payment.NewGoCardlessProvider(apiKey, "", env), nil
}

// ChargeWithBankDetails creates a GoCardless mandate from bank details, charges, and records locally.
// Used when staff enters bank details in the Take Payment modal for a client with no existing mandate.
func (h *ServiceHandler) ChargeWithBankDetails(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	userID, _ := uuid.Parse(middleware.GetUserID(r.Context()))

	var input struct {
		ClientID          string  `json:"client_id"`
		ServiceID         *string `json:"service_id"`
		Description       string  `json:"description"`
		Amount            float64 `json:"amount"`
		TaxAmount         float64 `json:"tax_amount"`
		Notes             *string `json:"notes"`
		AccountHolderName string  `json:"account_holder_name"`
		AccountNumber     string  `json:"account_number"`
		BSB               string  `json:"bsb"`
	}
	if err := decodeJSON(r, &input); err != nil || input.ClientID == "" || input.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "Client ID, amount, and bank details required")
		return
	}
	if input.BSB == "" || input.AccountNumber == "" {
		writeError(w, http.StatusBadRequest, "BSB and account number required")
		return
	}

	clientID, _ := uuid.Parse(input.ClientID)

	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil || prov == nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}

	client, err := h.clientRepo.FindByID(r.Context(), clientID, tenantID)
	if err != nil || client == nil {
		writeError(w, http.StatusNotFound, "Client not found")
		return
	}

	// 1. Create or reuse GoCardless customer
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
			log.Printf("GoCardless CreateCustomer: %v", err)
			writeProviderError(w, err)
			return
		}
		gcCustomerID = cid
		_ = h.clientRepo.SetGoCardlessCustomerID(r.Context(), client.ID, tenantID, cid)
	}

	// 2. Create mandate via direct debit setup
	holderName := input.AccountHolderName
	if holderName == "" {
		holderName = client.Name
	}
	mandateID, err := prov.SetupDirectDebit(r.Context(), payment.SetupDirectDebitInput{
		CustomerID:        gcCustomerID,
		AccountHolderName: holderName,
		AccountNumber:     input.AccountNumber,
		BSB:               input.BSB,
		Reference:         clientID.String(),
	})
	if err != nil {
		log.Printf("GoCardless SetupDirectDebit: %v", err)
		writeProviderError(w, err)
		return
	}

	// 3. Store mandate on client's active membership (if any) + create subscription
	memberships, _ := h.clientRepo.GetMemberships(r.Context(), clientID, tenantID)
	var membershipID uuid.UUID
	for _, m := range memberships {
		if m.Status == "active" && (m.GoCardlessMandateID == nil || *m.GoCardlessMandateID == "") {
			_ = h.memberRepo.SetGoCardlessMandateID(r.Context(), m.ID, tenantID, mandateID)
			membershipID = m.ID
			// Create subscription for recurring payments (pass num_payments from term option if set)
			if m.PaymentFrequency != nil && *m.PaymentFrequency != "upfront" && m.PaymentAmount != nil {
				var numPay *int
				continueAfterTerm := false
				if h.mtRepo != nil {
					if to, _ := h.mtRepo.GetTermOption(r.Context(), m.TermOptionID, tenantID); to != nil {
						if to.NumPayments != nil {
							numPay = to.NumPayments
						}
						continueAfterTerm = to.ContinueAfterTerm
					}
				}
				if continueAfterTerm {
					numPay = nil
				}
				createGoCardlessSubscription(r.Context(), prov, h.memberRepo, mandateID, &m,
					tenantID, *m.PaymentFrequency, *m.PaymentAmount, m.PaymentIntervalUnit, m.PaymentIntervalCount, "Membership", m.StartDate, m.MinTermEndDate, numPay, continueAfterTerm)
			}
			break
		}
	}

	// 4. Create GoCardless payment
	totalAmount := input.Amount + input.TaxAmount
	amountMinor := int64(totalAmount * 100)
	chargeDate := time.Now().AddDate(0, 0, 3)

	var serviceID *uuid.UUID
	if input.ServiceID != nil && *input.ServiceID != "" {
		sid, _ := uuid.Parse(*input.ServiceID)
		serviceID = &sid
	}

	// Record locally first
	charge, err := h.serviceRepo.CreateCharge(r.Context(), tenantID, clientID, serviceID, input.Description, input.Amount, input.TaxAmount, input.Notes, &userID)
	if err != nil {
		log.Printf("Error creating service charge: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create charge")
		return
	}
	h.dispatchServiceChargeEvent(r.Context(), events.ServiceChargeCreated, charge)
	h.dispatchServiceChargeEvent(r.Context(), events.ServiceChargeSucceeded, charge)
	h.syncXeroForCharge(r.Context(), charge)

	ref := fmt.Sprintf("charge-%s", charge.ID)
	gcPaymentID, err := prov.CreatePayment(r.Context(), payment.CreatePaymentInput{
		MandateID:   mandateID,
		Amount:      amountMinor,
		Currency:    "AUD",
		Description: input.Description,
		ChargeDate:  chargeDate,
		Reference:   ref,
	})
	if err != nil {
		log.Printf("GoCardless CreatePayment: %v", err)
		// Mandate was created but payment failed — still return success with warning
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"charge":                charge,
			"gocardless_mandate_id": mandateID,
			"gocardless_payment_id": nil,
			"warning":               "Mandate created but payment submission failed: " + err.Error(),
		})
		return
	}

	log.Printf("GoCardless mandate %s + payment %s created for client %s", mandateID, gcPaymentID, clientID)
	if membershipID != uuid.Nil {
		if _, _, perr := h.paymentRepo.CreateIfNotExists(
			r.Context(),
			tenantID,
			membershipID,
			clientID,
			totalAmount,
			"AUD",
			chargeDate,
			gcPaymentID,
			&repository.PaymentComponents{
				BaseAmount:          input.Amount,
				TaxAmount:           input.TaxAmount,
				ProcessingFeeAmount: 0,
			},
		); perr != nil {
			log.Printf("service.charge_with_bank_details ledger_insert_failed tenant=%s client=%s payment_ref=%s err=%v", tenantID, clientID, gcPaymentID, perr)
		}
	} else {
		log.Printf("service.charge_with_bank_details missing_membership_context tenant=%s client=%s payment_ref=%s", tenantID, clientID, gcPaymentID)
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"charge":                charge,
		"gocardless_mandate_id": mandateID,
		"gocardless_payment_id": gcPaymentID,
	})
}

// ChargeStripe creates a Stripe Checkout session for a one-off service charge.
func (h *ServiceHandler) ChargeStripe(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	userID, _ := uuid.Parse(middleware.GetUserID(r.Context()))

	var input struct {
		ClientID        string  `json:"client_id"`
		ServiceID       *string `json:"service_id"`
		Description     string  `json:"description"`
		Amount          float64 `json:"amount"`
		TaxAmount       float64 `json:"tax_amount"`
		Notes           *string `json:"notes"`
		PaymentMethod   string  `json:"payment_method"`
		PaymentMethodID string  `json:"payment_method_id,omitempty"`
	}
	if err := decodeJSON(r, &input); err != nil || input.ClientID == "" || input.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "Client ID and positive amount required")
		return
	}
	clientID, _ := uuid.Parse(input.ClientID)

	secretKey, err := h.getStripeSecretKey(r.Context(), tenantID)
	if err != nil || secretKey == "" {
		writeError(w, http.StatusBadRequest, "Stripe not configured for this tenant")
		return
	}
	stripe.Key = secretKey

	client, err := h.clientRepo.FindByID(r.Context(), clientID, tenantID)
	if err != nil || client == nil {
		writeError(w, http.StatusNotFound, "Client not found")
		return
	}

	var serviceID *uuid.UUID
	if input.ServiceID != nil && *input.ServiceID != "" {
		sid, _ := uuid.Parse(*input.ServiceID)
		serviceID = &sid
	}

	totalCents := int64((input.Amount + input.TaxAmount) * 100)
	if totalCents <= 0 {
		totalCents = int64(input.Amount * 100)
	}

	origin := strings.TrimSuffix(r.Header.Get("Origin"), "/")
	if origin == "" {
		origin = "https://app.clientroll.com"
	}

	feeMethod := "card"
	if input.PaymentMethod == "direct_debit" {
		feeMethod = "direct_debit"
	}
	if input.PaymentMethod == "" {
		input.PaymentMethod = feeMethod
	}
	feeCents := stripeProcessingFee(totalCents, feeMethod)
	feeAmount := float64(feeCents) / 100.0

	charge, err := h.serviceRepo.CreatePendingCharge(r.Context(), tenantID, clientID, serviceID, input.Description, input.Amount, input.TaxAmount, feeAmount, input.Notes, &userID)
	if err != nil {
		log.Printf("Error creating pending service charge: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create charge")
		return
	}
	h.dispatchServiceChargeEvent(r.Context(), events.ServiceChargeCreated, charge)
	h.syncXeroForCharge(r.Context(), charge)
	stripeCustomerID := ""
	if client.StripeCustomerID != nil {
		stripeCustomerID = strings.TrimSpace(*client.StripeCustomerID)
	}
	if stripeCustomerID == "" && h.paymentRepo != nil {
		stripeCustomerID, _ = h.paymentRepo.FindLatestStripeCustomerIDByClient(r.Context(), tenantID, clientID)
	}
	if strings.TrimSpace(input.PaymentMethodID) != "" {
		if stripeCustomerID == "" {
			_ = h.serviceRepo.UpdateChargeStatus(r.Context(), tenantID, charge.ID, "failed", nil)
			writeError(w, http.StatusBadRequest, "No saved Stripe customer/payment method found for this client")
			return
		}
		amountToChargeCents := totalCents + feeCents
		if amountToChargeCents <= 0 {
			_ = h.serviceRepo.UpdateChargeStatus(r.Context(), tenantID, charge.ID, "failed", nil)
			writeError(w, http.StatusBadRequest, "Charge total must be greater than zero")
			return
		}
		piParams := &stripe.PaymentIntentParams{
			Amount:        stripe.Int64(amountToChargeCents),
			Currency:      stripe.String("aud"),
			Customer:      stripe.String(stripeCustomerID),
			PaymentMethod: stripe.String(strings.TrimSpace(input.PaymentMethodID)),
			Confirm:       stripe.Bool(true),
			OffSession:    stripe.Bool(true),
			Description:   stripe.String(input.Description),
			Metadata: map[string]string{
				"tenant_id":      tenantID.String(),
				"client_id":      clientID.String(),
				"charge_id":      charge.ID.String(),
				"type":           "service_charge",
				"payment_method": input.PaymentMethod,
			},
		}
		pi, piErr := paymentintent.New(piParams)
		if piErr != nil {
			log.Printf("Stripe PaymentIntent error: %v", piErr)
			_ = h.serviceRepo.UpdateChargeStatus(r.Context(), tenantID, charge.ID, "failed", nil)
			writeError(w, http.StatusBadGateway, "Failed to charge saved payment method")
			return
		}
		providerRef := pi.ID
		paymentMethod := input.PaymentMethod
		_ = h.serviceRepo.UpdateChargeProviderContext(r.Context(), tenantID, charge.ID, &providerRef, &paymentMethod)
		_ = h.serviceRepo.MergeStripeDetailsByChargeID(r.Context(), tenantID, charge.ID, map[string]interface{}{
			"source":                   "stripe",
			"payment_intent_id":        pi.ID,
			"payment_intent_status":    string(pi.Status),
			"stripe_customer_id":       stripeCustomerID,
			"stripe_payment_method_id": strings.TrimSpace(input.PaymentMethodID),
		})
		switch strings.ToLower(strings.TrimSpace(string(pi.Status))) {
		case "succeeded":
			now := time.Now().UTC()
			_ = h.serviceRepo.UpdateChargeStatus(r.Context(), tenantID, charge.ID, "succeeded", &now)
			if refreshed, ferr := h.serviceRepo.FindChargeByID(r.Context(), tenantID, charge.ID); ferr == nil && refreshed != nil {
				h.dispatchServiceChargeEvent(r.Context(), events.ServiceChargeSucceeded, refreshed)
				h.syncXeroForCharge(r.Context(), refreshed)
			}
		case "processing", "requires_capture", "requires_action":
			_ = h.serviceRepo.UpdateChargeStatus(r.Context(), tenantID, charge.ID, "pending", nil)
			if refreshed, ferr := h.serviceRepo.FindChargeByID(r.Context(), tenantID, charge.ID); ferr == nil && refreshed != nil {
				h.syncXeroForCharge(r.Context(), refreshed)
			}
		default:
			_ = h.serviceRepo.UpdateChargeStatus(r.Context(), tenantID, charge.ID, "failed", nil)
		}
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"charge":                    charge,
			"payment_intent_id":         pi.ID,
			"charged_with_saved_method": true,
		})
		return
	}

	params := &stripe.CheckoutSessionParams{
		Mode:       stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL: stripe.String(origin + "/clients/" + clientID.String() + "?stripe_charge=success"),
		CancelURL:  stripe.String(origin + "/clients/" + clientID.String() + "?stripe_charge=cancelled"),
		Metadata: map[string]string{
			"tenant_id":      tenantID.String(),
			"client_id":      clientID.String(),
			"charge_id":      charge.ID.String(),
			"type":           "service_charge",
			"payment_method": input.PaymentMethod,
		},
		PaymentIntentData: &stripe.CheckoutSessionPaymentIntentDataParams{
			Metadata: map[string]string{
				"tenant_id":      tenantID.String(),
				"client_id":      clientID.String(),
				"charge_id":      charge.ID.String(),
				"type":           "service_charge",
				"payment_method": input.PaymentMethod,
			},
		},
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Quantity: stripe.Int64(1),
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String("aud"),
					UnitAmount: stripe.Int64(totalCents),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(input.Description),
					},
				},
			},
			{
				Quantity: stripe.Int64(1),
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String("aud"),
					UnitAmount: stripe.Int64(feeCents),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String("Processing Fee"),
					},
				},
			},
		},
	}
	directDebitType := ""
	if input.PaymentMethod == "direct_debit" {
		directDebitType = stripeDirectDebitTypeForCountry(client.Country)
		if directDebitType == "" {
			_ = h.serviceRepo.UpdateChargeStatus(r.Context(), tenantID, charge.ID, "failed", nil)
			writeError(w, http.StatusBadRequest, "Direct debit requires client country AU or US")
			return
		}
	}
	if input.PaymentMethod == "direct_debit" {
		params.PaymentMethodTypes = stripe.StringSlice([]string{directDebitType})
	}
	if stripeCustomerID != "" {
		params.Customer = stripe.String(stripeCustomerID)
		params.PaymentMethodCollection = stripe.String("if_required")
	}
	if params.Customer == nil && client.Email != nil && *client.Email != "" {
		params.CustomerEmail = stripe.String(*client.Email)
	}

	sess, err := checkoutsession.New(params)
	if err != nil {
		log.Printf("Stripe Checkout session error: %v", err)
		_ = h.serviceRepo.UpdateChargeStatus(r.Context(), tenantID, charge.ID, "failed", nil)
		writeError(w, http.StatusInternalServerError, "Failed to create Stripe checkout session")
		return
	}
	providerRef := sess.ID
	paymentMethod := input.PaymentMethod
	_ = h.serviceRepo.UpdateChargeProviderContext(r.Context(), tenantID, charge.ID, &providerRef, &paymentMethod)
	_ = h.serviceRepo.MergeStripeDetailsByChargeID(r.Context(), tenantID, charge.ID, map[string]interface{}{
		"source":                  "stripe",
		"checkout_session_id":     sess.ID,
		"checkout_status":         string(sess.Status),
		"checkout_mode":           string(sess.Mode),
		"checkout_payment_status": strings.ToLower(string(sess.PaymentStatus)),
		"currency":                strings.ToLower(string(sess.Currency)),
		"payment_method_type": map[string]string{
			"direct_debit": directDebitType,
			"card":         "card",
		}[input.PaymentMethod],
		"payment_method_label": map[string]string{
			"direct_debit": map[string]string{
				"au_becs_debit":   "Australia BECS Direct Debit",
				"us_bank_account": "US Bank Account (ACH Direct Debit)",
			}[directDebitType],
			"card": "Card",
		}[input.PaymentMethod],
	})

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"charge":       charge,
		"checkout_url": sess.URL,
		"session_id":   sess.ID,
	})
}

func (h *ServiceHandler) ListStripePaymentMethods(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	clientID, err := uuid.Parse(chi.URLParam(r, "clientID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid client ID")
		return
	}
	secretKey, err := h.getStripeSecretKey(r.Context(), tenantID)
	if err != nil || secretKey == "" {
		writeError(w, http.StatusBadRequest, "Stripe not configured for this tenant")
		return
	}
	stripe.Key = secretKey

	client, err := h.clientRepo.FindByID(r.Context(), clientID, tenantID)
	if err != nil || client == nil {
		writeError(w, http.StatusNotFound, "Client not found")
		return
	}
	customerID := ""
	if client.StripeCustomerID != nil {
		customerID = strings.TrimSpace(*client.StripeCustomerID)
	}
	if customerID == "" && h.paymentRepo != nil {
		customerID, _ = h.paymentRepo.FindLatestStripeCustomerIDByClient(r.Context(), tenantID, clientID)
	}
	if customerID == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{"payment_methods": []interface{}{}})
		return
	}
	defaultPM := ""
	if cust, cerr := customer.Get(customerID, nil); cerr == nil && cust != nil && cust.InvoiceSettings != nil && cust.InvoiceSettings.DefaultPaymentMethod != nil {
		defaultPM = strings.TrimSpace(cust.InvoiceSettings.DefaultPaymentMethod.ID)
	}
	type pmRow struct {
		ID        string `json:"id"`
		Type      string `json:"type"`
		Label     string `json:"label"`
		IsDefault bool   `json:"is_default"`
	}
	methods := make([]pmRow, 0)
	appendMethods := func(pmType string) {
		params := &stripe.PaymentMethodListParams{
			Customer: stripe.String(customerID),
			Type:     stripe.String(pmType),
		}
		it := paymentmethod.List(params)
		for it.Next() {
			pm := it.PaymentMethod()
			if pm == nil || strings.TrimSpace(pm.ID) == "" {
				continue
			}
			label := pmType
			switch pmType {
			case "card":
				brand := strings.Title(strings.ToLower(string(pm.Card.Brand)))
				last4 := pm.Card.Last4
				if strings.TrimSpace(brand) == "" {
					brand = "Card"
				}
				if strings.TrimSpace(last4) != "" {
					label = fmt.Sprintf("%s ending in %s", brand, last4)
				} else {
					label = brand
				}
			case "au_becs_debit":
				last4 := pm.AUBECSDebit.Last4
				bsb := pm.AUBECSDebit.BSBNumber
				if strings.TrimSpace(bsb) != "" && strings.TrimSpace(last4) != "" {
					label = fmt.Sprintf("AU BECS debit BSB %s · %s", bsb, last4)
				} else if strings.TrimSpace(last4) != "" {
					label = fmt.Sprintf("AU BECS debit ending in %s", last4)
				} else {
					label = "AU BECS direct debit"
				}
			case "us_bank_account":
				last4 := pm.USBankAccount.Last4
				if strings.TrimSpace(last4) != "" {
					label = fmt.Sprintf("US bank account ending in %s", last4)
				} else {
					label = "US bank account (ACH)"
				}
			}
			methods = append(methods, pmRow{
				ID:        pm.ID,
				Type:      pmType,
				Label:     label,
				IsDefault: defaultPM != "" && pm.ID == defaultPM,
			})
		}
	}
	appendMethods("card")
	appendMethods("au_becs_debit")
	appendMethods("us_bank_account")

	writeJSON(w, http.StatusOK, map[string]interface{}{"payment_methods": methods})
}

func (h *ServiceHandler) getStripeSecretKey(ctx context.Context, tenantID uuid.UUID) (string, error) {
	if h.credRepo == nil || h.encryptor == nil {
		return "", fmt.Errorf("stripe integration unavailable")
	}
	creds, err := h.credRepo.GetByProvider(ctx, tenantID, "stripe")
	if err != nil || creds == nil || creds.APIKeyEncrypted == "" {
		return "", fmt.Errorf("stripe not configured")
	}
	return h.encryptor.Decrypt(creds.APIKeyEncrypted)
}

func (h *ServiceHandler) ListCharges(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	clientID, _ := uuid.Parse(chi.URLParam(r, "clientID"))
	charges, err := h.serviceRepo.ListChargesByClient(r.Context(), clientID, tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list charges")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"charges": charges, "count": len(charges)})
}

func (h *ServiceHandler) dispatchServiceChargeEvent(ctx context.Context, eventType string, charge *repository.ServiceCharge) {
	if h.dispatcher == nil || charge == nil {
		return
	}
	h.dispatcher.Dispatch(ctx, events.NewEvent(eventType, charge.TenantID, map[string]interface{}{
		"charge_id":     charge.ID.String(),
		"tenant_id":     charge.TenantID.String(),
		"client_id":     charge.ClientID.String(),
		"service_id":    uuidToString(charge.ServiceID),
		"status":        charge.Status,
		"amount":        charge.Amount,
		"tax_amount":    charge.TaxAmount,
		"currency_code": charge.CurrencyCode,
		"paid_date":     optionalTime(charge.PaidDate),
		"description":   charge.Description,
	}))
}

func (h *ServiceHandler) syncXeroForCharge(ctx context.Context, charge *repository.ServiceCharge) {
	if h.xeroSync == nil || charge == nil || !h.xeroSync.IsTenantActive(ctx, charge.TenantID) {
		return
	}
	switch charge.Status {
	case "pending":
		_, _ = h.xeroSync.EnsureInvoiceForServiceCharge(ctx, charge)
	case "succeeded":
		_, _ = h.xeroSync.EnsureInvoiceForServiceCharge(ctx, charge)
		_ = h.xeroSync.ApplyPaymentForServiceCharge(ctx, charge)
	}
}

func optionalTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	v := t.UTC().Format(time.RFC3339)
	return &v
}

func uuidToString(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}
