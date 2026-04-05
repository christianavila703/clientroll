package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"log"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"clientroll/internal/config"
	"clientroll/internal/database"
	"clientroll/internal/email"
	"clientroll/internal/events"
	"clientroll/internal/middleware"
	"clientroll/internal/models"
	"clientroll/internal/payment"
	pdfpkg "clientroll/internal/pdf"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v82"
	checkoutsession "github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/invoice"
	"github.com/stripe/stripe-go/v82/paymentmethod"
	"github.com/stripe/stripe-go/v82/price"
	"github.com/stripe/stripe-go/v82/product"
	"github.com/stripe/stripe-go/v82/subscription"
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
	platformCfgRepo   *repository.PlatformBillingConfigRepo
	bankDetailsRepo   *repository.BankDetailsRepo
	agreementRepo     *repository.AgreementSnapshotRepo
	inviteRepo        *repository.MembershipSetupInviteRepo
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
	platformCfgRepo *repository.PlatformBillingConfigRepo,
	bdr *repository.BankDetailsRepo,
	agr *repository.AgreementSnapshotRepo,
	inviteRepo *repository.MembershipSetupInviteRepo,
	etr *repository.TenantEmailTemplateRepo,
	enc *database.Encryptor,
	es email.Provider,
	appCfg *config.Config,
) *SignupHandler {
	return &SignupHandler{
		tenantRepo: tr, mtRepo: mt, bonusRepo: br, termsRepo: terms,
		clientRepo: cr, memberRepo: mr, settingsRepo: sr,
		credRepo: credR, platformCfgRepo: platformCfgRepo, bankDetailsRepo: bdr, agreementRepo: agr, inviteRepo: inviteRepo, emailTemplateRepo: etr,
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
	// More specific paths before /{membershipSlug} (chi resolves correctly either way; explicit order for readers).
	r.Get("/signup/{tenantSlug}/invite/{inviteToken}", h.GetInviteDetail)
	r.Get("/signup/{tenantSlug}/checkout-session", h.VerifyCheckoutSessionReturn)
	r.Get("/signup/{tenantSlug}/{membershipSlug}/term/{termOptionID}/bonuses", h.GetTermBonuses)
	r.Get("/signup/{tenantSlug}/{membershipSlug}", h.GetMembershipDetail)
	r.Post("/signup/{tenantSlug}/submit", h.SubmitSignup)
}

// RegisterTenantRoutes registers authenticated tenant routes related to signup operations.
func (h *SignupHandler) RegisterTenantRoutes(r chi.Router) {
	r.Get("/signup-links", h.GetSignupLinks)
	r.Post("/signup-invites", h.CreateMembershipInvite)
	r.Get("/signup-invites/clients/{clientID}/active", h.GetActiveMembershipInvite)
	r.Post("/signup-invites/{inviteID}/revoke", h.RevokeMembershipInvite)
}

// GetTenantMemberships returns all active memberships for a tenant (public browsing).
func (h *SignupHandler) GetTenantMemberships(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "tenantSlug")
	if err := h.verifySignupAccess(r, slug, "", true); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	tenant, err := h.tenantRepo.FindBySlug(r.Context(), slug)
	if err != nil || tenant == nil || !tenant.IsActive {
		writeError(w, http.StatusNotFound, "Organisation not found")
		return
	}
	_ = EnsureSignupTenantDBContext(r.Context(), h.clientRepo.DB(), tenant.ID)

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
			"ask_emergency_contact":       tenant.AskEmergencyContact,
			"ask_company_information":     tenant.AskCompanyInformation,
			"google_places_api_key":       h.googlePlacesAPIKey(r.Context()),
		},
		"memberships":       active,
		"payment_providers": h.availablePaymentProviders(r.Context(), tenant.ID),
	})
}

// GetMembershipDetail returns full details for a membership type (public).
func (h *SignupHandler) GetMembershipDetail(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")
	membershipSlug := chi.URLParam(r, "membershipSlug")
	if err := h.verifySignupAccess(r, tenantSlug, membershipSlug, true); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	tenant, err := h.tenantRepo.FindBySlug(r.Context(), tenantSlug)
	if err != nil || tenant == nil || !tenant.IsActive {
		writeError(w, http.StatusNotFound, "Organisation not found")
		return
	}
	_ = EnsureSignupTenantDBContext(r.Context(), h.clientRepo.DB(), tenant.ID)

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
	terms, termsErr := h.termsRepo.GetForMembershipType(r.Context(), mt.ID, tenant.ID)
	if termsErr != nil {
		log.Printf("signup GetMembershipDetail terms: tenant=%s mt=%s err=%v", tenant.ID, mt.ID, termsErr)
	}

	// Use a struct (not map) so JSON field order is stable: put `terms` before large `membership`.
	// map[string]interface{} encodes keys sorted alphabetically, which placed `terms` last and could
	// lose it under response size limits when combined with invite payloads elsewhere.
	writeJSON(w, http.StatusOK, struct {
		Tenant           map[string]interface{}      `json:"tenant"`
		Terms            []models.TermsAndConditions `json:"terms"`
		PaymentProviders []string                    `json:"payment_providers"`
		Membership       *models.MembershipType      `json:"membership"`
	}{
		Tenant: map[string]interface{}{
			"name":                        tenant.Name,
			"slug":                        tenant.Slug,
			"logo_url":                    tenant.LogoURL,
			"primary_colour":              tenant.PrimaryColour,
			"secondary_colour":            tenant.SecondaryColour,
			"signup_price_display_mode":   tenant.SignupPriceDisplayMode,
			"enabled_payment_frequencies": tenant.EnabledPaymentFrequencies,
			"ask_emergency_contact":       tenant.AskEmergencyContact,
			"ask_company_information":     tenant.AskCompanyInformation,
			"google_places_api_key":       h.googlePlacesAPIKey(r.Context()),
		},
		Terms:            terms,
		PaymentProviders: h.availablePaymentProviders(r.Context(), tenant.ID),
		Membership:       mt,
	})
}

// GetTermBonuses returns bonuses for a specific term option (for dynamic display).
func (h *SignupHandler) GetTermBonuses(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")
	membershipSlug := chi.URLParam(r, "membershipSlug")
	if err := h.verifySignupAccess(r, tenantSlug, membershipSlug, true); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
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

// VerifyCheckoutSessionReturn loads a Stripe Checkout Session and confirms it completed for this tenant.
// Prevents showing a false "payment complete" screen from ?stripe=success alone (bookmarkable / stale URLs).
func (h *SignupHandler) VerifyCheckoutSessionReturn(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "tenantSlug")
	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}
	if !strings.HasPrefix(sessionID, "cs_") {
		writeError(w, http.StatusBadRequest, "invalid session_id")
		return
	}
	if err := h.verifySignupAccess(r, slug, "", true); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	tenant, err := h.tenantRepo.FindBySlug(r.Context(), slug)
	if err != nil || tenant == nil || !tenant.IsActive {
		writeError(w, http.StatusNotFound, "Organisation not found")
		return
	}
	_ = EnsureSignupTenantDBContext(r.Context(), h.clientRepo.DB(), tenant.ID)
	if h.credRepo == nil || h.encryptor == nil {
		writeError(w, http.StatusServiceUnavailable, "Payment verification unavailable")
		return
	}
	creds, err := h.credRepo.GetByProvider(r.Context(), tenant.ID, "stripe")
	if err != nil || creds == nil || creds.APIKeyEncrypted == "" {
		writeError(w, http.StatusServiceUnavailable, "Stripe not configured")
		return
	}
	secretKey, err := h.encryptor.Decrypt(creds.APIKeyEncrypted)
	if err != nil || secretKey == "" {
		writeError(w, http.StatusServiceUnavailable, "Stripe credentials invalid")
		return
	}
	stripe.Key = secretKey
	params := &stripe.CheckoutSessionParams{}
	params.AddExpand("subscription")
	sess, err := checkoutsession.Get(sessionID, params)
	if err != nil || sess == nil {
		if err != nil {
			log.Printf("signup checkout verify tenant=%s session=%s err=%v", slug, sessionID, err)
		}
		writeError(w, http.StatusBadRequest, "Could not load checkout session")
		return
	}
	meta := sess.Metadata
	if strings.TrimSpace(meta["tenant_id"]) != tenant.ID.String() {
		writeError(w, http.StatusForbidden, "This payment does not match this signup page")
		return
	}
	if sess.Status != stripe.CheckoutSessionStatusComplete {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"verified":        false,
			"checkout_status": string(sess.Status),
			"payment_status":  string(sess.PaymentStatus),
			"checkout_mode":   string(sess.Mode),
			"reason":          "checkout_not_complete",
		})
		return
	}
	out := map[string]interface{}{
		"verified":        true,
		"checkout_mode":   string(sess.Mode),
		"payment_status":  string(sess.PaymentStatus),
		"checkout_status": string(sess.Status),
	}
	if sess.Subscription != nil && strings.TrimSpace(sess.Subscription.ID) != "" {
		out["stripe_subscription_id"] = strings.TrimSpace(sess.Subscription.ID)
	}
	writeJSON(w, http.StatusOK, out)
}

// SubmitSignup processes a completed signup form.
func (h *SignupHandler) SubmitSignup(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")
	inviteTokenFromQuery := strings.TrimSpace(r.URL.Query().Get("invite"))
	if inviteTokenFromQuery == "" {
		if err := h.verifySignupAccess(r, tenantSlug, "", true); err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
	}

	tenant, err := h.tenantRepo.FindBySlug(r.Context(), tenantSlug)
	if err != nil || tenant == nil || !tenant.IsActive {
		writeError(w, http.StatusNotFound, "Organisation not found")
		return
	}
	// RLS: pool connections need per-request tenant so reads (e.g. membership_types) succeed for agreement snapshots.
	if err := EnsureSignupTenantDBContext(r.Context(), h.clientRepo.DB(), tenant.ID); err != nil {
		log.Printf("signup EnsureSignupTenantDBContext failed tenant=%s err=%v", tenant.ID, err)
		writeError(w, http.StatusInternalServerError, "Server error")
		return
	}

	var input SignupSubmission
	if err := decodeJSON(r, &input); err != nil {
		log.Printf("Signup decode error: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid submission data")
		return
	}
	inviteTokenRaw := strings.TrimSpace(input.InviteToken)
	if inviteTokenRaw == "" {
		inviteTokenRaw = strings.TrimSpace(r.URL.Query().Get("invite"))
	}
	var invite *repository.MembershipSetupInvite
	inviteStrategy := ""
	if inviteTokenRaw != "" {
		if h.inviteRepo == nil {
			writeError(w, http.StatusInternalServerError, "Invite repository unavailable")
			return
		}
		tokenUUID, tokenErr := uuid.Parse(inviteTokenRaw)
		if tokenErr != nil {
			writeError(w, http.StatusBadRequest, "Invalid invite token")
			return
		}
		invite, err = h.inviteRepo.FindByToken(r.Context(), tokenUUID)
		if err != nil || invite == nil || invite.TenantID != tenant.ID {
			writeError(w, http.StatusNotFound, "Invite not found")
			return
		}
		if invite.CompletedAt != nil {
			writeError(w, http.StatusGone, "Invite already completed")
			return
		}
		if time.Now().UTC().After(invite.ExpiresAt) {
			writeError(w, http.StatusGone, "Invite expired")
			return
		}
		inviteStrategy = invite.PaymentMethodStrategy
		input.MembershipTypeID = invite.MembershipTypeID.String()
		input.TermOptionID = invite.TermOptionID.String()
		input.StartDateRaw = invite.StartDate.Format("2006-01-02")
		input.PaymentFrequency = invite.PaymentFrequency
		if invite.PaymentMethodStrategy == "use_existing" {
			if invite.PaymentMethod != nil {
				input.PaymentMethod = *invite.PaymentMethod
			}
			if invite.PaymentMethodID != nil {
				input.PaymentMethodID = *invite.PaymentMethodID
			}
			if invite.GoCardlessMandateID != nil {
				input.GoCardlessMandateID = *invite.GoCardlessMandateID
			}
		}
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
	if err := validateMinimumFutureStartDate(input.StartDate, tenant.Timezone, minFutureMembershipStartDays); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if input.Client.Name == "" || input.TermOptionID == "" || input.MembershipTypeID == "" {
		writeError(w, http.StatusBadRequest, "Client name, membership type and term option are required")
		return
	}
	input.Client.Name = strings.TrimSpace(input.Client.Name)
	trimStringPtr := func(v **string) {
		if v == nil || *v == nil {
			return
		}
		t := strings.TrimSpace(**v)
		if t == "" {
			*v = nil
			return
		}
		**v = t
	}
	trimStringPtr(&input.Client.UnitNumber)
	trimStringPtr(&input.Client.Address)
	trimStringPtr(&input.Client.Suburb)
	trimStringPtr(&input.Client.State)
	trimStringPtr(&input.Client.Postcode)
	trimStringPtr(&input.Client.Country)
	trimStringPtr(&input.Client.Email)
	trimStringPtr(&input.Client.Phone)
	trimStringPtr(&input.Client.Gender)
	trimStringPtr(&input.Client.EmergencyContactName)
	trimStringPtr(&input.Client.EmergencyContactPhone)
	trimStringPtr(&input.Client.TaxNumber)
	trimStringPtr(&input.Client.CompanyName)
	trimStringPtr(&input.Client.CompanyNumber)
	trimStringPtr(&input.Client.CompanyPhone)
	trimStringPtr(&input.Client.CompanyWebsite)
	trimStringPtr(&input.Client.CompanyUnitNumber)
	trimStringPtr(&input.Client.CompanyAddress)
	trimStringPtr(&input.Client.CompanyCity)
	trimStringPtr(&input.Client.CompanyStateRegion)
	trimStringPtr(&input.Client.CompanyPostcode)
	trimStringPtr(&input.Client.CompanyCountry)
	input.Client.Country = normalizeClientCountryCode(input.Client.Country)
	{
		var missing []string
		if input.Client.Name == "" {
			missing = append(missing, "name")
		}
		if input.Client.Address == nil {
			missing = append(missing, "address")
		}
		if input.Client.Suburb == nil {
			missing = append(missing, "suburb")
		}
		if input.Client.State == nil {
			missing = append(missing, "state")
		}
		if input.Client.Postcode == nil {
			missing = append(missing, "postcode")
		}
		if input.Client.Country == nil {
			missing = append(missing, "country")
		}
		if input.Client.Email == nil {
			missing = append(missing, "email")
		}
		if input.Client.Phone == nil {
			missing = append(missing, "phone")
		}
		if input.Client.BirthDate == nil {
			missing = append(missing, "birth_date")
		}
		if input.Client.Gender == nil {
			missing = append(missing, "gender")
		}
		if tenant.AskEmergencyContact {
			if input.Client.EmergencyContactName == nil {
				missing = append(missing, "emergency_contact_name")
			}
			if input.Client.EmergencyContactPhone == nil {
				missing = append(missing, "emergency_contact_phone")
			}
		}
		if tenant.AskCompanyInformation {
			if input.Client.TaxNumber == nil {
				missing = append(missing, "tax_number")
			}
			if input.Client.CompanyName == nil {
				missing = append(missing, "company_name")
			}
			if input.Client.CompanyNumber == nil {
				missing = append(missing, "company_number")
			}
			if input.Client.CompanyPhone == nil {
				missing = append(missing, "company_phone")
			}
			if input.Client.CompanyAddress == nil {
				missing = append(missing, "company_address")
			}
			if input.Client.CompanyCity == nil {
				missing = append(missing, "company_city")
			}
			if input.Client.CompanyStateRegion == nil {
				missing = append(missing, "company_state_region")
			}
			if input.Client.CompanyPostcode == nil {
				missing = append(missing, "company_postcode")
			}
			if input.Client.CompanyCountry == nil {
				missing = append(missing, "company_country")
			}
		}
		if len(missing) > 0 {
			log.Printf("Signup validation: missing client fields: %v", missing)
			writeError(w, http.StatusBadRequest, "Please complete all required client details: "+strings.Join(missing, ", "))
			return
		}
	}
	if input.PaymentMethod == "" {
		input.PaymentMethod = "direct_debit"
	}
	input.PaymentMethod = strings.ToLower(strings.TrimSpace(input.PaymentMethod))
	input.PaymentMethodID = strings.TrimSpace(input.PaymentMethodID)
	input.GoCardlessMandateID = strings.TrimSpace(input.GoCardlessMandateID)
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
	// Look up existing client by email, or create new (invite flow always updates the targeted client).
	var client *models.Client
	if invite != nil {
		client, err = h.clientRepo.FindByID(r.Context(), invite.ClientID, tenant.ID)
		if err != nil || client == nil {
			writeError(w, http.StatusNotFound, "Client not found for invite")
			return
		}
		client, err = h.clientRepo.Update(r.Context(), client.ID, tenant.ID, &input.Client)
		if err != nil {
			log.Printf("Error updating invited client during signup: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to update client record")
			return
		}
	} else if input.Client.Email != nil && *input.Client.Email != "" {
		client, err = h.clientRepo.FindByEmail(r.Context(), tenant.ID, *input.Client.Email)
		if err != nil {
			log.Printf("Error looking up client by email: %v", err)
		}
	}
	if invite == nil && client != nil {
		// Update existing client's details with latest info from signup
		client, err = h.clientRepo.Update(r.Context(), client.ID, tenant.ID, &input.Client)
		if err != nil {
			log.Printf("Error updating existing client during signup: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to update client record")
			return
		}
	} else if invite == nil {
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
	staffID := parseOptionalUUID(input.StaffUserID)
	mtForCreate, _ := h.mtRepo.FindByID(r.Context(), mtID, tenant.ID)
	termForCreate, _ := h.mtRepo.GetTermOption(r.Context(), toID, tenant.ID)
	if mtForCreate == nil {
		writeError(w, http.StatusNotFound, "Membership type not found")
		return
	}
	if termForCreate == nil || termForCreate.MembershipTypeID != mtID {
		writeError(w, http.StatusBadRequest, "term_option_id does not belong to membership_type_id")
		return
	}
	canonical, err := deriveCanonicalMembershipPricing(termForCreate, input.PaymentFrequency)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if input.PaymentAmount > 0 && math.Abs(input.PaymentAmount-canonical.Amount) > 0.01 {
		writeError(w, http.StatusBadRequest, "payment_amount does not match selected term option")
		return
	}
	if input.NumPayments != nil && canonical.NumPayments != nil && *input.NumPayments != *canonical.NumPayments {
		writeError(w, http.StatusBadRequest, "num_payments does not match selected term option")
		return
	}
	if input.PaymentIntervalUnit != nil && canonical.IntervalUnit != nil && *input.PaymentIntervalUnit != *canonical.IntervalUnit {
		writeError(w, http.StatusBadRequest, "payment_interval_unit does not match selected term option")
		return
	}
	if input.PaymentIntervalCount > 0 && canonical.IntervalCount > 0 && input.PaymentIntervalCount != canonical.IntervalCount {
		writeError(w, http.StatusBadRequest, "payment_interval_count does not match selected term option")
		return
	}
	input.PaymentFrequency = canonical.Frequency
	input.PaymentAmount = canonical.Amount
	input.PaymentIntervalUnit = canonical.IntervalUnit
	input.PaymentIntervalCount = canonical.IntervalCount
	input.NumPayments = canonical.NumPayments
	applySalesTax := true
	applySalesTax = mtForCreate.ApplySalesTax
	membershipCurrency := "AUD"
	if strings.TrimSpace(mtForCreate.CurrencyCode) != "" {
		membershipCurrency = strings.ToUpper(strings.TrimSpace(mtForCreate.CurrencyCode))
	}
	dup, dupErr := h.memberRepo.FindUnpaidDuplicateCandidate(
		r.Context(),
		tenant.ID, client.ID, mtID, toID,
		input.StartDate, input.PaymentMethod, input.PaymentFrequency, input.PaymentAmount,
		input.PaymentIntervalUnit, input.PaymentIntervalCount,
	)
	if dupErr != nil {
		log.Printf("signup duplicate check failed tenant=%s client=%s err=%v", tenant.ID, client.ID, dupErr)
		writeError(w, http.StatusInternalServerError, "Failed to validate signup request")
		return
	}
	if dup != nil {
		writeJSON(w, http.StatusConflict, map[string]interface{}{
			"error":               "A matching membership was created in the last 5 minutes and is still awaiting first payment. Please wait 5 minutes before trying again.",
			"existing_membership": dup,
		})
		return
	}

	// Create membership
	membership, err := h.memberRepo.Create(r.Context(), tenant.ID, client.ID, mtID, toID,
		input.StartDate, input.PaymentMethod, input.PaymentFrequency, membershipCurrency, input.PaymentAmount,
		input.PaymentIntervalUnit, input.PaymentIntervalCount, input.NumPayments,
		termForCreate.ContinueAfterTerm,
		input.MemberSignature, staffID, input.StaffSignature)
	if err != nil {
		log.Printf("Error creating membership during signup: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create membership")
		return
	}
	markInviteCompleted := func() {
		if invite != nil && h.inviteRepo != nil {
			if err := h.inviteRepo.MarkCompleted(r.Context(), invite.ID); err != nil {
				log.Printf("signup invite mark complete failed invite=%s err=%v", invite.ID, err)
			}
		}
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

	// --- Stripe card path ---
	if input.PaymentMethod == "card" {
		pmID := strings.TrimSpace(input.PaymentMethodID)
		// Invite + reuse saved card + recurring: create subscription off-session (no Checkout).
		// Checkout often fails for saved PMs missing billing fields Stripe requires for the hosted page.
		if invite != nil && inviteStrategy == "use_existing" && pmID != "" && input.PaymentFrequency != "upfront" {
			if err := h.signupStripeCardSubscriptionWithSavedPM(r.Context(), tenant, client, membership, membershipCurrency, input.PaymentFrequency, input.PaymentIntervalUnit, input.PaymentIntervalCount, input.PaymentAmount, applySalesTax, termForCreate.ContinueAfterTerm, membership.MinTermEndDate, membership.StartDate, pmID); err != nil {
				log.Printf("signup saved-card stripe subscription tenant=%s membership=%s err=%v", tenant.ID, membership.ID, err)
				writeError(w, http.StatusBadGateway, stripeProviderErrorMessage(err, "Card payment setup failed"))
				return
			}
			h.createAgreementSnapshot(r, tenant, client, membership, input, mtID, toID)
			markInviteCompleted()
			mt := mtForCreate
			thankyou := buildThankyouResponse(tenant, mt)
			writeJSON(w, http.StatusCreated, map[string]interface{}{
				"membership": membership,
				"client":     client,
				"thankyou":   thankyou,
			})
			return
		}
		var signupInviteID *uuid.UUID
		if invite != nil {
			signupInviteID = &invite.ID
		}
		checkoutURL, err := h.createStripeCheckoutSession(r, tenant, client, membership, membershipCurrency, input.PaymentFrequency, input.PaymentIntervalUnit, input.PaymentIntervalCount, input.PaymentAmount, applySalesTax, termForCreate.ContinueAfterTerm, membership.MinTermEndDate, membership.StartDate, input.PaymentMethodID, signupInviteID)
		if err != nil {
			log.Printf("Error creating Stripe checkout session during signup: %v", err)
			writeError(w, http.StatusBadGateway, stripeProviderErrorMessage(err, "Card payment setup failed"))
			return
		}
		// Capture the signed legal agreement at submission time (before redirect to checkout).
		h.createAgreementSnapshot(r, tenant, client, membership, input, mtID, toID)
		// Invite is marked completed from Stripe webhooks once checkout payment succeeds (see signup_invite_id metadata).
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

	// --- Direct debit path ---
	gcMandateID := ""
	if input.PaymentMethod == "direct_debit" {
		if invite != nil && inviteStrategy == "use_existing" && input.GoCardlessMandateID != "" {
			prov, provErr := h.getGoCardlessProvider(r.Context(), tenant.ID)
			if provErr != nil || prov == nil {
				writeError(w, http.StatusBadGateway, "Direct debit provider unavailable")
				return
			}
			_ = h.memberRepo.SetGoCardlessMandateID(r.Context(), membership.ID, tenant.ID, input.GoCardlessMandateID)
			subName := "Membership"
			if mtForCreate != nil {
				subName = mtForCreate.Title
			}
			numPay := membership.NumPayments
			continueAfterTerm := false
			if to, _ := h.mtRepo.GetTermOption(r.Context(), toID, tenant.ID); to != nil {
				if numPay == nil && to.NumPayments != nil {
					numPay = to.NumPayments
				}
				continueAfterTerm = to.ContinueAfterTerm
			}
			if continueAfterTerm {
				numPay = nil
			}
			if _, gcErr := createGoCardlessSubscription(
				r.Context(),
				prov,
				h.memberRepo,
				input.GoCardlessMandateID,
				membership,
				tenant.ID,
				input.PaymentFrequency,
				input.PaymentAmount,
				membership.PaymentIntervalUnit,
				membership.PaymentIntervalCount,
				subName,
				membership.StartDate,
				membership.MinTermEndDate,
				numPay,
				continueAfterTerm,
			); gcErr != nil {
				writeError(w, http.StatusBadGateway, "Direct debit subscription setup failed")
				return
			}
			h.createAgreementSnapshot(r, tenant, client, membership, input, mtID, toID)
			markInviteCompleted()
			mt := mtForCreate
			thankyou := buildThankyouResponse(tenant, mt)
			writeJSON(w, http.StatusCreated, map[string]interface{}{
				"membership": membership,
				"client":     client,
				"thankyou":   thankyou,
			})
			return
		} else {
			var signupInviteID *uuid.UUID
			if invite != nil {
				signupInviteID = &invite.ID
			}
			checkoutURL, err := h.createStripeBECSCheckoutSession(r, tenant, client, membership, membershipCurrency, input.PaymentFrequency, input.PaymentIntervalUnit, input.PaymentIntervalCount, input.PaymentAmount, applySalesTax, termForCreate.ContinueAfterTerm, membership.MinTermEndDate, membership.StartDate, input.PaymentMethodID, signupInviteID)
			if err != nil {
				log.Printf("Error creating Stripe direct debit checkout session during signup: %v", err)
				writeError(w, http.StatusBadGateway, stripeProviderErrorMessage(err, "Direct debit payment setup failed"))
				return
			}
			h.createAgreementSnapshot(r, tenant, client, membership, input, mtID, toID)
			// Invite completed via Stripe webhook when DD/collection succeeds (signup_invite_id on session/subscription).
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
	}

	// Determine thank you page
	mt := mtForCreate
	thankyou := buildThankyouResponse(tenant, mt)

	// Create agreement snapshot for legal custody
	snapshotID := h.createAgreementSnapshot(r, tenant, client, membership, input, mtID, toID)
	markInviteCompleted()

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

// GetSignupLinks returns signed public signup links for the authenticated tenant.
func (h *SignupHandler) GetSignupLinks(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil || tenantID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant context")
		return
	}
	tenant, err := h.tenantRepo.FindByID(r.Context(), tenantID)
	if err != nil || tenant == nil {
		writeError(w, http.StatusNotFound, "Tenant not found")
		return
	}
	types, err := h.mtRepo.List(r.Context(), tenant.ID)
	if err != nil {
		log.Printf("signup links: membership list failed tenant=%s err=%v", tenant.ID, err)
		writeError(w, http.StatusInternalServerError, "Failed to load signup links")
		return
	}
	expiresAt := time.Now().UTC().Add(h.signupLinkTTL())
	exp := strconv.FormatInt(expiresAt.Unix(), 10)
	browseSig := h.signSignupPayload(tenant.Slug, "", exp)
	browsePath := fmt.Sprintf("/join/%s?exp=%s&sig=%s", tenant.Slug, exp, browseSig)

	type membershipLink struct {
		MembershipID   uuid.UUID `json:"membership_id"`
		MembershipSlug string    `json:"membership_slug"`
		Path           string    `json:"path"`
	}
	links := make([]membershipLink, 0, len(types))
	for _, mt := range types {
		if !mt.IsActive {
			continue
		}
		mslug := strings.TrimSpace(mt.Slug)
		if mslug == "" {
			continue
		}
		sig := h.signSignupPayload(tenant.Slug, mslug, exp)
		path := fmt.Sprintf("/%s/%s?m=%s&exp=%s&sig=%s", tenant.Slug, mslug, mslug, exp, sig)
		links = append(links, membershipLink{
			MembershipID:   mt.ID,
			MembershipSlug: mslug,
			Path:           path,
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tenant_slug": tenant.Slug,
		"expires_at":  expiresAt.Format(time.RFC3339),
		"browse_path": browsePath,
		"links":       links,
	})
}

func (h *SignupHandler) CreateMembershipInvite(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil || tenantID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant context")
		return
	}
	userID, _ := uuid.Parse(middleware.GetUserID(r.Context()))
	if h.inviteRepo == nil {
		writeError(w, http.StatusInternalServerError, "Invite repository unavailable")
		return
	}

	var input struct {
		ClientID              string  `json:"client_id"`
		MembershipTypeID      string  `json:"membership_type_id"`
		TermOptionID          string  `json:"term_option_id"`
		StartDate             string  `json:"start_date"`
		PaymentFrequency      string  `json:"payment_frequency"`
		PaymentMethod         *string `json:"payment_method"`
		PaymentMethodStrategy string  `json:"payment_method_strategy"`
		PaymentMethodID       *string `json:"payment_method_id"`
		GoCardlessMandateID   *string `json:"gocardless_mandate_id"`
		ExpiryDays            int     `json:"expiry_days"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request")
		return
	}
	input.ClientID = strings.TrimSpace(input.ClientID)
	input.MembershipTypeID = strings.TrimSpace(input.MembershipTypeID)
	input.TermOptionID = strings.TrimSpace(input.TermOptionID)
	input.StartDate = strings.TrimSpace(input.StartDate)
	input.PaymentFrequency = strings.TrimSpace(strings.ToLower(input.PaymentFrequency))
	input.PaymentMethodStrategy = strings.TrimSpace(strings.ToLower(input.PaymentMethodStrategy))
	if input.PaymentMethod != nil {
		pm := strings.TrimSpace(strings.ToLower(*input.PaymentMethod))
		input.PaymentMethod = &pm
	}
	if input.PaymentMethodID != nil {
		pmID := strings.TrimSpace(*input.PaymentMethodID)
		if pmID == "" {
			input.PaymentMethodID = nil
		} else {
			input.PaymentMethodID = &pmID
		}
	}
	if input.GoCardlessMandateID != nil {
		mid := strings.TrimSpace(*input.GoCardlessMandateID)
		if mid == "" {
			input.GoCardlessMandateID = nil
		} else {
			input.GoCardlessMandateID = &mid
		}
	}

	if input.ClientID == "" || input.MembershipTypeID == "" || input.TermOptionID == "" || input.StartDate == "" || input.PaymentFrequency == "" {
		writeError(w, http.StatusBadRequest, "Missing required fields")
		return
	}
	if input.PaymentMethodStrategy != "ask_client" && input.PaymentMethodStrategy != "use_existing" {
		writeError(w, http.StatusBadRequest, "payment_method_strategy must be ask_client or use_existing")
		return
	}
	if input.PaymentMethodStrategy == "use_existing" {
		if input.PaymentMethod == nil || *input.PaymentMethod == "" {
			writeError(w, http.StatusBadRequest, "payment_method is required when using existing method")
			return
		}
		if *input.PaymentMethod == "card" && input.PaymentMethodID == nil {
			writeError(w, http.StatusBadRequest, "payment_method_id is required for existing Stripe card method")
			return
		}
		if *input.PaymentMethod == "direct_debit" && input.GoCardlessMandateID == nil && input.PaymentMethodID == nil {
			writeError(w, http.StatusBadRequest, "Either payment_method_id (Stripe) or gocardless_mandate_id is required for existing direct debit method")
			return
		}
	}

	startDate, err := time.Parse("2006-01-02", input.StartDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid start_date")
		return
	}
	tenantModel, tenantErr := h.tenantRepo.FindByID(r.Context(), tenantID)
	if tenantErr != nil || tenantModel == nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant context")
		return
	}
	if err := validateMinimumFutureStartDate(startDate, tenantModel.Timezone, minFutureMembershipStartDays); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	clientID, err := uuid.Parse(input.ClientID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid client_id")
		return
	}
	membershipTypeID, err := uuid.Parse(input.MembershipTypeID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid membership_type_id")
		return
	}
	termOptionID, err := uuid.Parse(input.TermOptionID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid term_option_id")
		return
	}
	client, err := h.clientRepo.FindByID(r.Context(), clientID, tenantID)
	if err != nil || client == nil {
		writeError(w, http.StatusNotFound, "Client not found")
		return
	}
	mt, err := h.mtRepo.FindByID(r.Context(), membershipTypeID, tenantID)
	if err != nil || mt == nil {
		writeError(w, http.StatusNotFound, "Membership type not found")
		return
	}
	term, err := h.mtRepo.GetTermOption(r.Context(), termOptionID, tenantID)
	if err != nil || term == nil || term.MembershipTypeID != mt.ID {
		writeError(w, http.StatusBadRequest, "Term option not found")
		return
	}

	expiryDays := input.ExpiryDays
	if expiryDays <= 0 {
		expiryDays = 7
	}
	if expiryDays > 30 {
		expiryDays = 30
	}
	expiresAt := time.Now().UTC().Add(time.Duration(expiryDays) * 24 * time.Hour)
	var createdBy *uuid.UUID
	if userID != uuid.Nil {
		createdBy = &userID
	}
	invite, err := h.inviteRepo.Create(
		r.Context(),
		tenantID,
		clientID,
		membershipTypeID,
		termOptionID,
		startDate,
		input.PaymentFrequency,
		input.PaymentMethod,
		input.PaymentMethodStrategy,
		input.PaymentMethodID,
		input.GoCardlessMandateID,
		expiresAt,
		createdBy,
	)
	if err != nil {
		log.Printf("signup invite create failed tenant=%s err=%v", tenantID, err)
		writeError(w, http.StatusInternalServerError, "Failed to create invite")
		return
	}

	tenant := tenantModel
	tenantSlug := ""
	tenantName := "Your organisation"
	if tenant != nil {
		tenantSlug = tenant.Slug
		tenantName = tenant.Name
	}
	finalisePath := fmt.Sprintf("/join/%s/invite/%s", tenantSlug, invite.Token.String())
	baseURL := strings.TrimSuffix(h.appBaseURL(r), "/")
	finaliseURL := baseURL + finalisePath
	emailSent := false
	warning := ""

	clientEmail := ""
	if client.Email != nil {
		clientEmail = strings.TrimSpace(*client.Email)
	}
	if clientEmail == "" {
		warning = "Client has no email on file. Invite created as copy-link only."
	} else if h.emailSvc != nil {
		html := fmt.Sprintf(`
			<h2 style="font-size:18px;font-weight:600;color:#111827;margin:0 0 12px;">Complete your membership setup</h2>
			<p style="font-size:14px;color:#6b7280;line-height:1.6;">Hi %s,</p>
			<p style="font-size:14px;color:#6b7280;line-height:1.6;">You've been invited to complete your membership with %s. Use the secure link below to review details, choose payment (if requested), and sign your agreement.</p>
			<p style="margin:24px 0;"><a href="%s" style="display:inline-block;padding:12px 24px;background-color:#4f46e5;color:#ffffff;text-decoration:none;border-radius:8px;font-weight:600;font-size:14px;">Complete membership setup</a></p>
			<p style="font-size:12px;color:#9ca3af;">This link expires on %s.</p>
		`, client.Name, tenantName, finaliseURL, invite.ExpiresAt.In(time.UTC).Format("2 Jan 2006 15:04 MST"))
		_, sendErr := h.emailSvc.Send(context.Background(), email.SendInput{
			To:      clientEmail,
			Subject: fmt.Sprintf("Complete your membership setup with %s", tenantName),
			HTML:    html,
			Text:    fmt.Sprintf("Complete your membership setup: %s", finaliseURL),
		})
		if sendErr != nil {
			warning = "Invite created but email failed to send. Please copy and share the link."
			log.Printf("signup invite email failed tenant=%s client=%s err=%v", tenantID, client.ID, sendErr)
		} else {
			emailSent = true
		}
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"invite_id":        invite.ID,
		"invite_token":     invite.Token.String(),
		"invite_path":      finalisePath,
		"invite_url":       finaliseURL,
		"expires_at":       invite.ExpiresAt.Format(time.RFC3339),
		"email_sent":       emailSent,
		"warning":          warning,
		"payment_strategy": invite.PaymentMethodStrategy,
	})
}

func (h *SignupHandler) GetActiveMembershipInvite(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil || tenantID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant context")
		return
	}
	clientID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "clientID")))
	if err != nil || clientID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "Invalid client ID")
		return
	}
	if h.inviteRepo == nil {
		writeError(w, http.StatusInternalServerError, "Invite repository unavailable")
		return
	}
	client, err := h.clientRepo.FindByID(r.Context(), clientID, tenantID)
	if err != nil || client == nil {
		writeError(w, http.StatusNotFound, "Client not found")
		return
	}
	invite, err := h.inviteRepo.FindLatestActiveByClient(r.Context(), tenantID, clientID)
	if err != nil {
		log.Printf("signup invite active lookup failed tenant=%s client=%s err=%v", tenantID, clientID, err)
		writeError(w, http.StatusInternalServerError, "Failed to load signup invite")
		return
	}
	if invite == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"invite": nil})
		return
	}
	tenant, _ := h.tenantRepo.FindByID(r.Context(), tenantID)
	tenantSlug := ""
	if tenant != nil {
		tenantSlug = tenant.Slug
	}
	invitePath := fmt.Sprintf("/join/%s/invite/%s", tenantSlug, invite.Token.String())
	inviteURL := strings.TrimSuffix(h.appBaseURL(r), "/") + invitePath

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"invite": map[string]interface{}{
			"id":                      invite.ID,
			"token":                   invite.Token.String(),
			"client_id":               invite.ClientID,
			"membership_type_id":      invite.MembershipTypeID,
			"term_option_id":          invite.TermOptionID,
			"start_date":              invite.StartDate.Format("2006-01-02"),
			"payment_frequency":       invite.PaymentFrequency,
			"payment_method":          invite.PaymentMethod,
			"payment_method_strategy": invite.PaymentMethodStrategy,
			"payment_method_id":       invite.PaymentMethodID,
			"gocardless_mandate_id":   invite.GoCardlessMandateID,
			"expires_at":              invite.ExpiresAt.Format(time.RFC3339),
			"created_at":              invite.CreatedAt.Format(time.RFC3339),
			"invite_path":             invitePath,
			"invite_url":              inviteURL,
		},
	})
}

func (h *SignupHandler) RevokeMembershipInvite(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil || tenantID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant context")
		return
	}
	inviteID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "inviteID")))
	if err != nil || inviteID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "Invalid invite ID")
		return
	}
	if h.inviteRepo == nil {
		writeError(w, http.StatusInternalServerError, "Invite repository unavailable")
		return
	}
	ok, err := h.inviteRepo.Revoke(r.Context(), inviteID, tenantID)
	if err != nil {
		log.Printf("signup invite revoke failed tenant=%s invite=%s err=%v", tenantID, inviteID, err)
		writeError(w, http.StatusInternalServerError, "Failed to revoke invite")
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "Invite not found or already inactive")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"message": "Invite revoked"})
}

func (h *SignupHandler) GetInviteDetail(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")
	tokenRaw := strings.TrimSpace(chi.URLParam(r, "inviteToken"))
	if tenantSlug == "" || tokenRaw == "" {
		writeError(w, http.StatusBadRequest, "Invalid invite link")
		return
	}
	if h.inviteRepo == nil {
		writeError(w, http.StatusInternalServerError, "Invite repository unavailable")
		return
	}
	token, err := uuid.Parse(tokenRaw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid invite link")
		return
	}
	tenant, err := h.tenantRepo.FindBySlug(r.Context(), tenantSlug)
	if err != nil || tenant == nil {
		writeError(w, http.StatusNotFound, "Tenant not found")
		return
	}
	if err := EnsureSignupTenantDBContext(r.Context(), h.clientRepo.DB(), tenant.ID); err != nil {
		log.Printf("GetInviteDetail EnsureSignupTenantDBContext failed tenant=%s err=%v", tenant.ID, err)
		writeError(w, http.StatusInternalServerError, "Server error")
		return
	}
	invite, err := h.inviteRepo.FindByToken(r.Context(), token)
	if err != nil || invite == nil || invite.TenantID != tenant.ID {
		writeError(w, http.StatusNotFound, "Invite not found")
		return
	}
	if invite.CompletedAt != nil {
		writeError(w, http.StatusGone, "Invite already completed")
		return
	}
	if time.Now().UTC().After(invite.ExpiresAt) {
		writeError(w, http.StatusGone, "Invite expired")
		return
	}
	mt, err := h.mtRepo.FindByID(r.Context(), invite.MembershipTypeID, tenant.ID)
	if err != nil || mt == nil {
		writeError(w, http.StatusNotFound, "Membership not found")
		return
	}
	for i, to := range mt.TermOptions {
		bonuses, _ := h.bonusRepo.GetBonusesForTermOption(r.Context(), to.ID, tenant.ID)
		mt.TermOptions[i].Bonuses = bonuses
	}
	terms, termsErr := h.termsRepo.GetForMembershipType(r.Context(), mt.ID, tenant.ID)
	if termsErr != nil {
		log.Printf("signup GetInviteDetail terms: tenant=%s mt=%s err=%v", tenant.ID, mt.ID, termsErr)
	}
	client, _ := h.clientRepo.FindByID(r.Context(), invite.ClientID, tenant.ID)

	// Struct encoding preserves field order. `terms` must appear before large `membership` / `invite_client`
	// — map keys are sorted alphabetically and put `terms` last, which risked truncation on invite payloads.
	writeJSON(w, http.StatusOK, struct {
		Tenant           map[string]interface{}      `json:"tenant"`
		Terms            []models.TermsAndConditions `json:"terms"`
		Invite           map[string]interface{}      `json:"invite"`
		PaymentProviders []string                    `json:"payment_providers"`
		Membership       *models.MembershipType      `json:"membership"`
		InviteClient     *models.Client              `json:"invite_client"`
	}{
		Tenant: map[string]interface{}{
			"name":                          tenant.Name,
			"slug":                          tenant.Slug,
			"logo_url":                      tenant.LogoURL,
			"primary_colour":                tenant.PrimaryColour,
			"secondary_colour":              tenant.SecondaryColour,
			"default_thankyou_heading":      tenant.DefaultThankyouHeading,
			"default_thankyou_body":         tenant.DefaultThankyouBody,
			"default_thankyou_image_url":    tenant.DefaultThankyouImageURL,
			"default_thankyou_redirect_url": tenant.DefaultThankyouRedirectURL,
			"signup_price_display_mode":     tenant.SignupPriceDisplayMode,
			"enabled_payment_frequencies":   tenant.EnabledPaymentFrequencies,
			"ask_emergency_contact":         tenant.AskEmergencyContact,
			"ask_company_information":       tenant.AskCompanyInformation,
			"google_places_api_key":         h.googlePlacesAPIKey(r.Context()),
		},
		Terms: terms,
		Invite: map[string]interface{}{
			"token":                   invite.Token.String(),
			"membership_type_id":      invite.MembershipTypeID,
			"term_option_id":          invite.TermOptionID,
			"client_id":               invite.ClientID,
			"start_date":              invite.StartDate.Format("2006-01-02"),
			"payment_frequency":       invite.PaymentFrequency,
			"payment_method":          invite.PaymentMethod,
			"payment_method_strategy": invite.PaymentMethodStrategy,
			"payment_method_id":       invite.PaymentMethodID,
			"gocardless_mandate_id":   invite.GoCardlessMandateID,
			"expires_at":              invite.ExpiresAt.Format(time.RFC3339),
		},
		PaymentProviders: h.availablePaymentProviders(r.Context(), tenant.ID),
		Membership:       mt,
		InviteClient:     client,
	})
}

func (h *SignupHandler) signupLinkTTL() time.Duration {
	if h.cfg != nil && h.cfg.SignupLinkTTLMinutes > 0 {
		return time.Duration(h.cfg.SignupLinkTTLMinutes) * time.Minute
	}
	return 7 * 24 * time.Hour
}

func (h *SignupHandler) appBaseURL(r *http.Request) string {
	if h.cfg != nil && strings.TrimSpace(h.cfg.AppURL) != "" {
		return strings.TrimSpace(h.cfg.AppURL)
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin != "" {
		return origin
	}
	return "https://app.clientroll.com"
}

func (h *SignupHandler) signupSigningSecret() string {
	if h.cfg == nil {
		return ""
	}
	if strings.TrimSpace(h.cfg.SignupLinkSigningSecret) != "" {
		return strings.TrimSpace(h.cfg.SignupLinkSigningSecret)
	}
	return strings.TrimSpace(h.cfg.MasterEncryptionKey)
}

func (h *SignupHandler) signSignupPayload(tenantSlug, membershipSlug, exp string) string {
	secret := h.signupSigningSecret()
	if secret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strings.ToLower(strings.TrimSpace(tenantSlug))))
	mac.Write([]byte("|"))
	mac.Write([]byte(strings.ToLower(strings.TrimSpace(membershipSlug))))
	mac.Write([]byte("|"))
	mac.Write([]byte(strings.TrimSpace(exp)))
	return hex.EncodeToString(mac.Sum(nil))
}

func (h *SignupHandler) verifySignupAccess(r *http.Request, tenantSlug, pathMembershipSlug string, allowTenantWideFallback bool) error {
	secret := h.signupSigningSecret()
	requireSig := h.cfg != nil && h.cfg.SignupLinksRequireSignature
	if secret == "" && !requireSig {
		return nil
	}
	q := r.URL.Query()
	sig := strings.TrimSpace(q.Get("sig"))
	exp := strings.TrimSpace(q.Get("exp"))
	queryMembership := strings.TrimSpace(q.Get("m"))

	if sig == "" || exp == "" {
		if !requireSig {
			return nil
		}
		return errors.New("Signup link is invalid or expired")
	}

	expUnix, err := strconv.ParseInt(exp, 10, 64)
	if err != nil || expUnix <= 0 || time.Now().UTC().Unix() > expUnix {
		return errors.New("Signup link is invalid or expired")
	}

	candidates := []string{strings.TrimSpace(pathMembershipSlug), queryMembership}
	if allowTenantWideFallback {
		candidates = append(candidates, "")
	}
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		key := strings.ToLower(strings.TrimSpace(candidate))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		expected := h.signSignupPayload(tenantSlug, candidate, exp)
		if expected != "" && hmac.Equal([]byte(strings.ToLower(sig)), []byte(strings.ToLower(expected))) {
			return nil
		}
	}
	return errors.New("Signup link is invalid or expired")
}

// SignupSubmission is the full signup form data.
type SignupSubmission struct {
	Client               models.Client `json:"client"`
	InviteToken          string        `json:"invite_token,omitempty"`
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
	PaymentMethodID      string        `json:"payment_method_id,omitempty"`
	GoCardlessMandateID  string        `json:"gocardless_mandate_id,omitempty"`
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

// attachStripePaymentMethodToCustomer ensures pmID belongs to stripeCustomerID (attach if detached).
// Required before setting invoice_settings.default_payment_method and for Checkout with saved cards.
func attachStripePaymentMethodToCustomer(stripeCustomerID, pmID string) error {
	stripeCustomerID = strings.TrimSpace(stripeCustomerID)
	pmID = strings.TrimSpace(pmID)
	if stripeCustomerID == "" || pmID == "" {
		return nil
	}
	pm, err := paymentmethod.Get(pmID, nil)
	if err != nil {
		return fmt.Errorf("get payment method: %w", err)
	}
	if pm == nil {
		return errors.New("payment method not found")
	}
	existing := ""
	if pm.Customer != nil {
		existing = strings.TrimSpace(pm.Customer.ID)
	}
	if existing == stripeCustomerID {
		return nil
	}
	if existing != "" {
		return fmt.Errorf("payment method is attached to a different Stripe customer")
	}
	_, err = paymentmethod.Attach(pmID, &stripe.PaymentMethodAttachParams{
		Customer: stripe.String(stripeCustomerID),
	})
	if err != nil {
		return fmt.Errorf("attach payment method to customer: %w", err)
	}
	return nil
}

// signupStripeCardSubscriptionWithSavedPM creates a Stripe subscription without Checkout when the client
// already has a saved card (membership invite use_existing). Checkout often fails for saved PMs that lack
// full billing details Stripe requires to render the hosted page, even though off-session charges work.
func (h *SignupHandler) signupStripeCardSubscriptionWithSavedPM(
	ctx context.Context,
	tenant *models.Tenant,
	client *models.Client,
	membership *models.Membership,
	currencyCode string,
	frequency string,
	intervalUnit *string,
	intervalCount int,
	amount float64,
	applySalesTax bool,
	continueAfterTerm bool,
	termEndTime time.Time,
	membershipStart time.Time,
	paymentMethodID string,
) error {
	if h.credRepo == nil || h.encryptor == nil {
		return errors.New("stripe integration unavailable")
	}
	creds, err := h.credRepo.GetByProvider(ctx, tenant.ID, "stripe")
	if err != nil || creds == nil || creds.APIKeyEncrypted == "" {
		return errors.New("stripe not configured for tenant")
	}
	secretKey, err := h.encryptor.Decrypt(creds.APIKeyEncrypted)
	if err != nil || secretKey == "" {
		return errors.New("stripe credentials invalid")
	}
	stripe.Key = secretKey

	pmID := strings.TrimSpace(paymentMethodID)
	if pmID == "" {
		return errors.New("payment_method_id required")
	}

	unitAmount := int64(math.Round(amount * 100))
	if unitAmount <= 0 {
		unitAmount = 1
	}
	feeCents := stripeProcessingFee(unitAmount, "card")
	baseAmount, taxAmount := splitAmountByTaxMode(amount, tenantSalesTaxPercent(tenant), applySalesTax)
	totalAmount := float64(unitAmount+feeCents) / 100.0

	cur := strings.ToLower(strings.TrimSpace(currencyCode))
	if cur == "" {
		cur = string(stripe.CurrencyAUD)
	}

	stripeCustomerID := ""
	if client.StripeCustomerID != nil {
		stripeCustomerID = strings.TrimSpace(*client.StripeCustomerID)
	}
	if stripeCustomerID == "" {
		if fromHistory, _ := h.clientRepo.FindLatestStripeCustomerIDFromHistory(ctx, client.ID, tenant.ID); strings.TrimSpace(fromHistory) != "" {
			stripeCustomerID = strings.TrimSpace(fromHistory)
			_ = h.clientRepo.SetStripeCustomerID(ctx, client.ID, tenant.ID, stripeCustomerID)
		}
	}
	if stripeCustomerID == "" {
		cparams := &stripe.CustomerParams{
			Name: stripe.String(client.Name),
			Metadata: map[string]string{
				"tenant_id": tenant.ID.String(),
				"client_id": client.ID.String(),
			},
		}
		if client.Email != nil && strings.TrimSpace(*client.Email) != "" {
			cparams.Email = stripe.String(strings.TrimSpace(*client.Email))
		}
		cust, cErr := customer.New(cparams)
		if cErr != nil || cust == nil || strings.TrimSpace(cust.ID) == "" {
			if cErr != nil {
				return fmt.Errorf("create stripe customer: %w", cErr)
			}
			return errors.New("create stripe customer failed")
		}
		stripeCustomerID = strings.TrimSpace(cust.ID)
		_ = h.clientRepo.SetStripeCustomerID(ctx, client.ID, tenant.ID, stripeCustomerID)
	}

	if err := attachStripePaymentMethodToCustomer(stripeCustomerID, pmID); err != nil {
		return err
	}
	if _, updErr := customer.Update(stripeCustomerID, &stripe.CustomerParams{
		InvoiceSettings: &stripe.CustomerInvoiceSettingsParams{
			DefaultPaymentMethod: stripe.String(pmID),
		},
	}); updErr != nil {
		return fmt.Errorf("set default payment method: %w", updErr)
	}

	cc := strings.ToUpper(strings.TrimSpace(currencyCode))
	if cc == "" {
		cc = "AUD"
	}
	subMeta := map[string]string{
		"tenant_id":             tenant.ID.String(),
		"tenant_slug":           tenant.Slug,
		"client_id":             client.ID.String(),
		"membership_id":         membership.ID.String(),
		"payment_amount":        strconv.FormatFloat(totalAmount, 'f', 2, 64),
		"base_amount":           strconv.FormatFloat(baseAmount, 'f', 2, 64),
		"tax_amount":            strconv.FormatFloat(taxAmount, 'f', 2, 64),
		"processing_fee_amount": strconv.FormatFloat(float64(feeCents)/100, 'f', 2, 64),
		"currency_code":         cc,
		"payment_freq":          frequency,
	}
	if !continueAfterTerm {
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
			"clientroll":    "signup_subscription",
		},
	})
	if err != nil {
		return fmt.Errorf("stripe product: %w", err)
	}

	interval, count := stripeRecurringInterval(frequency, intervalUnit, intervalCount)
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
	var addItems []*stripe.SubscriptionAddInvoiceItemParams
	nowUTC := time.Now().UTC()
	if !membershipStart.IsZero() {
		startAt := tenantDateStartUTC(membershipStart, tenant.Timezone).Truncate(time.Second)
		if startAt.After(nowUTC) {
			v := startAt.Unix()
			trialEnd = &v
		} else {
			nextAnchor := nextRecurringAnchorUTC(startAt, interval, count, nowUTC)
			if nextAnchor.After(nowUTC) {
				subMeta["late_start_catch_up"] = "true"
				subMeta["late_start_next_anchor"] = nextAnchor.Format(time.RFC3339)
				na := nextAnchor.Unix()
				trialEnd = &na
				addItems = []*stripe.SubscriptionAddInvoiceItemParams{
					{
						Quantity: stripe.Int64(1),
						PriceData: &stripe.InvoiceItemPriceDataParams{
							Currency:    stripe.String(cur),
							Product:     stripe.String(prod.ID),
							UnitAmount:  stripe.Int64(unitAmount),
							TaxBehavior: stripe.String(taxExcl),
						},
					},
				}
				if feeCents > 0 {
					addItems = append(addItems, &stripe.SubscriptionAddInvoiceItemParams{
						Quantity: stripe.Int64(1),
						PriceData: &stripe.InvoiceItemPriceDataParams{
							Currency:    stripe.String(cur),
							Product:     stripe.String(prod.ID),
							UnitAmount:  stripe.Int64(feeCents),
							TaxBehavior: stripe.String(taxExcl),
						},
					})
				}
			}
		}
	}

	subParams := &stripe.SubscriptionParams{
		Customer:             stripe.String(stripeCustomerID),
		DefaultPaymentMethod: stripe.String(pmID),
		Items:                items,
		Metadata:             subMeta,
		AddInvoiceItems:      addItems,
		PaymentBehavior:      stripe.String("error_if_incomplete"),
		OffSession:           stripe.Bool(true),
	}
	if trialEnd != nil {
		anchor := time.Unix(*trialEnd, 0).UTC()
		if anchor.Sub(nowUTC) >= stripeMinTrialEndLead {
			subParams.TrialEnd = trialEnd
		} else {
			subParams.BillingCycleAnchor = trialEnd
			subParams.ProrationBehavior = stripe.String("none")
			subMeta["defer_first_charge_mode"] = "billing_cycle_anchor"
		}
	}
	subParams.AddExpand("latest_invoice")
	sub, err := subscription.New(subParams)
	if err != nil {
		return fmt.Errorf("stripe subscription: %w", err)
	}
	if sub == nil || strings.TrimSpace(sub.ID) == "" {
		return errors.New("stripe subscription missing id")
	}

	if sub.LatestInvoice != nil && strings.TrimSpace(sub.LatestInvoice.ID) != "" {
		invID := strings.TrimSpace(sub.LatestInvoice.ID)
		if _, invErr := invoice.Update(invID, &stripe.InvoiceParams{Metadata: subMeta}); invErr != nil {
			log.Printf("signup stripe invoice metadata tenant=%s invoice=%s err=%v", tenant.ID, invID, invErr)
		}
	}

	if !continueAfterTerm && termEndTime.Unix() > 0 {
		if _, caErr := subscription.Update(sub.ID, &stripe.SubscriptionParams{
			CancelAt:          stripe.Int64(termEndTime.Unix()),
			CancelAtPeriodEnd: stripe.Bool(false),
		}); caErr != nil {
			return fmt.Errorf("set subscription cancel_at: %w", caErr)
		}
	}
	return nil
}

func (h *SignupHandler) googlePlacesAPIKey(ctx context.Context) string {
	if h.platformCfgRepo == nil || h.encryptor == nil {
		return ""
	}
	cfg, err := h.platformCfgRepo.GetOrCreate(ctx)
	if err != nil || cfg == nil || cfg.GooglePlacesAPIKeyEncrypted == nil || *cfg.GooglePlacesAPIKeyEncrypted == "" || !cfg.GooglePlacesIsActive {
		return ""
	}
	key, err := h.encryptor.Decrypt(*cfg.GooglePlacesAPIKeyEncrypted)
	if err != nil {
		return ""
	}
	return key
}

func (h *SignupHandler) createStripeCheckoutSession(r *http.Request, tenant *models.Tenant, client *models.Client, membership *models.Membership, currencyCode string, frequency string, intervalUnit *string, intervalCount int, amount float64, applySalesTax bool, continueAfterTerm bool, termEndTime time.Time, membershipStart time.Time, paymentMethodID string, signupInviteID *uuid.UUID) (string, error) {
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
		SuccessURL:        stripe.String(origin + "/join/" + tenant.Slug + "?stripe=success&session_id={CHECKOUT_SESSION_ID}"),
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
	if signupInviteID != nil {
		params.Metadata["signup_invite_id"] = signupInviteID.String()
	}
	if !continueAfterTerm {
		params.Metadata["continue_after_term"] = "false"
		params.Metadata["cancel_at_timestamp"] = strconv.FormatInt(termEndTime.Unix(), 10)
	} else {
		params.Metadata["continue_after_term"] = "true"
	}
	if strings.TrimSpace(paymentMethodID) != "" {
		params.Metadata["requested_payment_method_id"] = strings.TrimSpace(paymentMethodID)
	}
	stripeCustomerID := ""
	if client.StripeCustomerID != nil {
		stripeCustomerID = strings.TrimSpace(*client.StripeCustomerID)
	}
	if stripeCustomerID == "" {
		if fromHistory, _ := h.clientRepo.FindLatestStripeCustomerIDFromHistory(r.Context(), client.ID, tenant.ID); strings.TrimSpace(fromHistory) != "" {
			stripeCustomerID = strings.TrimSpace(fromHistory)
			_ = h.clientRepo.SetStripeCustomerID(r.Context(), client.ID, tenant.ID, stripeCustomerID)
		}
	}
	if stripeCustomerID == "" {
		cparams := &stripe.CustomerParams{
			Name: stripe.String(client.Name),
			Metadata: map[string]string{
				"tenant_id": tenant.ID.String(),
				"client_id": client.ID.String(),
			},
		}
		if client.Email != nil && strings.TrimSpace(*client.Email) != "" {
			cparams.Email = stripe.String(strings.TrimSpace(*client.Email))
		}
		cust, cErr := customer.New(cparams)
		if cErr != nil || cust == nil || strings.TrimSpace(cust.ID) == "" {
			return "", errors.New("failed to create stripe customer")
		}
		stripeCustomerID = strings.TrimSpace(cust.ID)
		_ = h.clientRepo.SetStripeCustomerID(r.Context(), client.ID, tenant.ID, stripeCustomerID)
	}
	if stripeCustomerID != "" {
		params.Customer = stripe.String(stripeCustomerID)
		params.PaymentMethodCollection = stripe.String("if_required")
	}
	if strings.TrimSpace(paymentMethodID) != "" && stripeCustomerID != "" {
		pmID := strings.TrimSpace(paymentMethodID)
		if err := attachStripePaymentMethodToCustomer(stripeCustomerID, pmID); err != nil {
			return "", err
		}
		_, updErr := customer.Update(stripeCustomerID, &stripe.CustomerParams{
			InvoiceSettings: &stripe.CustomerInvoiceSettingsParams{
				DefaultPaymentMethod: stripe.String(pmID),
			},
		})
		if updErr != nil {
			return "", fmt.Errorf("set default payment method: %w", updErr)
		}
		params.Metadata["requested_payment_method_id"] = pmID
	}
	// Prefill email only when we are not passing an existing Stripe customer id.
	hasStripeCustomer := params.Customer != nil && strings.TrimSpace(*params.Customer) != ""
	if !hasStripeCustomer && client.Email != nil && strings.TrimSpace(*client.Email) != "" {
		params.CustomerEmail = stripe.String(strings.TrimSpace(*client.Email))
	}
	sanitizeCheckoutCustomerParams(params)

	cur := strings.ToLower(strings.TrimSpace(currencyCode))
	if cur == "" {
		cur = string(stripe.CurrencyAUD)
	}

	feeCents := stripeProcessingFee(unitAmount, "card")
	totalAmount := float64(unitAmount+feeCents) / 100.0
	baseAmount, taxAmount := splitAmountByTaxMode(amount, tenantSalesTaxPercent(tenant), applySalesTax)
	params.Metadata["payment_amount"] = strconv.FormatFloat(totalAmount, 'f', 2, 64)
	params.Metadata["base_amount"] = strconv.FormatFloat(baseAmount, 'f', 2, 64)
	params.Metadata["tax_amount"] = strconv.FormatFloat(taxAmount, 'f', 2, 64)
	params.Metadata["processing_fee_amount"] = strconv.FormatFloat(float64(feeCents)/100, 'f', 2, 64)

	lineItem := &stripe.CheckoutSessionLineItemParams{
		Quantity: stripe.Int64(1),
		PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
			Currency:   stripe.String(cur),
			UnitAmount: stripe.Int64(unitAmount),
			ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
				Name: stripe.String("Membership - " + tenant.Name),
			},
		},
	}
	feeItem := &stripe.CheckoutSessionLineItemParams{
		Quantity: stripe.Int64(1),
		PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
			Currency:   stripe.String(cur),
			UnitAmount: stripe.Int64(feeCents),
			ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
				Name: stripe.String("Processing Fee"),
			},
		},
	}
	buildOneOffLineItems := func() []*stripe.CheckoutSessionLineItemParams {
		return []*stripe.CheckoutSessionLineItemParams{
			{
				Quantity: stripe.Int64(1),
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String(cur),
					UnitAmount: stripe.Int64(unitAmount),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String("Membership - " + tenant.Name),
					},
				},
			},
			{
				Quantity: stripe.Int64(1),
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String(cur),
					UnitAmount: stripe.Int64(feeCents),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String("Processing Fee"),
					},
				},
			},
		}
	}
	if mode == stripe.CheckoutSessionModeSubscription {
		interval, count := stripeRecurringInterval(frequency, intervalUnit, intervalCount)
		recurring := &stripe.CheckoutSessionLineItemPriceDataRecurringParams{
			Interval:      stripe.String(interval),
			IntervalCount: stripe.Int64(count),
		}
		lineItem.PriceData.Recurring = recurring
		feeItem.PriceData.Recurring = recurring
		subMeta := map[string]string{
			"tenant_id":             tenant.ID.String(),
			"tenant_slug":           tenant.Slug,
			"client_id":             client.ID.String(),
			"membership_id":         membership.ID.String(),
			"payment_amount":        params.Metadata["payment_amount"],
			"base_amount":           params.Metadata["base_amount"],
			"tax_amount":            params.Metadata["tax_amount"],
			"processing_fee_amount": params.Metadata["processing_fee_amount"],
			"currency_code":         currencyCode,
			"payment_freq":          frequency,
		}
		if signupInviteID != nil {
			subMeta["signup_invite_id"] = signupInviteID.String()
		}
		params.SubscriptionData = &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: subMeta,
		}
		// Option A behavior:
		// - Future start date: defer first recurring invoice to configured start date.
		// - Start date already passed at signup time: charge now, but anchor recurring from next cycle date.
		if !membershipStart.IsZero() && params.SubscriptionData != nil {
			nowUTC := time.Now().UTC()
			startAt := tenantDateStartUTC(membershipStart, tenant.Timezone).Truncate(time.Second)
			if startAt.After(nowUTC) {
				applyDeferFirstSubscriptionChargeCheckout(params.SubscriptionData, startAt, nowUTC, params.Metadata)
			} else {
				nextAnchor := nextRecurringAnchorUTC(startAt, interval, count, nowUTC)
				if nextAnchor.After(nowUTC) {
					applyDeferFirstSubscriptionChargeCheckout(params.SubscriptionData, nextAnchor, nowUTC, params.Metadata)
					params.Metadata["late_start_catch_up"] = "true"
					params.Metadata["late_start_next_anchor"] = nextAnchor.Format(time.RFC3339)
					// Add a one-off initial charge now while recurring billing starts on next anchor.
					params.LineItems = append(buildOneOffLineItems(), lineItem, feeItem)
				}
			}
		}
	}
	if len(params.LineItems) == 0 {
		params.LineItems = []*stripe.CheckoutSessionLineItemParams{lineItem, feeItem}
	}

	sanitizeCheckoutCustomerParams(params)
	sess, err := checkoutsession.New(params)
	if err != nil {
		return "", err
	}
	return sess.URL, nil
}

// createStripeBECSCheckoutSession creates a Stripe Checkout session for Stripe direct debit.
// The direct debit rail is selected by client country (AU -> BECS, US -> ACH).
func (h *SignupHandler) createStripeBECSCheckoutSession(r *http.Request, tenant *models.Tenant, client *models.Client, membership *models.Membership, currencyCode string, frequency string, intervalUnit *string, intervalCount int, amount float64, applySalesTax bool, continueAfterTerm bool, termEndTime time.Time, membershipStart time.Time, paymentMethodID string, signupInviteID *uuid.UUID) (string, error) {
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

	cur := strings.ToLower(strings.TrimSpace(currencyCode))
	if cur == "" {
		cur = "aud"
	}

	unitAmount := int64(math.Round(amount * 100))
	if unitAmount <= 0 {
		unitAmount = 1
	}

	mode := stripe.CheckoutSessionModePayment
	if frequency != "upfront" {
		mode = stripe.CheckoutSessionModeSubscription
	}
	directDebitType := stripeDirectDebitTypeForCountry(client.Country)
	if directDebitType == "" {
		return "", errors.New("Direct debit is only available when client country is AU or US. Set client country first")
	}

	params := &stripe.CheckoutSessionParams{
		Mode:               stripe.String(string(mode)),
		SuccessURL:         stripe.String(origin + "/join/" + tenant.Slug + "?stripe=success&session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:          stripe.String(origin + "/join/" + tenant.Slug + "?stripe=cancelled"),
		ClientReferenceID:  stripe.String(client.ID.String()),
		PaymentMethodTypes: stripe.StringSlice([]string{directDebitType}),
		Metadata: map[string]string{
			"tenant_id":           tenant.ID.String(),
			"tenant_slug":         tenant.Slug,
			"client_id":           client.ID.String(),
			"membership_id":       membership.ID.String(),
			"payment_amount":      strconv.FormatFloat(amount, 'f', 2, 64),
			"currency_code":       strings.ToUpper(cur),
			"payment_freq":        frequency,
			"payment_method":      "direct_debit",
			"payment_method_type": directDebitType,
			"payment_method_label": map[string]string{
				"au_becs_debit":   "Australia BECS Direct Debit",
				"us_bank_account": "US Bank Account (ACH Direct Debit)",
			}[directDebitType],
		},
	}
	if signupInviteID != nil {
		params.Metadata["signup_invite_id"] = signupInviteID.String()
	}
	stripeCustomerID := ""
	if client.StripeCustomerID != nil {
		stripeCustomerID = strings.TrimSpace(*client.StripeCustomerID)
	}
	if stripeCustomerID == "" {
		if fromHistory, _ := h.clientRepo.FindLatestStripeCustomerIDFromHistory(r.Context(), client.ID, tenant.ID); strings.TrimSpace(fromHistory) != "" {
			stripeCustomerID = strings.TrimSpace(fromHistory)
			_ = h.clientRepo.SetStripeCustomerID(r.Context(), client.ID, tenant.ID, stripeCustomerID)
		}
	}
	if stripeCustomerID == "" {
		cparams := &stripe.CustomerParams{
			Name: stripe.String(client.Name),
			Metadata: map[string]string{
				"tenant_id": tenant.ID.String(),
				"client_id": client.ID.String(),
			},
		}
		if client.Email != nil && strings.TrimSpace(*client.Email) != "" {
			cparams.Email = stripe.String(strings.TrimSpace(*client.Email))
		}
		cust, cErr := customer.New(cparams)
		if cErr != nil || cust == nil || strings.TrimSpace(cust.ID) == "" {
			return "", errors.New("failed to create stripe customer")
		}
		stripeCustomerID = strings.TrimSpace(cust.ID)
		_ = h.clientRepo.SetStripeCustomerID(r.Context(), client.ID, tenant.ID, stripeCustomerID)
	}
	if stripeCustomerID != "" {
		params.Customer = stripe.String(stripeCustomerID)
		params.PaymentMethodCollection = stripe.String("if_required")
	}
	if strings.TrimSpace(paymentMethodID) != "" && stripeCustomerID != "" {
		pmID := strings.TrimSpace(paymentMethodID)
		if err := attachStripePaymentMethodToCustomer(stripeCustomerID, pmID); err != nil {
			return "", err
		}
		_, updErr := customer.Update(stripeCustomerID, &stripe.CustomerParams{
			InvoiceSettings: &stripe.CustomerInvoiceSettingsParams{
				DefaultPaymentMethod: stripe.String(pmID),
			},
		})
		if updErr != nil {
			return "", fmt.Errorf("set default payment method: %w", updErr)
		}
		params.Metadata["requested_payment_method_id"] = pmID
	}
	if !continueAfterTerm {
		params.Metadata["continue_after_term"] = "false"
		params.Metadata["cancel_at_timestamp"] = strconv.FormatInt(termEndTime.Unix(), 10)
	} else {
		params.Metadata["continue_after_term"] = "true"
	}
	hasStripeCustomer := params.Customer != nil && strings.TrimSpace(*params.Customer) != ""
	if !hasStripeCustomer && client.Email != nil && strings.TrimSpace(*client.Email) != "" {
		params.CustomerEmail = stripe.String(strings.TrimSpace(*client.Email))
	}
	sanitizeCheckoutCustomerParams(params)

	feeCents := stripeProcessingFee(unitAmount, "direct_debit")
	totalAmount := float64(unitAmount+feeCents) / 100.0
	baseAmount, taxAmount := splitAmountByTaxMode(amount, tenantSalesTaxPercent(tenant), applySalesTax)
	params.Metadata["payment_amount"] = strconv.FormatFloat(totalAmount, 'f', 2, 64)
	params.Metadata["base_amount"] = strconv.FormatFloat(baseAmount, 'f', 2, 64)
	params.Metadata["tax_amount"] = strconv.FormatFloat(taxAmount, 'f', 2, 64)
	params.Metadata["processing_fee_amount"] = strconv.FormatFloat(float64(feeCents)/100, 'f', 2, 64)

	lineItem := &stripe.CheckoutSessionLineItemParams{
		Quantity: stripe.Int64(1),
		PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
			Currency:   stripe.String(cur),
			UnitAmount: stripe.Int64(unitAmount),
			ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
				Name: stripe.String("Membership - " + tenant.Name),
			},
		},
	}
	feeItem := &stripe.CheckoutSessionLineItemParams{
		Quantity: stripe.Int64(1),
		PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
			Currency:   stripe.String(cur),
			UnitAmount: stripe.Int64(feeCents),
			ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
				Name: stripe.String("Processing Fee"),
			},
		},
	}
	buildOneOffLineItems := func() []*stripe.CheckoutSessionLineItemParams {
		return []*stripe.CheckoutSessionLineItemParams{
			{
				Quantity: stripe.Int64(1),
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String(cur),
					UnitAmount: stripe.Int64(unitAmount),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String("Membership - " + tenant.Name),
					},
				},
			},
			{
				Quantity: stripe.Int64(1),
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String(cur),
					UnitAmount: stripe.Int64(feeCents),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String("Processing Fee"),
					},
				},
			},
		}
	}
	if mode == stripe.CheckoutSessionModeSubscription {
		interval, count := stripeRecurringInterval(frequency, intervalUnit, intervalCount)
		recurring := &stripe.CheckoutSessionLineItemPriceDataRecurringParams{
			Interval:      stripe.String(interval),
			IntervalCount: stripe.Int64(count),
		}
		lineItem.PriceData.Recurring = recurring
		feeItem.PriceData.Recurring = recurring
		becsSubMeta := map[string]string{
			"tenant_id":             tenant.ID.String(),
			"tenant_slug":           tenant.Slug,
			"client_id":             client.ID.String(),
			"membership_id":         membership.ID.String(),
			"payment_method":        "direct_debit",
			"payment_amount":        params.Metadata["payment_amount"],
			"base_amount":           params.Metadata["base_amount"],
			"tax_amount":            params.Metadata["tax_amount"],
			"processing_fee_amount": params.Metadata["processing_fee_amount"],
			"currency_code":         strings.ToUpper(strings.TrimSpace(currencyCode)),
			"payment_freq":          frequency,
			"payment_method_type":   params.Metadata["payment_method_type"],
			"payment_method_label":  params.Metadata["payment_method_label"],
		}
		if signupInviteID != nil {
			becsSubMeta["signup_invite_id"] = signupInviteID.String()
		}
		params.SubscriptionData = &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: becsSubMeta,
		}
		// Option A behavior:
		// - Future start date: defer first recurring invoice to configured start date.
		// - Start date already passed at signup time: charge now, but anchor recurring from next cycle date.
		if !membershipStart.IsZero() && params.SubscriptionData != nil {
			nowUTC := time.Now().UTC()
			startAt := tenantDateStartUTC(membershipStart, tenant.Timezone).Truncate(time.Second)
			if startAt.After(nowUTC) {
				applyDeferFirstSubscriptionChargeCheckout(params.SubscriptionData, startAt, nowUTC, params.Metadata)
			} else {
				nextAnchor := nextRecurringAnchorUTC(startAt, interval, count, nowUTC)
				if nextAnchor.After(nowUTC) {
					applyDeferFirstSubscriptionChargeCheckout(params.SubscriptionData, nextAnchor, nowUTC, params.Metadata)
					params.Metadata["late_start_catch_up"] = "true"
					params.Metadata["late_start_next_anchor"] = nextAnchor.Format(time.RFC3339)
					// Add a one-off initial charge now while recurring billing starts on next anchor.
					params.LineItems = append(buildOneOffLineItems(), lineItem, feeItem)
				}
			}
		}
	}
	if len(params.LineItems) == 0 {
		params.LineItems = []*stripe.CheckoutSessionLineItemParams{lineItem, feeItem}
	}

	sanitizeCheckoutCustomerParams(params)
	sess, err := checkoutsession.New(params)
	if err != nil {
		return "", err
	}
	return sess.URL, nil
}

// stripeProcessingFee computes the surcharge in cents using the gross-up formula
// so the merchant nets the original amount after Stripe deducts its fees.
//
//	Card:         1.75 % + 30 c  (no cap)
//	Direct Debit: 1.00 % + 30 c  (total Stripe fee capped at $3.50 / 350 c)
func stripeProcessingFee(baseAmountCents int64, method string) int64 {
	base := float64(baseAmountCents)
	if method == "direct_debit" {
		grossUp := (base + 30) / (1 - 0.01)
		if grossUp*0.01+30 > 350 {
			return 350 // cap: surcharge = flat $3.50 when Stripe fee hits cap
		}
		return int64(math.Ceil(grossUp - base))
	}
	grossUp := (base + 30) / (1 - 0.0175)
	fee := int64(math.Ceil(grossUp - base))
	if fee < 0 {
		return 0
	}
	return fee
}

// stripeMinTrialEndLead is Stripe's minimum horizon for subscription trial_end (Checkout + Subscriptions).
const stripeMinTrialEndLead = 48 * time.Hour

// sanitizeCheckoutCustomerParams clears customer_email whenever a Stripe customer id is set.
// Stripe returns 400 if both are present on checkout.session.create.
func sanitizeCheckoutCustomerParams(p *stripe.CheckoutSessionParams) {
	if p == nil {
		return
	}
	if p.Customer != nil && strings.TrimSpace(*p.Customer) != "" {
		p.CustomerEmail = nil
	}
}

// stripeProviderErrorMessage returns a user-facing Stripe error when available.
func stripeProviderErrorMessage(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	var se *stripe.Error
	if errors.As(err, &se) && strings.TrimSpace(se.Msg) != "" {
		return strings.TrimSpace(se.Msg)
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return fallback
	}
	return msg
}

// applyDeferFirstSubscriptionChargeCheckout defers the first recurring charge to anchor (membership start
// or next billing anchor). Uses trial_end when it is at least ~48h ahead; otherwise billing_cycle_anchor
// because Stripe rejects shorter trial_end values.
func applyDeferFirstSubscriptionChargeCheckout(sub *stripe.CheckoutSessionSubscriptionDataParams, anchor, nowUTC time.Time, meta map[string]string) {
	if sub == nil || !anchor.After(nowUTC) {
		return
	}
	if anchor.Sub(nowUTC) >= stripeMinTrialEndLead {
		sub.TrialEnd = stripe.Int64(anchor.Unix())
		return
	}
	sub.BillingCycleAnchor = stripe.Int64(anchor.Unix())
	if sub.ProrationBehavior == nil {
		sub.ProrationBehavior = stripe.String("none")
	}
	if meta != nil {
		meta["defer_first_charge_mode"] = "billing_cycle_anchor"
	}
}

// tenantDateStartUTC returns UTC timestamp for local 00:00 on the same calendar date
// in the tenant's timezone. This keeps start-date semantics anchored to tenant-local dates.
func tenantDateStartUTC(value time.Time, timezone string) time.Time {
	loc := time.UTC
	if tz := strings.TrimSpace(timezone); tz != "" {
		if loaded, err := time.LoadLocation(tz); err == nil {
			loc = loaded
		}
	}
	y, m, d := value.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc).UTC()
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

func nextRecurringAnchorUTC(startAt time.Time, interval string, count int64, nowUTC time.Time) time.Time {
	if count <= 0 {
		count = 1
	}
	anchor := startAt
	for !anchor.After(nowUTC) {
		if interval == "week" {
			anchor = anchor.AddDate(0, 0, int(7*count))
			continue
		}
		anchor = anchor.AddDate(0, int(count), 0)
	}
	return anchor
}

func (h *SignupHandler) createAgreementSnapshot(r *http.Request, tenant *models.Tenant, client *models.Client, membership *models.Membership, input SignupSubmission, mtID, toID uuid.UUID) string {
	if h.agreementRepo == nil {
		log.Printf("createAgreementSnapshot: agreementRepo nil membership=%s tenant=%s", membership.ID, tenant.ID)
		return ""
	}
	mt, _ := h.mtRepo.FindByID(r.Context(), mtID, tenant.ID)
	if mt == nil {
		log.Printf("createAgreementSnapshot: membership type not visible or missing tenant=%s mt_id=%s membership=%s (check DB RLS / tenant context)", tenant.ID, mtID, membership.ID)
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
		if client.UnitNumber != nil && *client.UnitNumber != "" {
			addr = "Unit " + *client.UnitNumber + ", " + addr
		}
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
			b.WriteString(fmt.Sprintf(`<div class="tc-content">%s</div>`, formatTermsContentForAgreementPDF(t.ContentHTML, snap)))
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

var (
	termsTemplateControlPattern = regexp.MustCompile(`\{%\s*[^%]*\s*%\}`)
	termsTemplateValuePattern   = regexp.MustCompile(`\{\{\s*[^}]*\s*\}\}`)
	termsHTMLTagPattern         = regexp.MustCompile(`</?[a-zA-Z][^>]*>`)
)

func formatTermsContentForAgreementPDF(raw string, snap *repository.AgreementSnapshot) string {
	content := strings.TrimSpace(replaceKnownTermsPlaceholders(raw, snap))
	if content == "" {
		return `<p>No terms content available.</p>`
	}
	content = termsTemplateControlPattern.ReplaceAllString(content, "")
	content = termsTemplateValuePattern.ReplaceAllString(content, "")
	content = strings.TrimSpace(content)
	if content == "" {
		return `<p>No terms content available.</p>`
	}

	// If terms content is already HTML (e.g. from rich-text editor), keep it as HTML
	// after removing unresolved template placeholders.
	if termsHTMLTagPattern.MatchString(content) {
		return content
	}

	// Plain-text fallback: preserve paragraphs and line breaks.
	content = strings.ReplaceAll(content, "\r\n", "\n")
	parts := strings.Split(content, "\n\n")
	var out strings.Builder
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		p = html.EscapeString(p)
		p = strings.ReplaceAll(p, "\n", "<br>")
		out.WriteString("<p>")
		out.WriteString(p)
		out.WriteString("</p>")
	}
	if out.Len() == 0 {
		return `<p>No terms content available.</p>`
	}
	return out.String()
}

func replaceKnownTermsPlaceholders(raw string, snap *repository.AgreementSnapshot) string {
	if strings.TrimSpace(raw) == "" || snap == nil {
		return raw
	}

	deref := func(s *string) string {
		if s != nil {
			return strings.TrimSpace(*s)
		}
		return ""
	}

	clientName := strings.TrimSpace(snap.ClientName)
	if clientName == "" {
		clientName = "Client"
	}
	clientAddress := deref(snap.ClientAddress)
	if clientAddress == "" {
		clientAddress = "Client address"
	}

	pairs := []string{
		// Legacy dotted placeholders (backwards compat)
		"{{client.name}}", clientName,
		"{{ client.name }}", clientName,
		"{{contact.name}}", clientName,
		"{{ contact.name }}", clientName,
		"{{member.name}}", clientName,
		"{{ member.name }}", clientName,
		"{{client.address}}", clientAddress,
		"{{ client.address }}", clientAddress,

		// Standard snake_case placeholders
		"{{client_name}}", clientName,
		"{{client_email}}", deref(snap.ClientEmail),
		"{{client_phone}}", deref(snap.ClientPhone),
		"{{client_address}}", clientAddress,
		"{{client_dob}}", deref(snap.ClientDateOfBirth),
		"{{client_gender}}", deref(snap.ClientGender),
		"{{emergency_contact_name}}", deref(snap.ClientEmergencyContactName),
		"{{emergency_contact_phone}}", deref(snap.ClientEmergencyContactPhone),

		"{{membership_title}}", snap.MembershipTypeTitle,
		"{{term_months}}", fmt.Sprintf("%d", snap.TermMonths),
		"{{term_weeks}}", fmt.Sprintf("%d", snap.TermMonths*52/12),
		"{{total_fee}}", fmt.Sprintf("%.2f", snap.TotalFee),
		"{{start_date}}", snap.StartDate,
		"{{end_date}}", deref(snap.EndDate),
		"{{payment_frequency}}", deref(snap.PaymentFrequency),
		"{{payment_method}}", deref(snap.PaymentMethod),

		"{{company_name}}", deref(snap.OrgCompanyName),
		"{{company_trading_name}}", deref(snap.OrgTradingName),
		"{{company_abn}}", deref(snap.OrgABN),
		"{{company_address}}", deref(snap.OrgAddress),
		"{{company_email}}", deref(snap.OrgEmail),
		"{{company_phone}}", deref(snap.OrgPhone),

		"{{today_date}}", time.Now().Format("2 January 2006"),
	}

	if snap.PaymentAmount != nil {
		pairs = append(pairs, "{{payment_amount}}", fmt.Sprintf("%.2f", *snap.PaymentAmount))
	}

	return strings.NewReplacer(pairs...).Replace(raw)
}

// RenderAgreementHTMLForSnapshot builds the legal agreement HTML for a prepared snapshot.
// Exported for operational tooling such as historical snapshot backfills.
func RenderAgreementHTMLForSnapshot(snap *repository.AgreementSnapshot, terms []repository.AcceptedTermInput, tenant *models.Tenant) string {
	return renderAgreementHTML(snap, terms, tenant)
}
