package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"clientroll/internal/config"
	"clientroll/internal/database"
	"clientroll/internal/email"
	"clientroll/internal/events"
	"clientroll/internal/models"
	"clientroll/internal/payment"
	pdfpkg "clientroll/internal/pdf"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v82"
	checkoutsession "github.com/stripe/stripe-go/v82/checkout/session"
)

// SignupHandler handles the public membership signup flow.
// These endpoints are accessible without authentication (resolved via tenant slug).
type SignupHandler struct {
	tenantRepo        *repository.TenantRepo
	mtRepo            *repository.MembershipTypeRepo
	bonusRepo         *repository.BonusRepo
	termsRepo         *repository.TermsRepo
	clientRepo        *repository.ClientRepo
	memberRepo        *repository.MembershipRepo
	settingsRepo      *repository.SettingsRepo
	credRepo          *repository.TenantCredentialRepo
	bankDetailsRepo   *repository.BankDetailsRepo
	agreementRepo     *repository.AgreementSnapshotRepo
	emailTemplateRepo *repository.TenantEmailTemplateRepo
	encryptor         *database.Encryptor
	emailSvc          email.Provider
	dispatcher        *events.Dispatcher
	cfg               *config.Config
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
	agr *repository.AgreementSnapshotRepo,
	etr *repository.TenantEmailTemplateRepo,
	enc *database.Encryptor,
	es email.Provider,
	appCfg *config.Config,
) *SignupHandler {
	return &SignupHandler{
		tenantRepo: tr, mtRepo: mt, bonusRepo: br, termsRepo: terms,
		clientRepo: cr, memberRepo: mr, settingsRepo: sr,
		credRepo: credR, bankDetailsRepo: bdr, agreementRepo: agr, emailTemplateRepo: etr,
		encryptor: enc, emailSvc: es, cfg: appCfg,
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
			"name":                        tenant.Name,
			"slug":                        tenant.Slug,
			"logo_url":                    tenant.LogoURL,
			"primary_colour":              tenant.PrimaryColour,
			"secondary_colour":            tenant.SecondaryColour,
			"signup_price_display_mode":   tenant.SignupPriceDisplayMode,
			"enabled_payment_frequencies": tenant.EnabledPaymentFrequencies,
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
			"name":                        tenant.Name,
			"slug":                        tenant.Slug,
			"logo_url":                    tenant.LogoURL,
			"primary_colour":              tenant.PrimaryColour,
			"secondary_colour":            tenant.SecondaryColour,
			"signup_price_display_mode":   tenant.SignupPriceDisplayMode,
			"enabled_payment_frequencies": tenant.EnabledPaymentFrequencies,
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
		log.Printf("Signup decode error: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid submission data")
		return
	}

	// Parse start_date flexibly (accepts "2006-01-02" or RFC3339)
	if input.StartDateRaw != "" {
		if t, err := time.Parse(time.RFC3339, input.StartDateRaw); err == nil {
			input.StartDate = t
		} else if t, err := time.Parse("2006-01-02", input.StartDateRaw); err == nil {
			input.StartDate = t
		} else {
			writeError(w, http.StatusBadRequest, "Invalid start_date format")
			return
		}
	} else {
		input.StartDate = time.Now()
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

	// Look up existing client by email, or create new
	var client *models.Client
	if input.Client.Email != nil && *input.Client.Email != "" {
		client, err = h.clientRepo.FindByEmail(r.Context(), tenant.ID, *input.Client.Email)
		if err != nil {
			log.Printf("Error looking up client by email: %v", err)
		}
	}
	if client != nil {
		// Update existing client's details with latest info from signup
		client, err = h.clientRepo.Update(r.Context(), client.ID, tenant.ID, &input.Client)
		if err != nil {
			log.Printf("Error updating existing client during signup: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to update client record")
			return
		}
	} else {
		client, err = h.clientRepo.Create(r.Context(), tenant.ID, &input.Client)
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
	}

	// Parse IDs
	mtID, _ := uuid.Parse(input.MembershipTypeID)
	toID, _ := uuid.Parse(input.TermOptionID)
	staffID := parseOptionalUUID(input.StaffUserID)
	mtForCreate, _ := h.mtRepo.FindByID(r.Context(), mtID, tenant.ID)
	membershipCurrency := "AUD"
	if mtForCreate != nil && strings.TrimSpace(mtForCreate.CurrencyCode) != "" {
		membershipCurrency = strings.ToUpper(strings.TrimSpace(mtForCreate.CurrencyCode))
	}

	// Create membership
	membership, err := h.memberRepo.Create(r.Context(), tenant.ID, client.ID, mtID, toID,
		input.StartDate, input.PaymentMethod, input.PaymentFrequency, membershipCurrency, input.PaymentAmount,
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
		checkoutURL, err := h.createStripeCheckoutSession(r, tenant, client, membership, membershipCurrency, input.PaymentFrequency, input.PaymentIntervalUnit, input.PaymentIntervalCount, input.PaymentAmount)
		if err != nil {
			log.Printf("Error creating Stripe checkout session during signup: %v", err)
			writeError(w, http.StatusBadGateway, "Card payment setup failed")
			return
		}
		mt := mtForCreate
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
		if membershipCurrency != "AUD" {
			writeError(w, http.StatusBadRequest, "Direct debit currently supports AUD memberships only")
			return
		}
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
			log.Printf("GoCardless not active for tenant %s — skipping direct debit setup, membership created without mandate", tenant.ID)
			h.createAgreementSnapshot(r, tenant, client, membership, input, mtID, toID)
			mt, _ := h.mtRepo.FindByID(r.Context(), mtID, tenant.ID)
			thankyou := buildThankyouResponse(tenant, mt)
			thankyou["body"] = "Your membership has been created. Payment details will be set up separately."
			writeJSON(w, http.StatusCreated, map[string]interface{}{
				"membership":       membership,
				"client":           client,
				"thankyou":         thankyou,
				"gocardless_error": "Direct debit is not yet configured. Payment will be arranged separately.",
			})
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
		mtForSub := mtForCreate
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
	mt := mtForCreate
	thankyou := buildThankyouResponse(tenant, mt)

	// Create agreement snapshot for legal custody
	snapshotID := h.createAgreementSnapshot(r, tenant, client, membership, input, mtID, toID)

	// Send membership confirmation email to client (async)
	if client.Email != nil && *client.Email != "" && mt != nil {
		sendAgreement := input.SendAgreementEmail
		clientEmail := *client.Email
		clientName := client.Name
		mtTitle := mt.Title
		tenantName := tenant.Name
		memID := membership.ID
		tenID := tenant.ID
		payAmt := input.PaymentAmount
		go func() {
			templateKey := "membership_confirmation"
			emailInput := email.MembershipConfirmationEmail(clientName, mtTitle, 0, payAmt, tenantName)
			emailInput.To = clientEmail
			if sendAgreement && snapshotID != "" && h.agreementRepo != nil {
				snap, _ := h.agreementRepo.FindByMembership(context.Background(), memID, tenID)
				if snap != nil && snap.AgreementHTML != nil {
					templateKey = "membership_agreement"
					emailInput = email.MembershipAgreementEmail(clientName, mtTitle, tenantName)
					emailInput.To = clientEmail
					pdfBytes, pdfErr := pdfpkg.HTMLToPDF(*snap.AgreementHTML)
					if pdfErr != nil {
						log.Printf("PDF generation for email failed: %v", pdfErr)
					} else {
						emailInput.Attachments = []email.Attachment{{
							Filename: fmt.Sprintf("Agreement-%s.pdf", snap.CreatedAt.Format("2006-01-02")),
							Content:  pdfBytes,
							MimeType: "application/pdf",
						}}
					}
				}
			}
			if !h.isEmailTemplateEnabled(context.Background(), tenID, templateKey) {
				log.Printf("signup.email skipped tenant=%s template=%s membership=%s", tenID, templateKey, memID)
				return
			}
			if _, err := h.emailSvc.Send(context.Background(), emailInput); err != nil {
				log.Printf("Error sending membership confirmation to %s: %v", clientEmail, err)
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
	StartDateRaw         string        `json:"start_date"`
	StartDate            time.Time     `json:"-"`
	PaymentMethod        string        `json:"payment_method"`
	PaymentFrequency     string        `json:"payment_frequency"`
	PaymentIntervalUnit  *string       `json:"payment_interval_unit,omitempty"`
	PaymentIntervalCount int           `json:"payment_interval_count,omitempty"`
	PaymentAmount        float64       `json:"payment_amount"`
	NumPayments          *int          `json:"num_payments,omitempty"`
	MemberSignature      string        `json:"member_signature"`
	AcceptedTermsIDs     []string      `json:"accepted_terms_ids,omitempty"`
	SendAgreementEmail   bool          `json:"send_agreement_email,omitempty"`
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
	if err != nil || creds == nil || creds.APIKeyEncrypted == "" || !creds.IsActive {
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

func (h *SignupHandler) isEmailTemplateEnabled(ctx context.Context, tenantID uuid.UUID, templateKey string) bool {
	if h.emailTemplateRepo == nil {
		return true
	}
	enabled, err := h.emailTemplateRepo.IsEnabled(ctx, tenantID, templateKey)
	if err != nil {
		log.Printf("signup.email toggle check failed tenant=%s template=%s err=%v", tenantID, templateKey, err)
		return true
	}
	return enabled
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

func (h *SignupHandler) createStripeCheckoutSession(r *http.Request, tenant *models.Tenant, client *models.Client, membership *models.Membership, currencyCode string, frequency string, intervalUnit *string, intervalCount int, amount float64) (string, error) {
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
			"currency_code":  currencyCode,
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
	if strings.TrimSpace(currencyCode) != "" {
		lineItem.PriceData.Currency = stripe.String(strings.ToLower(currencyCode))
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

func (h *SignupHandler) createAgreementSnapshot(r *http.Request, tenant *models.Tenant, client *models.Client, membership *models.Membership, input SignupSubmission, mtID, toID uuid.UUID) string {
	if h.agreementRepo == nil {
		return ""
	}
	mt, _ := h.mtRepo.FindByID(r.Context(), mtID, tenant.ID)
	if mt == nil {
		return ""
	}
	agreementSettings, _ := h.settingsRepo.GetAgreementSettings(r.Context(), tenant.ID)
	now := time.Now()

	snap := &repository.AgreementSnapshot{
		TenantID:     tenant.ID,
		MembershipID: membership.ID,
		ClientID:     client.ID,

		// Client details
		ClientName:          client.Name,
		ClientEmail:         client.Email,
		ClientPhone:         client.Phone,
		MemberSignatureData: &input.MemberSignature,
		MemberSignedAt:      &now,
	}

	// Client address
	if client.Address != nil {
		addr := *client.Address
		if client.Suburb != nil {
			addr += ", " + *client.Suburb
		}
		if client.State != nil {
			addr += " " + *client.State
		}
		if client.Postcode != nil {
			addr += " " + *client.Postcode
		}
		snap.ClientAddress = &addr
	}
	// Client DOB, gender, emergency
	if client.BirthDate != nil {
		dob := client.BirthDate.Format("2006-01-02")
		snap.ClientDateOfBirth = &dob
	}
	snap.ClientGender = client.Gender
	snap.ClientEmergencyContactName = client.EmergencyContactName
	snap.ClientEmergencyContactPhone = client.EmergencyContactPhone

	// Staff signature
	if input.StaffSignature != "" {
		snap.StaffSignatureData = &input.StaffSignature
		snap.StaffSignedAt = &now
	}

	// Membership type details
	snap.MembershipTypeTitle = mt.Title
	snap.MembershipName = mt.MembershipName
	snap.MembershipLevel = mt.MembershipLevel
	snap.MembershipDescription = mt.Description
	snap.MembershipInclusions = mt.Inclusions
	snap.MembershipBenefits = mt.Benefits

	// Payment from input
	snap.PaymentMethod = &input.PaymentMethod
	snap.PaymentFrequency = &input.PaymentFrequency
	snap.PaymentAmount = &input.PaymentAmount
	snap.PaymentIntervalUnit = input.PaymentIntervalUnit
	snap.PaymentIntervalCount = input.PaymentIntervalCount
	snap.NumPayments = input.NumPayments
	snap.StartDate = input.StartDate.Format("2006-01-02")

	// End date from membership
	endDate := membership.MinTermEndDate.Format("2006-01-02")
	snap.EndDate = &endDate

	// Term option details
	snap.TotalFee = input.PaymentAmount
	if to, _ := h.mtRepo.GetTermOption(r.Context(), toID, tenant.ID); to != nil {
		snap.TermMonths = to.DurationMonths
		snap.TotalFee = to.FeeTotal
		termLabel := fmt.Sprintf("%d months", to.DurationMonths)
		snap.TermLabel = &termLabel

		// Fee breakdown
		snap.FeeWeekly = to.FeeWeekly
		snap.FeeFortnightly = to.FeeFortnightly
		snap.FeeMonthly = to.FeeMonthly
		snap.FeeQuarterly = to.FeeQuarterly
		snap.AllowUpfront = to.AllowUpfront

		if to.NumPayments != nil && snap.NumPayments == nil {
			snap.NumPayments = to.NumPayments
		}

		// Custom schedule
		if to.CustomScheduleEnabled {
			snap.CustomScheduleLabel = to.CustomScheduleLabel
			snap.CustomScheduleAmount = to.CustomScheduleAmount
			snap.CustomScheduleIntervalUnit = to.CustomScheduleIntervalUnit
			snap.CustomScheduleIntervalCount = to.CustomScheduleIntervalCount
			snap.CustomScheduleNumPayments = to.CustomScheduleNumPayments
		}
	}

	// Organisation details
	if agreementSettings != nil {
		snap.OrgCompanyName = agreementSettings.CompanyName
		snap.OrgTradingName = agreementSettings.TradingName
		snap.OrgABN = agreementSettings.ABN
		snap.OrgACN = agreementSettings.ACN
		orgAddr := ""
		if agreementSettings.Address != nil {
			orgAddr = *agreementSettings.Address
		}
		if agreementSettings.State != nil {
			orgAddr += " " + *agreementSettings.State
		}
		if agreementSettings.Postcode != nil {
			orgAddr += " " + *agreementSettings.Postcode
		}
		orgAddr = strings.TrimSpace(orgAddr)
		if orgAddr != "" {
			snap.OrgAddress = &orgAddr
		}
		snap.OrgEmail = agreementSettings.Email
		snap.OrgPhone = agreementSettings.Phone
	}

	// Metadata
	ipAddr := r.Header.Get("X-Forwarded-For")
	if ipAddr == "" {
		ipAddr = r.RemoteAddr
	}
	snap.IPAddress = &ipAddr
	ua := r.Header.Get("User-Agent")
	snap.UserAgent = &ua

	// Accepted T&C
	var termsInputs []repository.AcceptedTermInput
	for _, tidStr := range input.AcceptedTermsIDs {
		tid, err := uuid.Parse(tidStr)
		if err != nil {
			continue
		}
		tc, err := h.termsRepo.FindByID(r.Context(), tid, tenant.ID)
		if err != nil || tc == nil {
			continue
		}
		termsInputs = append(termsInputs, repository.AcceptedTermInput{
			TermsID:     tc.ID,
			Title:       tc.Title,
			Version:     tc.Version,
			ContentHTML: tc.Content,
		})
	}

	agreementHTML := renderAgreementHTML(snap, termsInputs, tenant)
	snap.AgreementHTML = &agreementHTML

	created, err := h.agreementRepo.Create(r.Context(), snap, termsInputs)
	if err != nil {
		log.Printf("Error creating agreement snapshot: %v", err)
		return ""
	}
	return created.ID.String()
}

func renderAgreementHTML(snap *repository.AgreementSnapshot, terms []repository.AcceptedTermInput, tenant *models.Tenant) string {
	deref := func(s *string) string {
		if s != nil {
			return *s
		}
		return ""
	}
	derefF := func(f *float64) string {
		if f != nil {
			return fmt.Sprintf("$%.2f", *f)
		}
		return "—"
	}
	derefI := func(i *int) string {
		if i != nil {
			return fmt.Sprintf("%d", *i)
		}
		return "—"
	}
	row := func(label, value string) string {
		if value == "" || value == "—" {
			return ""
		}
		return fmt.Sprintf(`<tr><td>%s</td><td>%s</td></tr>`, label, value)
	}

	var b strings.Builder
	b.WriteString(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Membership Agreement</title>`)
	b.WriteString(`<style>
		*{box-sizing:border-box;}
		body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;max-width:800px;margin:0 auto;padding:40px 30px;color:#1f2937;font-size:12px;line-height:1.55;}
		h1{font-size:22px;margin:0 0 2px;color:#111827;}
		h2{font-size:15px;font-weight:700;margin:28px 0 10px;border-bottom:2px solid #111827;padding-bottom:5px;color:#111827;text-transform:uppercase;letter-spacing:0.5px;}
		h3{font-size:13px;font-weight:700;margin:16px 0 6px;color:#111827;}
		table.d{width:100%;border-collapse:collapse;margin:6px 0 14px;}
		table.d td{padding:5px 10px;border:1px solid #d1d5db;vertical-align:top;}
		table.d td:first-child{background:#f3f4f6;font-weight:600;width:180px;color:#374151;}
		table.sched{width:100%;border-collapse:collapse;margin:6px 0 14px;font-size:11px;}
		table.sched th{background:#111827;color:#fff;padding:6px 10px;text-align:left;font-weight:600;font-size:11px;}
		table.sched td{padding:5px 10px;border:1px solid #d1d5db;}
		table.sched tr:nth-child(even){background:#f9fafb;}
		.sig-block{margin:16px 0;padding:14px;border:1px solid #d1d5db;border-radius:6px;background:#fafafa;}
		.sig-block img{max-height:70px;margin:6px 0;}
		.tc-block{margin:12px 0;padding:14px;border:1px solid #d1d5db;border-radius:6px;background:#f9fafb;page-break-inside:avoid;}
		.tc-block h3{margin:0 0 6px;font-size:13px;}
		.tc-accepted{color:#059669;font-weight:700;margin-top:8px;font-size:12px;}
		.tc-content{font-size:11px;line-height:1.5;color:#374151;}
		.tc-content h1,.tc-content h2,.tc-content h3{font-size:12px;margin:8px 0 4px;}
		.tc-content p{margin:4px 0;} .tc-content ul,.tc-content ol{margin:4px 0 4px 16px;}
		.section-desc{font-size:11px;color:#4b5563;line-height:1.5;margin:4px 0 12px;padding:10px 12px;background:#f9fafb;border-left:3px solid #6b7280;border-radius:0 4px 4px 0;}
		.footer{margin-top:30px;padding-top:14px;border-top:2px solid #111827;font-size:10px;color:#6b7280;}
		.legal-notice{background:#fffbeb;border:1px solid #fde68a;border-radius:6px;padding:12px;margin:20px 0;font-size:11px;color:#92400e;line-height:1.5;}
		@media print{body{padding:15px;font-size:11px;} @page{margin:1.5cm;size:A4;}}
	</style></head><body>`)

	// ─── Header ───
	b.WriteString(fmt.Sprintf(`<h1>Membership Agreement</h1>`))
	b.WriteString(fmt.Sprintf(`<p style="color:#6b7280;margin:0 0 4px;font-size:13px;">%s</p>`, tenant.Name))
	b.WriteString(fmt.Sprintf(`<p style="color:#9ca3af;margin:0 0 20px;font-size:11px;">Agreement ID: %s &nbsp;|&nbsp; Date: %s</p>`,
		snap.MembershipID, time.Now().Format("2 January 2006")))

	// ─── 1. Parties ───
	b.WriteString(`<h2>1. Parties to This Agreement</h2>`)

	b.WriteString(`<h3>Service Provider</h3><table class="d">`)
	b.WriteString(row("Company Name", deref(snap.OrgCompanyName)))
	b.WriteString(row("Trading As", deref(snap.OrgTradingName)))
	b.WriteString(row("ABN", deref(snap.OrgABN)))
	b.WriteString(row("ACN", deref(snap.OrgACN)))
	b.WriteString(row("Address", deref(snap.OrgAddress)))
	b.WriteString(row("Email", deref(snap.OrgEmail)))
	b.WriteString(row("Phone", deref(snap.OrgPhone)))
	b.WriteString(`</table>`)

	b.WriteString(`<h3>Member</h3><table class="d">`)
	b.WriteString(row("Full Name", snap.ClientName))
	b.WriteString(row("Email", deref(snap.ClientEmail)))
	b.WriteString(row("Phone", deref(snap.ClientPhone)))
	b.WriteString(row("Address", deref(snap.ClientAddress)))
	b.WriteString(row("Date of Birth", deref(snap.ClientDateOfBirth)))
	b.WriteString(row("Gender", deref(snap.ClientGender)))
	b.WriteString(row("Emergency Contact", deref(snap.ClientEmergencyContactName)))
	b.WriteString(row("Emergency Phone", deref(snap.ClientEmergencyContactPhone)))
	b.WriteString(`</table>`)

	// ─── 2. Membership Details ───
	b.WriteString(`<h2>2. Membership Details</h2><table class="d">`)
	title := snap.MembershipTypeTitle
	if snap.MembershipName != nil && *snap.MembershipName != "" {
		title = *snap.MembershipName
	}
	b.WriteString(row("Membership", title))
	if snap.MembershipLevel != nil && *snap.MembershipLevel != "" {
		b.WriteString(row("Level", *snap.MembershipLevel))
	}
	b.WriteString(row("Commencement Date", snap.StartDate))
	b.WriteString(row("Minimum Term End Date", deref(snap.EndDate)))
	b.WriteString(row("Minimum Term", fmt.Sprintf("%d months", snap.TermMonths)))
	b.WriteString(`</table>`)

	if snap.MembershipDescription != nil && *snap.MembershipDescription != "" {
		b.WriteString(`<h3>Membership Description</h3>`)
		b.WriteString(fmt.Sprintf(`<div class="section-desc">%s</div>`, *snap.MembershipDescription))
	}
	if snap.MembershipInclusions != nil && *snap.MembershipInclusions != "" {
		b.WriteString(`<h3>What's Included</h3>`)
		b.WriteString(fmt.Sprintf(`<div class="section-desc">%s</div>`, *snap.MembershipInclusions))
	}
	if snap.MembershipBenefits != nil && *snap.MembershipBenefits != "" {
		b.WriteString(`<h3>Benefits</h3>`)
		b.WriteString(fmt.Sprintf(`<div class="section-desc">%s</div>`, *snap.MembershipBenefits))
	}

	// ─── 3. Financial Summary ───
	b.WriteString(`<h2>3. Financial Summary</h2><table class="d">`)
	b.WriteString(fmt.Sprintf(`<tr><td>Total Contract Value</td><td><strong>$%.2f</strong></td></tr>`, snap.TotalFee))
	b.WriteString(row("Payment Method", deref(snap.PaymentMethod)))
	b.WriteString(row("Payment Frequency", deref(snap.PaymentFrequency)))
	b.WriteString(fmt.Sprintf(`<tr><td>Payment Amount</td><td><strong>%s</strong></td></tr>`, derefF(snap.PaymentAmount)))
	if snap.PaymentIntervalUnit != nil && *snap.PaymentIntervalUnit != "" {
		intervalDesc := fmt.Sprintf("Every %d %s(s)", snap.PaymentIntervalCount, *snap.PaymentIntervalUnit)
		b.WriteString(row("Payment Interval", intervalDesc))
	}
	b.WriteString(row("Number of Payments", derefI(snap.NumPayments)))
	b.WriteString(`</table>`)

	// Fee options breakdown
	hasFees := snap.FeeWeekly != nil || snap.FeeFortnightly != nil || snap.FeeMonthly != nil || snap.FeeQuarterly != nil
	if hasFees {
		b.WriteString(`<h3>Available Fee Options</h3><table class="d">`)
		if snap.FeeWeekly != nil {
			b.WriteString(fmt.Sprintf(`<tr><td>Weekly</td><td>$%.2f per week</td></tr>`, *snap.FeeWeekly))
		}
		if snap.FeeFortnightly != nil {
			b.WriteString(fmt.Sprintf(`<tr><td>Fortnightly</td><td>$%.2f per fortnight</td></tr>`, *snap.FeeFortnightly))
		}
		if snap.FeeMonthly != nil {
			b.WriteString(fmt.Sprintf(`<tr><td>Monthly</td><td>$%.2f per month</td></tr>`, *snap.FeeMonthly))
		}
		if snap.FeeQuarterly != nil {
			b.WriteString(fmt.Sprintf(`<tr><td>Quarterly</td><td>$%.2f per quarter</td></tr>`, *snap.FeeQuarterly))
		}
		if snap.AllowUpfront {
			b.WriteString(`<tr><td>Upfront Payment</td><td>Available — full term payable in advance</td></tr>`)
		}
		b.WriteString(`</table>`)
	}

	// Custom schedule
	if snap.CustomScheduleLabel != nil && *snap.CustomScheduleLabel != "" {
		b.WriteString(`<h3>Custom Payment Schedule</h3><table class="d">`)
		b.WriteString(row("Schedule", *snap.CustomScheduleLabel))
		if snap.CustomScheduleAmount != nil {
			b.WriteString(fmt.Sprintf(`<tr><td>Amount per Payment</td><td>$%.2f</td></tr>`, *snap.CustomScheduleAmount))
		}
		if snap.CustomScheduleIntervalUnit != nil && snap.CustomScheduleIntervalCount != nil {
			b.WriteString(row("Interval", fmt.Sprintf("Every %d %s(s)", *snap.CustomScheduleIntervalCount, *snap.CustomScheduleIntervalUnit)))
		}
		b.WriteString(row("Total Payments", derefI(snap.CustomScheduleNumPayments)))
		b.WriteString(`</table>`)
	}

	// ─── 4. Payment Schedule ───
	b.WriteString(`<h2>4. Payment Schedule</h2>`)
	if snap.PaymentAmount != nil && *snap.PaymentAmount > 0 {
		startDate, _ := time.Parse("2006-01-02", snap.StartDate)
		if startDate.IsZero() {
			startDate = time.Now()
		}

		numPay := 0
		if snap.NumPayments != nil && *snap.NumPayments > 0 {
			numPay = *snap.NumPayments
		} else if snap.PaymentAmount != nil && *snap.PaymentAmount > 0 && snap.TotalFee > 0 {
			numPay = int(math.Ceil(snap.TotalFee / *snap.PaymentAmount))
		}

		if numPay > 0 && numPay <= 260 {
			intervalUnit := deref(snap.PaymentIntervalUnit)
			intervalCount := snap.PaymentIntervalCount
			if intervalUnit == "" {
				switch deref(snap.PaymentFrequency) {
				case "weekly":
					intervalUnit, intervalCount = "week", 1
				case "fortnightly":
					intervalUnit, intervalCount = "week", 2
				case "quarterly":
					intervalUnit, intervalCount = "month", 3
				default:
					intervalUnit, intervalCount = "month", 1
				}
			}
			if intervalCount < 1 {
				intervalCount = 1
			}

			b.WriteString(`<table class="sched"><thead><tr><th>#</th><th>Due Date</th><th>Amount</th><th>Running Total</th></tr></thead><tbody>`)
			runningTotal := 0.0
			payAmt := *snap.PaymentAmount
			for i := 0; i < numPay; i++ {
				var dueDate time.Time
				switch intervalUnit {
				case "week":
					dueDate = startDate.AddDate(0, 0, i*7*intervalCount)
				case "month":
					dueDate = startDate.AddDate(0, i*intervalCount, 0)
				default:
					dueDate = startDate.AddDate(0, i, 0)
				}
				thisAmt := payAmt
				if runningTotal+thisAmt > snap.TotalFee {
					thisAmt = snap.TotalFee - runningTotal
				}
				if thisAmt <= 0 {
					break
				}
				runningTotal += thisAmt
				b.WriteString(fmt.Sprintf(`<tr><td>%d</td><td>%s</td><td>$%.2f</td><td>$%.2f</td></tr>`,
					i+1, dueDate.Format("02 Jan 2006"), thisAmt, runningTotal))
			}
			b.WriteString(`</tbody></table>`)
			b.WriteString(fmt.Sprintf(`<p style="font-size:11px;color:#6b7280;margin:4px 0 0;">Total scheduled: <strong>$%.2f</strong> across <strong>%d</strong> payments</p>`, runningTotal, numPay))
		} else {
			b.WriteString(fmt.Sprintf(`<p>Ongoing payments of <strong>%s</strong> %s until the end of the minimum term (%s).</p>`,
				derefF(snap.PaymentAmount), deref(snap.PaymentFrequency), deref(snap.EndDate)))
		}
	}

	// ─── 5. Terms & Conditions ───
	if len(terms) > 0 {
		b.WriteString(`<h2>5. Terms & Conditions</h2>`)
		for _, t := range terms {
			b.WriteString(fmt.Sprintf(`<div class="tc-block"><h3>%s (Version %d)</h3>`, t.Title, t.Version))
			b.WriteString(fmt.Sprintf(`<div class="tc-content">%s</div>`, t.ContentHTML))
			b.WriteString(`<p class="tc-accepted">&#10003; Read, understood, and accepted by the Member</p></div>`)
		}
	}

	// ─── 6. Declaration & Signatures ───
	sectionNum := 5
	if len(terms) > 0 {
		sectionNum = 6
	}
	b.WriteString(fmt.Sprintf(`<h2>%d. Declaration & Signatures</h2>`, sectionNum))
	b.WriteString(`<div class="legal-notice">
		<strong>Declaration:</strong> By signing this agreement, the Member confirms that:<br>
		&bull; All information provided is true and correct.<br>
		&bull; The Member has read and understood all terms and conditions of this agreement.<br>
		&bull; The Member agrees to be bound by the terms set out in this agreement for the minimum term specified.<br>
		&bull; The Member authorises the payment schedule as detailed in Section 4 above.<br>
		&bull; The Member acknowledges that this agreement is a legally binding contract.
	</div>`)

	if snap.MemberSignatureData != nil && *snap.MemberSignatureData != "" {
		b.WriteString(`<div class="sig-block"><strong>Member Signature</strong><br>`)
		b.WriteString(fmt.Sprintf(`<img src="%s" alt="Member Signature">`, *snap.MemberSignatureData))
		b.WriteString(fmt.Sprintf(`<br><span style="font-size:11px;color:#374151;">Name: %s</span>`, snap.ClientName))
		if snap.MemberSignedAt != nil {
			b.WriteString(fmt.Sprintf(`<br><span style="font-size:11px;color:#6b7280;">Signed: %s</span>`, snap.MemberSignedAt.Format("2 January 2006 at 3:04:05 PM")))
		}
		b.WriteString(`</div>`)
	}
	if snap.StaffSignatureData != nil && *snap.StaffSignatureData != "" {
		b.WriteString(`<div class="sig-block"><strong>Staff / Witness Signature</strong><br>`)
		b.WriteString(fmt.Sprintf(`<img src="%s" alt="Staff Signature">`, *snap.StaffSignatureData))
		if snap.StaffSignedAt != nil {
			b.WriteString(fmt.Sprintf(`<br><span style="font-size:11px;color:#6b7280;">Signed: %s</span>`, snap.StaffSignedAt.Format("2 January 2006 at 3:04:05 PM")))
		}
		b.WriteString(`</div>`)
	}

	// ─── Footer / Verification ───
	b.WriteString(fmt.Sprintf(`<div class="footer">
		<p><strong>Agreement Reference:</strong> %s</p>
		<p><strong>Generated:</strong> %s</p>
		<p><strong>IP Address:</strong> %s</p>
		<p><strong>User Agent:</strong> %s</p>
		<p style="margin-top:8px;">This document is an electronic record of the membership agreement executed via the ClientRoll platform.
		It constitutes a legally binding contract between the parties named above. The Member's electronic signature
		and acceptance of terms were captured digitally at the date and time indicated. A cryptographic hash of each
		accepted Terms & Conditions document has been stored to verify document integrity.</p>
	</div>`, snap.MembershipID, time.Now().Format("2 January 2006 at 3:04:05 PM AEST"), deref(snap.IPAddress), deref(snap.UserAgent)))

	b.WriteString(`</body></html>`)
	return b.String()
}
