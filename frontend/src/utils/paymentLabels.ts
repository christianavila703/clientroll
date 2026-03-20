/**
 * Admin-facing payment copy: what the customer was charged vs what belongs to the organisation
 * (membership/service + GST), excluding pass-through processing surcharges.
 */

export type PaymentLike = {
  amount: number;
  base_amount?: number;
  tax_amount?: number;
  processing_fee_amount?: number;
  currency_code?: string;
};

export function fmtMoney(value: number): string {
  return value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

/** Total debited from the customer / shown on their receipt */
export function customerCharged(p: PaymentLike): number {
  return Number(p.amount) || 0;
}

/** Subtotal that maps to your product + GST (excludes card/DD processing surcharge line) */
export function organisationSubtotal(p: PaymentLike): number {
  const base = p.base_amount;
  const tax = p.tax_amount ?? 0;
  if (base !== undefined && base !== null && !Number.isNaN(base)) {
    return base + tax;
  }
  const fee = p.processing_fee_amount ?? 0;
  return Math.max(0, customerCharged(p) - fee);
}

export function processingSurcharge(p: PaymentLike): number {
  return p.processing_fee_amount ?? 0;
}

/** True when we should show the split (surcharge, GST, or base ≠ total) */
export function shouldShowPaymentSplit(p: PaymentLike): boolean {
  const fee = processingSurcharge(p);
  const tax = p.tax_amount ?? 0;
  const base = p.base_amount;
  const total = customerCharged(p);
  if (fee > 0.004 || tax > 0.004) return true;
  if (base !== undefined && base !== null && !Number.isNaN(base) && Math.abs(total - base - tax) > 0.02) return true;
  return false;
}

/** Same gross-up as server signup (card 1.75%+30c, BECS 1%+30c capped) */
export function estimateStripeProcessingFeeCents(baseCents: number, method: "card" | "direct_debit"): number {
  const base = Math.max(0, Math.round(baseCents));
  if (method === "direct_debit") {
    const grossUp = (base + 30) / (1 - 0.01);
    if (grossUp * 0.01 + 30 > 350) return 350;
    return Math.ceil(grossUp - base);
  }
  const grossUp = (base + 30) / (1 - 0.0175);
  return Math.max(0, Math.ceil(grossUp - base));
}
