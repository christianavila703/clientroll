package handlers

import (
	"encoding/json"
	"strconv"
	"strings"

	"clientroll/internal/repository"
)

// parseMetadataPaymentComponents reads base/tax/fee from Stripe Checkout or Invoice metadata.
func parseMetadataPaymentComponents(metadata map[string]string, total float64) *repository.PaymentComponents {
	if len(metadata) == 0 {
		return nil
	}
	parse := func(key string) (float64, bool) {
		s := strings.TrimSpace(metadata[key])
		if s == "" {
			return 0, false
		}
		v, err := strconv.ParseFloat(s, 64)
		return v, err == nil
	}
	b, okB := parse("base_amount")
	t, okT := parse("tax_amount")
	f, okF := parse("processing_fee_amount")
	if !okB && !okT && !okF {
		return nil
	}
	if !okB {
		b = total - t - f
	}
	if b < 0 {
		b = 0
	}
	return &repository.PaymentComponents{
		BaseAmount:          b,
		TaxAmount:           t,
		ProcessingFeeAmount: f,
	}
}

// paymentComponentsFromInvoiceRaw tries metadata on the invoice payload, then line items (membership vs processing fee).
func paymentComponentsFromInvoiceRaw(raw json.RawMessage, total float64) *repository.PaymentComponents {
	var inv struct {
		Metadata map[string]string `json:"metadata"`
		Lines    *struct {
			Data []struct {
				Description string `json:"description"`
				Amount      int64  `json:"amount"`
			} `json:"data"`
		} `json:"lines"`
		TotalTaxAmounts []struct {
			Amount int64 `json:"amount"`
		} `json:"total_tax_amounts"`
	}
	if err := json.Unmarshal(raw, &inv); err != nil {
		return nil
	}
	if c := parseMetadataPaymentComponents(inv.Metadata, total); c != nil {
		return c
	}
	if inv.Lines == nil || len(inv.Lines.Data) == 0 {
		return nil
	}
	var baseCents, feeCents int64
	for _, line := range inv.Lines.Data {
		desc := strings.ToLower(strings.TrimSpace(line.Description))
		if strings.Contains(desc, "processing fee") {
			feeCents += line.Amount
			continue
		}
		baseCents += line.Amount
	}
	var taxCents int64
	for _, tx := range inv.TotalTaxAmounts {
		taxCents += tx.Amount
	}
	if baseCents == 0 && feeCents == 0 && taxCents == 0 {
		return nil
	}
	return &repository.PaymentComponents{
		BaseAmount:          float64(baseCents) / 100,
		TaxAmount:           float64(taxCents) / 100,
		ProcessingFeeAmount: float64(feeCents) / 100,
	}
}
