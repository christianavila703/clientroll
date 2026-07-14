package handlers

import (
	"os"
	"testing"

	pdfpkg "clientroll/internal/pdf"
)

// TestGenerateSampleReceiptPDF is a manual smoke test: renders a sample receipt
// to a PDF file. Run with: SMOKE_PDF_OUT=/tmp/receipt.pdf go test -run TestGenerateSampleReceiptPDF ./internal/handlers/
func TestGenerateSampleReceiptPDF(t *testing.T) {
	out := os.Getenv("SMOKE_PDF_OUT")
	if out == "" {
		t.Skip("set SMOKE_PDF_OUT to generate a sample receipt PDF")
	}
	d := receiptData{
		ReceiptNumber:   "R-8F3A21C09B",
		IssuedDate:      "14 July 2026",
		PaidDate:        "12 July 2026",
		BusinessName:    "Ignite Results Pty Ltd",
		BusinessABN:     "12 345 678 901",
		BusinessAddress: []string{"42 Example Street", "QLD 4000"},
		BusinessEmail:   "accounts@businessnet.com.au",
		BusinessPhone:   "07 1234 5678",
		ClientName:      "Jane Citizen",
		ClientEmail:     "jane@example.com",
		ClientAddress:   []string{"7 Sample Avenue", "Brisbane QLD 4001"},
		Description:     "Gold Coaching — membership payment",
		BaseAmount:      450.00,
		TaxAmount:       45.00,
		FeeAmount:       8.75,
		TotalAmount:     503.75,
		CurrencyCode:    "AUD",
		PaymentMethod:   "Visa •••• 4242",
		Reference:       "INV-0042",
	}
	pdfBytes, err := pdfpkg.HTMLToPDF(renderReceiptHTML(d))
	if err != nil {
		t.Fatalf("HTMLToPDF: %v", err)
	}
	if err := os.WriteFile(out, pdfBytes, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Logf("wrote %d bytes to %s", len(pdfBytes), out)
}
