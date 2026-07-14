package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"strings"
	"time"

	"clientroll/internal/email"
	"clientroll/internal/middleware"
	pdfpkg "clientroll/internal/pdf"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// PaymentReceiptHandler generates PDF receipts for succeeded payments and
// emails them to the client on request.
type PaymentReceiptHandler struct {
	paymentRepo       *repository.PaymentRepo
	serviceRepo       *repository.ServiceRepo
	clientRepo        *repository.ClientRepo
	memberRepo        *repository.MembershipRepo
	mtRepo            *repository.MembershipTypeRepo
	settingsRepo      *repository.SettingsRepo
	tenantRepo        *repository.TenantRepo
	emailTemplateRepo *repository.TenantEmailTemplateRepo
	emailSvc          email.Provider
}

// NewPaymentReceiptHandler creates a new payment receipt handler.
func NewPaymentReceiptHandler(
	paymentRepo *repository.PaymentRepo,
	serviceRepo *repository.ServiceRepo,
	clientRepo *repository.ClientRepo,
	memberRepo *repository.MembershipRepo,
	mtRepo *repository.MembershipTypeRepo,
	settingsRepo *repository.SettingsRepo,
	tenantRepo *repository.TenantRepo,
	emailTemplateRepo *repository.TenantEmailTemplateRepo,
	emailSvc email.Provider,
) *PaymentReceiptHandler {
	return &PaymentReceiptHandler{
		paymentRepo:       paymentRepo,
		serviceRepo:       serviceRepo,
		clientRepo:        clientRepo,
		memberRepo:        memberRepo,
		mtRepo:            mtRepo,
		settingsRepo:      settingsRepo,
		tenantRepo:        tenantRepo,
		emailTemplateRepo: emailTemplateRepo,
		emailSvc:          emailSvc,
	}
}

// RegisterRoutes registers receipt routes.
func (h *PaymentReceiptHandler) RegisterRoutes(r chi.Router) {
	r.Get("/payments/{paymentID}/receipt.pdf", h.DownloadReceiptPDF)
	r.Post("/payments/{paymentID}/send-receipt", h.SendReceipt)
}

// receiptData holds everything rendered onto the receipt document.
type receiptData struct {
	ReceiptNumber string
	IssuedDate    string
	PaidDate      string

	BusinessName    string
	BusinessABN     string
	BusinessAddress []string
	BusinessEmail   string
	BusinessPhone   string

	ClientName    string
	ClientEmail   string
	ClientAddress []string

	Description   string
	BaseAmount    float64
	TaxAmount     float64
	FeeAmount     float64
	TotalAmount   float64
	CurrencyCode  string
	PaymentMethod string
	Reference     string
}

func (d receiptData) money(v float64) string {
	return fmt.Sprintf("%s $%.2f", d.CurrencyCode, v)
}

// buildReceipt loads the payment (membership payment or service charge),
// the client, and the tenant's business details, and assembles receipt data.
// Returns an HTTP status + message on failure.
func (h *PaymentReceiptHandler) buildReceipt(ctx context.Context, tenantID, paymentID uuid.UUID, source string) (*receiptData, *string, int, string) {
	d := &receiptData{
		ReceiptNumber: "R-" + strings.ToUpper(strings.ReplaceAll(paymentID.String(), "-", "")[:10]),
		IssuedDate:    time.Now().Format("2 January 2006"),
		CurrencyCode:  "AUD",
	}

	var clientID uuid.UUID
	var stripeDetails json.RawMessage
	var providerRef *string

	switch source {
	case "service":
		charge, err := h.serviceRepo.FindChargeByID(ctx, tenantID, paymentID)
		if err != nil || charge == nil {
			return nil, nil, http.StatusNotFound, "Payment not found"
		}
		if charge.Status != "succeeded" {
			return nil, nil, http.StatusBadRequest, "A receipt can only be issued for a successful payment"
		}
		clientID = charge.ClientID
		d.Description = charge.Description
		if strings.TrimSpace(d.Description) == "" {
			d.Description = "Service charge"
		}
		d.BaseAmount = charge.Amount
		d.TaxAmount = charge.TaxAmount
		d.FeeAmount = charge.ProcessingFeeAmount
		d.TotalAmount = charge.Amount + charge.TaxAmount + charge.ProcessingFeeAmount
		if strings.TrimSpace(charge.CurrencyCode) != "" {
			d.CurrencyCode = strings.ToUpper(charge.CurrencyCode)
		}
		if charge.PaidDate != nil {
			d.PaidDate = charge.PaidDate.Format("2 January 2006")
		} else {
			d.PaidDate = charge.CreatedAt.Format("2 January 2006")
		}
		if charge.PaymentMethod != nil {
			d.PaymentMethod = paymentMethodLabel(*charge.PaymentMethod)
		}
		stripeDetails = charge.StripeDetails
		providerRef = charge.ProviderReference

	default: // membership payment
		p, err := h.paymentRepo.GetByID(ctx, paymentID, tenantID)
		if err != nil || p == nil {
			return nil, nil, http.StatusNotFound, "Payment not found"
		}
		if p.Status != "succeeded" {
			return nil, nil, http.StatusBadRequest, "A receipt can only be issued for a successful payment"
		}
		clientID = p.ClientID
		d.Description = "Membership payment"
		if membership, mErr := h.memberRepo.FindByID(ctx, p.MembershipID, tenantID); mErr == nil && membership != nil {
			if mt, mtErr := h.mtRepo.FindByID(ctx, membership.MembershipTypeID, tenantID); mtErr == nil && mt != nil && strings.TrimSpace(mt.Title) != "" {
				d.Description = mt.Title + " — membership payment"
			}
		}
		d.BaseAmount = p.BaseAmount
		d.TaxAmount = p.TaxAmount
		d.FeeAmount = p.ProcessingFeeAmount
		d.TotalAmount = p.Amount
		if d.BaseAmount == 0 && d.TaxAmount == 0 && d.FeeAmount == 0 {
			d.BaseAmount = p.Amount
		}
		if strings.TrimSpace(p.CurrencyCode) != "" {
			d.CurrencyCode = strings.ToUpper(p.CurrencyCode)
		}
		if p.PaidDate != nil {
			d.PaidDate = p.PaidDate.Format("2 January 2006")
		} else {
			d.PaidDate = p.DueDate.Format("2 January 2006")
		}
		stripeDetails = p.StripeDetails
		providerRef = p.ProviderReference
	}

	// Payment method + reference from Stripe details when available.
	if len(stripeDetails) > 0 {
		var sd map[string]interface{}
		if err := json.Unmarshal(stripeDetails, &sd); err == nil {
			if label, _ := sd["payment_method_label"].(string); strings.TrimSpace(label) != "" {
				d.PaymentMethod = label
			} else if pmType, _ := sd["payment_method_type"].(string); d.PaymentMethod == "" && strings.TrimSpace(pmType) != "" {
				d.PaymentMethod = paymentMethodLabel(pmType)
			}
			if invNum, _ := sd["invoice_number"].(string); strings.TrimSpace(invNum) != "" {
				d.Reference = invNum
			}
		}
	}
	if d.Reference == "" && providerRef != nil {
		d.Reference = *providerRef
	}
	if d.PaymentMethod == "" && providerRef != nil && *providerRef != "" {
		d.PaymentMethod = "Direct Debit"
	}

	// Client details.
	client, err := h.clientRepo.FindByID(ctx, clientID, tenantID)
	if err != nil || client == nil {
		return nil, nil, http.StatusNotFound, "Client not found for this payment"
	}
	d.ClientName = client.Name
	if client.Email != nil {
		d.ClientEmail = strings.TrimSpace(*client.Email)
	}
	d.ClientAddress = joinAddressLines(
		derefStr(client.UnitNumber), derefStr(client.Address),
		strings.TrimSpace(strings.Join(nonEmpty(derefStr(client.Suburb), derefStr(client.State), derefStr(client.Postcode)), " ")),
	)

	// Business details from agreement settings, falling back to tenant name.
	if settings, sErr := h.settingsRepo.GetAgreementSettings(ctx, tenantID); sErr == nil && settings != nil {
		d.BusinessName = firstNonEmpty(derefStr(settings.TradingName), derefStr(settings.BusinessName), derefStr(settings.CompanyName))
		d.BusinessABN = derefStr(settings.ABN)
		d.BusinessEmail = derefStr(settings.Email)
		d.BusinessPhone = derefStr(settings.Phone)
		d.BusinessAddress = joinAddressLines(
			derefStr(settings.Address),
			strings.TrimSpace(strings.Join(nonEmpty(derefStr(settings.State), derefStr(settings.Postcode)), " ")),
		)
	}
	if d.BusinessName == "" {
		if tenant, tErr := h.tenantRepo.FindByID(ctx, tenantID); tErr == nil && tenant != nil {
			d.BusinessName = tenant.Name
		}
	}
	if d.BusinessName == "" {
		d.BusinessName = "Your business"
	}

	var clientEmail *string
	if d.ClientEmail != "" {
		clientEmail = &d.ClientEmail
	}
	return d, clientEmail, 0, ""
}

// DownloadReceiptPDF streams the receipt PDF to the admin.
// GET /payments/{paymentID}/receipt.pdf?source=membership|service
func (h *PaymentReceiptHandler) DownloadReceiptPDF(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	paymentID, err := uuid.Parse(chi.URLParam(r, "paymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid payment ID")
		return
	}

	data, _, status, msg := h.buildReceipt(r.Context(), tenantID, paymentID, r.URL.Query().Get("source"))
	if status != 0 {
		writeError(w, status, msg)
		return
	}

	pdfBytes, err := pdfpkg.HTMLToPDF(renderReceiptHTML(*data))
	if err != nil {
		log.Printf("receipt.pdf: PDF generation failed payment=%s tenant=%s err=%v", paymentID, tenantID, err)
		writeError(w, http.StatusInternalServerError, "Failed to generate PDF. This usually means headless Chromium is not installed on the server.")
		return
	}

	filename := fmt.Sprintf("Receipt-%s.pdf", data.ReceiptNumber)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
	w.WriteHeader(http.StatusOK)
	w.Write(pdfBytes)
}

// SendReceipt generates the receipt PDF and emails it to the client.
// POST /payments/{paymentID}/send-receipt?source=membership|service
func (h *PaymentReceiptHandler) SendReceipt(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	paymentID, err := uuid.Parse(chi.URLParam(r, "paymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid payment ID")
		return
	}

	data, clientEmail, status, msg := h.buildReceipt(r.Context(), tenantID, paymentID, r.URL.Query().Get("source"))
	if status != 0 {
		writeError(w, status, msg)
		return
	}
	if clientEmail == nil {
		writeError(w, http.StatusBadRequest, "This client has no email address on file")
		return
	}

	pdfBytes, err := pdfpkg.HTMLToPDF(renderReceiptHTML(*data))
	if err != nil {
		log.Printf("send-receipt: PDF generation failed payment=%s tenant=%s err=%v", paymentID, tenantID, err)
		writeError(w, http.StatusInternalServerError, "Failed to generate the receipt PDF. This usually means headless Chromium is not installed on the server.")
		return
	}

	input := h.buildReceiptEmail(r.Context(), tenantID, *data)
	input.To = *clientEmail
	input.Attachments = []email.Attachment{{
		Filename: fmt.Sprintf("Receipt-%s.pdf", data.ReceiptNumber),
		Content:  pdfBytes,
		MimeType: "application/pdf",
	}}

	if _, err := h.emailSvc.Send(r.Context(), input); err != nil {
		log.Printf("send-receipt: email send failed payment=%s tenant=%s to=%s err=%v", paymentID, tenantID, *clientEmail, err)
		writeError(w, http.StatusBadGateway, "The receipt PDF was generated but the email could not be sent. Please try again.")
		return
	}

	log.Printf("send-receipt: receipt %s emailed payment=%s tenant=%s to=%s", data.ReceiptNumber, paymentID, tenantID, *clientEmail)
	writeJSON(w, http.StatusOK, map[string]string{
		"message":        "Receipt sent",
		"sent_to":        *clientEmail,
		"receipt_number": data.ReceiptNumber,
	})
}

// buildReceiptEmail renders the email body from the tenant's payment_receipt
// template (subject/body overrides respected). Because this send is an explicit
// admin action, the template's enabled flag is not consulted.
func (h *PaymentReceiptHandler) buildReceiptEmail(ctx context.Context, tenantID uuid.UUID, d receiptData) email.SendInput {
	subject := fmt.Sprintf("Payment receipt — $%.2f", d.TotalAmount)
	body := fmt.Sprintf(
		"<p>Hi %s,</p><p>Please find your payment receipt attached as a PDF.</p><ul><li><strong>Receipt number:</strong> %s</li><li><strong>Total charged:</strong> %s</li><li><strong>Date:</strong> %s</li></ul><p>Thank you for your payment.</p>",
		html.EscapeString(d.ClientName), d.ReceiptNumber, d.money(d.TotalAmount), d.PaidDate)

	if def, ok := email.OperationalTemplateByKey("payment_receipt"); ok {
		subject = def.Subject
		body = def.Body
		if h.emailTemplateRepo != nil {
			if override, err := h.emailTemplateRepo.GetByKey(ctx, tenantID, "payment_receipt"); err == nil && override != nil {
				if override.SubjectOverride != nil && strings.TrimSpace(*override.SubjectOverride) != "" {
					subject = *override.SubjectOverride
				}
				if override.BodyOverride != nil && strings.TrimSpace(*override.BodyOverride) != "" {
					body = *override.BodyOverride
				}
			}
		}
		money := func(v float64) string { return fmt.Sprintf("%.2f", v) }
		vars := map[string]string{
			"client_name":            html.EscapeString(d.ClientName),
			"amount":                 money(d.TotalAmount),
			"total_charged":          money(d.TotalAmount),
			"amount_due":             money(d.TotalAmount),
			"base_amount":            money(d.BaseAmount),
			"tax_amount":             money(d.TaxAmount),
			"processing_fee_amount":  money(d.FeeAmount),
			"to_organisation_amount": money(d.BaseAmount + d.TaxAmount),
			"currency_code":          d.CurrencyCode,
			"membership_title":       html.EscapeString(d.Description),
			"payment_date":           d.PaidDate,
			"company_name":           html.EscapeString(d.BusinessName),
		}
		for k, v := range vars {
			subject = strings.ReplaceAll(subject, "{{"+k+"}}", v)
			body = strings.ReplaceAll(body, "{{"+k+"}}", v)
		}
		body += "<p>Your receipt is attached to this email as a PDF for your records.</p>"
	}

	return email.SendInput{
		Subject: subject,
		HTML:    email.WrapHTML(body),
		Text:    fmt.Sprintf("Hi %s, your payment receipt %s for %s is attached as a PDF.", d.ClientName, d.ReceiptNumber, d.money(d.TotalAmount)),
	}
}

// renderReceiptHTML produces the standalone A4 receipt document rendered to PDF.
func renderReceiptHTML(d receiptData) string {
	esc := html.EscapeString

	row := func(label, value string, bold bool) string {
		weight := "400"
		if bold {
			weight = "700"
		}
		return fmt.Sprintf(
			`<tr><td style="padding:10px 0;border-top:1px solid #e5e7eb;color:#374151;">%s</td><td style="padding:10px 0;border-top:1px solid #e5e7eb;text-align:right;font-weight:%s;color:#111827;">%s</td></tr>`,
			esc(label), weight, esc(value))
	}

	var amountRows strings.Builder
	amountRows.WriteString(row(d.Description, d.money(d.BaseAmount), false))
	if d.TaxAmount > 0 {
		amountRows.WriteString(row("GST", d.money(d.TaxAmount), false))
	}
	if d.FeeAmount > 0 {
		amountRows.WriteString(row("Payment processing fee", d.money(d.FeeAmount), false))
	}
	amountRows.WriteString(fmt.Sprintf(
		`<tr><td style="padding:12px 0;border-top:2px solid #111827;font-weight:700;color:#111827;">Total paid</td><td style="padding:12px 0;border-top:2px solid #111827;text-align:right;font-weight:700;font-size:16px;color:#111827;">%s</td></tr>`,
		esc(d.money(d.TotalAmount))))

	meta := func(label, value string) string {
		if strings.TrimSpace(value) == "" {
			return ""
		}
		return fmt.Sprintf(`<p style="margin:2px 0;font-size:12px;color:#6b7280;">%s: <span style="color:#111827;">%s</span></p>`, esc(label), esc(value))
	}

	addrLines := func(lines []string) string {
		var b strings.Builder
		for _, l := range lines {
			if strings.TrimSpace(l) == "" {
				continue
			}
			b.WriteString(fmt.Sprintf(`<p style="margin:2px 0;font-size:12px;color:#6b7280;">%s</p>`, esc(l)))
		}
		return b.String()
	}

	businessContact := ""
	if d.BusinessEmail != "" || d.BusinessPhone != "" {
		businessContact = fmt.Sprintf(`<p style="margin:2px 0;font-size:12px;color:#6b7280;">%s</p>`,
			esc(strings.Join(nonEmpty(d.BusinessEmail, d.BusinessPhone), " · ")))
	}

	abnLine := ""
	if d.BusinessABN != "" {
		abnLine = fmt.Sprintf(`<p style="margin:2px 0;font-size:12px;color:#6b7280;">ABN %s</p>`, esc(d.BusinessABN))
	}

	gstNote := ""
	if d.TaxAmount > 0 {
		gstNote = fmt.Sprintf(`<p style="margin:16px 0 0;font-size:11px;color:#9ca3af;">The total includes GST of %s.</p>`, esc(d.money(d.TaxAmount)))
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
  @page { size: A4; margin: 0; }
  body { margin: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; color: #111827; }
</style>
</head>
<body>
<div style="padding:48px 56px;">
  <table style="width:100%%;border-collapse:collapse;">
    <tr>
      <td style="vertical-align:top;">
        <h1 style="margin:0 0 4px;font-size:22px;color:#111827;">%s</h1>
        %s%s%s
      </td>
      <td style="vertical-align:top;text-align:right;">
        <p style="margin:0;font-size:20px;font-weight:700;letter-spacing:1px;color:#4f46e5;">RECEIPT</p>
        %s%s%s
      </td>
    </tr>
  </table>

  <div style="margin-top:36px;">
    <p style="margin:0 0 4px;font-size:11px;text-transform:uppercase;letter-spacing:1px;color:#9ca3af;">Received from</p>
    <p style="margin:0;font-size:14px;font-weight:600;color:#111827;">%s</p>
    %s%s
  </div>

  <table style="width:100%%;border-collapse:collapse;margin-top:32px;font-size:13px;">
    <tr>
      <th style="text-align:left;padding:0 0 8px;font-size:11px;text-transform:uppercase;letter-spacing:1px;color:#9ca3af;font-weight:600;">Description</th>
      <th style="text-align:right;padding:0 0 8px;font-size:11px;text-transform:uppercase;letter-spacing:1px;color:#9ca3af;font-weight:600;">Amount</th>
    </tr>
    %s
  </table>

  <div style="margin-top:24px;">
    %s%s
  </div>
  %s

  <p style="margin:48px 0 0;font-size:11px;color:#d1d5db;">Receipt issued on %s%s</p>
</div>
</body>
</html>`,
		esc(d.BusinessName), abnLine, addrLines(d.BusinessAddress), businessContact,
		meta("Receipt no", d.ReceiptNumber), meta("Date paid", d.PaidDate), meta("Reference", d.Reference),
		esc(d.ClientName), addrLines(d.ClientAddress), meta("Email", d.ClientEmail),
		amountRows.String(),
		meta("Payment method", d.PaymentMethod), meta("Currency", d.CurrencyCode),
		gstNote,
		esc(d.IssuedDate),
		func() string {
			if d.BusinessName != "" {
				return " by " + esc(d.BusinessName)
			}
			return ""
		}(),
	)
}

func paymentMethodLabel(method string) string {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "card":
		return "Card"
	case "direct_debit", "au_becs_debit", "becs_debit":
		return "Direct Debit"
	case "":
		return ""
	default:
		label := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(method)), "_", " ")
		return strings.ToUpper(label[:1]) + label[1:]
	}
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

func nonEmpty(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}

func joinAddressLines(lines ...string) []string {
	return nonEmpty(lines...)
}
