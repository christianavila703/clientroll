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

const cadenceAdverb = (
  unit: "week" | "month" | null | undefined,
  count: number | null | undefined,
): string => {
  const c = count ?? 1;
  if (unit === "week" && c === 1) return "weekly";
  if (unit === "week" && c === 2) return "fortnightly";
  if (unit === "month" && c === 1) return "monthly";
  if (unit === "month" && c === 3) return "quarterly";
  return `every ${cadenceWord(unit, count)}`;
};

const defaultSchedule = (term: TermOption): PaymentSchedule | undefined => {
  const active = (term.payment_schedules ?? []).filter(s => !s.archived_at);
  return active.find(s => s.is_default) ?? active[0];
};

export const describeTermPricing = (term: TermOption): TermPricingDisplay => {
  const s = defaultSchedule(term);
  const feeTotal = Number(term.fee_total) || 0;

  if (s) {
    if (s.kind === "upfront") {
      return { amount: s.amount, suffix: "pay in full" };
    }
    const adverb = cadenceAdverb(s.interval_unit ?? null, s.interval_count ?? null);
    if (s.num_payments && s.num_payments > 0) {
      // Fixed-count instalment plan (e.g. "× 3 monthly payments").
      const total = feeTotal > 0 ? feeTotal : Math.round(s.amount * s.num_payments * 100) / 100;
      return {
        amount: s.amount,
        suffix: `× ${s.num_payments} ${adverb} payment${s.num_payments === 1 ? "" : "s"}`,
        total,
      };
    }
    // Open-ended recurring subscription.
    const word = cadenceWord(s.interval_unit ?? null, s.interval_count ?? null);
    return {
      amount: s.amount,
      suffix: `per ${word}`,
      total: feeTotal > 0 ? feeTotal : undefined,
    };
  }

  // No payment_schedules — fall back to legacy fee fields. We only do this
  // for terms that genuinely have no schedule rows; new terms always have one.
  if (term.allow_upfront && (!term.fee_monthly || term.fee_monthly <= 0) && (!term.fee_weekly || term.fee_weekly <= 0) && (!term.fee_fortnightly || term.fee_fortnightly <= 0)) {
    return { amount: feeTotal, suffix: "pay in full" };
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
  return { amount: feeTotal, suffix: "pay in full" };
};
