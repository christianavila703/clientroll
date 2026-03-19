import type { MembershipType } from "./SignupPage";

interface Props {
  memberships: MembershipType[];
  tenantSlug: string;
  priceDisplayMode: "total" | "weekly" | "monthly";
  onSelect: (mt: MembershipType) => void;
}

export default function MembershipSelect({ memberships, priceDisplayMode, onSelect }: Props) {
  const membershipName = (mt: MembershipType) => (mt.membership_name || "").trim();
  const membershipLevel = (mt: MembershipType) => (mt.membership_level || "").trim();
  const fallbackTitle = (mt: MembershipType) => mt.title;
  const levelRank: Record<string, number> = {
    SUPPORT: 1,
    RESET: 1,
    KICKSTART: 1,
    SHAPE: 1,
    "TONE LITE": 1,
    THERAPEUTIC: 2,
    FLOW: 2,
    TRANSFORM: 2,
    SCULPT: 2,
    "TONE SCULPT": 2,
    INTENSIVE: 3,
    "WELLBEING LIFESTYLE": 3,
    ACCELERATE: 3,
    "ELITE SCULPT": 3,
    "TONE PRO": 3,
  };
  const groupedMemberships = (() => {
    const groups = new Map<string, MembershipType[]>();
    for (const mt of memberships) {
      const key = membershipName(mt) || fallbackTitle(mt);
      if (!groups.has(key)) groups.set(key, []);
      groups.get(key)!.push(mt);
    }
    const ordered: MembershipType[] = [];
    for (const [, groupItems] of groups) {
      const sortedGroup = [...groupItems].sort((a, b) => {
        const aLevel = membershipLevel(a).toUpperCase();
        const bLevel = membershipLevel(b).toUpperCase();
        const aRank = levelRank[aLevel] ?? 999;
        const bRank = levelRank[bLevel] ?? 999;
        if (aRank !== bRank) return aRank - bRank;
        return (membershipLevel(a) || fallbackTitle(a)).localeCompare(membershipLevel(b) || fallbackTitle(b));
      });
      ordered.push(...sortedGroup);
    }
    return ordered;
  })();
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
  const minWeekly = (mt: MembershipType): number | null => {
    const values = (mt.term_options || [])
      .map((to) => to.fee_weekly || toWeekly(to.fee_total, to.duration_months))
      .filter((v): v is number => typeof v === "number" && v > 0);
    if (!values.length) return null;
    return Math.min(...values);
  };
  const minMonthly = (mt: MembershipType): number | null => {
    const values = (mt.term_options || [])
      .map((to) => to.fee_monthly || toMonthly(to.fee_total, to.duration_months))
      .filter((v): v is number => typeof v === "number" && v > 0);
    if (!values.length) return null;
    return Math.min(...values);
  };

  if (memberships.length === 0) {
    return (
      <div className="text-center py-20">
        <svg className="mx-auto h-16 w-16 text-gray-200" fill="none" stroke="currentColor" strokeWidth={1} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" /></svg>
        <p className="mt-4 text-lg text-gray-500">No memberships available at this time.</p>
      </div>
    );
  }

  return (
    <div>
      <div className="text-center mb-10">
        <h1 className="text-3xl font-semibold text-gray-900 sm:text-4xl">
          Choose Your Membership
        </h1>
        <p className="mt-3 text-[15px] text-gray-500 max-w-xl mx-auto">
          Select the membership that best fits your goals
        </p>
      </div>

      <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
        {groupedMemberships.map((mt) => (
          <button
            key={mt.id}
            onClick={() => onSelect(mt)}
            className={`group text-left rounded-2xl border bg-white overflow-hidden hover:shadow-lg transition-all duration-200 brand-hover-border ${
              mt.is_most_popular ? "shadow-sm" : "border-gray-200"
            }`}
            style={mt.is_most_popular ? { borderColor: "color-mix(in srgb, var(--brandPrimary) 40%, transparent)" } : undefined}
          >
            {mt.top_image_url && (
              <img
                src={mt.top_image_url}
                alt={membershipName(mt) || fallbackTitle(mt)}
                className="w-full h-48 object-cover"
              />
            )}
            <div className="px-6 py-5">
              {mt.is_most_popular && (
                <div className="mb-3">
                  <span className="badge badge-purple">Most Popular</span>
                </div>
              )}
              <h3 className="text-[19px] font-semibold text-gray-900 transition-colors brand-hover-text">
                {membershipName(mt) || fallbackTitle(mt)}
              </h3>
              {membershipLevel(mt) && (
                <p className="mt-1 text-[13px] font-medium" style={{ color: "var(--brandPrimary)" }}>
                  {membershipLevel(mt)}
                </p>
              )}
              {((mt as any).summary || mt.description) && (
                <p className="mt-2.5 text-[13px] text-gray-500 leading-relaxed line-clamp-3">
                  {(mt as any).summary || mt.description}
                </p>
              )}
              <div className="mt-4 flex items-baseline gap-2">
                <span className="text-[22px] font-medium tabular-nums" style={{ color: "var(--brandPrimary)" }}>
                  {priceDisplayMode === "weekly" && minWeekly(mt) !== null
                    ? `$${formatMoney(minWeekly(mt)!)}`
                    : priceDisplayMode === "monthly" && minMonthly(mt) !== null
                    ? `$${formatMoney(minMonthly(mt)!)}`
                    : `$${(() => {
                        const opts = mt.term_options || [];
                        if (opts.length > 0) {
                          const fees = opts.map(o => o.fee_total).sort((a, b) => a - b);
                          return fees.length === 1 || fees[0] === fees[fees.length - 1]
                            ? fees[0].toLocaleString()
                            : `${fees[0].toLocaleString()} – $${fees[fees.length - 1].toLocaleString()}`;
                        }
                        return "0";
                      })()}`}
                </span>
                <span className="text-[12px] text-gray-400">
                  {priceDisplayMode === "weekly" && minWeekly(mt) !== null
                    ? "per week"
                    : priceDisplayMode === "monthly" && minMonthly(mt) !== null
                    ? "per month"
                    : "membership value"}
                </span>
              </div>
              {mt.term_options && mt.term_options.length > 0 && (
                <p className="mt-1.5 text-[11px] text-gray-400">
                  {mt.term_options.length} term option{mt.term_options.length !== 1 ? "s" : ""} available
                </p>
              )}
              <div
                className="mt-5 w-full rounded-xl text-white text-center py-3 text-[13px] font-semibold transition-colors"
                style={{ backgroundColor: "var(--brandPrimary)" }}
              >
                View Details
              </div>
            </div>
          </button>
        ))}
      </div>
    </div>
  );
}
