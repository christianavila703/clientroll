import type { MembershipType } from "./SignupPage";

interface Props {
  memberships: MembershipType[];
  tenantSlug: string;
  onSelect: (mt: MembershipType) => void;
}

export default function MembershipSelect({ memberships, onSelect }: Props) {
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
      <div className="text-center mb-12">
        <h1 className="text-4xl font-bold text-gray-900 sm:text-5xl">
          Choose Your Membership
        </h1>
        <p className="mt-4 text-lg text-gray-500 max-w-xl mx-auto">
          Select the membership that best fits your goals
        </p>
      </div>

      <div className="grid grid-cols-1 gap-8 md:grid-cols-2 lg:grid-cols-3">
        {memberships.map((mt) => (
          <button
            key={mt.id}
            onClick={() => onSelect(mt)}
            className="group text-left rounded-2xl border border-gray-200 bg-white overflow-hidden hover:shadow-xl hover:border-indigo-300 transition-all duration-200"
          >
            {mt.top_image_url && (
              <img
                src={mt.top_image_url}
                alt={mt.title}
                className="w-full h-48 object-cover"
              />
            )}
            <div className="px-7 py-6">
              <h3 className="text-xl font-bold text-gray-900 group-hover:text-indigo-600 transition-colors">
                {mt.title}
              </h3>
              {mt.description && (
                <p className="mt-3 text-[14px] text-gray-500 leading-relaxed line-clamp-3">
                  {mt.description}
                </p>
              )}
              <div className="mt-5 flex items-baseline gap-2">
                <span className="text-3xl font-bold text-indigo-600 tabular-nums">
                  ${mt.total_value.toLocaleString()}
                </span>
                <span className="text-[13px] text-gray-400">total value</span>
              </div>
              {mt.term_options && mt.term_options.length > 0 && (
                <p className="mt-2 text-[12px] text-gray-400">
                  {mt.term_options.length} term option{mt.term_options.length !== 1 ? "s" : ""} available
                </p>
              )}
              <div className="mt-6 w-full rounded-xl bg-indigo-600 text-white text-center py-3.5 text-[14px] font-semibold group-hover:bg-indigo-700 transition-colors">
                View Details
              </div>
            </div>
          </button>
        ))}
      </div>
    </div>
  );
}
