// Package terms hosts the single source of truth for which {{variables}} a
// T&C document may reference. The same registry is consumed by:
//   - The PDF/HTML renderer on the server (replaceKnownTermsPlaceholders).
//   - The admin "Variables" picker in the React T&C editor.
//   - The signup client-side preview that substitutes placeholders before
//     the member sees the document.
//
// To keep the frontend list in sync run `go generate ./internal/terms/...`,
// which writes frontend/src/utils/termsVariables.generated.ts.
//
//go:generate go run ../../cmd/gen-terms-variables -out ../../frontend/src/utils/termsVariables.generated.ts
package terms

// Variable describes one placeholder available in T&C documents.
type Variable struct {
	// Key is the identifier inside double braces: e.g. "client_name" -> {{client_name}}.
	Key string
	// Label is the human-readable name shown in the admin variable picker.
	Label string
	// Desc explains what the value represents.
	Desc string
}

// All is the canonical list, ordered for display.
var All = []Variable{
	{Key: "client_name", Label: "Client Name", Desc: "Full name of the member"},
	{Key: "client_email", Label: "Client Email", Desc: "Member's email address"},
	{Key: "client_phone", Label: "Client Phone", Desc: "Member's phone number"},
	{Key: "client_address", Label: "Client Address", Desc: "Member's full address"},
	{Key: "client_dob", Label: "Date of Birth", Desc: "Member's date of birth"},
	{Key: "client_gender", Label: "Gender", Desc: "Member's gender"},
	{Key: "emergency_contact_name", Label: "Emergency Contact Name", Desc: "Member's emergency contact name"},
	{Key: "emergency_contact_phone", Label: "Emergency Contact Phone", Desc: "Member's emergency contact phone"},

	{Key: "membership_title", Label: "Membership Title", Desc: "Name of the membership type"},
	{Key: "term_months", Label: "Term (Months)", Desc: "Duration in months (e.g. 6, 12)"},
	{Key: "term_weeks", Label: "Term (Weeks)", Desc: "Duration in weeks (e.g. 26, 52)"},
	{Key: "total_fee", Label: "Total Fee", Desc: "Total contract value"},
	{Key: "payment_amount", Label: "Payment Amount", Desc: "Per-period payment amount"},
	{Key: "payment_frequency", Label: "Payment Frequency", Desc: "Weekly, fortnightly, or monthly"},
	{Key: "start_date", Label: "Start Date", Desc: "When the membership begins"},
	{Key: "end_date", Label: "End Date", Desc: "When the minimum term ends"},
	{Key: "payment_method", Label: "Payment Method", Desc: "Card, direct debit, etc."},

	{Key: "company_name", Label: "Company Name", Desc: "Your business name"},
	{Key: "company_trading_name", Label: "Trading Name", Desc: "Your business trading name"},
	{Key: "company_abn", Label: "ABN", Desc: "Australian Business Number"},
	{Key: "company_address", Label: "Company Address", Desc: "Your business address"},
	{Key: "company_email", Label: "Company Email", Desc: "Your business email"},
	{Key: "company_phone", Label: "Company Phone", Desc: "Your business phone"},

	{Key: "today_date", Label: "Today's Date", Desc: "Date the agreement is signed"},
}

// Keys returns just the placeholder keys (used by validators / tests).
func Keys() []string {
	out := make([]string, 0, len(All))
	for _, v := range All {
		out = append(out, v.Key)
	}
	return out
}
