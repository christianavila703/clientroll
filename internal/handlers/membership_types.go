package handlers

import (
	"log"
	"math"
	"net/http"
	"regexp"
	"strings"

	"clientroll/internal/middleware"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

var isoCurrencyRegex = regexp.MustCompile(`^[A-Z]{3}$`)
var allowedMembershipPaymentFrequencies = map[string]struct{}{
	"upfront":         {},
	"weekly":          {},
	"fortnightly":     {},
	"monthly":         {},
	"quarterly":       {},
	"custom_schedule": {},
}

func buildMembershipDisplayTitle(name, level string) string {
	name = strings.TrimSpace(name)
	level = strings.TrimSpace(level)
	if name == "" {
		return level
	}
	if level == "" {
		return name
	}
	return name + " - " + level
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func ptrFloat64(v float64) *float64 {
	x := v
	return &x
}

func deriveTermFrequencyFees(durationMonths int, feeTotal float64) (weekly, fortnightly, monthly, quarterly *float64) {
	if durationMonths <= 0 || feeTotal <= 0 {
		return nil, nil, nil, nil
	}
	weeks := (float64(durationMonths) * 52) / 12
	if weeks > 0 {
		weeklyAmount := round2(feeTotal / weeks)
		weekly = ptrFloat64(weeklyAmount)
		fortnightly = ptrFloat64(round2(weeklyAmount * 2))
	}
	monthly = ptrFloat64(round2(feeTotal / float64(durationMonths)))
	if durationMonths >= 3 {
		quarterly = ptrFloat64(round2(feeTotal / (float64(durationMonths) / 3)))
	}
	return weekly, fortnightly, monthly, quarterly
}

func customScheduleMatchesContractTotal(feeTotal float64, amount *float64, numPayments *int) bool {
	if amount == nil || numPayments == nil || *numPayments <= 0 {
		return false
	}
	expected := round2(feeTotal)
	customTotal := round2((*amount) * float64(*numPayments))
	return math.Abs(expected-customTotal) <= 0.01
}

// MembershipTypeHandler handles membership type CRUD.
type MembershipTypeHandler struct {
	mtRepo    *repository.MembershipTypeRepo
	termsRepo *repository.TermsRepo
}

// NewMembershipTypeHandler creates a new handler.
func NewMembershipTypeHandler(mt *repository.MembershipTypeRepo, tr *repository.TermsRepo) *MembershipTypeHandler {
	return &MembershipTypeHandler{mtRepo: mt, termsRepo: tr}
}

// RegisterRoutes registers membership type routes.
func (h *MembershipTypeHandler) RegisterRoutes(r chi.Router) {
	r.Get("/membership-types", h.List)
	r.Post("/membership-types", h.Create)
	r.Get("/membership-types/{mtID}", h.Get)
	r.Put("/membership-types/{mtID}", h.Update)
	r.Delete("/membership-types/{mtID}", h.Delete)
	r.Post("/membership-types/{mtID}/archive", h.Archive)
	r.Post("/membership-types/{mtID}/activate", h.Activate)
	r.Post("/membership-types/{mtID}/duplicate", h.Duplicate)
	r.Post("/membership-types/{mtID}/term-options", h.CreateTermOption)
	r.Put("/membership-types/{mtID}/term-options/{toID}", h.UpdateTermOption)
	r.Delete("/term-options/{toID}", h.DeleteTermOption)
	r.Post("/membership-types/{mtID}/terms/{termsID}", h.AttachTerms)
	r.Delete("/membership-types/{mtID}/terms/{termsID}", h.DetachTerms)
	r.Get("/membership-types/{mtID}/terms", h.GetTerms)
}

// List returns all membership types.
func (h *MembershipTypeHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	types, err := h.mtRepo.List(r.Context(), tenantID)
	if err != nil {
		log.Printf("Error listing membership types: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list membership types")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"membership_types": types, "count": len(types)})
}

// Create creates a new membership type.
func (h *MembershipTypeHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	var input struct {
		Title           string  `json:"title"`
		MembershipName  *string `json:"membership_name"`
		MembershipLevel *string `json:"membership_level"`
		Slug            string  `json:"slug"`
		CurrencyCode    string  `json:"currency_code"`
		Description     *string `json:"description"`
		Inclusions      *string `json:"inclusions"`
		TopImageURL     *string `json:"top_image_url"`
		ApplySalesTax   *bool   `json:"apply_sales_tax"`
		IsMostPopular   bool    `json:"is_most_popular"`
		SortOrder       int     `json:"sort_order"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request")
		return
	}
	var membershipName *string
	if input.MembershipName != nil {
		v := strings.TrimSpace(*input.MembershipName)
		if v != "" {
			membershipName = &v
		}
	}
	var membershipLevel *string
	if input.MembershipLevel != nil {
		v := strings.TrimSpace(*input.MembershipLevel)
		if v != "" {
			membershipLevel = &v
		}
	}
	composedTitle := buildMembershipDisplayTitle(
		func() string {
			if membershipName == nil {
				return ""
			}
			return *membershipName
		}(),
		func() string {
			if membershipLevel == nil {
				return ""
			}
			return *membershipLevel
		}(),
	)
	input.Title = strings.TrimSpace(input.Title)
	if composedTitle != "" {
		input.Title = composedTitle
	}
	if input.Title == "" {
		writeError(w, http.StatusBadRequest, "Membership name is required")
		return
	}
	if input.Slug == "" {
		input.Slug = repository.GenerateSlug(input.Title)
	}
	input.CurrencyCode = strings.ToUpper(strings.TrimSpace(input.CurrencyCode))
	if input.CurrencyCode == "" {
		input.CurrencyCode = "AUD"
	}
	if !isoCurrencyRegex.MatchString(input.CurrencyCode) {
		writeError(w, http.StatusBadRequest, "currency_code must be a 3-letter ISO code")
		return
	}
	applySalesTax := true
	if input.ApplySalesTax != nil {
		applySalesTax = *input.ApplySalesTax
	}

	mt, err := h.mtRepo.Create(r.Context(), tenantID, membershipName, membershipLevel, input.Title, input.Slug,
		input.Description, input.Inclusions, input.TopImageURL, input.SortOrder, input.CurrencyCode, applySalesTax)
	if err != nil {
		log.Printf("Error creating membership type: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create membership type")
		return
	}
	if input.IsMostPopular {
		if err := h.mtRepo.SetMostPopular(r.Context(), tenantID, mt.ID); err != nil {
			log.Printf("Error setting most popular on create: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to set most popular")
			return
		}
		mt, _ = h.mtRepo.FindByID(r.Context(), mt.ID, tenantID)
	}
	writeJSON(w, http.StatusCreated, mt)
}

// Update updates a membership type.
func (h *MembershipTypeHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	mtID, err := uuid.Parse(chi.URLParam(r, "mtID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid ID")
		return
	}
	var input struct {
		Title                     *string   `json:"title"`
		MembershipName            *string   `json:"membership_name"`
		MembershipLevel           *string   `json:"membership_level"`
		Slug                      *string   `json:"slug"`
		Summary                   *string   `json:"summary"`
		Description               *string   `json:"description"`
		Inclusions                *string   `json:"inclusions"`
		Benefits                  *string   `json:"benefits"`
		TopImageURL               *string   `json:"top_image_url"`
		CurrencyCode              *string   `json:"currency_code"`
		SortOrder                 *int      `json:"sort_order"`
		EnabledPaymentFrequencies *[]string `json:"enabled_payment_frequencies"`
		ApplySalesTax             *bool     `json:"apply_sales_tax"`
		IsActive                  *bool     `json:"is_active"`
		IsMostPopular             *bool     `json:"is_most_popular"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	existing, findErr := h.mtRepo.FindByID(r.Context(), mtID, tenantID)
	if findErr != nil || existing == nil {
		writeError(w, http.StatusNotFound, "Membership type not found")
		return
	}

	normalize := func(v *string) *string {
		if v == nil {
			return nil
		}
		t := strings.TrimSpace(*v)
		if t == "" {
			return nil
		}
		return &t
	}

	nextName := normalize(input.MembershipName)
	nextLevel := normalize(input.MembershipLevel)
	nextTitle := normalize(input.Title)
	if input.MembershipName != nil || input.MembershipLevel != nil {
		baseName := ""
		if nextName != nil {
			baseName = *nextName
		} else if existing.MembershipName != nil {
			baseName = *existing.MembershipName
		}
		baseLevel := ""
		if nextLevel != nil {
			baseLevel = *nextLevel
		} else if existing.MembershipLevel != nil {
			baseLevel = *existing.MembershipLevel
		}
		composed := buildMembershipDisplayTitle(baseName, baseLevel)
		if strings.TrimSpace(composed) != "" {
			nextTitle = &composed
		}
	}

	nextSlug := normalize(input.Slug)
	if nextSlug == nil && nextTitle != nil {
		generated := repository.GenerateSlug(*nextTitle)
		if generated != "" {
			nextSlug = &generated
		}
	}

	var nextCurrency *string
	if input.CurrencyCode != nil {
		normalized := strings.ToUpper(strings.TrimSpace(*input.CurrencyCode))
		if normalized == "" {
			normalized = "AUD"
		}
		if !isoCurrencyRegex.MatchString(normalized) {
			writeError(w, http.StatusBadRequest, "currency_code must be a 3-letter ISO code")
			return
		}
		nextCurrency = &normalized
	}

	var nextEnabledPaymentFrequencies *[]string
	if input.EnabledPaymentFrequencies != nil {
		seen := map[string]struct{}{}
		normalized := make([]string, 0, len(*input.EnabledPaymentFrequencies))
		for _, raw := range *input.EnabledPaymentFrequencies {
			frequency := strings.ToLower(strings.TrimSpace(raw))
			if frequency == "" {
				continue
			}
			if _, ok := allowedMembershipPaymentFrequencies[frequency]; !ok {
				writeError(w, http.StatusBadRequest, "unsupported value in enabled_payment_frequencies")
				return
			}
			if _, ok := seen[frequency]; ok {
				continue
			}
			seen[frequency] = struct{}{}
			normalized = append(normalized, frequency)
		}
		if len(normalized) == 0 {
			writeError(w, http.StatusBadRequest, "enabled_payment_frequencies must include at least one frequency when provided")
			return
		}
		nextEnabledPaymentFrequencies = &normalized
	}

	if _, err := h.mtRepo.UpdateBasics(
		r.Context(),
		mtID,
		tenantID,
		nextName,
		nextLevel,
		nextTitle,
		nextSlug,
		input.Summary,
		input.Description,
		input.Inclusions,
		input.Benefits,
		input.TopImageURL,
		nextCurrency,
		input.SortOrder,
		nextEnabledPaymentFrequencies,
		input.ApplySalesTax,
	); err != nil {
		log.Printf("Error updating membership basics: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update")
		return
	}

	if input.IsActive != nil {
		if *input.IsActive {
			_ = h.mtRepo.Activate(r.Context(), mtID, tenantID)
		} else {
			_ = h.mtRepo.Archive(r.Context(), mtID, tenantID)
		}
	}
	if input.IsMostPopular != nil {
		if *input.IsMostPopular {
			if err := h.mtRepo.SetMostPopular(r.Context(), tenantID, mtID); err != nil {
				log.Printf("Error setting most popular: %v", err)
				writeError(w, http.StatusInternalServerError, "Failed to update")
				return
			}
		} else {
			if err := h.mtRepo.ClearMostPopular(r.Context(), tenantID, mtID); err != nil {
				log.Printf("Error clearing most popular: %v", err)
				writeError(w, http.StatusInternalServerError, "Failed to update")
				return
			}
		}
	}
	mt, err := h.mtRepo.FindByID(r.Context(), mtID, tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update")
		return
	}
	writeJSON(w, http.StatusOK, mt)
}

// Delete soft-deletes a membership type. It disappears from listings but
// existing client memberships are completely unaffected.
func (h *MembershipTypeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	mtID, err := uuid.Parse(chi.URLParam(r, "mtID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid ID")
		return
	}
	if err := h.mtRepo.SoftDelete(r.Context(), mtID, tenantID); err != nil {
		log.Printf("Error soft-deleting membership type: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// Archive soft-deletes a membership type (sets is_active=false).
func (h *MembershipTypeHandler) Archive(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	mtID, _ := uuid.Parse(chi.URLParam(r, "mtID"))
	if err := h.mtRepo.Archive(r.Context(), mtID, tenantID); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to archive")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "archived"})
}

// Activate re-activates a membership type.
func (h *MembershipTypeHandler) Activate(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	mtID, _ := uuid.Parse(chi.URLParam(r, "mtID"))
	if err := h.mtRepo.Activate(r.Context(), mtID, tenantID); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to activate")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "activated"})
}

// Get returns a membership type with term options.
func (h *MembershipTypeHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	mtID, err := uuid.Parse(chi.URLParam(r, "mtID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid membership type ID")
		return
	}
	mt, err := h.mtRepo.FindByID(r.Context(), mtID, tenantID)
	if err != nil || mt == nil {
		writeError(w, http.StatusNotFound, "Membership type not found")
		return
	}
	writeJSON(w, http.StatusOK, mt)
}

// Duplicate deep-copies a membership type.
func (h *MembershipTypeHandler) Duplicate(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	mtID, err := uuid.Parse(chi.URLParam(r, "mtID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid membership type ID")
		return
	}
	mt, err := h.mtRepo.Duplicate(r.Context(), mtID, tenantID)
	if err != nil {
		log.Printf("Error duplicating membership type: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to duplicate membership type")
		return
	}
	writeJSON(w, http.StatusCreated, mt)
}

// CreateTermOption adds a term option to a membership type.
func (h *MembershipTypeHandler) CreateTermOption(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	mtID, err := uuid.Parse(chi.URLParam(r, "mtID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid membership type ID")
		return
	}
	var input struct {
		DurationMonths              int      `json:"duration_months"`
		FeeTotal                    float64  `json:"fee_total"`
		FeeWeekly                   *float64 `json:"fee_weekly"`
		FeeFortnightly              *float64 `json:"fee_fortnightly"`
		FeeMonthly                  *float64 `json:"fee_monthly"`
		FeeQuarterly                *float64 `json:"fee_quarterly"`
		AllowUpfront                bool     `json:"allow_upfront"`
		ContinueAfterTerm           bool     `json:"continue_after_term"`
		CustomScheduleEnabled       bool     `json:"custom_schedule_enabled"`
		CustomScheduleLabel         *string  `json:"custom_schedule_label"`
		CustomScheduleAmount        *float64 `json:"custom_schedule_amount"`
		CustomScheduleIntervalUnit  *string  `json:"custom_schedule_interval_unit"`
		CustomScheduleIntervalCount *int     `json:"custom_schedule_interval_count"`
		CustomScheduleNumPayments   *int     `json:"custom_schedule_num_payments"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if input.DurationMonths <= 0 {
		writeError(w, http.StatusBadRequest, "Duration must be a whole number of months")
		return
	}
	if input.CustomScheduleEnabled {
		if input.CustomScheduleAmount == nil || *input.CustomScheduleAmount <= 0 {
			writeError(w, http.StatusBadRequest, "custom_schedule_amount must be greater than 0")
			return
		}
		if input.CustomScheduleIntervalCount == nil || *input.CustomScheduleIntervalCount <= 0 {
			writeError(w, http.StatusBadRequest, "custom_schedule_interval_count must be greater than 0")
			return
		}
		if input.CustomScheduleNumPayments == nil || *input.CustomScheduleNumPayments <= 0 {
			writeError(w, http.StatusBadRequest, "custom_schedule_num_payments must be greater than 0")
			return
		}
		if input.CustomScheduleIntervalUnit == nil || (*input.CustomScheduleIntervalUnit != "week" && *input.CustomScheduleIntervalUnit != "month") {
			writeError(w, http.StatusBadRequest, "custom_schedule_interval_unit must be 'week' or 'month'")
			return
		}
		if !customScheduleMatchesContractTotal(input.FeeTotal, input.CustomScheduleAmount, input.CustomScheduleNumPayments) {
			writeError(w, http.StatusBadRequest, "custom schedule total must equal contract total")
			return
		}
	} else {
		input.CustomScheduleLabel = nil
		input.CustomScheduleAmount = nil
		input.CustomScheduleIntervalUnit = nil
		input.CustomScheduleIntervalCount = nil
		input.CustomScheduleNumPayments = nil
	}

	feeWeekly, feeFortnightly, feeMonthly, feeQuarterly := deriveTermFrequencyFees(input.DurationMonths, input.FeeTotal)
	to, err := h.mtRepo.CreateTermOption(r.Context(), tenantID, mtID,
		input.DurationMonths, input.FeeTotal, feeWeekly,
		feeFortnightly, feeMonthly, feeQuarterly, input.AllowUpfront, nil,
		input.ContinueAfterTerm,
		input.CustomScheduleEnabled, input.CustomScheduleLabel, input.CustomScheduleAmount,
		input.CustomScheduleIntervalUnit, input.CustomScheduleIntervalCount, input.CustomScheduleNumPayments)
	if err != nil {
		log.Printf("Error creating term option: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create term option")
		return
	}
	writeJSON(w, http.StatusCreated, to)
}

// UpdateTermOption updates a term option.
func (h *MembershipTypeHandler) UpdateTermOption(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	toID, err := uuid.Parse(chi.URLParam(r, "toID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid term option ID")
		return
	}
	var input struct {
		DurationMonths              int      `json:"duration_months"`
		FeeTotal                    float64  `json:"fee_total"`
		FeeWeekly                   *float64 `json:"fee_weekly"`
		FeeFortnightly              *float64 `json:"fee_fortnightly"`
		FeeMonthly                  *float64 `json:"fee_monthly"`
		FeeQuarterly                *float64 `json:"fee_quarterly"`
		AllowUpfront                bool     `json:"allow_upfront"`
		ContinueAfterTerm           bool     `json:"continue_after_term"`
		CustomScheduleEnabled       bool     `json:"custom_schedule_enabled"`
		CustomScheduleLabel         *string  `json:"custom_schedule_label"`
		CustomScheduleAmount        *float64 `json:"custom_schedule_amount"`
		CustomScheduleIntervalUnit  *string  `json:"custom_schedule_interval_unit"`
		CustomScheduleIntervalCount *int     `json:"custom_schedule_interval_count"`
		CustomScheduleNumPayments   *int     `json:"custom_schedule_num_payments"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if input.DurationMonths <= 0 {
		writeError(w, http.StatusBadRequest, "Duration must be a whole number of months")
		return
	}
	if input.CustomScheduleEnabled {
		if input.CustomScheduleAmount == nil || *input.CustomScheduleAmount <= 0 {
			writeError(w, http.StatusBadRequest, "custom_schedule_amount must be greater than 0")
			return
		}
		if input.CustomScheduleIntervalCount == nil || *input.CustomScheduleIntervalCount <= 0 {
			writeError(w, http.StatusBadRequest, "custom_schedule_interval_count must be greater than 0")
			return
		}
		if input.CustomScheduleNumPayments == nil || *input.CustomScheduleNumPayments <= 0 {
			writeError(w, http.StatusBadRequest, "custom_schedule_num_payments must be greater than 0")
			return
		}
		if input.CustomScheduleIntervalUnit == nil || (*input.CustomScheduleIntervalUnit != "week" && *input.CustomScheduleIntervalUnit != "month") {
			writeError(w, http.StatusBadRequest, "custom_schedule_interval_unit must be 'week' or 'month'")
			return
		}
		if !customScheduleMatchesContractTotal(input.FeeTotal, input.CustomScheduleAmount, input.CustomScheduleNumPayments) {
			writeError(w, http.StatusBadRequest, "custom schedule total must equal contract total")
			return
		}
	} else {
		input.CustomScheduleLabel = nil
		input.CustomScheduleAmount = nil
		input.CustomScheduleIntervalUnit = nil
		input.CustomScheduleIntervalCount = nil
		input.CustomScheduleNumPayments = nil
	}

	feeWeekly, feeFortnightly, feeMonthly, feeQuarterly := deriveTermFrequencyFees(input.DurationMonths, input.FeeTotal)
	to, err := h.mtRepo.UpdateTermOption(r.Context(), toID, tenantID, input.DurationMonths,
		input.FeeTotal, feeWeekly, feeFortnightly, feeMonthly, feeQuarterly, input.AllowUpfront, nil,
		input.ContinueAfterTerm,
		input.CustomScheduleEnabled, input.CustomScheduleLabel, input.CustomScheduleAmount,
		input.CustomScheduleIntervalUnit, input.CustomScheduleIntervalCount, input.CustomScheduleNumPayments)
	if err != nil {
		log.Printf("Error updating term option: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update term option")
		return
	}
	writeJSON(w, http.StatusOK, to)
}

// DeleteTermOption removes a term option.
func (h *MembershipTypeHandler) DeleteTermOption(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	toID, _ := uuid.Parse(chi.URLParam(r, "toID"))
	if err := h.mtRepo.DeleteTermOption(r.Context(), toID, tenantID); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete term option")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Term option deleted"})
}

// AttachTerms links a T&C to a membership type.
func (h *MembershipTypeHandler) AttachTerms(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	mtID, _ := uuid.Parse(chi.URLParam(r, "mtID"))
	termsID, _ := uuid.Parse(chi.URLParam(r, "termsID"))
	if err := h.termsRepo.AttachToMembershipType(r.Context(), tenantID, mtID, termsID); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to attach terms")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Terms attached"})
}

// DetachTerms removes a T&C link from a membership type.
func (h *MembershipTypeHandler) DetachTerms(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	mtID, _ := uuid.Parse(chi.URLParam(r, "mtID"))
	termsID, _ := uuid.Parse(chi.URLParam(r, "termsID"))
	if err := h.termsRepo.DetachFromMembershipType(r.Context(), tenantID, mtID, termsID); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to detach terms")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Terms detached"})
}

// GetTerms returns T&Cs attached to a membership type.
func (h *MembershipTypeHandler) GetTerms(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	mtID, _ := uuid.Parse(chi.URLParam(r, "mtID"))
	terms, err := h.termsRepo.GetForMembershipType(r.Context(), mtID, tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get terms")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"terms": terms})
}
