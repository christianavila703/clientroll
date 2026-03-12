package handlers

import (
	"context"
	"errors"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"clientroll/internal/database"
	"clientroll/internal/email"
	"clientroll/internal/events"
	"clientroll/internal/models"
	"clientroll/internal/payment"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v82"
	checkoutsession "github.com/stripe/stripe-go/v82/checkout/session"
)

// SignupHandler handles the public membership signup flow.
// These endpoints are accessible without authentication (resolved via tenant slug).
type SignupHandler struct {
	tenantRepo      *repository.TenantRepo
	mtRepo          *repository.MembershipTypeRepo
	bonusRepo       *repository.BonusRepo
	termsRepo       *repository.TermsRepo
	clientRepo      *repository.ClientRepo
	memberRepo      *repository.MembershipRepo
	settingsRepo    *repository.SettingsRepo
	credRepo        *repository.TenantCredentialRepo
	bankDetailsRepo *repository.BankDetailsRepo
	encryptor       *database.Encryptor
	emailSvc        email.Provider
	dispatcher      *events.Dispatcher
}

// NewSignupHandler creates a new signup handler.
func NewSignupHandler(
	tr *repository.TenantRepo,
	mt *repository.MembershipTypeRepo,
	br *repository.BonusRepo,
	terms *repository.TermsRepo,
	cr *repository.ClientRepo,
	mr *repository.MembershipRepo,
	sr *repository.SettingsRepo,
	credR *repository.TenantCredentialRepo,
	bdr *repository.BankDetailsRepo,
	enc *database.Encryptor,
	es email.Provider,
) *SignupHandler {
	return &SignupHandler{
		tenantRepo: tr, mtRepo: mt, bonusRepo: br, termsRepo: terms,
		clientRepo: cr, memberRepo: mr, settingsRepo: sr,
		credRepo: credR, bankDetailsRepo: bdr, encryptor: enc, emailSvc: es,
	}
}

// SetDispatcher sets optional outbound webhook event dispatcher.
func (h *SignupHandler) SetDispatcher(dispatcher *events.Dispatcher) {
	h.dispatcher = dispatcher
}

// RegisterPublicRoutes registers public signup routes (no auth required).
func (h *SignupHandler) RegisterPublicRoutes(r chi.Router) {
	r.Get("/signup/{tenantSlug}", h.GetTenantMemberships)
	r.Get("/signup/{tenantSlug}/{membershipSlug}", h.GetMembershipDetail)
	r.Get("/signup/{tenantSlug}/{membershipSlug}/term/{termOptionID}/bonuses", h.GetTermBonuses)
	r.Post("/signup/{tenantSlug}/submit", h.SubmitSignup)
}

// GetTenantMemberships returns all active memberships for a tenant (public browsing).
func (h *SignupHandler) GetTenantMemberships(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "tenantSlug")
	tenant, err := h.tenantRepo.FindBySlug(r.Context(), slug)
	if err != nil || tenant == nil || !tenant.IsActive {
		writeError(w, http.StatusNotFound, "Organisation not found")
		return
	}

	types, err := h.mtRepo.List(r.Context(), tenant.ID)
	if err != nil {
		log.Printf("Error listing memberships for signup: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to load memberships")
		return
	}

	// Filter to active only and attach term options for price display
	var active []models.MembershipType
	for _, mt := range types {
		if mt.IsActive {
			full, err := h.mtRepo.FindByID(r.Context(), mt.ID, tenant.ID)
			if err == nil && full != nil {
				active = append(active, *full)
			} else {
				active = append(active, mt)
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tenant": map[string]interface{}{
			"name":                      tenant.Name,
			"slug":                      tenant.Slug,
			"logo_url":                  tenant.LogoURL,
			"primary_colour":            tenant.PrimaryColour,
			"secondary_colour":          tenant.SecondaryColour,
			"signup_price_display_mode": tenant.SignupPriceDisplayMode,
		},
		"memberships":       active,
		"payment_providers": h.availablePaymentProviders(r.Context(), tenant.ID),
	})
}

// GetMembershipDetail returns full details for a membership type (public).
func (h *SignupHandler) GetMembershipDetail(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")
	membershipSlug := chi.URLParam(r, "membershipSlug")

	tenant, err := h.tenantRepo.FindBySlug(r.Context(), tenantSlug)
	if err != nil || tenant == nil || !tenant.IsActive {
		writeError(w, http.StatusNotFound, "Organisation not found")
		return
	}

	mt, err := h.mtRepo.FindBySlug(r.Context(), membershipSlug, tenant.ID)
	if err != nil || mt == nil || !mt.IsActive {
		writeError(w, http.StatusNotFound, "Membership not found")
		return
	}

	// Load bonuses for each term option
	for i, to := range mt.TermOptions {
		bonuses, _ := h.bonusRepo.GetBonusesForTermOption(r.Context(), to.ID, tenant.ID)
		mt.TermOptions[i].Bonuses = bonuses
	}

	// Load attached T&Cs
	terms, _ := h.termsRepo.GetForMembershipType(r.Context(), mt.ID, tenant.ID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tenant": map[string]interface{}{
			"name":                      tenant.Name,
			"slug":                      tenant.Slug,
			"logo_url":                  tenant.LogoURL,
			"primary_colour":            tenant.PrimaryColour,
			"secondary_colour":          tenant.SecondaryColour,
			"signup_price_display_mode": tenant.SignupPriceDisplayMode,
		},
		"membership":        mt,
		"terms":             terms,
		"payment_providers": h.availablePaymentProviders(r.Context(), tenant.ID),
	})
}

// GetTermBonuses returns bonuses for a specific term option (for dynamic display).
func (h *SignupHandler) GetTermBonuses(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")
	termOptionID, err := uuid.Parse(chi.URLParam(r, "termOptionID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid term option ID")
		return
	}

	tenant, err := h.tenantRepo.FindBySlug(r.Context(), tenantSlug)
	if err != nil || tenant == nil {
		writeError(w, http.StatusNotFound, "Organisation not found")
		return
	}

	bonuses, err := h.bonusRepo.GetBonusesForTermOption(r.Context(), termOptionID, tenant.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load bonuses")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"bonuses": bonuses})
}

// SubmitSignup processes a completed signup form.
func (h *SignupHandler) SubmitSignup(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")

	tenant, err := h.tenantRepo.FindBySlug(r.Context(), tenantSlug)
	if err != nil || tenant == nil || !tenant.IsActive {
		writeError(w, http.StatusNotFound, "Organisation not found")
		return
	}

	var input SignupSubmission
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid submission data")
		return
	}

	if input.Client.Name == "" || input.TermOptionID == "" || input.MembershipTypeID == "" {
		writeError(w, http.StatusBadRequest, "Client name, membership type and term option are required")
		return
	}
	if input.PaymentMethod == "" {
		input.PaymentMethod = "direct_debit"
	}
	if input.PaymentIntervalCount <= 0 {
		input.PaymentIntervalCount = 1
	}
	if input.PaymentIntervalUnit != nil {
		unit := strings.ToLower(strings.TrimSpace(*input.PaymentIntervalUnit))
		if unit != "week" && unit != "month" {
			writeError(w, http.StatusBadRequest, "payment_interval_unit must be 'week' or 'month'")
			return
		}
		input.PaymentIntervalUnit = &unit
	}
	if input.PaymentFrequency == "custom_schedule" {
		if input.PaymentIntervalUnit == nil || *input.PaymentIntervalUnit == "" {
			writeError(w, http.StatusBadRequest, "custom schedule requires payment_interval_unit")
			return
		}
		if input.PaymentIntervalCount <= 0 {
			writeError(w, http.StatusBadRequest, "custom schedule requires payment_interval_count > 0")
			return
		}
		if input.NumPayments == nil || *input.NumPayments <= 0 {
			writeError(w, http.StatusBadRequest, "custom schedule requires num_payments > 0")
			return
		}
	}

	// Create or find client
	client, err := h.clientRepo.Create(r.Context(), tenant.ID, &input.Client)
	if err != nil {
		log.Printf("Error creating client during signup: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create client record")
		return
	}
	if h.dispatcher != nil {
		h.dispatcher.Dispatch(r.Context(), events.NewEvent(events.ClientCreated, tenant.ID, map[string]interface{}{
			"client_id": client.ID,
			"name":      client.Name,
			"email":     client.Email,
			"phone":     client.Phone,
			"source":    "signup",
		}))
	}

	// Parse IDs
	mtID, _ := uuid.Parse(input.MembershipTypeID)
	toID, _ := uuid.Parse(input.TermOptionID)
	staffID := parseOptionalUUID(input.StaffUserID)

	// Create membership
	membership, err := h.memberRepo.Create(r.Context(), tenant.ID, client.ID, mtID, toID,
		input.StartDate, input.PaymentMethod, input.PaymentFrequency, input.PaymentAmount,
		input.PaymentIntervalUnit, input.PaymentIntervalCount, input.NumPayments,
		input.MemberSignature, staffID, input.StaffSignature)
	if err != nil {
		log.Printf("Error creating membership during signup: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create membership")
		return
	}
	if h.dispatcher != nil {
		h.dispatcher.Dispatch(r.Context(), events.NewEvent(events.MembershipCreated, tenant.ID, map[string]interface{}{
			"membership_id":      membership.ID,
			"client_id":          membership.ClientID,
			"membership_type_id": membership.MembershipTypeID,
			"term_option_id":     membership.TermOptionID,
			"status":             membership.Status,
			"source":             "signup",
		}))
	}

	// --- Stripe card checkout path ---
	if input.PaymentMethod == "card" {
		checkoutURL, err := h.createStripeCheckoutSession(r, tenant, client, membership, input.PaymentFrequency, input.PaymentIntervalUnit, input.PaymentIntervalCount, input.PaymentAmount)
		if err != nil {
			log.Printf("Error creating Stripe checkout session during signup: %v", err)
			writeError(w, http.StatusBadGateway, "Card payment setup failed")
			return
		}
		mt, _ := h.mtRepo.FindByID(r.Context(), mtID, tenant.ID)
		thankyou := buildThankyouResponse(tenant, mt)
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"membership":   membership,
			"client":       client,
			"thankyou":     thankyou,
			"checkout_url": checkoutURL,
		})
		return
	}

	// --- GoCardless direct debit path ---
	gcMandateID := ""
	if input.PaymentMethod == "direct_debit" {
		if strings.TrimSpace(input.BSB) == "" || strings.TrimSpace(input.AccountNumber) == "" {
			writeError(w, http.StatusBadRequest, "BSB and account number are required for direct debit")
			return
		}

		prov, provErr := h.getGoCardlessProvider(r.Context(), tenant.ID)
		if provErr != nil {
			log.Printf("GoCardless provider init during signup: %v", provErr)
			writeError(w, http.StatusBadGateway, "Direct debit setup failed")
			return
		}
		if prov == nil {
			writeError(w, http.StatusServiceUnavailable, "Direct debit is not configured")
			return
		}

		// Create or reuse GoCardless customer
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
				var gce *payment.GCError
				if errors.As(err, &gce) {
					log.Printf("GoCardless CreateCustomer during signup: %s (raw: %s)", gce.Message, gce.Raw)
					writeError(w, http.StatusBadGateway, gce.Message)
				} else {
					log.Printf("GoCardless CreateCustomer during signup: %v", err)
					writeError(w, http.StatusBadGateway, "Direct debit setup failed")
				}
				return
			}
			gcCustomerID = cid
			_ = h.clientRepo.SetGoCardlessCustomerID(r.Context(), client.ID, tenant.ID, cid)
		}

		holderName := input.AccountHolderName
		if holderName == "" {
			holderName = client.Name
		}
		mandateID, err := prov.SetupDirectDebit(r.Context(), payment.SetupDirectDebitInput{
			CustomerID:        gcCustomerID,
			AccountHolderName: holderName,
			AccountNumber:     input.AccountNumber,
			BSB:               input.BSB,
			Reference:         membership.ID.String(),
		})
		if err != nil {
			log.Printf("GoCardless SetupDirectDebit during signup: %v", err)
			writeError(w, http.StatusBadGateway, "Direct debit setup failed")
			return
		}
		gcMandateID = mandateID
		_ = h.memberRepo.SetGoCardlessMandateID(r.Context(), membership.ID, tenant.ID, mandateID)
		log.Printf("GoCardless mandate %s created for membership %s (client %s)", mandateID, membership.ID, client.ID)

		// Store bank details (last 4 digits only)
		if h.bankDetailsRepo != nil {
			last4 := input.AccountNumber
			if len(last4) > 4 {
				last4 = last4[len(last4)-4:]
			}
			_ = h.bankDetailsRepo.Create(r.Context(), tenant.ID, client.ID,
				input.AccountHolderName, input.BSB, last4, input.InstitutionName, "")
		}

		// Create GoCardless subscription for recurring payments.
		mtForSub, _ := h.mtRepo.FindByID(r.Context(), mtID, tenant.ID)
		subName := "Membership"
		if mtForSub != nil {
			subName = mtForSub.Title
		}
		numPay := membership.NumPayments
		if numPay == nil {
			if to, _ := h.mtRepo.GetTermOption(r.Context(), toID, tenant.ID); to != nil && to.NumPayments != nil {
				numPay = to.NumPayments
			}
		}
		if _, err := createGoCardlessSubscription(
			r.Context(),
			prov,
			h.memberRepo,
			mandateID,
			membership,
			tenant.ID,
			input.PaymentFrequency,
			input.PaymentAmount,
			membership.PaymentIntervalUnit,
			membership.PaymentIntervalCount,
			subName,
			membership.MinTermEndDate,
			numPay,
		); err != nil {
			writeError(w, http.StatusBadGateway, "Direct debit subscription setup failed")
			return
		}
	}

	// Determine thank you page
	mt, _ := h.mtRepo.FindByID(r.Context(), mtID, tenant.ID)
	thankyou := buildThankyouResponse(tenant, mt)

	// Send membership confirmation email to client (async)
	if client.Email != nil && *client.Email != "" && mt != nil {
		go func() {
			emailInput := email.MembershipConfirmationEmail(
				client.Name, mt.Title, 0, input.PaymentAmount, tenant.Name)
			emailInput.To = *client.Email
			if _, err := h.emailSvc.Send(context.Background(), emailInput); err != nil {
				log.Printf("Error sending membership confirmation to %s: %v", *client.Email, err)
			}
		}()
	}

	resp := map[string]interface{}{
		"membership": membership,
		"client":     client,
		"thankyou":   thankyou,
	}
	if gcMandateID != "" {
		resp["gocardless_mandate_id"] = gcMandateID
	}
	writeJSON(w, http.StatusCreated, resp)
}

// SignupSubmission is the full signup form data.
type SignupSubmission struct {
	Client               models.Client `json:"client"`
	MembershipTypeID     string        `json:"membership_type_id"`
	TermOptionID         string        `json:"term_option_id"`
	StartDate            time.Time     `json:"start_date"`
	PaymentMethod        string        `json:"payment_method"`
	PaymentFrequency     string        `json:"payment_frequency"`
	PaymentIntervalUnit  *string       `json:"payment_interval_unit,omitempty"`
	PaymentIntervalCount int           `json:"payment_interval_count,omitempty"`
	PaymentAmount        float64       `json:"payment_amount"`
	NumPayments          *int          `json:"num_payments,omitempty"`
	MemberSignature      string        `json:"member_signature"`
	StaffUserID          string        `json:"staff_user_id,omitempty"`
	StaffSignature       string        `json:"staff_signature,omitempty"`
	// Bank details
	InstitutionName   string `json:"institution_name,omitempty"`
	BranchLocation    string `json:"branch_location,omitempty"`
	AccountHolderName string `json:"account_holder_name,omitempty"`
	AccountNumber     string `json:"account_number,omitempty"`
	BSB               string `json:"bsb,omitempty"`
}

func buildThankyouResponse(tenant *models.Tenant, mt *models.MembershipType) map[string]interface{} {
	result := map[string]interface{}{
		"heading":      "Thank You!",
		"body":         "Your membership has been successfully created.",
		"image_url":    nil,
		"redirect_url": nil,
	}

	// Per-membership override takes priority
	if mt != nil && mt.UseCustomThankyou {
		if mt.ThankyouHeading != nil {
			result["heading"] = *mt.ThankyouHeading
		}
		if mt.ThankyouBody != nil {
			result["body"] = *mt.ThankyouBody
		}
		result["image_url"] = mt.ThankyouImageURL
		result["redirect_url"] = mt.ThankyouRedirectURL
		return result
	}

	// Fall back to tenant default
	if tenant.DefaultThankyouHeading != nil {
		result["heading"] = *tenant.DefaultThankyouHeading
	}
	if tenant.DefaultThankyouBody != nil {
		result["body"] = *tenant.DefaultThankyouBody
	}
	result["image_url"] = tenant.DefaultThankyouImageURL
	result["redirect_url"] = tenant.DefaultThankyouRedirectURL
	return result
}

// getGoCardlessProvider returns a GoCardless provider for the tenant (if configured).
func (h *SignupHandler) getGoCardlessProvider(ctx context.Context, tenantID uuid.UUID) (payment.GoCardlessExtended, error) {
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

func parseOptionalUUID(s string) *uuid.UUID {
	if s == "" {
		return nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil
	}
	return &id
}

func (h *SignupHandler) availablePaymentProviders(ctx context.Context, tenantID uuid.UUID) []string {
	providers := make([]string, 0, 2)
	if gc, _ := h.credRepo.GetByProvider(ctx, tenantID, "gocardless"); gc != nil && gc.APIKeyEncrypted != "" && gc.IsActive {
		providers = append(providers, "gocardless")
	}
	if stripeCreds, _ := h.credRepo.GetByProvider(ctx, tenantID, "stripe"); stripeCreds != nil && stripeCreds.APIKeyEncrypted != "" && stripeCreds.IsActive {
		providers = append(providers, "stripe")
	}
	return providers
}

func (h *SignupHandler) createStripeCheckoutSession(r *http.Request, tenant *models.Tenant, client *models.Client, membership *models.Membership, frequency string, intervalUnit *string, intervalCount int, amount float64) (string, error) {
	if h.credRepo == nil || h.encryptor == nil {
		return "", errors.New("stripe integration unavailable")
	}
	creds, err := h.credRepo.GetByProvider(r.Context(), tenant.ID, "stripe")
	if err != nil || creds == nil || creds.APIKeyEncrypted == "" {
		return "", errors.New("stripe not configured for tenant")
	}
	secretKey, err := h.encryptor.Decrypt(creds.APIKeyEncrypted)
	if err != nil || secretKey == "" {
		return "", errors.New("stripe credentials invalid")
	}

	stripe.Key = secretKey
	origin := strings.TrimSuffix(r.Header.Get("Origin"), "/")
	if origin == "" {
		origin = "https://app.clientroll.com"
	}

	unitAmount := int64(math.Round(amount * 100))
	if unitAmount <= 0 {
		unitAmount = 1
	}

	mode := stripe.CheckoutSessionModePayment
	if frequency != "upfront" {
		mode = stripe.CheckoutSessionModeSubscription
	}

	params := &stripe.CheckoutSessionParams{
		Mode:              stripe.String(string(mode)),
		SuccessURL:        stripe.String(origin + "/join/" + tenant.Slug + "?stripe=success"),
		CancelURL:         stripe.String(origin + "/join/" + tenant.Slug + "?stripe=cancelled"),
		ClientReferenceID: stripe.String(client.ID.String()),
		Metadata: map[string]string{
			"tenant_id":      tenant.ID.String(),
			"tenant_slug":    tenant.Slug,
			"client_id":      client.ID.String(),
			"membership_id":  membership.ID.String(),
			"payment_amount": strconv.FormatFloat(amount, 'f', 2, 64),
			"payment_freq":   frequency,
		},
	}
	if client.Email != nil && *client.Email != "" {
		params.CustomerEmail = stripe.String(*client.Email)
	}

	lineItem := &stripe.CheckoutSessionLineItemParams{
		Quantity: stripe.Int64(1),
		PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
			Currency:   stripe.String(string(stripe.CurrencyAUD)),
			UnitAmount: stripe.Int64(unitAmount),
			ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
				Name: stripe.String("Membership - " + tenant.Name),
			},
		},
	}
	if mode == stripe.CheckoutSessionModeSubscription {
		interval, count := stripeRecurringInterval(frequency, intervalUnit, intervalCount)
		lineItem.PriceData.Recurring = &stripe.CheckoutSessionLineItemPriceDataRecurringParams{
			Interval:      stripe.String(interval),
			IntervalCount: stripe.Int64(count),
		}
		params.SubscriptionData = &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: map[string]string{
				"tenant_id":     tenant.ID.String(),
				"tenant_slug":   tenant.Slug,
				"client_id":     client.ID.String(),
				"membership_id": membership.ID.String(),
			},
		}
	}
	params.LineItems = []*stripe.CheckoutSessionLineItemParams{lineItem}

	sess, err := checkoutsession.New(params)
	if err != nil {
		return "", err
	}
	return sess.URL, nil
}

func stripeRecurringInterval(freq string, intervalUnit *string, intervalCount int) (string, int64) {
	if intervalUnit != nil && *intervalUnit != "" {
		count := int64(intervalCount)
		if count <= 0 {
			count = 1
		}
		if *intervalUnit == "week" {
			return "week", count
		}
		return "month", count
	}
	switch freq {
	case "weekly":
		return "week", 1
	case "fortnightly":
		return "week", 2
	case "quarterly":
		return "month", 3
	default:
		return "month", 1
	}
}
