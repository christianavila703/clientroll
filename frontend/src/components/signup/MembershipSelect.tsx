import type { MembershipType } from "./SignupPage";

interface Props {
  memberships: MembershipType[];
  tenantSlug: string;
  onSelect: (mt: MembershipType) => void;
}

export default function MembershipSelect({ memberships, onSelect }: Props) {
  if (memberships.length === 0) {
    return (
      <div className="text-center py-16">
        <p className="text-lg text-gray-500">No memberships available at this time.</p>
      </div>
    );
  }

  return (
    <div>
      <div className="text-center mb-8">
        <h1 className="text-3xl font-bold text-gray-900 sm:text-4xl">
          Choose Your Membership
        </h1>
        <p className="mt-3 text-lg text-gray-600">
          Select the membership that best fits your goals
        </p>
      </div>

      <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
        {memberships.map((mt) => (
          <button
            key={mt.id}
            onClick={() => onSelect(mt)}
            className="group text-left card hover:shadow-lg hover:border-primary-300 transition-all"
          >
            {mt.top_image_url && (
              <img
                src={mt.top_image_url}
                alt={mt.title}
                className="w-full h-48 object-cover rounded-lg mb-4"
              />
            )}
            <h3 className="text-xl font-bold text-gray-900 group-hover:text-primary-600 transition-colors">
              {mt.title}
            </h3>
            {mt.description && (
              <p className="mt-2 text-sm text-gray-600 line-clamp-3">
                {mt.description}
              </p>
            )}
            <div className="mt-4 flex items-baseline gap-1">
              <span className="text-2xl font-bold text-primary-600">
                ${mt.total_value.toLocaleString()}
              </span>
              <span className="text-sm text-gray-500">total value</span>
            </div>
            {mt.term_options && mt.term_options.length > 0 && (
              <p className="mt-2 text-xs text-gray-400">
                {mt.term_options.length} term option{mt.term_options.length !== 1 ? "s" : ""} available
              </p>
            )}
            <div className="mt-4 btn-primary w-full text-center">
              View Details
            </div>
          </button>
        ))}
      </div>
    </div>
  );
}
