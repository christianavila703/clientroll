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

export type PaymentRetryLike = PaymentLike & {
  status: string;
  retry_count?: number;
  payment_method?: string;
  stripe_details?: Record<string, string | number | boolean | undefined> | null;
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
function parseStripeNextPaymentAttempt(raw: unknown): Date | null {
  if (raw === null || raw === undefined || raw === "") return null;
  const d = new Date(String(raw));
  return Number.isNaN(d.getTime()) ? null : d;
}

function failedAttemptCount(p: PaymentRetryLike): number {
  const stripeAttempts = p.stripe_details?.stripe_attempt_count;
  if (typeof stripeAttempts === "number" && stripeAttempts > 0) return stripeAttempts;
  return p.retry_count ?? 0;
}

function stripeInvoiceStatus(p: PaymentRetryLike): string {
  return String(p.stripe_details?.stripe_invoice_status ?? "").toLowerCase();
}

/** Strip legacy UI suffixes if ever stored on failure_reason. */
export function normalizeFailureReason(reason?: string | null): string {
  if (!reason) return "";
  return reason.replace(/\s*\(\d+\s+retries?\)\s*$/i, "").trim();
}

/**
 * Whether Stripe is expected to charge again (card / BECS subscription invoices).
 * Returns null when we cannot tell from stored data.
 */
export function paymentRetryStatusLabel(p: PaymentRetryLike): string | null {
  if (p.status !== "failed") return null;
  const attempts = failedAttemptCount(p);
  if (attempts <= 0) return null;

  const attemptPhrase =
    attempts === 1 ? "1 failed charge so far" : `${attempts} failed charges so far`;

  const nextRetry = parseStripeNextPaymentAttempt(p.stripe_details?.stripe_next_payment_attempt);
  if (nextRetry && nextRetry.getTime() > Date.now()) {
    const when = nextRetry.toLocaleString("en-AU", {
      day: "numeric",
      month: "short",
      year: "numeric",
      hour: "numeric",
      minute: "2-digit",
    });
    return `${attemptPhrase} — Stripe will try again on ${when}`;
  }

  const hasStripeSchedule =
    p.stripe_details?.stripe_next_payment_attempt !== undefined ||
    typeof p.stripe_details?.stripe_attempt_count === "number";
  if (hasStripeSchedule) {
    return `${attemptPhrase} — Stripe will not try again automatically`;
  }

  const invStatus = stripeInvoiceStatus(p);
  if (invStatus === "open") {
    return `${attemptPhrase} — Stripe may still try again (invoice still open)`;
  }
  if (invStatus === "uncollectible" || invStatus === "void") {
    return `${attemptPhrase} — Stripe will not try again automatically`;
  }

  return `${attemptPhrase} — use Show details to see if Stripe will try again`;
}

/** Single failed-payment line for lists (replaces ambiguous “(N retries)”). */
export function formatFailedPaymentMessage(p: PaymentRetryLike): string {
  const reason = normalizeFailureReason(
    typeof p.failure_reason === "string" ? p.failure_reason : undefined,
  );
  const retry = paymentRetryStatusLabel(p);
  if (!reason && !retry) return "Payment failed";
  if (!retry) return reason ? `Payment failed: ${reason}` : "Payment failed";
  if (!reason) return `Payment failed — ${retry}`;
  return `Payment failed: ${reason} — ${retry}`;
}

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
