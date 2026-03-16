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

// SettingsHandler handles tenant settings endpoints.
type SettingsHandler struct {
	settingsRepo *repository.SettingsRepo
	tenantRepo   *repository.TenantRepo
}

// NewSettingsHandler creates a new settings handler.
func NewSettingsHandler(sr *repository.SettingsRepo, tr *repository.TenantRepo) *SettingsHandler {
	return &SettingsHandler{settingsRepo: sr, tenantRepo: tr}
}

// RegisterRoutes registers settings routes.
func (h *SettingsHandler) RegisterRoutes(r chi.Router) {
	r.Get("/settings/agreement", h.GetAgreementSettings)
	r.Put("/settings/agreement", h.UpdateAgreementSettings)
	r.Get("/settings/business-lookup", h.LookupBusiness)
	r.Get("/settings/tenant", h.GetTenantSettings)
	r.Put("/settings/tenant", h.UpdateTenantSettings)
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
var abrTableRowRegex = regexp.MustCompile(`(?is)<tr[^>]*>\s*<td[^>]*>\s*<a[^>]*href="/ABN/View\?abn=\d+"[^>]*>\s*([0-9 ]{11,20})\s*</a>.*?</td>\s*<td[^>]*>\s*([^<]+?)\s*</td>\s*<td[^>]*>\s*([^<]+?)\s*</td>\s*<td[^>]*>\s*([^<]+?)\s*</td>`)
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
		rowMatches := abrTableRowRegex.FindAllStringSubmatch(string(body), 20)
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
			}, strings.TrimSpace(html.UnescapeString(row[1])))
			if len(abnDigits) != 11 {
				continue
			}
			abn := fmt.Sprintf("%s %s %s %s", abnDigits[:2], abnDigits[2:5], abnDigits[5:8], abnDigits[8:11])
			if _, exists := seenABN[abn]; exists {
				continue
			}
			seenABN[abn] = struct{}{}

			entityName := strings.TrimSpace(html.UnescapeString(row[2]))
			nameType := strings.ToUpper(strings.TrimSpace(html.UnescapeString(row[3])))
			location := strings.TrimSpace(html.UnescapeString(row[4]))

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

	writeJSON(w, http.StatusOK, tenant)
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
		Timezone                   *string  `json:"timezone,omitempty"`
		Slug                       *string  `json:"slug,omitempty"`
		EnabledCurrencies          []string `json:"enabled_currencies,omitempty"`
		PrimaryColour              *string  `json:"primary_colour,omitempty"`
		SecondaryColour            *string  `json:"secondary_colour,omitempty"`
		SignupPriceDisplayMode     *string  `json:"signup_price_display_mode,omitempty"`
		LogoURL                    *string  `json:"logo_url,omitempty"`
		DefaultThankyouHeading     *string  `json:"default_thankyou_heading,omitempty"`
		DefaultThankyouBody        *string  `json:"default_thankyou_body,omitempty"`
		DefaultThankyouImageURL    *string  `json:"default_thankyou_image_url,omitempty"`
		DefaultThankyouRedirectURL *string  `json:"default_thankyou_redirect_url,omitempty"`
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

	updateInput := models.UpdateTenantInput{
		Timezone:                   input.Timezone,
		Slug:                       nextSlug,
		EnabledCurrencies:          nextEnabledCurrencies,
		PrimaryColour:              input.PrimaryColour,
		SecondaryColour:            input.SecondaryColour,
		SignupPriceDisplayMode:     nextDisplayMode,
		LogoURL:                    input.LogoURL,
		DefaultThankyouHeading:     input.DefaultThankyouHeading,
		DefaultThankyouBody:        input.DefaultThankyouBody,
		DefaultThankyouImageURL:    input.DefaultThankyouImageURL,
		DefaultThankyouRedirectURL: input.DefaultThankyouRedirectURL,
	}

	tenant, err := h.tenantRepo.Update(r.Context(), tenantID, updateInput)
	if err != nil {
		log.Printf("Error updating tenant settings: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update settings")
		return
	}

	writeJSON(w, http.StatusOK, tenant)
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
