package handlers

import (
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"clientroll/internal/database"
	"clientroll/internal/config"
	"clientroll/internal/email"
	"clientroll/internal/events"
	"clientroll/internal/middleware"
	"clientroll/internal/models"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

var tenantSlugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,48}[a-z0-9]$`)
var currencyCodeRegex = regexp.MustCompile(`^[A-Z]{3}$`)
var allowedTenantCurrencies = map[string]struct{}{
	"AUD": {},
	"USD": {},
	"EUR": {},
	"GBP": {},
	"NZD": {},
	"CAD": {},
	"SGD": {},
}
var allowedTenantPaymentFrequencies = map[string]struct{}{
	"upfront":         {},
	"weekly":          {},
	"fortnightly":     {},
	"monthly":         {},
	"quarterly":       {},
	"custom_schedule": {},
}

// SettingsHandler handles tenant settings endpoints.
type SettingsHandler struct {
	settingsRepo      *repository.SettingsRepo
	tenantRepo        *repository.TenantRepo
	emailTemplateRepo *repository.TenantEmailTemplateRepo
	encryptor         *database.Encryptor
	signupWebhookSvc  *events.SignupWebhookService
	cfg               *config.Config
}

// NewSettingsHandler creates a new settings handler.
func NewSettingsHandler(sr *repository.SettingsRepo, tr *repository.TenantRepo, etr *repository.TenantEmailTemplateRepo) *SettingsHandler {
	return &SettingsHandler{settingsRepo: sr, tenantRepo: tr, emailTemplateRepo: etr}
}

func (h *SettingsHandler) SetSignupWebhookSupport(enc *database.Encryptor, svc *events.SignupWebhookService, cfg *config.Config) {
	h.encryptor = enc
	h.signupWebhookSvc = svc
	h.cfg = cfg
}

// RegisterRoutes registers settings routes.
func (h *SettingsHandler) RegisterRoutes(r chi.Router) {
	r.Get("/settings/agreement", h.GetAgreementSettings)
	r.Put("/settings/agreement", h.UpdateAgreementSettings)
	r.Get("/settings/business-lookup", h.LookupBusiness)
	r.Get("/settings/tenant", h.GetTenantSettings)
	r.Put("/settings/tenant", h.UpdateTenantSettings)
	r.Post("/settings/signup-webhook/test", h.TestSignupWebhook)
	r.Get("/settings/email-templates", h.ListEmailTemplates)
	r.Put("/settings/email-templates/{templateKey}", h.UpdateEmailTemplate)
	r.Put("/settings/email-templates/category/{category}/toggle-all", h.ToggleEmailTemplatesByCategory)
	r.Post("/complete-setup", h.CompleteSetup)
}

type businessLookupResult struct {
	Name     string `json:"name"`
	ABN      string `json:"abn"`
	ACN      string `json:"acn,omitempty"`
	State    string `json:"state,omitempty"`
	Postcode string `json:"postcode,omitempty"`
	Score    int    `json:"score,omitempty"`
}

var abrCompressedItemRegex = regexp.MustCompile(`name="Results_NameItems\[\d+\]\.Compressed" type="hidden" value="([^"]+)"`)
var abrCompanyNameHintRegex = regexp.MustCompile(`\b(PTY|LTD|LIMITED|PROPRIETARY|CORPORATION|CORP)\b`)
var abrHTMLTagRegex = regexp.MustCompile(`(?is)<[^>]+>`)
var abrTableRowRegex = regexp.MustCompile(`(?is)<tr[^>]*>\s*<td[^>]*>[\s\S]*?<a[^>]*href=["'][^"']*ABN/View\?abn=\d+[^"']*["'][^>]*>\s*([0-9 ]{11,20})\s*</a>[\s\S]*?</td>\s*<td[^>]*>\s*([\s\S]*?)\s*</td>\s*<td[^>]*>\s*([\s\S]*?)\s*</td>\s*<td[^>]*>\s*([\s\S]*?)\s*</td>`)
var abrLocationPostcodeStateRegex = regexp.MustCompile(`\b(\d{4})\s+([A-Z]{2,3})\b`)
var abrLocationStatePostcodeRegex = regexp.MustCompile(`\b([A-Z]{2,3})\s+(\d{4})\b`)

// LookupBusiness searches ABN Lookup by company name and returns candidate ABN/ACN values.
func (h *SettingsHandler) LookupBusiness(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if len(name) < 2 {
		writeError(w, http.StatusBadRequest, "name must be at least 2 characters")
		return
	}

	searchURL := fmt.Sprintf("https://abr.business.gov.au/Search/ResultsActive?SearchText=%s", url.QueryEscape(name))
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(searchURL)
	if err != nil {
		log.Printf("Business lookup request failed: %v", err)
		writeError(w, http.StatusBadGateway, "Failed to lookup business")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, "Business lookup unavailable")
		return
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		writeError(w, http.StatusBadGateway, "Failed to read business lookup response")
		return
	}

	matches := abrCompressedItemRegex.FindAllStringSubmatch(string(body), 10)
	results := make([]businessLookupResult, 0, len(matches))
	seenABN := map[string]struct{}{}

	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		raw := html.UnescapeString(m[1])
		parts := strings.Split(raw, ",")
		if len(parts) < 13 {
			continue
		}

		abnDigits := strings.TrimSpace(parts[0])
		abn := strings.TrimSpace(parts[1])
		if abn == "" && len(abnDigits) == 11 {
			abn = fmt.Sprintf("%s %s %s %s", abnDigits[:2], abnDigits[2:5], abnDigits[5:8], abnDigits[8:11])
		}
		if abn == "" {
			continue
		}
		if _, exists := seenABN[abn]; exists {
			continue
		}
		seenABN[abn] = struct{}{}

		entityName := strings.TrimSpace(parts[5])
		state := strings.TrimSpace(parts[11])
		postcode := strings.TrimSpace(parts[12])
		score := 0
		if len(parts) > 13 {
			if s, convErr := strconv.Atoi(strings.TrimSpace(parts[13])); convErr == nil {
				score = s
			}
		}

		acn := ""
		// For company-style entities, ACN is commonly the trailing 9 digits of ABN.
		if len(abnDigits) == 11 && abrCompanyNameHintRegex.MatchString(strings.ToUpper(entityName)) {
			acnDigits := abnDigits[2:]
			acn = fmt.Sprintf("%s %s %s", acnDigits[:3], acnDigits[3:6], acnDigits[6:9])
		}

		results = append(results, businessLookupResult{
			Name:     entityName,
			ABN:      abn,
			ACN:      acn,
			State:    state,
			Postcode: postcode,
			Score:    score,
		})
	}

	// Fallback parser for ABR's current table-based result markup.
	if len(results) == 0 {
		rowMatches := abrTableRowRegex.FindAllStringSubmatch(string(body), 40)
		results = make([]businessLookupResult, 0, len(rowMatches))
		for _, row := range rowMatches {
			if len(row) < 5 {
				continue
			}

			abnDigits := strings.Map(func(r rune) rune {
				if r >= '0' && r <= '9' {
					return r
				}
				return -1
			}, strings.TrimSpace(html.UnescapeString(abrHTMLTagRegex.ReplaceAllString(row[1], " "))))
			if len(abnDigits) != 11 {
				continue
			}

			abn := fmt.Sprintf("%s %s %s %s", abnDigits[:2], abnDigits[2:5], abnDigits[5:8], abnDigits[8:11])
			if _, exists := seenABN[abn]; exists {
				continue
			}
			seenABN[abn] = struct{}{}

			entityName := strings.TrimSpace(html.UnescapeString(abrHTMLTagRegex.ReplaceAllString(row[2], " ")))
			nameType := strings.ToUpper(strings.TrimSpace(html.UnescapeString(abrHTMLTagRegex.ReplaceAllString(row[3], " "))))
			location := strings.TrimSpace(html.UnescapeString(abrHTMLTagRegex.ReplaceAllString(row[4], " ")))

			state, postcode := "", ""
			if m := abrLocationPostcodeStateRegex.FindStringSubmatch(location); len(m) == 3 {
				postcode = m[1]
				state = m[2]
			} else if m := abrLocationStatePostcodeRegex.FindStringSubmatch(location); len(m) == 3 {
				state = m[1]
				postcode = m[2]
			}

			acn := ""
			if (abrCompanyNameHintRegex.MatchString(strings.ToUpper(entityName)) || strings.Contains(nameType, "ENTITY")) && len(abnDigits) == 11 {
				acnDigits := abnDigits[2:]
				acn = fmt.Sprintf("%s %s %s", acnDigits[:3], acnDigits[3:6], acnDigits[6:9])
			}

			results = append(results, businessLookupResult{
				Name:     entityName,
				ABN:      abn,
				ACN:      acn,
				State:    state,
				Postcode: postcode,
				Score:    0,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"results": results})
}

// GetAgreementSettings returns the agreement settings (Section A).
func (h *SettingsHandler) GetAgreementSettings(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant context")
		return
	}

	settings, err := h.settingsRepo.GetAgreementSettings(r.Context(), tenantID)
	if err != nil {
		log.Printf("Error getting agreement settings: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get settings")
		return
	}

	if settings == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{})
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// UpdateAgreementSettings updates the agreement settings (Section A).
func (h *SettingsHandler) UpdateAgreementSettings(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant context")
		return
	}

	var input repository.AgreementSettings
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	settings, err := h.settingsRepo.UpsertAgreementSettings(r.Context(), tenantID, input)
	if err != nil {
		log.Printf("Error updating agreement settings: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update settings")
		return
	}

	writeJSON(w, http.StatusOK, settings)
}

func tenantSettingsResponse(tenant *models.Tenant) map[string]interface{} {
	return map[string]interface{}{
		"id":                            tenant.ID,
		"name":                          tenant.Name,
		"slug":                          tenant.Slug,
		"timezone":                      tenant.Timezone,
		"sales_tax_name":                tenant.SalesTaxName,
		"sales_tax_percent":             tenant.SalesTaxPercent,
		"enabled_currencies":            tenant.EnabledCurrencies,
		"enabled_payment_frequencies":   tenant.EnabledPaymentFrequencies,
		"ask_emergency_contact":         tenant.AskEmergencyContact,
		"ask_company_information":       tenant.AskCompanyInformation,
		"ask_birth_date":                tenant.AskBirthDate,
		"signup_webhook_url":            tenant.SignupWebhookURL,
		"signup_webhook_enabled":        tenant.SignupWebhookEnabled,
		"signup_webhook_has_secret":     strings.TrimSpace(tenant.SignupWebhookSecretEncrypted) != "",
		"is_active":                     tenant.IsActive,
		"logo_url":                      tenant.LogoURL,
		"primary_colour":                tenant.PrimaryColour,
		"secondary_colour":              tenant.SecondaryColour,
		"signup_price_display_mode":     tenant.SignupPriceDisplayMode,
		"default_thankyou_heading":      tenant.DefaultThankyouHeading,
		"default_thankyou_body":         tenant.DefaultThankyouBody,
		"default_thankyou_image_url":    tenant.DefaultThankyouImageURL,
		"default_thankyou_redirect_url": tenant.DefaultThankyouRedirectURL,
		"setup_completed_at":            tenant.SetupCompletedAt,
		"created_at":                    tenant.CreatedAt,
		"updated_at":                    tenant.UpdatedAt,
	}
}

func validateSignupWebhookURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("signup_webhook_url must be a valid absolute URL")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", fmt.Errorf("signup_webhook_url must use http or https")
	}
	return trimmed, nil
}

// GetTenantSettings returns the tenant's own configuration.
func (h *SettingsHandler) GetTenantSettings(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant context")
		return
	}

	tenant, err := h.tenantRepo.FindByID(r.Context(), tenantID)
	if err != nil || tenant == nil {
		writeError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	writeJSON(w, http.StatusOK, tenantSettingsResponse(tenant))
}

// UpdateTenantSettings updates the tenant's own configuration.
func (h *SettingsHandler) UpdateTenantSettings(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant context")
		return
	}

	role := middleware.GetRole(r.Context())
	if role != "tenant_admin" {
		writeError(w, http.StatusForbidden, "Only tenant admins can update settings")
		return
	}

	var input struct {
		Timezone                         *string  `json:"timezone,omitempty"`
		Slug                             *string  `json:"slug,omitempty"`
		SalesTaxName                     *string  `json:"sales_tax_name,omitempty"`
		SalesTaxPercent                  *float64 `json:"sales_tax_percent,omitempty"`
		EnabledCurrencies                []string `json:"enabled_currencies,omitempty"`
		EnabledPaymentFrequencies        []string `json:"enabled_payment_frequencies,omitempty"`
		AskEmergencyContact              *bool    `json:"ask_emergency_contact,omitempty"`
		AskCompanyInformation            *bool    `json:"ask_company_information,omitempty"`
		AskBirthDate                     *bool    `json:"ask_birth_date,omitempty"`
		SignupWebhookURL                 *string  `json:"signup_webhook_url,omitempty"`
		SignupWebhookEnabled             *bool    `json:"signup_webhook_enabled,omitempty"`
		RegenerateSignupWebhookSecret    *bool    `json:"regenerate_signup_webhook_secret,omitempty"`
		PrimaryColour                    *string  `json:"primary_colour,omitempty"`
		SecondaryColour                  *string  `json:"secondary_colour,omitempty"`
		SignupPriceDisplayMode           *string  `json:"signup_price_display_mode,omitempty"`
		LogoURL                          *string  `json:"logo_url,omitempty"`
		DefaultThankyouHeading           *string  `json:"default_thankyou_heading,omitempty"`
		DefaultThankyouBody              *string  `json:"default_thankyou_body,omitempty"`
		DefaultThankyouImageURL          *string  `json:"default_thankyou_image_url,omitempty"`
		DefaultThankyouRedirectURL       *string  `json:"default_thankyou_redirect_url,omitempty"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var nextSlug *string
	if input.Slug != nil {
		normalized := strings.ToLower(strings.TrimSpace(*input.Slug))
		if !tenantSlugRegex.MatchString(normalized) {
			writeError(w, http.StatusBadRequest, "Slug must be 3-50 characters, lowercase letters, numbers, and hyphens only")
			return
		}
		if existing, _ := h.tenantRepo.FindBySlug(r.Context(), normalized); existing != nil && existing.ID != tenantID {
			writeError(w, http.StatusConflict, "This URL is already taken. Please choose a different one.")
			return
		}
		nextSlug = &normalized
	}

	var nextDisplayMode *string
	if input.SignupPriceDisplayMode != nil {
		mode := strings.ToLower(strings.TrimSpace(*input.SignupPriceDisplayMode))
		if mode != "total" && mode != "weekly" && mode != "monthly" {
			writeError(w, http.StatusBadRequest, "signup_price_display_mode must be 'total', 'weekly', or 'monthly'")
			return
		}
		nextDisplayMode = &mode
	}

	var nextSalesTaxName *string
	if input.SalesTaxName != nil {
		name := strings.ToUpper(strings.TrimSpace(*input.SalesTaxName))
		if name == "" {
			name = "GST"
		}
		if len(name) > 30 {
			writeError(w, http.StatusBadRequest, "sales_tax_name must be 30 characters or fewer")
			return
		}
		nextSalesTaxName = &name
	}

	var nextSalesTaxPercent *float64
	if input.SalesTaxPercent != nil {
		v := *input.SalesTaxPercent
		if v < 0 || v > 100 {
			writeError(w, http.StatusBadRequest, "sales_tax_percent must be between 0 and 100")
			return
		}
		nextSalesTaxPercent = &v
	}

	var nextEnabledCurrencies []string
	if input.EnabledCurrencies != nil {
		seen := map[string]struct{}{}
		nextEnabledCurrencies = make([]string, 0, len(input.EnabledCurrencies)+1)
		for _, raw := range input.EnabledCurrencies {
			code := strings.ToUpper(strings.TrimSpace(raw))
			if code == "" {
				continue
			}
			if !currencyCodeRegex.MatchString(code) {
				writeError(w, http.StatusBadRequest, "enabled_currencies must contain ISO-4217 uppercase currency codes")
				return
			}
			if _, ok := allowedTenantCurrencies[code]; !ok {
				writeError(w, http.StatusBadRequest, "unsupported currency in enabled_currencies")
				return
			}
			if _, ok := seen[code]; ok {
				continue
			}
			seen[code] = struct{}{}
			nextEnabledCurrencies = append(nextEnabledCurrencies, code)
		}
		if _, ok := seen["AUD"]; !ok {
			nextEnabledCurrencies = append([]string{"AUD"}, nextEnabledCurrencies...)
		}
	}

	var nextEnabledPaymentFrequencies []string
	if input.EnabledPaymentFrequencies != nil {
		seen := map[string]struct{}{}
		nextEnabledPaymentFrequencies = make([]string, 0, len(input.EnabledPaymentFrequencies))
		for _, raw := range input.EnabledPaymentFrequencies {
			frequency := strings.ToLower(strings.TrimSpace(raw))
			if frequency == "" {
				continue
			}
			if _, ok := allowedTenantPaymentFrequencies[frequency]; !ok {
				writeError(w, http.StatusBadRequest, "unsupported value in enabled_payment_frequencies")
				return
			}
			if _, ok := seen[frequency]; ok {
				continue
			}
			seen[frequency] = struct{}{}
			nextEnabledPaymentFrequencies = append(nextEnabledPaymentFrequencies, frequency)
		}
		if len(nextEnabledPaymentFrequencies) == 0 {
			nextEnabledPaymentFrequencies = []string{"upfront", "weekly", "fortnightly", "monthly", "quarterly", "custom_schedule"}
		}
	}

	var nextSignupWebhookURL *string
	var nextSignupWebhookEnabled *bool
	var nextSignupWebhookSecret *string
	var plainSignupWebhookSecret string

	if input.SignupWebhookURL != nil || input.SignupWebhookEnabled != nil || input.RegenerateSignupWebhookSecret != nil {
		if h.encryptor == nil {
			writeError(w, http.StatusInternalServerError, "Signup webhook settings unavailable")
			return
		}
		current, err := h.tenantRepo.FindByID(r.Context(), tenantID)
		if err != nil || current == nil {
			writeError(w, http.StatusNotFound, "Tenant not found")
			return
		}

		currentURL := ""
		if current.SignupWebhookURL != nil {
			currentURL = strings.TrimSpace(*current.SignupWebhookURL)
		}
		hasSecret := strings.TrimSpace(current.SignupWebhookSecretEncrypted) != ""

		if input.SignupWebhookURL != nil {
			normalized, err := validateSignupWebhookURL(*input.SignupWebhookURL)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			if normalized == "" {
				empty := ""
				nextSignupWebhookURL = &empty
				disabled := false
				nextSignupWebhookEnabled = &disabled
				emptySecret := ""
				nextSignupWebhookSecret = &emptySecret
			} else {
				nextSignupWebhookURL = &normalized
			}
		}
		if input.SignupWebhookEnabled != nil {
			enabled := *input.SignupWebhookEnabled
			nextSignupWebhookEnabled = &enabled
		}

		urlAfter := currentURL
		if nextSignupWebhookURL != nil {
			urlAfter = strings.TrimSpace(*nextSignupWebhookURL)
		}
		enabledAfter := current.SignupWebhookEnabled
		if nextSignupWebhookEnabled != nil {
			enabledAfter = *nextSignupWebhookEnabled
		}
		if enabledAfter && urlAfter == "" {
			writeError(w, http.StatusBadRequest, "signup_webhook_url is required when signup webhook is enabled")
			return
		}

		regenerate := input.RegenerateSignupWebhookSecret != nil && *input.RegenerateSignupWebhookSecret
		needsSecret := urlAfter != "" && enabledAfter && (!hasSecret || regenerate || (nextSignupWebhookURL != nil && urlAfter != currentURL))
		if needsSecret {
			secret := events.GenerateWebhookSecret()
			encrypted, err := h.encryptor.Encrypt(secret)
			if err != nil {
				log.Printf("Error encrypting signup webhook secret: %v", err)
				writeError(w, http.StatusInternalServerError, "Failed to save signup webhook settings")
				return
			}
			nextSignupWebhookSecret = &encrypted
			plainSignupWebhookSecret = secret
		}
	}

	updateInput := models.UpdateTenantInput{
		Timezone:                     input.Timezone,
		Slug:                         nextSlug,
		SalesTaxName:                 nextSalesTaxName,
		SalesTaxPercent:              nextSalesTaxPercent,
		EnabledCurrencies:            nextEnabledCurrencies,
		EnabledPaymentFrequencies:    nextEnabledPaymentFrequencies,
		AskEmergencyContact:          input.AskEmergencyContact,
		AskCompanyInformation:        input.AskCompanyInformation,
		AskBirthDate:                 input.AskBirthDate,
		SignupWebhookURL:             nextSignupWebhookURL,
		SignupWebhookEnabled:         nextSignupWebhookEnabled,
		SignupWebhookSecretEncrypted: nextSignupWebhookSecret,
		PrimaryColour:                input.PrimaryColour,
		SecondaryColour:              input.SecondaryColour,
		SignupPriceDisplayMode:       nextDisplayMode,
		LogoURL:                      input.LogoURL,
		DefaultThankyouHeading:       input.DefaultThankyouHeading,
		DefaultThankyouBody:          input.DefaultThankyouBody,
		DefaultThankyouImageURL:      input.DefaultThankyouImageURL,
		DefaultThankyouRedirectURL:   input.DefaultThankyouRedirectURL,
	}

	tenant, err := h.tenantRepo.Update(r.Context(), tenantID, updateInput)
	if err != nil {
		log.Printf("Error updating tenant settings: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update settings")
		return
	}

	resp := tenantSettingsResponse(tenant)
	if plainSignupWebhookSecret != "" {
		resp["signup_webhook_signing_secret"] = plainSignupWebhookSecret
	}
	writeJSON(w, http.StatusOK, resp)
}

// TestSignupWebhook sends a sample signup.completed payload to the configured destination.
func (h *SettingsHandler) TestSignupWebhook(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant context")
		return
	}
	if middleware.GetRole(r.Context()) != "tenant_admin" {
		writeError(w, http.StatusForbidden, "Only tenant admins can test signup webhooks")
		return
	}
	if h.signupWebhookSvc == nil || h.encryptor == nil {
		writeError(w, http.StatusInternalServerError, "Signup webhook test unavailable")
		return
	}

	tenant, err := h.tenantRepo.FindByID(r.Context(), tenantID)
	if err != nil || tenant == nil {
		writeError(w, http.StatusNotFound, "Tenant not found")
		return
	}
	if tenant.SignupWebhookURL == nil || strings.TrimSpace(*tenant.SignupWebhookURL) == "" {
		writeError(w, http.StatusBadRequest, "Configure a signup webhook URL before testing")
		return
	}
	if strings.TrimSpace(tenant.SignupWebhookSecretEncrypted) == "" {
		writeError(w, http.StatusBadRequest, "Signup webhook signing secret is missing — save settings to generate one")
		return
	}

	appURL := "https://app.example.com"
	if h.cfg != nil && strings.TrimSpace(h.cfg.AppURL) != "" {
		appURL = strings.TrimSpace(h.cfg.AppURL)
	}

	httpStatus, responseBody, err := h.signupWebhookSvc.SendTest(r.Context(), tenant, appURL)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"http_status":    httpStatus,
		"response_body":  responseBody,
		"success":        httpStatus >= 200 && httpStatus < 300,
	})
}

// CompleteSetup marks the tenant's onboarding wizard as done.
func (h *SettingsHandler) CompleteSetup(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant context")
		return
	}

	tenant, err := h.tenantRepo.CompleteSetup(r.Context(), tenantID)
	if err != nil {
		log.Printf("Error completing tenant setup: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to complete setup")
		return
	}

	writeJSON(w, http.StatusOK, tenant)
}

// ListEmailTemplates returns operational email templates merged with tenant overrides.
func (h *SettingsHandler) ListEmailTemplates(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant context")
		return
	}
	if h.emailTemplateRepo == nil {
		writeError(w, http.StatusInternalServerError, "Email template settings unavailable")
		return
	}

	stored, err := h.emailTemplateRepo.ListByTenant(r.Context(), tenantID)
	if err != nil {
		log.Printf("Error listing email templates: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to load email templates")
		return
	}
	storedByKey := make(map[string]repository.TenantEmailTemplate, len(stored))
	for _, row := range stored {
		storedByKey[row.TemplateKey] = row
	}

	catalog := email.OperationalTemplateCatalog()
	templates := make([]map[string]interface{}, 0, len(catalog))
	for _, def := range catalog {
		subject := def.Subject
		body := def.Body
		enabled := def.DefaultEnabled
		var updatedAt *time.Time
		if row, ok := storedByKey[def.Key]; ok {
			enabled = row.Enabled
			if row.SubjectOverride != nil {
				subject = *row.SubjectOverride
			}
			if row.BodyOverride != nil {
				body = *row.BodyOverride
			}
			updatedAt = &row.UpdatedAt
		}
		templates = append(templates, map[string]interface{}{
			"id":          def.Key,
			"name":        def.Name,
			"subject":     subject,
			"body":        body,
			"category":    def.Category,
			"description": def.Description,
			"variables":   def.Variables,
			"enabled":     enabled,
			"updated_at":  updatedAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"templates": templates})
}

// UpdateEmailTemplate updates one template's enabled state and optional subject/body overrides.
func (h *SettingsHandler) UpdateEmailTemplate(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant context")
		return
	}
	role := middleware.GetRole(r.Context())
	if role != "tenant_admin" {
		writeError(w, http.StatusForbidden, "Only tenant admins can update email templates")
		return
	}
	if h.emailTemplateRepo == nil {
		writeError(w, http.StatusInternalServerError, "Email template settings unavailable")
		return
	}

	templateKey := strings.TrimSpace(chi.URLParam(r, "templateKey"))
	def, ok := email.OperationalTemplateByKey(templateKey)
	if !ok {
		writeError(w, http.StatusBadRequest, "Unknown template key")
		return
	}

	var input struct {
		Enabled *bool   `json:"enabled,omitempty"`
		Subject *string `json:"subject,omitempty"`
		Body    *string `json:"body,omitempty"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	enabled := def.DefaultEnabled
	if current, err := h.emailTemplateRepo.IsEnabled(r.Context(), tenantID, templateKey); err == nil {
		enabled = current
	}
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	normalize := func(value *string) *string {
		if value == nil {
			return nil
		}
		trimmed := strings.TrimSpace(*value)
		if trimmed == "" {
			return nil
		}
		return &trimmed
	}
	subjectOverride := normalize(input.Subject)
	bodyOverride := normalize(input.Body)
	if subjectOverride != nil && *subjectOverride == def.Subject {
		subjectOverride = nil
	}
	if bodyOverride != nil && *bodyOverride == def.Body {
		bodyOverride = nil
	}

	updated, err := h.emailTemplateRepo.Upsert(r.Context(), tenantID, templateKey, def.Category, enabled, subjectOverride, bodyOverride)
	if err != nil {
		log.Printf("Error updating email template %s: %v", templateKey, err)
		writeError(w, http.StatusInternalServerError, "Failed to update email template")
		return
	}

	subject := def.Subject
	body := def.Body
	if updated.SubjectOverride != nil {
		subject = *updated.SubjectOverride
	}
	if updated.BodyOverride != nil {
		body = *updated.BodyOverride
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":          def.Key,
		"name":        def.Name,
		"subject":     subject,
		"body":        body,
		"category":    def.Category,
		"description": def.Description,
		"variables":   def.Variables,
		"enabled":     updated.Enabled,
		"updated_at":  updated.UpdatedAt,
	})
}

// ToggleEmailTemplatesByCategory enables or disables all templates in a category for this tenant.
func (h *SettingsHandler) ToggleEmailTemplatesByCategory(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant context")
		return
	}
	role := middleware.GetRole(r.Context())
	if role != "tenant_admin" {
		writeError(w, http.StatusForbidden, "Only tenant admins can update email templates")
		return
	}
	if h.emailTemplateRepo == nil {
		writeError(w, http.StatusInternalServerError, "Email template settings unavailable")
		return
	}

	category := strings.TrimSpace(chi.URLParam(r, "category"))
	if !email.IsOperationalTemplateCategory(category) {
		writeError(w, http.StatusBadRequest, "Unknown template category")
		return
	}
	var input struct {
		Enabled *bool `json:"enabled"`
	}
	if err := decodeJSON(r, &input); err != nil || input.Enabled == nil {
		writeError(w, http.StatusBadRequest, "enabled is required")
		return
	}

	keys := email.OperationalTemplateKeysByCategory(category)
	if len(keys) == 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{"updated": 0})
		return
	}
	if err := h.emailTemplateRepo.ToggleCategory(r.Context(), tenantID, category, keys, *input.Enabled); err != nil {
		log.Printf("Error toggling email templates category %s: %v", category, err)
		writeError(w, http.StatusInternalServerError, "Failed to update email templates")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"category": category,
		"enabled":  *input.Enabled,
		"updated":  len(keys),
	})
}
