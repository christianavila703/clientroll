import { useState, useEffect } from "react";
import type { MembershipType, TermOption, Bonus } from "./SignupPage";

interface Props {
  membership: MembershipType;
  tenantSlug: string;
  priceDisplayMode: "total" | "weekly" | "monthly";
  enabledPaymentFrequencies: string[];
  onSelectTerm: (term: TermOption) => void;
  onBack: () => void;
}

export default function MembershipDetail({ membership, tenantSlug, priceDisplayMode, enabledPaymentFrequencies, onSelectTerm, onBack }: Props) {
  const membershipName = (mt: MembershipType) => (mt.membership_name || "").trim();
  const membershipLevel = (mt: MembershipType) => (mt.membership_level || "").trim();
  const fallbackTitle = (mt: MembershipType) => mt.title;
  const [selectedTermIdx, setSelectedTermIdx] = useState(0);
  const [bonuses, setBonuses] = useState<Bonus[]>([]);

  const terms = membership.term_options || [];
  const currentTerm = terms[selectedTermIdx];
  const toWeekly = (feeTotal: number, durationMonths: number): number | null => {
    if (!feeTotal || !durationMonths) return null;
    const weeks = (durationMonths * 52) / 12;
    if (!weeks) return null;
    return feeTotal / weeks;
  };
  const toMonthly = (feeTotal: number, durationMonths: number): number | null => {
    if (!feeTotal || !durationMonths) return null;
    return feeTotal / durationMonths;
  };
  const formatMoney = (value: number) =>
    value.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 2 });
  const weeklyFromTerm = currentTerm?.fee_weekly ?? (currentTerm ? toWeekly(currentTerm.fee_total, currentTerm.duration_months) : null);
  const weeklyFallback = terms
    .map((t) => t.fee_weekly || toWeekly(t.fee_total, t.duration_months))
    .filter((v): v is number => typeof v === "number" && v > 0)
    .sort((a, b) => a - b)[0];
  const weeklyValue = weeklyFromTerm ?? weeklyFallback ?? null;
  const monthlyFromTerm = currentTerm?.fee_monthly ?? (currentTerm ? toMonthly(currentTerm.fee_total, currentTerm.duration_months) : null);
  const monthlyFallback = terms
    .map((t) => t.fee_monthly || toMonthly(t.fee_total, t.duration_months))
    .filter((v): v is number => typeof v === "number" && v > 0)
    .sort((a, b) => a - b)[0];
  const monthlyValue = monthlyFromTerm ?? monthlyFallback ?? null;
  const effectiveEnabledFrequencies =
    membership.enabled_payment_frequencies && membership.enabled_payment_frequencies.length > 0
      ? membership.enabled_payment_frequencies
      : enabledPaymentFrequencies;
  const frequencySet = new Set((effectiveEnabledFrequencies || []).map((f) => String(f).toLowerCase()));
  const resolvedDisplayMode: "total" | "weekly" | "monthly" =
    priceDisplayMode === "weekly" && frequencySet.has("weekly") && weeklyValue !== null
      ? "weekly"
      : priceDisplayMode === "monthly" && frequencySet.has("monthly") && monthlyValue !== null
      ? "monthly"
      : frequencySet.has("monthly") && monthlyValue !== null
      ? "monthly"
      : frequencySet.has("weekly") && weeklyValue !== null
      ? "weekly"
      : "total";
  const showWeekly = resolvedDisplayMode === "weekly";
  const showMonthly = resolvedDisplayMode === "monthly";

  useEffect(() => {
    if (currentTerm) {
      // Load bonuses for selected term
      if (currentTerm.bonuses) {
        setBonuses(currentTerm.bonuses);
      } else {
        fetch(`/api/signup/${tenantSlug}/${membership.slug}/term/${currentTerm.id}/bonuses`)
          .then((r) => r.json())
          .then((data) => setBonuses(data.bonuses || []))
          .catch(() => setBonuses([]));
      }
    }
  }, [selectedTermIdx, currentTerm, tenantSlug, membership.slug]);

  return (
    <div>
      <button onClick={onBack} className="mb-6 text-[13px] font-medium flex items-center gap-1.5 group" style={{ color: "var(--brandPrimary)" }}>
        <svg className="h-4 w-4 transition-transform group-hover:-translate-x-0.5" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
        </svg>
        Back to memberships
      </button>

      {/* Hero image - full width, prominent */}
      {membership.top_image_url && (
        <div className="mb-8 rounded-2xl overflow-hidden shadow-md">
          <img
            src={membership.top_image_url}
            alt={membership.title}
            className="w-full h-56 sm:h-72 md:h-80 object-cover"
          />
        </div>
      )}

      <div className="grid grid-cols-1 gap-8 lg:grid-cols-3">
        {/* Main content - walkthrough/selling section */}
        <div className="lg:col-span-2 space-y-8">
          {/* Title and value */}
          <div>
            <h1 className="text-3xl sm:text-4xl font-semibold text-gray-900 mb-3">
              {membershipName(membership) || fallbackTitle(membership)}
            </h1>
            {membershipLevel(membership) && (
              <p className="text-lg sm:text-xl font-medium mb-2.5" style={{ color: "var(--brandPrimary)" }}>
                {membershipLevel(membership)}
              </p>
            )}
            <div className="flex items-baseline gap-3">
              <span className="text-2xl font-medium tabular-nums" style={{ color: "var(--brandPrimary)" }}>
                {showWeekly
                  ? `$${formatMoney(weeklyValue!)}`
                  : showMonthly
                  ? `$${formatMoney(monthlyValue!)}`
                  : `$${((membership as any).intrinsic_value || membership.total_value + bonuses.reduce((s, b) => s + b.value, 0)).toLocaleString()}`}
              </span>
              <span className="text-[13px] text-gray-500">
                {showWeekly ? "per week" : showMonthly ? "per month" : "total membership value"}
              </span>
            </div>
            {bonuses.length > 0 && (
              <p className="text-[12px] text-gray-400 mt-1">
                Includes ${bonuses.reduce((s, b) => s + b.value, 0).toLocaleString()} in bonuses for the {currentTerm?.duration_months}-month term
              </p>
            )}
          </div>

          {/* Description - selling copy */}
          {membership.description && (
            <div>
              <h2 className="text-xl font-semibold text-gray-900 mb-3">About This Membership</h2>
              <p className="text-gray-700 text-[15px] leading-relaxed whitespace-pre-line">
                {membership.description}
              </p>
            </div>
          )}

          {/* Inclusions - what's included */}
          {membership.inclusions && (
            <div>
              <h2 className="text-xl font-semibold text-gray-900 mb-4">What's Included</h2>
              <div className="text-gray-700 text-[15px] leading-relaxed whitespace-pre-line bg-gray-50 rounded-2xl p-6 border border-gray-200">
                {membership.inclusions}
              </div>
            </div>
          )}

          {/* Benefits */}
          {(membership as any).benefits && (
            <div>
              <h2 className="text-xl font-semibold text-gray-900 mb-4">Expected Benefits</h2>
              <div className="text-gray-700 text-[15px] leading-relaxed whitespace-pre-line bg-gradient-to-br from-green-50 to-emerald-50 rounded-2xl p-6 border border-green-200">
                {(membership as any).benefits}
              </div>
            </div>
          )}

          {/* Bonuses section - visual cards */}
          {bonuses.length > 0 && (
            <div>
              <h2 className="text-xl font-semibold text-gray-900 mb-4">
                Exclusive Bonuses with {currentTerm?.duration_months}-Month Term
              </h2>
              <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">
                {bonuses.map((bonus) => (
                  <div key={bonus.id} className="rounded-2xl border border-green-300 bg-gradient-to-br from-green-50 to-green-100 p-5 shadow-sm hover:shadow-md transition-shadow">
                    {bonus.image_url && (
                      <img 
                        src={bonus.image_url} 
                        alt={bonus.title} 
                        className="w-full h-40 object-cover rounded-xl mb-4 shadow-sm" 
                      />
                    )}
                    <h3 className="text-lg font-semibold text-gray-900 mb-2">{bonus.title}</h3>
                    {bonus.description && (
                      <p className="text-gray-700 mb-3 leading-relaxed">{bonus.description}</p>
                    )}
                    <div className="flex items-center justify-between pt-3 border-t border-green-200">
                      <span className="text-sm text-gray-600">Bonus Value</span>
                      <span className="text-xl font-semibold text-green-700 tabular-nums">
                        ${bonus.value.toLocaleString()}
                      </span>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        {/* Sidebar: term selection - sticky for iPad */}
        <div className="lg:col-span-1">
          <div className="sticky top-6">
            <div className="rounded-2xl border border-gray-200 bg-white shadow-md px-5 py-5">
              <h3 className="text-lg font-semibold text-gray-900 mb-4">Choose Your Term</h3>

              <div className="space-y-2.5 mb-5">
                {terms.map((term, idx) => (
                  <button
                    key={term.id}
                    onClick={() => setSelectedTermIdx(idx)}
                    className={`w-full text-left rounded-xl border p-4 transition-all ${
                      idx === selectedTermIdx
                        ? "border-primary-600 bg-primary-50 shadow-md"
                        : "border-gray-200 hover:border-gray-300 hover:shadow-sm"
                    }`}
                  >
                    <div className="flex items-center justify-between mb-1.5">
                      <span className="font-semibold text-gray-900 text-[17px]">
                        {term.duration_months} Months
                      </span>
                      {term.bonuses && term.bonuses.length > 0 && (
                        <span className={`text-xs font-semibold rounded-full px-3 py-1 ${
                          idx === selectedTermIdx 
                            ? "bg-primary-200 text-primary-800" 
                            : "bg-gray-100 text-gray-600"
                        }`}>
                          {term.bonuses.length} bonus{term.bonuses.length !== 1 ? "es" : ""}
                        </span>
                      )}
                    </div>
                    <div className="mt-1 text-xl font-medium text-gray-900 tabular-nums">
                      {resolvedDisplayMode === "weekly" && (term.fee_weekly || toWeekly(term.fee_total, term.duration_months))
                        ? `$${formatMoney(term.fee_weekly || toWeekly(term.fee_total, term.duration_months) || 0)}`
                        : resolvedDisplayMode === "monthly" && (term.fee_monthly || toMonthly(term.fee_total, term.duration_months))
                        ? `$${formatMoney(term.fee_monthly || toMonthly(term.fee_total, term.duration_months) || 0)}`
                        : `$${term.fee_total.toLocaleString()}`}
                    </div>
                    {resolvedDisplayMode === "weekly" && (term.fee_weekly || toWeekly(term.fee_total, term.duration_months)) && (
                      <p className="text-[12px] text-gray-500 mt-1">per week</p>
                    )}
                    {resolvedDisplayMode === "monthly" && (term.fee_monthly || toMonthly(term.fee_total, term.duration_months)) && (
                      <p className="text-[12px] text-gray-500 mt-1">per month</p>
                    )}
                    {resolvedDisplayMode === "total" && term.fee_monthly && (
                      <p className="text-[12px] text-gray-500 mt-1">
                        or ${term.fee_monthly.toLocaleString()}/month
                      </p>
                    )}
                    {term.custom_schedule_enabled && term.custom_schedule_amount && term.custom_schedule_num_payments && term.custom_schedule_interval_count && (
                      <p className="text-xs font-medium mt-1" style={{ color: "var(--brandPrimary)" }}>
                        {term.custom_schedule_label || `${term.custom_schedule_num_payments} payments every ${term.custom_schedule_interval_count} ${term.custom_schedule_interval_unit || "month"}${term.custom_schedule_interval_count === 1 ? "" : "s"}`} (${term.custom_schedule_amount.toLocaleString()} each)
                      </p>
                    )}
                    {(term as any).num_payments && (
                      <p className="text-xs font-medium mt-1" style={{ color: "var(--brandPrimary)" }}>
                        {(term as any).num_payments} payments then membership stays active
                      </p>
                    )}
                  </button>
                ))}
              </div>

              {currentTerm && (
                <button
                  onClick={() => onSelectTerm(currentTerm)}
                  className="w-full rounded-xl text-white text-[15px] py-3 font-semibold shadow-sm hover:shadow-md transition-all"
                  style={{ backgroundColor: "var(--brandPrimary)" }}
                >
                  Continue to Signup →
                </button>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
