// termOptionDisplay
//
// Single source of truth for rendering a TermOption's price + payment cadence
// in member-facing views. Prefers the new `payment_schedules` row (which
// reflects exactly what the admin chose in the simplified editor) over the
// legacy auto-derived fee_* columns.
//
// Returns:
//   amount  → the primary number to render large (per-period amount, or the
//             upfront total for upfront-only terms)
//   suffix  → the descriptive line under the amount, e.g. "pay in full",
//             "per month", "× 3 monthly payments"
//   total   → set when the amount is a per-payment figure and we know the
//             contract total (helpful for "total $X" microcopy)

import type { PaymentSchedule, TermOption } from "../types/memberships";

export interface TermPricingDisplay {
  amount: number;
  suffix: string;
  total?: number;
}

const cadenceWord = (
  unit: "week" | "month" | null | undefined,
  count: number | null | undefined,
): string => {
  const c = count ?? 1;
  if (unit === "week" && c === 1) return "week";
  if (unit === "week" && c === 2) return "fortnight";
  if (unit === "month" && c === 1) return "month";
  if (unit === "month" && c === 3) return "quarter";
  if (unit === "month") return `${c} months`;
  if (unit === "week") return `${c} weeks`;
  return "period";
};

const everyPhrase = (
  unit: "week" | "month" | null | undefined,
  count: number | null | undefined,
): string => {
  const c = count ?? 1;
  if (c === 1) return `every ${unit ?? "period"}`;
  return `every ${c} ${unit ?? "period"}s`;
};

const defaultSchedule = (term: TermOption): PaymentSchedule | undefined => {
  const active = (term.payment_schedules ?? []).filter(s => !s.archived_at);
  return active.find(s => s.is_default) ?? active[0];
};

/**
 * Human-readable duration label, e.g. "12 Months", "6 Weeks".
 * Prefers the v2 duration_value/duration_unit fields; falls back to
 * legacy duration_months.
 */
export const termDurationLabel = (term: TermOption): string => {
  const value = term.duration_value ?? term.duration_months ?? 0;
  const unit = term.duration_unit ?? "month";
  const noun = unit === "week" ? (value === 1 ? "Week" : "Weeks") : (value === 1 ? "Month" : "Months");
  return `${value} ${noun}`;
};

export const describeTermPricing = (term: TermOption): TermPricingDisplay => {
  const s = defaultSchedule(term);
  const feeTotal = Number(term.fee_total) || 0;

  if (s) {
    if (s.kind === "upfront") {
      return { amount: s.amount, suffix: "Pay in full" };
    }
    const unit = s.interval_unit ?? null;
    const count = s.interval_count ?? null;
    if (s.num_payments && s.num_payments > 0) {
      // Fixed-count instalment plan. Prefer the stored contract total
      // (fee_total) when it's within rounding distance of amount × N — that
      // avoids "$5,000.01" type drift caused by repeated 2dp rounding of a
      // per-payment figure that doesn't divide evenly. If fee_total is way
      // off (legacy admin set them independently), trust the schedule.
      const derived = Math.round(s.amount * s.num_payments * 100) / 100;
      const total = feeTotal > 0 && Math.abs(feeTotal - derived) <= Math.max(1, s.num_payments)
        ? Math.round(feeTotal * 100) / 100
        : derived;
      const noun = s.num_payments === 1 ? "payment" : "payments";
      const phrase = `${s.num_payments} ${noun} total · ${everyPhrase(unit, count)}`;
      return { amount: s.amount, suffix: phrase, total };
    }
    // Open-ended recurring subscription.
    const word = cadenceWord(unit, count);
    return {
      amount: s.amount,
      suffix: `per ${word}`,
      total: feeTotal > 0 ? feeTotal : undefined,
    };
  }

  // No payment_schedules — fall back to legacy fee fields. We only do this
  // for terms that genuinely have no schedule rows; new terms always have one.
  if (term.allow_upfront && (!term.fee_monthly || term.fee_monthly <= 0) && (!term.fee_weekly || term.fee_weekly <= 0) && (!term.fee_fortnightly || term.fee_fortnightly <= 0)) {
    return { amount: feeTotal, suffix: "Pay in full" };
  }
  if (term.fee_monthly && term.fee_monthly > 0) {
    return { amount: term.fee_monthly, suffix: "per month", total: feeTotal };
  }
  if (term.fee_weekly && term.fee_weekly > 0) {
    return { amount: term.fee_weekly, suffix: "per week", total: feeTotal };
  }
  if (term.fee_fortnightly && term.fee_fortnightly > 0) {
    return { amount: term.fee_fortnightly, suffix: "per fortnight", total: feeTotal };
  }
  return { amount: feeTotal, suffix: "Pay in full" };
};
