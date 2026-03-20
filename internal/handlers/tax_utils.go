package handlers

import (
	"math"

	"clientroll/internal/models"
	"clientroll/internal/repository"
)

func tenantSalesTaxPercent(tenant *models.Tenant) float64 {
	if tenant == nil {
		return 10
	}
	if tenant.SalesTaxPercent < 0 {
		return 0
	}
	if tenant.SalesTaxPercent > 100 {
		return 100
	}
	return tenant.SalesTaxPercent
}

func splitInclusiveAmount(total float64, taxPercent float64) (base float64, tax float64) {
	if total <= 0 {
		return 0, 0
	}
	if taxPercent <= 0 {
		return total, 0
	}
	rawTax := total * (taxPercent / (100 + taxPercent))
	tax = math.Round(rawTax*100) / 100
	base = total - tax
	if base < 0 {
		base = 0
	}
	return base, tax
}

func splitAmountByTaxMode(total, taxPercent float64, applySalesTax bool) (base float64, tax float64) {
	if total <= 0 {
		return 0, 0
	}
	if !applySalesTax {
		return total, 0
	}
	return splitInclusiveAmount(total, taxPercent)
}

func paymentComponentsByTaxMode(total, fee, taxPercent float64, applySalesTax bool) *repository.PaymentComponents {
	net := total - fee
	if net < 0 {
		net = 0
	}
	base, tax := splitAmountByTaxMode(net, taxPercent, applySalesTax)
	return &repository.PaymentComponents{
		BaseAmount:          base,
		TaxAmount:           tax,
		ProcessingFeeAmount: fee,
	}
}
