import type { TermOption } from "../types/memberships";

export const DEFAULT_SIGNUP_PAYMENT_FREQUENCIES = [
  "upfront",
  "weekly",
  "fortnightly",
  "monthly",
  "quarterly",
  "custom_schedule",
] as const;

export type SignupPriceDisplayMode = "total" | "weekly" | "monthly";

function positiveNumber(value: unknown): number | null {
  const n = Number(value);
  return Number.isFinite(n) && n > 0 ? n : null;
}

export function termHasExplicitFee(term: TermOption, frequency: string): boolean {
  switch (frequency) {
    case "upfront":
      return !!term.allow_upfront && positiveNumber(term.fee_total) !== null;
    case "weekly":
      return positiveNumber(term.fee_weekly) !== null;
    case "fortnightly":
      return positiveNumber(term.fee_fortnightly) !== null;
    case "monthly":
      return positiveNumber(term.fee_monthly) !== null;
    case "quarterly":
      return positiveNumber(term.fee_quarterly) !== null;
    case "custom_schedule":
      return (
        !!term.custom_schedule_enabled &&
        positiveNumber(term.custom_schedule_amount) !== null &&
        positiveNumber(term.custom_schedule_num_payments) !== null
      );
    default:
      return false;
  }
}

/** Payment frequencies actually offered for this term (explicit fees only — no fee_total ÷ months guesses). */
export function availableTermPaymentFrequencies(
  term: TermOption,
  enabledPaymentFrequencies: string[] = [...DEFAULT_SIGNUP_PAYMENT_FREQUENCIES],
): string[] {
  const enabled = new Set(enabledPaymentFrequencies.map((f) => String(f).toLowerCase()));
  return DEFAULT_SIGNUP_PAYMENT_FREQUENCIES.filter(
    (freq) => enabled.has(freq) && termHasExplicitFee(term, freq),
  );
}

export function isUpfrontOnlyTerm(
  term: TermOption,
  enabledPaymentFrequencies?: string[],
): boolean {
  const available = availableTermPaymentFrequencies(term, enabledPaymentFrequencies);
  return available.length === 1 && available[0] === "upfront";
}

export function defaultPaymentFrequencyForTerm(
  term: TermOption,
  enabledPaymentFrequencies?: string[],
): string {
  const available = availableTermPaymentFrequencies(term, enabledPaymentFrequencies);
  if (available.includes("upfront")) return "upfront";
  return available[0] || "upfront";
}

export function explicitFrequencyAmount(term: TermOption, frequency: string): number | null {
  switch (frequency) {
    case "upfront":
      return positiveNumber(term.fee_total);
    case "weekly":
      return positiveNumber(term.fee_weekly);
    case "fortnightly":
      return positiveNumber(term.fee_fortnightly);
    case "monthly":
      return positiveNumber(term.fee_monthly);
    case "quarterly":
      return positiveNumber(term.fee_quarterly);
    case "custom_schedule":
      return positiveNumber(term.custom_schedule_amount);
    default:
      return null;
  }
}

export function effectiveSignupDisplayMode(
  tenantMode: SignupPriceDisplayMode | string | undefined,
  term: TermOption,
  options?: { lockedFrequency?: string; enabledPaymentFrequencies?: string[] },
): SignupPriceDisplayMode | "fortnightly" | "quarterly" | "custom_schedule" {
  const locked = String(options?.lockedFrequency || "").toLowerCase();
  if (locked === "upfront") return "total";
  if (locked === "weekly" && termHasExplicitFee(term, "weekly")) return "weekly";
  if (locked === "fortnightly" && termHasExplicitFee(term, "fortnightly")) return "fortnightly";
  if (locked === "monthly" && termHasExplicitFee(term, "monthly")) return "monthly";
  if (locked === "quarterly" && termHasExplicitFee(term, "quarterly")) return "quarterly";
  if (locked === "custom_schedule" && termHasExplicitFee(term, "custom_schedule")) return "custom_schedule";

  if (isUpfrontOnlyTerm(term, options?.enabledPaymentFrequencies)) return "total";

  const mode = (tenantMode || "total") as SignupPriceDisplayMode;
  if (mode === "weekly" && termHasExplicitFee(term, "weekly")) return "weekly";
  if (mode === "monthly" && termHasExplicitFee(term, "monthly")) return "monthly";
  return "total";
}

export function signupPriceSuffix(mode: ReturnType<typeof effectiveSignupDisplayMode>): string {
  switch (mode) {
    case "weekly":
      return "per week";
    case "monthly":
      return "per month";
    case "fortnightly":
      return "per fortnight";
    case "quarterly":
      return "per quarter";
    case "custom_schedule":
      return "per instalment";
    default:
      return "pay in full";
  }
}

export function signupDisplayAmount(
  term: TermOption,
  mode: ReturnType<typeof effectiveSignupDisplayMode>,
): number {
  if (mode === "total") return term.fee_total;
  if (mode === "custom_schedule") return explicitFrequencyAmount(term, "custom_schedule") || term.fee_total;
  return explicitFrequencyAmount(term, mode) || term.fee_total;
}

/** Lowest headline price for a membership card — upfront-only terms use total, not ÷ months. */
export function membershipHeadlinePrice(
  termOptions: TermOption[],
  tenantMode: SignupPriceDisplayMode | string | undefined,
  enabledPaymentFrequencies?: string[],
): { amount: number; suffix: string } {
  if (!termOptions.length) return { amount: 0, suffix: "membership value" };

  const totals = termOptions.map((t) => t.fee_total).filter((v) => v > 0).sort((a, b) => a - b);
  const allUpfrontOnly = termOptions.every((t) => isUpfrontOnlyTerm(t, enabledPaymentFrequencies));
  if (allUpfrontOnly && totals.length) {
    return { amount: totals[0], suffix: signupPriceSuffix("total") };
  }

  const mode = (tenantMode || "total") as SignupPriceDisplayMode;
  const amounts = termOptions
    .map((term) => {
      const displayMode = effectiveSignupDisplayMode(mode, term, { enabledPaymentFrequencies });
      return signupDisplayAmount(term, displayMode);
    })
    .filter((v) => v > 0)
    .sort((a, b) => a - b);

  if (!amounts.length && totals.length) {
    return { amount: totals[0], suffix: signupPriceSuffix("total") };
  }

  const displayMode = effectiveSignupDisplayMode(mode, termOptions[0], { enabledPaymentFrequencies });
  return { amount: amounts[0] || 0, suffix: signupPriceSuffix(displayMode) };
}
