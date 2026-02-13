package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// GoCardlessProvider implements the payment Provider interface for GoCardless.
// Per-tenant credentials are injected at construction time.
type GoCardlessProvider struct {
	apiKey      string
	apiSecret   string
	environment string // "sandbox" or "live"
}

// NewGoCardlessProvider creates a new GoCardless provider with tenant credentials.
func NewGoCardlessProvider(apiKey, apiSecret, environment string) *GoCardlessProvider {
	return &GoCardlessProvider{
		apiKey:      apiKey,
		apiSecret:   apiSecret,
		environment: environment,
	}
}

func (g *GoCardlessProvider) client() *gcClient {
	return newGCClient(g.apiKey, g.environment)
}

// splitName splits a full name into given_name and family_name (GoCardless requires both).
func splitName(name string) (given, family string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Customer", "Account"
	}
	parts := strings.Fields(name)
	if len(parts) == 1 {
		return parts[0], "Account"
	}
	return parts[0], strings.Join(parts[1:], " ")
}

// normaliseAUPhone converts Australian phone numbers to international format for GoCardless.
func normaliseAUPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return ""
	}
	// Already international
	if strings.HasPrefix(phone, "+") {
		return phone
	}
	// Australian mobile/landline starting with 0
	if strings.HasPrefix(phone, "0") && len(phone) >= 10 {
		return "+61" + phone[1:]
	}
	return phone
}

func (g *GoCardlessProvider) CreateCustomer(ctx context.Context, input CreateCustomerInput) (string, error) {
	given, family := splitName(input.Name)
	req := struct {
		Customers struct {
			Email       string            `json:"email"`
			GivenName   string            `json:"given_name"`
			FamilyName  string            `json:"family_name"`
			PhoneNumber string            `json:"phone_number,omitempty"`
			Metadata    map[string]string `json:"metadata,omitempty"`
		} `json:"customers"`
	}{}
	req.Customers.Email = input.Email
	req.Customers.GivenName = given
	req.Customers.FamilyName = family
	req.Customers.PhoneNumber = normaliseAUPhone(input.Phone)
	if input.Reference != "" {
		req.Customers.Metadata = map[string]string{"reference": input.Reference}
	}

	var res struct {
		Customers struct {
			ID string `json:"id"`
		} `json:"customers"`
	}
	if err := g.client().post(ctx, "/customers", input.Reference, req, &res); err != nil {
		return "", err
	}
	return res.Customers.ID, nil
}

func (g *GoCardlessProvider) SetupDirectDebit(ctx context.Context, input SetupDirectDebitInput) (string, error) {
	cl := g.client()
	// 1. Create customer bank account (AU: BSB = branch_code, country AU, scheme becs)
	countryCode := "AU"
	scheme := "becs" // BECS for Australia; use "bacs" for UK
	if input.BSB == "" {
		return "", fmt.Errorf("SetupDirectDebit: BSB (branch_code) required for AU")
	}
	cbaReq := struct {
		CustomerBankAccounts struct {
			AccountHolderName string `json:"account_holder_name"`
			AccountNumber     string `json:"account_number"`
			BranchCode        string `json:"branch_code"`
			CountryCode       string `json:"country_code"`
			Links             struct {
				Customer string `json:"customer"`
			} `json:"links"`
		} `json:"customer_bank_accounts"`
	}{}
	cbaReq.CustomerBankAccounts.AccountHolderName = input.AccountHolderName
	cbaReq.CustomerBankAccounts.AccountNumber = input.AccountNumber
	cbaReq.CustomerBankAccounts.BranchCode = input.BSB
	cbaReq.CustomerBankAccounts.CountryCode = countryCode
	cbaReq.CustomerBankAccounts.Links.Customer = input.CustomerID

	var cbaRes struct {
		CustomerBankAccounts struct {
			ID string `json:"id"`
		} `json:"customer_bank_accounts"`
	}
	if err := cl.post(ctx, "/customer_bank_accounts", input.Reference, cbaReq, &cbaRes); err != nil {
		return "", err
	}
	bankAccountID := cbaRes.CustomerBankAccounts.ID

	// 2. Create mandate linked to the bank account
	mandateReq := struct {
		Mandates struct {
			Links struct {
				CustomerBankAccount string `json:"customer_bank_account"`
			} `json:"links"`
			Reference string `json:"reference,omitempty"`
			Scheme    string `json:"scheme"`
		} `json:"mandates"`
	}{}
	mandateReq.Mandates.Links.CustomerBankAccount = bankAccountID
	// Don't set a custom reference — many schemes don't allow it
	mandateReq.Mandates.Scheme = scheme

	var mandateRes struct {
		Mandates struct {
			ID string `json:"id"`
		} `json:"mandates"`
	}
	if err := cl.post(ctx, "/mandates", input.Reference, mandateReq, &mandateRes); err != nil {
		return "", err
	}
	return mandateRes.Mandates.ID, nil
}

func (g *GoCardlessProvider) CreatePayment(ctx context.Context, input CreatePaymentInput) (string, error) {
	chargeDate := input.ChargeDate.Format("2006-01-02")
	req := struct {
		Payments struct {
			Links struct {
				Mandate string `json:"mandate"`
			} `json:"links"`
			Amount      int64  `json:"amount"`
			Currency    string `json:"currency"`
			Description string `json:"description,omitempty"`
			ChargeDate  string `json:"charge_date"`
			Reference   string `json:"reference,omitempty"`
		} `json:"payments"`
	}{}
	req.Payments.Links.Mandate = input.MandateID
	req.Payments.Amount = input.Amount
	req.Payments.Currency = input.Currency
	req.Payments.Description = input.Description
	req.Payments.ChargeDate = chargeDate
	req.Payments.Reference = input.Reference

	var res struct {
		Payments struct {
			ID string `json:"id"`
		} `json:"payments"`
	}
	idempotencyKey := input.Reference
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("pay-%s-%d", input.MandateID, time.Now().UnixNano())
	}
	if err := g.client().post(ctx, "/payments", idempotencyKey, req, &res); err != nil {
		return "", err
	}
	return res.Payments.ID, nil
}

func (g *GoCardlessProvider) GetPaymentStatus(ctx context.Context, paymentID string) (PaymentStatus, error) {
	var res struct {
		Payments struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Amount int64  `json:"amount"`
		} `json:"payments"`
	}
	if err := g.client().get(ctx, "/payments/"+paymentID, &res); err != nil {
		return PaymentStatus{}, err
	}
	status := mapGCPaymentStatusToOurs(res.Payments.Status)
	return PaymentStatus{
		ID:     res.Payments.ID,
		Status: status,
		Amount: res.Payments.Amount,
	}, nil
}

// mapGCPaymentStatusToOurs maps GoCardless payment status to our generic status.
func mapGCPaymentStatusToOurs(gc string) string {
	switch gc {
	case "pending_submission", "submitted", "confirmed":
		return gc
	case "paid_out":
		return "paid_out"
	case "failed", "charged_back", "cancelled":
		return gc
	default:
		return gc
	}
}

func (g *GoCardlessProvider) CancelMandate(ctx context.Context, mandateID string) error {
	path := "/mandates/" + mandateID + "/actions/cancel"
	body := struct{}{}
	return g.client().do(ctx, "POST", path, "", body, nil)
}

// HandleWebhook parses the GoCardless webhook payload and optionally verifies the signature.
// If webhookSecret is non-empty and signature is provided, HMAC-SHA256 is verified.
// Returns the first event in the batch as WebhookEvent; callers can loop over the raw payload for multiple events.
func (g *GoCardlessProvider) HandleWebhook(ctx context.Context, payload []byte, signature string) (*WebhookEvent, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("empty webhook payload")
	}
	if g.apiSecret != "" && signature != "" {
		mac := hmac.New(sha256.New, []byte(g.apiSecret))
		mac.Write(payload)
		expected := hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(signature), []byte(expected)) {
			return nil, fmt.Errorf("webhook signature mismatch")
		}
	}

	var envelope struct {
		Events []struct {
			ID           string `json:"id"`
			ResourceType string `json:"resource_type"`
			Action       string `json:"action"`
			Links        struct {
				Payment   string `json:"payment,omitempty"`
				Mandate   string `json:"mandate,omitempty"`
				Organisation string `json:"organisation,omitempty"`
			} `json:"links"`
			Details struct {
				Origin string `json:"origin,omitempty"`
				Cause  string `json:"cause,omitempty"`
				Reason string `json:"reason,omitempty"`
			} `json:"details"`
		} `json:"events"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, fmt.Errorf("webhook parse: %w", err)
	}
	if len(envelope.Events) == 0 {
		return nil, nil
	}

	ev := envelope.Events[0]
	eventType := ev.ResourceType + "." + ev.Action
	resourceID := ""
	switch ev.ResourceType {
	case "payments":
		resourceID = ev.Links.Payment
	case "mandates":
		resourceID = ev.Links.Mandate
	default:
		resourceID = ev.ID
	}
	details := make(map[string]interface{})
	details["event_id"] = ev.ID
	if ev.Details.Origin != "" {
		details["origin"] = ev.Details.Origin
	}
	if ev.Details.Cause != "" {
		details["cause"] = ev.Details.Cause
	}
	if ev.Details.Reason != "" {
		details["reason"] = ev.Details.Reason
	}
	return &WebhookEvent{
		Type:       eventType,
		ResourceID: resourceID,
		Action:     ev.Action,
		Details:    details,
	}, nil
}

// --- Phase B: Payment lifecycle ---

func (g *GoCardlessProvider) RetryPayment(ctx context.Context, paymentID string) error {
	return g.client().do(ctx, "POST", "/payments/"+paymentID+"/actions/retry", "", struct{}{}, nil)
}

func (g *GoCardlessProvider) CancelPayment(ctx context.Context, paymentID string) error {
	return g.client().do(ctx, "POST", "/payments/"+paymentID+"/actions/cancel", "", struct{}{}, nil)
}

func (g *GoCardlessProvider) CreateRefund(ctx context.Context, paymentID string, amountMinor int64, reference string) (string, error) {
	req := struct {
		Refunds struct {
			Links struct {
				Payment string `json:"payment"`
			} `json:"links"`
			Amount    int64  `json:"amount"`
			Reference string `json:"reference,omitempty"`
		} `json:"refunds"`
	}{}
	req.Refunds.Links.Payment = paymentID
	req.Refunds.Amount = amountMinor
	req.Refunds.Reference = reference
	var res struct {
		Refunds struct {
			ID string `json:"id"`
		} `json:"refunds"`
	}
	if err := g.client().post(ctx, "/refunds", reference, req, &res); err != nil {
		return "", err
	}
	return res.Refunds.ID, nil
}

func (g *GoCardlessProvider) GetRefund(ctx context.Context, refundID string) (RefundInfo, error) {
	var res struct {
		Refunds struct {
			ID     string `json:"id"`
			Amount int64  `json:"amount"`
			Status string `json:"status"`
			Reference string `json:"reference"`
			Links   struct {
				Payment string `json:"payment"`
			} `json:"links"`
		} `json:"refunds"`
	}
	if err := g.client().get(ctx, "/refunds/"+refundID, &res); err != nil {
		return RefundInfo{}, err
	}
	return RefundInfo{
		ID:        res.Refunds.ID,
		PaymentID: res.Refunds.Links.Payment,
		Amount:    res.Refunds.Amount,
		Status:    res.Refunds.Status,
		Reference: res.Refunds.Reference,
	}, nil
}

// --- Phase C: Mandate and lists ---

func (g *GoCardlessProvider) ReinstateMandate(ctx context.Context, mandateID string) error {
	return g.client().do(ctx, "POST", "/mandates/"+mandateID+"/actions/reinstate", "", struct{}{}, nil)
}

func (g *GoCardlessProvider) GetMandate(ctx context.Context, mandateID string) (MandateInfo, error) {
	var res struct {
		Mandates struct {
			ID        string `json:"id"`
			Status    string `json:"status"`
			Scheme    string `json:"scheme"`
			Reference string `json:"reference"`
			Links     struct {
				Customer string `json:"customer"`
			} `json:"links"`
		} `json:"mandates"`
	}
	if err := g.client().get(ctx, "/mandates/"+mandateID, &res); err != nil {
		return MandateInfo{}, err
	}
	return MandateInfo{
		ID:         res.Mandates.ID,
		Status:     res.Mandates.Status,
		Scheme:     res.Mandates.Scheme,
		Reference:  res.Mandates.Reference,
		CustomerID: res.Mandates.Links.Customer,
	}, nil
}

func (g *GoCardlessProvider) ListMandates(ctx context.Context, customerID string) ([]MandateSummary, error) {
	params := make(url.Values)
	if customerID != "" {
		params.Set("customer", customerID)
	}
	params.Set("limit", "100")
	var res struct {
		Mandates []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"mandates"`
	}
	if err := g.client().getWithParams(ctx, "/mandates", params, &res); err != nil {
		return nil, err
	}
	out := make([]MandateSummary, len(res.Mandates))
	for i, m := range res.Mandates {
		out[i] = MandateSummary{ID: m.ID, Status: m.Status}
	}
	return out, nil
}

func (g *GoCardlessProvider) ListCustomers(ctx context.Context, referencePrefix string) ([]CustomerSummary, error) {
	params := make(url.Values)
	if referencePrefix != "" {
		params.Set("metadata[reference]", referencePrefix)
	}
	params.Set("limit", "100")
	var res struct {
		Customers []struct {
			ID        string `json:"id"`
			Email     string `json:"email"`
			GivenName string `json:"given_name"`
			FamilyName string `json:"family_name"`
		} `json:"customers"`
	}
	if err := g.client().getWithParams(ctx, "/customers", params, &res); err != nil {
		return nil, err
	}
	out := make([]CustomerSummary, len(res.Customers))
	for i, c := range res.Customers {
		out[i] = CustomerSummary{
			ID:         c.ID,
			Email:      c.Email,
			GivenName:  c.GivenName,
			FamilyName: c.FamilyName,
		}
	}
	return out, nil
}

func (g *GoCardlessProvider) ListPayments(ctx context.Context, mandateID, customerID, status string, limit int) ([]PaymentSummary, error) {
	params := make(url.Values)
	if mandateID != "" {
		params.Set("mandate", mandateID)
	}
	if customerID != "" {
		params.Set("customer", customerID)
	}
	if status != "" {
		params.Set("status", status)
	}
	if limit <= 0 {
		limit = 100
	}
	params.Set("limit", fmt.Sprintf("%d", limit))
	var res struct {
		Payments []struct {
			ID         string `json:"id"`
			Status     string `json:"status"`
			Amount     int64  `json:"amount"`
			ChargeDate string `json:"charge_date"`
			Reference  string `json:"reference"`
		} `json:"payments"`
	}
	if err := g.client().getWithParams(ctx, "/payments", params, &res); err != nil {
		return nil, err
	}
	out := make([]PaymentSummary, len(res.Payments))
	for i, p := range res.Payments {
		out[i] = PaymentSummary{
			ID:         p.ID,
			Status:     p.Status,
			Amount:     p.Amount,
			ChargeDate: p.ChargeDate,
			Reference:  p.Reference,
		}
	}
	return out, nil
}

// --- Phase D: Redirect flows (hosted mandate setup) ---

func (g *GoCardlessProvider) CreateRedirectFlow(ctx context.Context, redirectURI, description, sessionToken string) (redirectURL, flowID string, err error) {
	req := struct {
		RedirectFlows struct {
			SessionToken       string `json:"session_token"`
			SuccessRedirectURL string `json:"success_redirect_url"`
			Description        string `json:"description,omitempty"`
		} `json:"redirect_flows"`
	}{}
	req.RedirectFlows.SessionToken = sessionToken
	req.RedirectFlows.SuccessRedirectURL = redirectURI
	req.RedirectFlows.Description = description
	var res struct {
		RedirectFlows struct {
			ID          string `json:"id"`
			RedirectURL string `json:"redirect_url"`
		} `json:"redirect_flows"`
	}
	if err := g.client().post(ctx, "/redirect_flows", "", req, &res); err != nil {
		return "", "", err
	}
	return res.RedirectFlows.RedirectURL, res.RedirectFlows.ID, nil
}

func (g *GoCardlessProvider) CompleteRedirectFlow(ctx context.Context, flowID, sessionToken string) (mandateID, customerID string, err error) {
	req := struct {
		Data struct {
			SessionToken string `json:"session_token"`
		} `json:"data"`
	}{}
	req.Data.SessionToken = sessionToken
	var res struct {
		RedirectFlows struct {
			Links struct {
				Mandate  string `json:"mandate"`
				Customer string `json:"customer"`
			} `json:"links"`
		} `json:"redirect_flows"`
	}
	if err := g.client().do(ctx, "POST", "/redirect_flows/"+flowID+"/actions/complete", "", req, &res); err != nil {
		return "", "", err
	}
	return res.RedirectFlows.Links.Mandate, res.RedirectFlows.Links.Customer, nil
}

// --- Phase E: Subscriptions ---

func (g *GoCardlessProvider) CreateSubscription(ctx context.Context, mandateID string, amountMinor int64, currency, interval string, intervalCount int, name, reference string, endDate *time.Time) (string, error) {
	req := struct {
		Subscriptions struct {
			Links struct {
				Mandate string `json:"mandate"`
			} `json:"links"`
			Amount       int64  `json:"amount"`
			Currency     string `json:"currency"`
			IntervalUnit string `json:"interval_unit"`
			Interval     int    `json:"interval"`
			Name         string `json:"name,omitempty"`
			EndDate      string `json:"end_date,omitempty"`
		} `json:"subscriptions"`
	}{}
	req.Subscriptions.Links.Mandate = mandateID
	req.Subscriptions.Amount = amountMinor
	req.Subscriptions.Currency = currency
	req.Subscriptions.IntervalUnit = interval // "monthly", "weekly", "yearly"
	req.Subscriptions.Interval = intervalCount
	req.Subscriptions.Name = name
	if endDate != nil {
		req.Subscriptions.EndDate = endDate.Format("2006-01-02")
	}
	var res struct {
		Subscriptions struct {
			ID string `json:"id"`
		} `json:"subscriptions"`
	}
	if err := g.client().post(ctx, "/subscriptions", reference, req, &res); err != nil {
		return "", err
	}
	return res.Subscriptions.ID, nil
}

func (g *GoCardlessProvider) GetSubscription(ctx context.Context, subscriptionID string) (SubscriptionInfo, error) {
	var res struct {
		Subscriptions struct {
			ID               string `json:"id"`
			Status           string `json:"status"`
			Amount           int64  `json:"amount"`
			Currency         string `json:"currency"`
			IntervalUnit     string `json:"interval_unit"`
			IntervalLength   int    `json:"interval_length"`
			Name             string `json:"name"`
			NextChargeDate   string `json:"next_charge_date"`
		} `json:"subscriptions"`
	}
	if err := g.client().get(ctx, "/subscriptions/"+subscriptionID, &res); err != nil {
		return SubscriptionInfo{}, err
	}
	return SubscriptionInfo{
		ID:              res.Subscriptions.ID,
		Status:          res.Subscriptions.Status,
		Amount:          res.Subscriptions.Amount,
		Currency:        res.Subscriptions.Currency,
		Interval:        res.Subscriptions.IntervalUnit,
		IntervalCount:   res.Subscriptions.IntervalLength,
		NextChargeDate:  res.Subscriptions.NextChargeDate,
		Name:            res.Subscriptions.Name,
	}, nil
}

func (g *GoCardlessProvider) UpdateSubscription(ctx context.Context, subscriptionID string, amountMinor *int64, name string, endDate *time.Time) error {
	req := struct {
		Subscriptions struct {
			Amount   *int64 `json:"amount,omitempty"`
			Name     string `json:"name,omitempty"`
			EndDate  string `json:"end_date,omitempty"`
		} `json:"subscriptions"`
	}{}
	req.Subscriptions.Amount = amountMinor
	req.Subscriptions.Name = name
	if endDate != nil {
		req.Subscriptions.EndDate = endDate.Format("2006-01-02")
	}
	return g.client().do(ctx, "PUT", "/subscriptions/"+subscriptionID, "", req, nil)
}

func (g *GoCardlessProvider) CancelSubscription(ctx context.Context, subscriptionID string) error {
	return g.client().do(ctx, "POST", "/subscriptions/"+subscriptionID+"/actions/cancel", "", struct{}{}, nil)
}

func (g *GoCardlessProvider) PauseSubscription(ctx context.Context, subscriptionID string) error {
	return g.client().do(ctx, "POST", "/subscriptions/"+subscriptionID+"/actions/pause", "", struct{}{}, nil)
}

func (g *GoCardlessProvider) ResumeSubscription(ctx context.Context, subscriptionID string) error {
	return g.client().do(ctx, "POST", "/subscriptions/"+subscriptionID+"/actions/resume", "", struct{}{}, nil)
}

func (g *GoCardlessProvider) ListSubscriptions(ctx context.Context, customerID, mandateID, status string, limit int) ([]SubscriptionSummary, error) {
	params := make(url.Values)
	if customerID != "" {
		params.Set("customer", customerID)
	}
	if mandateID != "" {
		params.Set("mandate", mandateID)
	}
	if status != "" {
		params.Set("status", status)
	}
	if limit <= 0 {
		limit = 100
	}
	params.Set("limit", fmt.Sprintf("%d", limit))
	var res struct {
		Subscriptions []struct {
			ID             string `json:"id"`
			Status         string `json:"status"`
			Amount         int64  `json:"amount"`
			NextChargeDate string `json:"next_charge_date"`
		} `json:"subscriptions"`
	}
	if err := g.client().getWithParams(ctx, "/subscriptions", params, &res); err != nil {
		return nil, err
	}
	out := make([]SubscriptionSummary, len(res.Subscriptions))
	for i, s := range res.Subscriptions {
		out[i] = SubscriptionSummary{
			ID:             s.ID,
			Status:         s.Status,
			Amount:         s.Amount,
			NextChargeDate: s.NextChargeDate,
		}
	}
	return out, nil
}

func (g *GoCardlessProvider) ValidateBankDetails(ctx context.Context, countryCode, accountNumber, branchCode, accountHolderName string) (BankDetailsResult, error) {
	req := struct {
		BankDetailsLookups struct {
			AccountNumber     string `json:"account_number"`
			BranchCode        string `json:"branch_code"`
			CountryCode       string `json:"country_code"`
			AccountHolderName string `json:"account_holder_name"`
		} `json:"bank_details_lookups"`
	}{}
	req.BankDetailsLookups.AccountNumber = accountNumber
	req.BankDetailsLookups.BranchCode = branchCode
	req.BankDetailsLookups.CountryCode = countryCode
	req.BankDetailsLookups.AccountHolderName = accountHolderName
	var res struct {
		BankDetailsLookups struct {
			BankName      string `json:"bank_name"`
			AccountHolder string `json:"account_holder_name"`
		} `json:"bank_details_lookups"`
	}
	if err := g.client().post(ctx, "/bank_details_lookups", "", req, &res); err != nil {
		return BankDetailsResult{Valid: false}, err
	}
	return BankDetailsResult{
		Valid:         true,
		BankName:      res.BankDetailsLookups.BankName,
		AccountHolder:  res.BankDetailsLookups.AccountHolder,
	}, nil
}

// --- Section 13: Instalment schedules, Payouts, Blocks, Mandate PDF, Customer/bank ---

func (g *GoCardlessProvider) CreateInstalmentSchedule(ctx context.Context, mandateID string, totalAmountMinor int64, currency string, instalments []InstalmentDate, name, reference string) (string, error) {
	// GoCardless "create with dates" expects instalments array with charge_date and amount per instalment
	req := struct {
		InstalmentSchedules struct {
			Links     struct { Mandate string `json:"mandate"` } `json:"links"`
			Currency string   `json:"currency"`
			Name     string   `json:"name,omitempty"`
			Reference string  `json:"reference,omitempty"`
			Instalments []struct {
				ChargeDate string `json:"charge_date"`
				Amount    int64  `json:"amount"`
			} `json:"instalments"`
		} `json:"instalment_schedules"`
	}{}
	req.InstalmentSchedules.Links.Mandate = mandateID
	req.InstalmentSchedules.Currency = currency
	req.InstalmentSchedules.Name = name
	req.InstalmentSchedules.Reference = reference
	req.InstalmentSchedules.Instalments = make([]struct {
		ChargeDate string `json:"charge_date"`
		Amount    int64  `json:"amount"`
	}, len(instalments))
	for i, in := range instalments {
		req.InstalmentSchedules.Instalments[i].ChargeDate = in.ChargeDate
		req.InstalmentSchedules.Instalments[i].Amount = in.Amount
	}
	var res struct {
		InstalmentSchedules struct {
			ID string `json:"id"`
		} `json:"instalment_schedules"`
	}
	if err := g.client().post(ctx, "/instalment_schedules", reference, req, &res); err != nil {
		return "", err
	}
	return res.InstalmentSchedules.ID, nil
}

func (g *GoCardlessProvider) ListPayouts(ctx context.Context, since, until time.Time) ([]PayoutSummary, error) {
	params := make(url.Values)
	if !since.IsZero() {
		params.Set("created_at[gte]", since.Format(time.RFC3339))
	}
	if !until.IsZero() {
		params.Set("created_at[lte]", until.Format(time.RFC3339))
	}
	params.Set("limit", "100")
	var res struct {
		Payouts []struct {
			ID          string `json:"id"`
			Amount      int64  `json:"amount"`
			Currency    string `json:"currency"`
			ArrivalDate string `json:"arrival_date"`
			Status      string `json:"status"`
		} `json:"payouts"`
	}
	if err := g.client().getWithParams(ctx, "/payouts", params, &res); err != nil {
		return nil, err
	}
	out := make([]PayoutSummary, len(res.Payouts))
	for i, p := range res.Payouts {
		out[i] = PayoutSummary{ID: p.ID, Amount: p.Amount, Currency: p.Currency, ArrivalDate: p.ArrivalDate, Status: p.Status}
	}
	return out, nil
}

func (g *GoCardlessProvider) GetPayout(ctx context.Context, payoutID string) (PayoutInfo, error) {
	var res struct {
		Payouts struct {
			ID          string `json:"id"`
			Amount      int64  `json:"amount"`
			Currency    string `json:"currency"`
			ArrivalDate string `json:"arrival_date"`
			Status      string `json:"status"`
		} `json:"payouts"`
	}
	if err := g.client().get(ctx, "/payouts/"+payoutID, &res); err != nil {
		return PayoutInfo{}, err
	}
	return PayoutInfo{
		ID:          res.Payouts.ID,
		Amount:      res.Payouts.Amount,
		Currency:    res.Payouts.Currency,
		ArrivalDate: res.Payouts.ArrivalDate,
		Status:      res.Payouts.Status,
	}, nil
}

func (g *GoCardlessProvider) CreateBlock(ctx context.Context, customerID, reason string) (string, error) {
	req := struct {
		Blocks struct {
			Links  struct { Customer string `json:"customer"` } `json:"links"`
			Reason string `json:"reason"`
		} `json:"blocks"`
	}{}
	req.Blocks.Links.Customer = customerID
	req.Blocks.Reason = reason
	var res struct {
		Blocks struct {
			ID string `json:"id"`
		} `json:"blocks"`
	}
	if err := g.client().post(ctx, "/blocks", "", req, &res); err != nil {
		return "", err
	}
	return res.Blocks.ID, nil
}

func (g *GoCardlessProvider) DisableBlock(ctx context.Context, blockID string) error {
	return g.client().do(ctx, "POST", "/blocks/"+blockID+"/actions/disable", "", struct{}{}, nil)
}

func (g *GoCardlessProvider) CreateMandatePDF(ctx context.Context, mandateID string) (pdfURL string, err error) {
	req := struct {
		MandatePdfs struct {
			Links struct { Mandate string `json:"mandate"` } `json:"links"`
		} `json:"mandate_pdfs"`
	}{}
	req.MandatePdfs.Links.Mandate = mandateID
	var res struct {
		MandatePdfs struct {
			URL string `json:"url"`
		} `json:"mandate_pdfs"`
	}
	if err := g.client().post(ctx, "/mandate_pdfs", "", req, &res); err != nil {
		return "", err
	}
	return res.MandatePdfs.URL, nil
}

func (g *GoCardlessProvider) UpdateCustomer(ctx context.Context, customerID, email, name, phone string) error {
	given, family := splitName(name)
	req := struct {
		Customers struct {
			Email     string `json:"email,omitempty"`
			GivenName string `json:"given_name,omitempty"`
			FamilyName string `json:"family_name,omitempty"`
			Phone     string `json:"phone,omitempty"`
		} `json:"customers"`
	}{}
	req.Customers.Email = email
	req.Customers.GivenName = given
	req.Customers.FamilyName = family
	req.Customers.Phone = phone
	return g.client().do(ctx, "PUT", "/customers/"+customerID, "", req, nil)
}

func (g *GoCardlessProvider) RemoveCustomer(ctx context.Context, customerID string) error {
	return g.client().do(ctx, "POST", "/customers/"+customerID+"/actions/remove", "", struct{}{}, nil)
}

func (g *GoCardlessProvider) DisableCustomerBankAccount(ctx context.Context, bankAccountID string) error {
	return g.client().do(ctx, "POST", "/customer_bank_accounts/"+bankAccountID+"/actions/disable", "", struct{}{}, nil)
}

// --- Missing CRUD for existing resources ---

func (g *GoCardlessProvider) GetPayment(ctx context.Context, paymentID string) (PaymentDetail, error) {
	var res struct {
		Payments struct {
			ID          string            `json:"id"`
			Status      string            `json:"status"`
			Amount      int64             `json:"amount"`
			Currency    string            `json:"currency"`
			ChargeDate  string            `json:"charge_date"`
			Description string            `json:"description"`
			Reference   string            `json:"reference"`
			Metadata    map[string]string `json:"metadata"`
			Links       struct {
				Mandate string `json:"mandate"`
			} `json:"links"`
		} `json:"payments"`
	}
	if err := g.client().get(ctx, "/payments/"+paymentID, &res); err != nil {
		return PaymentDetail{}, err
	}
	return PaymentDetail{
		ID:          res.Payments.ID,
		Status:      res.Payments.Status,
		Amount:      res.Payments.Amount,
		Currency:    res.Payments.Currency,
		ChargeDate:  res.Payments.ChargeDate,
		Description: res.Payments.Description,
		Reference:   res.Payments.Reference,
		MandateID:   res.Payments.Links.Mandate,
		Metadata:    res.Payments.Metadata,
	}, nil
}

func (g *GoCardlessProvider) UpdatePayment(ctx context.Context, paymentID string, metadata map[string]string) error {
	req := struct {
		Payments struct {
			Metadata map[string]string `json:"metadata"`
		} `json:"payments"`
	}{}
	req.Payments.Metadata = metadata
	return g.client().do(ctx, "PUT", "/payments/"+paymentID, "", req, nil)
}

func (g *GoCardlessProvider) UpdateMandate(ctx context.Context, mandateID string, metadata map[string]string) error {
	req := struct {
		Mandates struct {
			Metadata map[string]string `json:"metadata"`
		} `json:"mandates"`
	}{}
	req.Mandates.Metadata = metadata
	return g.client().do(ctx, "PUT", "/mandates/"+mandateID, "", req, nil)
}

func (g *GoCardlessProvider) GetCustomer(ctx context.Context, customerID string) (CustomerDetail, error) {
	var res struct {
		Customers struct {
			ID         string            `json:"id"`
			Email      string            `json:"email"`
			GivenName  string            `json:"given_name"`
			FamilyName string            `json:"family_name"`
			Phone      string            `json:"phone_number"`
			Metadata   map[string]string `json:"metadata"`
		} `json:"customers"`
	}
	if err := g.client().get(ctx, "/customers/"+customerID, &res); err != nil {
		return CustomerDetail{}, err
	}
	return CustomerDetail{
		ID:         res.Customers.ID,
		Email:      res.Customers.Email,
		GivenName:  res.Customers.GivenName,
		FamilyName: res.Customers.FamilyName,
		Phone:      res.Customers.Phone,
		Metadata:   res.Customers.Metadata,
	}, nil
}

func (g *GoCardlessProvider) ListRefunds(ctx context.Context, paymentID string, limit int) ([]RefundSummary, error) {
	params := make(url.Values)
	if paymentID != "" {
		params.Set("payment", paymentID)
	}
	if limit <= 0 {
		limit = 100
	}
	params.Set("limit", fmt.Sprintf("%d", limit))
	var res struct {
		Refunds []struct {
			ID     string `json:"id"`
			Amount int64  `json:"amount"`
			Status string `json:"status"`
			Links  struct {
				Payment string `json:"payment"`
			} `json:"links"`
		} `json:"refunds"`
	}
	if err := g.client().getWithParams(ctx, "/refunds", params, &res); err != nil {
		return nil, err
	}
	out := make([]RefundSummary, len(res.Refunds))
	for i, r := range res.Refunds {
		out[i] = RefundSummary{ID: r.ID, PaymentID: r.Links.Payment, Amount: r.Amount, Status: r.Status}
	}
	return out, nil
}

func (g *GoCardlessProvider) UpdateRefund(ctx context.Context, refundID string, metadata map[string]string) error {
	req := struct {
		Refunds struct {
			Metadata map[string]string `json:"metadata"`
		} `json:"refunds"`
	}{}
	req.Refunds.Metadata = metadata
	return g.client().do(ctx, "PUT", "/refunds/"+refundID, "", req, nil)
}

func (g *GoCardlessProvider) GetRedirectFlow(ctx context.Context, flowID string) (RedirectFlowInfo, error) {
	var res struct {
		RedirectFlows struct {
			ID          string `json:"id"`
			RedirectURL string `json:"redirect_url"`
			Scheme      string `json:"scheme"`
			Links       struct {
				Mandate  string `json:"mandate"`
				Customer string `json:"customer"`
			} `json:"links"`
		} `json:"redirect_flows"`
	}
	if err := g.client().get(ctx, "/redirect_flows/"+flowID, &res); err != nil {
		return RedirectFlowInfo{}, err
	}
	return RedirectFlowInfo{
		ID:          res.RedirectFlows.ID,
		RedirectURL: res.RedirectFlows.RedirectURL,
		Scheme:      res.RedirectFlows.Scheme,
		MandateID:   res.RedirectFlows.Links.Mandate,
		CustomerID:  res.RedirectFlows.Links.Customer,
	}, nil
}

// --- Customer Bank Account CRUD ---

func (g *GoCardlessProvider) CreateCustomerBankAccount(ctx context.Context, customerID, accountHolderName, accountNumber, branchCode, countryCode string) (string, error) {
	req := struct {
		CustomerBankAccounts struct {
			AccountHolderName string `json:"account_holder_name"`
			AccountNumber     string `json:"account_number"`
			BranchCode        string `json:"branch_code"`
			CountryCode       string `json:"country_code"`
			Links             struct {
				Customer string `json:"customer"`
			} `json:"links"`
		} `json:"customer_bank_accounts"`
	}{}
	req.CustomerBankAccounts.AccountHolderName = accountHolderName
	req.CustomerBankAccounts.AccountNumber = accountNumber
	req.CustomerBankAccounts.BranchCode = branchCode
	req.CustomerBankAccounts.CountryCode = countryCode
	req.CustomerBankAccounts.Links.Customer = customerID
	var res struct {
		CustomerBankAccounts struct {
			ID string `json:"id"`
		} `json:"customer_bank_accounts"`
	}
	if err := g.client().post(ctx, "/customer_bank_accounts", "", req, &res); err != nil {
		return "", err
	}
	return res.CustomerBankAccounts.ID, nil
}

func (g *GoCardlessProvider) GetCustomerBankAccount(ctx context.Context, bankAccountID string) (BankAccountInfo, error) {
	var res struct {
		CustomerBankAccounts struct {
			ID                string `json:"id"`
			AccountHolderName string `json:"account_holder_name"`
			AccountNumberEnd  string `json:"account_number_ending"`
			BankName          string `json:"bank_name"`
			CountryCode       string `json:"country_code"`
			Currency          string `json:"currency"`
			Enabled           bool   `json:"enabled"`
		} `json:"customer_bank_accounts"`
	}
	if err := g.client().get(ctx, "/customer_bank_accounts/"+bankAccountID, &res); err != nil {
		return BankAccountInfo{}, err
	}
	return BankAccountInfo{
		ID:                res.CustomerBankAccounts.ID,
		AccountHolderName: res.CustomerBankAccounts.AccountHolderName,
		AccountNumberEnd:  res.CustomerBankAccounts.AccountNumberEnd,
		BankName:          res.CustomerBankAccounts.BankName,
		CountryCode:       res.CustomerBankAccounts.CountryCode,
		Currency:          res.CustomerBankAccounts.Currency,
		Enabled:           res.CustomerBankAccounts.Enabled,
	}, nil
}

func (g *GoCardlessProvider) ListCustomerBankAccounts(ctx context.Context, customerID string) ([]BankAccountSummary, error) {
	params := make(url.Values)
	if customerID != "" {
		params.Set("customer", customerID)
	}
	params.Set("limit", "100")
	var res struct {
		CustomerBankAccounts []struct {
			ID                string `json:"id"`
			AccountHolderName string `json:"account_holder_name"`
			AccountNumberEnd  string `json:"account_number_ending"`
			BankName          string `json:"bank_name"`
			Enabled           bool   `json:"enabled"`
		} `json:"customer_bank_accounts"`
	}
	if err := g.client().getWithParams(ctx, "/customer_bank_accounts", params, &res); err != nil {
		return nil, err
	}
	out := make([]BankAccountSummary, len(res.CustomerBankAccounts))
	for i, a := range res.CustomerBankAccounts {
		out[i] = BankAccountSummary{
			ID: a.ID, AccountHolderName: a.AccountHolderName,
			AccountNumberEnd: a.AccountNumberEnd, BankName: a.BankName, Enabled: a.Enabled,
		}
	}
	return out, nil
}

func (g *GoCardlessProvider) UpdateCustomerBankAccount(ctx context.Context, bankAccountID string, metadata map[string]string) error {
	req := struct {
		CustomerBankAccounts struct {
			Metadata map[string]string `json:"metadata"`
		} `json:"customer_bank_accounts"`
	}{}
	req.CustomerBankAccounts.Metadata = metadata
	return g.client().do(ctx, "PUT", "/customer_bank_accounts/"+bankAccountID, "", req, nil)
}

// --- Instalment Schedule CRUD ---

func (g *GoCardlessProvider) GetInstalmentSchedule(ctx context.Context, scheduleID string) (InstalmentScheduleInfo, error) {
	var res struct {
		InstalmentSchedules struct {
			ID       string `json:"id"`
			Status   string `json:"status"`
			Currency string `json:"currency"`
			Name     string `json:"name"`
			Links    struct {
				Mandate string `json:"mandate"`
			} `json:"links"`
		} `json:"instalment_schedules"`
	}
	if err := g.client().get(ctx, "/instalment_schedules/"+scheduleID, &res); err != nil {
		return InstalmentScheduleInfo{}, err
	}
	return InstalmentScheduleInfo{
		ID: res.InstalmentSchedules.ID, Status: res.InstalmentSchedules.Status,
		Currency: res.InstalmentSchedules.Currency, Name: res.InstalmentSchedules.Name,
		MandateID: res.InstalmentSchedules.Links.Mandate,
	}, nil
}

func (g *GoCardlessProvider) ListInstalmentSchedules(ctx context.Context, mandateID string, limit int) ([]InstalmentScheduleSummary, error) {
	params := make(url.Values)
	if mandateID != "" {
		params.Set("mandate", mandateID)
	}
	if limit <= 0 {
		limit = 100
	}
	params.Set("limit", fmt.Sprintf("%d", limit))
	var res struct {
		InstalmentSchedules []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Name   string `json:"name"`
		} `json:"instalment_schedules"`
	}
	if err := g.client().getWithParams(ctx, "/instalment_schedules", params, &res); err != nil {
		return nil, err
	}
	out := make([]InstalmentScheduleSummary, len(res.InstalmentSchedules))
	for i, s := range res.InstalmentSchedules {
		out[i] = InstalmentScheduleSummary{ID: s.ID, Status: s.Status, Name: s.Name}
	}
	return out, nil
}

func (g *GoCardlessProvider) UpdateInstalmentSchedule(ctx context.Context, scheduleID string, metadata map[string]string) error {
	req := struct {
		InstalmentSchedules struct {
			Metadata map[string]string `json:"metadata"`
		} `json:"instalment_schedules"`
	}{}
	req.InstalmentSchedules.Metadata = metadata
	return g.client().do(ctx, "PUT", "/instalment_schedules/"+scheduleID, "", req, nil)
}

func (g *GoCardlessProvider) CancelInstalmentSchedule(ctx context.Context, scheduleID string) error {
	return g.client().do(ctx, "POST", "/instalment_schedules/"+scheduleID+"/actions/cancel", "", struct{}{}, nil)
}

// --- Payouts extended ---

func (g *GoCardlessProvider) UpdatePayout(ctx context.Context, payoutID string, reference string) error {
	req := struct {
		Payouts struct {
			Reference string `json:"reference"`
		} `json:"payouts"`
	}{}
	req.Payouts.Reference = reference
	return g.client().do(ctx, "PUT", "/payouts/"+payoutID, "", req, nil)
}

func (g *GoCardlessProvider) ListPayoutItems(ctx context.Context, payoutID string) ([]PayoutItem, error) {
	params := make(url.Values)
	params.Set("payout", payoutID)
	params.Set("limit", "100")
	var res struct {
		PayoutItems []struct {
			Type   string `json:"type"`
			Amount int64  `json:"amount"`
			Links  struct {
				Payment string `json:"payment,omitempty"`
				Mandate string `json:"mandate,omitempty"`
				Refund  string `json:"refund,omitempty"`
			} `json:"links"`
		} `json:"payout_items"`
	}
	if err := g.client().getWithParams(ctx, "/payout_items", params, &res); err != nil {
		return nil, err
	}
	out := make([]PayoutItem, len(res.PayoutItems))
	for i, p := range res.PayoutItems {
		links := map[string]string{}
		if p.Links.Payment != "" {
			links["payment"] = p.Links.Payment
		}
		if p.Links.Mandate != "" {
			links["mandate"] = p.Links.Mandate
		}
		if p.Links.Refund != "" {
			links["refund"] = p.Links.Refund
		}
		out[i] = PayoutItem{Type: p.Type, Amount: p.Amount, Links: links}
	}
	return out, nil
}

// --- Blocks extended ---

func (g *GoCardlessProvider) GetBlock(ctx context.Context, blockID string) (BlockInfo, error) {
	var res struct {
		Blocks struct {
			ID        string `json:"id"`
			BlockType string `json:"block_type"`
			Reason    string `json:"reason_description"`
			Active    bool   `json:"active"`
			Links     struct {
				Customer string `json:"customer"`
			} `json:"links"`
		} `json:"blocks"`
	}
	if err := g.client().get(ctx, "/blocks/"+blockID, &res); err != nil {
		return BlockInfo{}, err
	}
	return BlockInfo{
		ID: res.Blocks.ID, BlockType: res.Blocks.BlockType,
		Reason: res.Blocks.Reason, CustomerID: res.Blocks.Links.Customer,
		Active: res.Blocks.Active,
	}, nil
}

func (g *GoCardlessProvider) ListBlocks(ctx context.Context, customerID string) ([]BlockSummary, error) {
	params := make(url.Values)
	if customerID != "" {
		params.Set("block[customer]", customerID)
	}
	params.Set("limit", "100")
	var res struct {
		Blocks []struct {
			ID        string `json:"id"`
			BlockType string `json:"block_type"`
			Reason    string `json:"reason_description"`
			Active    bool   `json:"active"`
		} `json:"blocks"`
	}
	if err := g.client().getWithParams(ctx, "/blocks", params, &res); err != nil {
		return nil, err
	}
	out := make([]BlockSummary, len(res.Blocks))
	for i, b := range res.Blocks {
		out[i] = BlockSummary{ID: b.ID, BlockType: b.BlockType, Reason: b.Reason, Active: b.Active}
	}
	return out, nil
}

func (g *GoCardlessProvider) EnableBlock(ctx context.Context, blockID string) error {
	return g.client().do(ctx, "POST", "/blocks/"+blockID+"/actions/enable", "", struct{}{}, nil)
}

// --- Events ---

func (g *GoCardlessProvider) ListEvents(ctx context.Context, resourceType, action string, limit int) ([]EventSummary, error) {
	params := make(url.Values)
	if resourceType != "" {
		params.Set("resource_type", resourceType)
	}
	if action != "" {
		params.Set("action", action)
	}
	if limit <= 0 {
		limit = 100
	}
	params.Set("limit", fmt.Sprintf("%d", limit))
	var res struct {
		Events []struct {
			ID           string `json:"id"`
			ResourceType string `json:"resource_type"`
			Action       string `json:"action"`
			CreatedAt    string `json:"created_at"`
		} `json:"events"`
	}
	if err := g.client().getWithParams(ctx, "/events", params, &res); err != nil {
		return nil, err
	}
	out := make([]EventSummary, len(res.Events))
	for i, e := range res.Events {
		out[i] = EventSummary{ID: e.ID, ResourceType: e.ResourceType, Action: e.Action, CreatedAt: e.CreatedAt}
	}
	return out, nil
}

func (g *GoCardlessProvider) GetEvent(ctx context.Context, eventID string) (EventDetail, error) {
	var res struct {
		Events struct {
			ID           string `json:"id"`
			ResourceType string `json:"resource_type"`
			Action       string `json:"action"`
			CreatedAt    string `json:"created_at"`
			Links        struct {
				Payment      string `json:"payment,omitempty"`
				Mandate      string `json:"mandate,omitempty"`
				Subscription string `json:"subscription,omitempty"`
				Refund       string `json:"refund,omitempty"`
			} `json:"links"`
			Details struct {
				Origin string `json:"origin"`
				Cause  string `json:"cause"`
				Reason string `json:"reason_code"`
			} `json:"details"`
		} `json:"events"`
	}
	if err := g.client().get(ctx, "/events/"+eventID, &res); err != nil {
		return EventDetail{}, err
	}
	resourceID := ""
	switch res.Events.ResourceType {
	case "payments":
		resourceID = res.Events.Links.Payment
	case "mandates":
		resourceID = res.Events.Links.Mandate
	case "subscriptions":
		resourceID = res.Events.Links.Subscription
	case "refunds":
		resourceID = res.Events.Links.Refund
	}
	details := map[string]string{}
	if res.Events.Details.Origin != "" {
		details["origin"] = res.Events.Details.Origin
	}
	if res.Events.Details.Cause != "" {
		details["cause"] = res.Events.Details.Cause
	}
	if res.Events.Details.Reason != "" {
		details["reason"] = res.Events.Details.Reason
	}
	return EventDetail{
		ID: res.Events.ID, ResourceType: res.Events.ResourceType,
		Action: res.Events.Action, CreatedAt: res.Events.CreatedAt,
		ResourceID: resourceID, Details: details,
	}, nil
}

// --- Creditors ---

func (g *GoCardlessProvider) ListCreditors(ctx context.Context) ([]CreditorSummary, error) {
	var res struct {
		Creditors []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"creditors"`
	}
	if err := g.client().get(ctx, "/creditors", &res); err != nil {
		return nil, err
	}
	out := make([]CreditorSummary, len(res.Creditors))
	for i, c := range res.Creditors {
		out[i] = CreditorSummary{ID: c.ID, Name: c.Name}
	}
	return out, nil
}

func (g *GoCardlessProvider) GetCreditor(ctx context.Context, creditorID string) (CreditorDetail, error) {
	var res struct {
		Creditors struct {
			ID                 string `json:"id"`
			Name               string `json:"name"`
			AddressLine1       string `json:"address_line1"`
			City               string `json:"city"`
			PostalCode         string `json:"postal_code"`
			CountryCode        string `json:"country_code"`
			VerificationStatus string `json:"verification_status"`
		} `json:"creditors"`
	}
	if err := g.client().get(ctx, "/creditors/"+creditorID, &res); err != nil {
		return CreditorDetail{}, err
	}
	return CreditorDetail{
		ID: res.Creditors.ID, Name: res.Creditors.Name,
		AddressLine1: res.Creditors.AddressLine1, City: res.Creditors.City,
		PostalCode: res.Creditors.PostalCode, CountryCode: res.Creditors.CountryCode,
		VerificationStatus: res.Creditors.VerificationStatus,
	}, nil
}

// --- Webhooks management ---

func (g *GoCardlessProvider) ListWebhooks(ctx context.Context, limit int) ([]WebhookSummary, error) {
	params := make(url.Values)
	if limit <= 0 {
		limit = 100
	}
	params.Set("limit", fmt.Sprintf("%d", limit))
	var res struct {
		Webhooks []struct {
			ID             string `json:"id"`
			URL            string `json:"request_url"`
			IsTestEndpoint bool   `json:"is_test_endpoint"`
			SuccessCount   int    `json:"successful_count"`
		} `json:"webhooks"`
	}
	if err := g.client().getWithParams(ctx, "/webhooks", params, &res); err != nil {
		return nil, err
	}
	out := make([]WebhookSummary, len(res.Webhooks))
	for i, w := range res.Webhooks {
		out[i] = WebhookSummary{ID: w.ID, URL: w.URL, IsTestEndpoint: w.IsTestEndpoint, SuccessCount: w.SuccessCount}
	}
	return out, nil
}

func (g *GoCardlessProvider) GetWebhook(ctx context.Context, webhookID string) (WebhookDetail, error) {
	var res struct {
		Webhooks struct {
			ID             string `json:"id"`
			URL            string `json:"request_url"`
			IsTestEndpoint bool   `json:"is_test_endpoint"`
			ResponseCode   int    `json:"response_code"`
			ResponseBody   string `json:"response_body"`
		} `json:"webhooks"`
	}
	if err := g.client().get(ctx, "/webhooks/"+webhookID, &res); err != nil {
		return WebhookDetail{}, err
	}
	return WebhookDetail{
		ID: res.Webhooks.ID, URL: res.Webhooks.URL,
		IsTestEndpoint: res.Webhooks.IsTestEndpoint,
		ResponseCode: res.Webhooks.ResponseCode, ResponseBody: res.Webhooks.ResponseBody,
	}, nil
}

func (g *GoCardlessProvider) RetryWebhook(ctx context.Context, webhookID string) error {
	return g.client().do(ctx, "POST", "/webhooks/"+webhookID+"/actions/retry", "", struct{}{}, nil)
}

// --- Scheme Identifiers ---

func (g *GoCardlessProvider) ListSchemeIdentifiers(ctx context.Context) ([]SchemeIdentifierSummary, error) {
	var res struct {
		SchemeIdentifiers []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Scheme string `json:"scheme"`
			Status string `json:"status"`
		} `json:"scheme_identifiers"`
	}
	if err := g.client().get(ctx, "/scheme_identifiers", &res); err != nil {
		return nil, err
	}
	out := make([]SchemeIdentifierSummary, len(res.SchemeIdentifiers))
	for i, s := range res.SchemeIdentifiers {
		out[i] = SchemeIdentifierSummary{ID: s.ID, Name: s.Name, Scheme: s.Scheme, Status: s.Status}
	}
	return out, nil
}

func (g *GoCardlessProvider) GetSchemeIdentifier(ctx context.Context, schemeID string) (SchemeIdentifierDetail, error) {
	var res struct {
		SchemeIdentifiers struct {
			ID               string `json:"id"`
			Name             string `json:"name"`
			Scheme           string `json:"scheme"`
			Status           string `json:"status"`
			Reference        string `json:"reference"`
			MinAdvanceNotice int    `json:"minimum_advance_notice"`
		} `json:"scheme_identifiers"`
	}
	if err := g.client().get(ctx, "/scheme_identifiers/"+schemeID, &res); err != nil {
		return SchemeIdentifierDetail{}, err
	}
	return SchemeIdentifierDetail{
		ID: res.SchemeIdentifiers.ID, Name: res.SchemeIdentifiers.Name,
		Scheme: res.SchemeIdentifiers.Scheme, Status: res.SchemeIdentifiers.Status,
		Reference: res.SchemeIdentifiers.Reference, MinAdvanceNotice: res.SchemeIdentifiers.MinAdvanceNotice,
	}, nil
}
