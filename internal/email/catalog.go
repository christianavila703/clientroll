package email

type OperationalTemplateDefinition struct {
	Key            string   `json:"id"`
	Name           string   `json:"name"`
	Subject        string   `json:"subject"`
	Body           string   `json:"body"`
	Category       string   `json:"category"`
	Description    string   `json:"description"`
	Variables      []string `json:"variables"`
	DefaultEnabled bool     `json:"default_enabled"`
}

var operationalTemplateCatalog = []OperationalTemplateDefinition{
	{
		Key:            "membership_confirmation",
		Name:           "Membership Confirmation",
		Subject:        "Your {{membership_title}} membership is confirmed",
		Body:           "<p>Hi {{client_name}},</p><p>Your membership has been successfully set up:</p><ul><li><strong>Membership:</strong> {{membership_title}}</li><li><strong>Term:</strong> {{term_months}} months</li><li><strong>Total Fee:</strong> ${{total_fee}}</li></ul><p>If you have any questions, please contact us.</p>",
		Category:       "client",
		Description:    "Sent to client after completing membership signup",
		Variables:      []string{"client_name", "membership_title", "term_months", "total_fee", "company_name"},
		DefaultEnabled: true,
	},
	{
		Key:            "membership_agreement",
		Name:           "Membership Agreement (PDF)",
		Subject:        "Your signed membership agreement — {{membership_title}}",
		Body:           "<p>Hi {{client_name}},</p><p>Your signed membership agreement is attached as a PDF.</p><p>Please keep this document for your records.</p>",
		Category:       "client",
		Description:    "Sent to client when agreement PDF attachment is requested at signup",
		Variables:      []string{"client_name", "membership_title", "company_name"},
		DefaultEnabled: true,
	},
	{
		Key:            "membership_welcome",
		Name:           "New Member Welcome",
		Subject:        "Welcome to {{company_name}}, {{client_name}}!",
		Body:           "<p>Hi {{client_name}},</p><p>Welcome to <strong>{{company_name}}</strong>! We're excited to have you as a member.</p><p>Your <strong>{{membership_title}}</strong> membership is now active. Here's what to expect:</p><ul><li>Your first payment of ${{payment_amount}} will be debited {{payment_frequency}}</li><li>Your membership runs until {{end_date}}</li></ul><p>If you have any questions, don't hesitate to reach out.</p>",
		Category:       "client",
		Description:    "Welcome email sent to new clients after signing up",
		Variables:      []string{"client_name", "company_name", "membership_title", "payment_amount", "payment_frequency", "end_date"},
		DefaultEnabled: true,
	},
	{
		Key:            "payment_receipt",
		Name:           "Payment Receipt",
		Subject:        "Payment receipt — ${{total_charged}}",
		Body:           "<p>Hi {{client_name}},</p><p>We've received your payment:</p><ul><li><strong>Total charged:</strong> {{currency_code}} ${{total_charged}}</li><li><strong>Base amount (ex tax):</strong> {{currency_code}} ${{base_amount}}</li><li><strong>Tax:</strong> {{currency_code}} ${{tax_amount}}</li><li><strong>Processing fee:</strong> {{currency_code}} ${{processing_fee_amount}}</li><li><strong>To organisation:</strong> {{currency_code}} ${{to_organisation_amount}}</li><li><strong>Membership:</strong> {{membership_title}}</li><li><strong>Date:</strong> {{payment_date}}</li></ul><p>Thank you for your payment.</p>",
		Category:       "payments",
		Description:    "Sent to client after a successful payment",
		Variables:      []string{"client_name", "amount", "total_charged", "amount_due", "base_amount", "tax_amount", "processing_fee_amount", "to_organisation_amount", "currency_code", "membership_title", "payment_date", "company_name"},
		DefaultEnabled: true,
	},
	{
		Key:            "payment_failed",
		Name:           "Payment Failed (Client)",
		Subject:        "Payment failed — action required",
		Body:           "<p>Hi {{client_name}},</p><p>Unfortunately your scheduled payment could not be processed.</p><ul><li><strong>Amount due:</strong> {{currency_code}} ${{amount_due}}</li><li><strong>Base amount (ex tax):</strong> {{currency_code}} ${{base_amount}}</li><li><strong>Tax:</strong> {{currency_code}} ${{tax_amount}}</li><li><strong>Processing fee:</strong> {{currency_code}} ${{processing_fee_amount}}</li><li><strong>Membership:</strong> {{membership_title}}</li><li><strong>Reason:</strong> {{failure_reason}}</li></ul><p>Please ensure your bank details are up to date. We'll automatically retry this payment.</p>",
		Category:       "payments",
		Description:    "Sent to client when their payment fails",
		Variables:      []string{"client_name", "amount", "total_charged", "amount_due", "base_amount", "tax_amount", "processing_fee_amount", "to_organisation_amount", "currency_code", "membership_title", "failure_reason", "company_name"},
		DefaultEnabled: true,
	},
	{
		Key:            "payment_failed_alert",
		Name:           "Payment Failed (Staff Alert)",
		Subject:        "Payment failed: {{client_name}} — ${{amount_due}}",
		Body:           "<p>Hi {{admin_name}},</p><p>A payment has failed for one of your clients:</p><ul><li><strong>Client:</strong> {{client_name}}</li><li><strong>Amount due:</strong> {{currency_code}} ${{amount_due}}</li><li><strong>Base amount (ex tax):</strong> {{currency_code}} ${{base_amount}}</li><li><strong>Tax:</strong> {{currency_code}} ${{tax_amount}}</li><li><strong>Processing fee:</strong> {{currency_code}} ${{processing_fee_amount}}</li><li><strong>Membership:</strong> {{membership_title}}</li><li><strong>Reason:</strong> {{failure_reason}}</li></ul><p>Review this in your dashboard under Payments.</p>",
		Category:       "internal",
		Description:    "Sent to staff when a client's payment fails",
		Variables:      []string{"admin_name", "client_name", "amount", "total_charged", "amount_due", "base_amount", "tax_amount", "processing_fee_amount", "to_organisation_amount", "currency_code", "membership_title", "failure_reason"},
		DefaultEnabled: true,
	},
	{
		Key:            "dispute_alert",
		Name:           "Dispute Alert",
		Subject:        "Dispute opened: {{client_name}} — ${{amount}}",
		Body:           "<p>Hi {{admin_name}},</p><p>A payment dispute has been opened:</p><ul><li><strong>Client:</strong> {{client_name}}</li><li><strong>Amount:</strong> ${{amount}}</li><li><strong>Reason:</strong> {{reason}}</li></ul><p>Please review and respond in your dashboard.</p>",
		Category:       "internal",
		Description:    "Sent to staff when a client disputes a payment",
		Variables:      []string{"admin_name", "client_name", "amount", "reason"},
		DefaultEnabled: true,
	},
	{
		Key:            "service_charge_receipt",
		Name:           "Service Charge Receipt",
		Subject:        "Payment receipt — ${{total_charged}} for {{description}}",
		Body:           "<p>Hi {{client_name}},</p><p>We've received your payment for a service charge:</p><ul><li><strong>Description:</strong> {{description}}</li><li><strong>Amount (ex tax):</strong> {{currency_code}} ${{base_amount}}</li><li><strong>Tax:</strong> {{currency_code}} ${{tax_amount}}</li><li><strong>Processing fee:</strong> {{currency_code}} ${{processing_fee_amount}}</li><li><strong>Total charged:</strong> {{currency_code}} ${{total_charged}}</li><li><strong>Date:</strong> {{payment_date}}</li></ul><p>Thank you for your payment.</p>",
		Category:       "payments",
		Description:    "Sent to client after a successful service charge payment",
		Variables:      []string{"client_name", "description", "base_amount", "tax_amount", "processing_fee_amount", "total_charged", "currency_code", "payment_date", "company_name"},
		DefaultEnabled: true,
	},
	{
		Key:            "service_charge_failed",
		Name:           "Service Charge Failed (Client)",
		Subject:        "Payment failed for {{description}}",
		Body:           "<p>Hi {{client_name}},</p><p>Unfortunately your payment could not be processed:</p><ul><li><strong>Description:</strong> {{description}}</li><li><strong>Amount due:</strong> {{currency_code}} ${{total_charged}}</li><li><strong>Reason:</strong> {{failure_reason}}</li></ul><p>Please ensure your payment details are up to date.</p>",
		Category:       "payments",
		Description:    "Sent to client when a service charge payment fails",
		Variables:      []string{"client_name", "description", "base_amount", "tax_amount", "processing_fee_amount", "total_charged", "currency_code", "failure_reason", "company_name"},
		DefaultEnabled: true,
	},
	{
		Key:            "service_charge_receipt_alert",
		Name:           "Service Charge Received (Staff Alert)",
		Subject:        "Payment received: {{client_name}} — ${{total_charged}} for {{description}}",
		Body:           "<p>Hi {{admin_name}},</p><p>A service charge payment has been received:</p><ul><li><strong>Client:</strong> {{client_name}}</li><li><strong>Description:</strong> {{description}}</li><li><strong>Amount (ex tax):</strong> {{currency_code}} ${{base_amount}}</li><li><strong>Tax:</strong> {{currency_code}} ${{tax_amount}}</li><li><strong>Processing fee:</strong> {{currency_code}} ${{processing_fee_amount}}</li><li><strong>Total:</strong> {{currency_code}} ${{total_charged}}</li><li><strong>Date:</strong> {{payment_date}}</li></ul>",
		Category:       "internal",
		Description:    "Sent to staff when a service charge payment succeeds",
		Variables:      []string{"admin_name", "client_name", "description", "base_amount", "tax_amount", "processing_fee_amount", "total_charged", "currency_code", "payment_date"},
		DefaultEnabled: true,
	},
	{
		Key:            "service_charge_failed_alert",
		Name:           "Service Charge Failed (Staff Alert)",
		Subject:        "Payment failed: {{client_name}} — ${{total_charged}} for {{description}}",
		Body:           "<p>Hi {{admin_name}},</p><p>A service charge payment has failed:</p><ul><li><strong>Client:</strong> {{client_name}}</li><li><strong>Description:</strong> {{description}}</li><li><strong>Amount due:</strong> {{currency_code}} ${{total_charged}}</li><li><strong>Reason:</strong> {{failure_reason}}</li></ul><p>Review this in your dashboard.</p>",
		Category:       "internal",
		Description:    "Sent to staff when a service charge payment fails",
		Variables:      []string{"admin_name", "client_name", "description", "base_amount", "tax_amount", "processing_fee_amount", "total_charged", "currency_code", "failure_reason"},
		DefaultEnabled: true,
	},
	{
		Key:            "membership_expiring",
		Name:           "Membership Expiring",
		Subject:        "Your {{membership_title}} membership expires soon",
		Body:           "<p>Hi {{client_name}},</p><p>Your <strong>{{membership_title}}</strong> membership is set to expire on <strong>{{expiry_date}}</strong>.</p><p>Please contact us if you'd like to renew or discuss your options.</p>",
		Category:       "reminders",
		Description:    "Sent to client when membership is nearing end of term",
		Variables:      []string{"client_name", "membership_title", "expiry_date", "company_name"},
		DefaultEnabled: true,
	},
}

func OperationalTemplateCatalog() []OperationalTemplateDefinition {
	out := make([]OperationalTemplateDefinition, len(operationalTemplateCatalog))
	copy(out, operationalTemplateCatalog)
	return out
}

func OperationalTemplateByKey(key string) (OperationalTemplateDefinition, bool) {
	for _, t := range operationalTemplateCatalog {
		if t.Key == key {
			return t, true
		}
	}
	return OperationalTemplateDefinition{}, false
}

func IsOperationalTemplateCategory(category string) bool {
	switch category {
	case "client", "payments", "internal", "reminders":
		return true
	default:
		return false
	}
}

func OperationalTemplateKeysByCategory(category string) []string {
	keys := make([]string, 0, len(operationalTemplateCatalog))
	for _, t := range operationalTemplateCatalog {
		if t.Category == category {
			keys = append(keys, t.Key)
		}
	}
	return keys
}
