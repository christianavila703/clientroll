package events

import (
	"fmt"
	"strings"
	"time"

	"clientroll/internal/models"
	"clientroll/internal/repository"

	"github.com/google/uuid"
)

// BuildSignupWebhookPayload returns a flat Ignition-style payload for automation tools (Zapier, Make, etc.).
func BuildSignupWebhookPayload(
	tenant *models.Tenant,
	client *models.Client,
	membership *models.Membership,
	mt *models.MembershipType,
	term *models.TermOption,
	snap *repository.AgreementSnapshot,
	appBaseURL string,
) map[string]interface{} {
	appBaseURL = strings.TrimSuffix(strings.TrimSpace(appBaseURL), "/")
	if appBaseURL == "" {
		appBaseURL = "https://app.example.com"
	}

	membershipTitle := mt.Title
	if mt.MembershipName != nil && strings.TrimSpace(*mt.MembershipName) != "" {
		membershipTitle = strings.TrimSpace(*mt.MembershipName)
		if mt.MembershipLevel != nil && strings.TrimSpace(*mt.MembershipLevel) != "" {
			membershipTitle += " - " + strings.TrimSpace(*mt.MembershipLevel)
		}
	}

	totalFee := 0.0
	if term != nil {
		totalFee = term.FeeTotal
	} else if membership.PaymentAmount != nil {
		totalFee = *membership.PaymentAmount
	}

	if snap != nil && snap.TotalFee > 0 {
		totalFee = snap.TotalFee
	}

	startDate := membership.StartDate.Format("2006-01-02")
	endDate := membership.MinTermEndDate.Format("2006-01-02")
	if snap != nil {
		if snap.StartDate != "" {
			startDate = snap.StartDate
		}
		if snap.EndDate != nil && strings.TrimSpace(*snap.EndDate) != "" {
			endDate = strings.TrimSpace(*snap.EndDate)
		}
	}

	contactName := client.Name
	contactEmail := derefString(client.Email)
	contactPhone := derefString(client.Phone)
	clientDisplayName := client.Name
	if client.CompanyName != nil && strings.TrimSpace(*client.CompanyName) != "" {
		clientDisplayName = strings.TrimSpace(*client.CompanyName)
	}

	payload := map[string]interface{}{
		"event_type":                SignupCompleted,
		"Membership Id":             membership.ID.String(),
		"Membership Name":           membershipTitle,
		"Membership Reference":      strings.ToUpper(membership.ID.String()[:8]),
		"Membership Value":          formatCurrency(totalFee, membership.CurrencyCode),
		"Membership Status":         membership.Status,
		"Membership Signup Url":     fmt.Sprintf("%s/join/%s", appBaseURL, tenant.Slug),
		"Client Id":                 client.ID.String(),
		"Client Name":               clientDisplayName,
		"Contact Name":              contactName,
		"Contact Email":             contactEmail,
		"Contact Phone":             contactPhone,
		"Start Date":                startDate,
		"End Date":                  endDate,
		"Accepted By":               contactName,
		"Accepted By Client":        true,
		"Created Via Web":           true,
		"Payment Method":            derefString(membership.PaymentMethod),
		"Payment Frequency":         derefString(membership.PaymentFrequency),
		"Currency Code":             membership.CurrencyCode,
		"Signatories Contact Name":  contactName,
		"Signatories Contact Email": contactEmail,
		"Signatories State":         "signed",
		"Signatories Role":          "primary",
		"signatories": []map[string]interface{}{
			{
				"Contact Name":  contactName,
				"Contact Email": contactEmail,
				"State":         "signed",
				"Role":          "primary",
			},
		},
	}

	if membership.PaymentAmount != nil {
		payload["Payment Amount"] = formatCurrency(*membership.PaymentAmount, membership.CurrencyCode)
	}
	if term != nil {
		payload["Term Months"] = term.DurationMonths
	}
	if membership.MemberSignedAt != nil {
		payload["Signed At"] = membership.MemberSignedAt.UTC().Format(time.RFC3339)
	}

	addrLine := strings.TrimSpace(derefString(client.Address))
	if client.UnitNumber != nil && strings.TrimSpace(*client.UnitNumber) != "" {
		addrLine = strings.TrimSpace(*client.UnitNumber) + " " + addrLine
	}
	if addrLine != "" {
		payload["Client Address Lines"] = addrLine
	}
	if client.Suburb != nil {
		payload["Client Address City"] = strings.TrimSpace(*client.Suburb)
	}
	if client.State != nil {
		payload["Client Address Region"] = strings.TrimSpace(*client.State)
	}
	if client.Postcode != nil {
		payload["Client Address Post Code"] = strings.TrimSpace(*client.Postcode)
	}
	if client.Country != nil {
		payload["Client Address Country"] = countryLabel(*client.Country)
	}

	if client.CompanyName != nil && strings.TrimSpace(*client.CompanyName) != "" {
		payload["Company Name"] = strings.TrimSpace(*client.CompanyName)
	}
	if client.CompanyNumber != nil && strings.TrimSpace(*client.CompanyNumber) != "" {
		payload["Company Number"] = strings.TrimSpace(*client.CompanyNumber)
	}
	companyLine := strings.TrimSpace(derefString(client.CompanyAddress))
	if client.CompanyUnitNumber != nil && strings.TrimSpace(*client.CompanyUnitNumber) != "" {
		companyLine = strings.TrimSpace(*client.CompanyUnitNumber) + " " + companyLine
	}
	if companyLine != "" {
		payload["Client Company Address Lines"] = companyLine
	}
	if client.CompanyCity != nil {
		payload["Client Company Address City"] = strings.TrimSpace(*client.CompanyCity)
	}
	if client.CompanyStateRegion != nil {
		payload["Client Company Address Region"] = strings.TrimSpace(*client.CompanyStateRegion)
	}
	if client.CompanyPostcode != nil {
		payload["Client Company Address Post Code"] = strings.TrimSpace(*client.CompanyPostcode)
	}
	if client.CompanyCountry != nil {
		payload["Client Company Address Country"] = countryLabel(*client.CompanyCountry)
	}

	payload["Agreement PDF URL"] = fmt.Sprintf("%s/api/tenant/memberships/%s/agreement.pdf", appBaseURL, membership.ID)
	payload["Agreement API URL"] = fmt.Sprintf("%s/api/tenant/memberships/%s/agreement", appBaseURL, membership.ID)
	if snap != nil {
		payload["Agreement Snapshot Id"] = snap.ID.String()
	}

	return payload
}

// BuildSignupWebhookTestPayload returns a sample flat payload for webhook destination testing.
func BuildSignupWebhookTestPayload(tenant *models.Tenant, appBaseURL string) map[string]interface{} {
	appBaseURL = strings.TrimSuffix(strings.TrimSpace(appBaseURL), "/")
	if appBaseURL == "" {
		appBaseURL = "https://app.example.com"
	}
	membershipID := uuid.New()
	clientID := uuid.New()
	return map[string]interface{}{
		"event_type":                   SignupCompleted,
		"test":                         true,
		"Membership Id":                membershipID.String(),
		"Membership Name":              "Example Membership",
		"Membership Reference":         "EXAMPLE01",
		"Membership Value":             "$1,000.00",
		"Membership Status":            "active",
		"Membership Signup Url":        fmt.Sprintf("%s/join/%s", appBaseURL, tenant.Slug),
		"Client Id":                    clientID.String(),
		"Client Name":                  "Example Client Pty Ltd",
		"Contact Name":                 "Jane Example",
		"Contact Email":                "jane@example.com",
		"Contact Phone":                "0400 000 000",
		"Start Date":                   time.Now().UTC().Format("2006-01-02"),
		"End Date":                     time.Now().UTC().AddDate(1, 0, 0).Format("2006-01-02"),
		"Accepted By":                  "Jane Example",
		"Accepted By Client":           true,
		"Created Via Web":              true,
		"Payment Method":               "card",
		"Payment Frequency":            "monthly",
		"Currency Code":                "AUD",
		"Client Company Address Lines": "1 Example Street",
		"Client Company Address City":  "Melbourne",
		"Client Company Address Region": "Victoria",
		"Client Company Address Post Code": "3000",
		"Client Company Address Country":   "Australia",
		"Agreement PDF URL":            fmt.Sprintf("%s/api/tenant/memberships/%s/agreement.pdf", appBaseURL, membershipID),
		"Signatories Contact Name":     "Jane Example",
		"Signatories Contact Email":    "jane@example.com",
		"Signatories State":            "signed",
		"Signatories Role":             "primary",
		"signatories": []map[string]interface{}{
			{
				"Contact Name":  "Jane Example",
				"Contact Email": "jane@example.com",
				"State":         "signed",
				"Role":          "primary",
			},
		},
	}
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

func formatCurrency(amount float64, currencyCode string) string {
	symbol := "$"
	switch strings.ToUpper(strings.TrimSpace(currencyCode)) {
	case "GBP":
		symbol = "£"
	case "EUR":
		symbol = "€"
	}
	return fmt.Sprintf("%s%s", symbol, formatAmount(amount))
}

func formatAmount(amount float64) string {
	s := fmt.Sprintf("%.2f", amount)
	parts := strings.Split(s, ".")
	intPart := parts[0]
	if len(intPart) <= 3 {
		return s
	}
	var b strings.Builder
	for i, ch := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteRune(',')
		}
		b.WriteRune(ch)
	}
	if len(parts) > 1 {
		return b.String() + "." + parts[1]
	}
	return b.String()
}

func countryLabel(code string) string {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "AU":
		return "Australia"
	case "US":
		return "United States"
	case "NZ":
		return "New Zealand"
	case "GB", "UK":
		return "United Kingdom"
	default:
		return strings.ToUpper(strings.TrimSpace(code))
	}
}
