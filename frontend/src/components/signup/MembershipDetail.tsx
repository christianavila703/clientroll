import { useState, useEffect } from "react";
import type { MembershipType, TermOption, Bonus } from "./SignupPage";

interface Props {
  membership: MembershipType;
  tenantSlug: string;
  onSelectTerm: (term: TermOption) => void;
  onBack: () => void;
}

export default function MembershipDetail({ membership, tenantSlug, onSelectTerm, onBack }: Props) {
  const [selectedTermIdx, setSelectedTermIdx] = useState(0);
  const [bonuses, setBonuses] = useState<Bonus[]>([]);

  const terms = membership.term_options || [];
  const currentTerm = terms[selectedTermIdx];

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
      <button onClick={onBack} className="mb-6 text-sm text-primary-600 hover:text-primary-700 font-medium">
        ← Back to memberships
      </button>

      {/* Hero */}
      {membership.top_image_url && (
        <img
          src={membership.top_image_url}
          alt={membership.title}
          className="w-full h-56 sm:h-72 object-cover rounded-xl mb-8"
        />
      )}

      <div className="grid grid-cols-1 gap-8 lg:grid-cols-3">
        {/* Main content */}
        <div className="lg:col-span-2">
          <h1 className="text-3xl font-bold text-gray-900 sm:text-4xl">
            {membership.title}
          </h1>

          <div className="mt-2 flex items-baseline gap-2">
            <span className="text-3xl font-bold text-primary-600">
              ${membership.total_value.toLocaleString()}
            </span>
            <span className="text-gray-500">total membership value</span>
          </div>

          {membership.description && (
            <div className="mt-6">
              <h2 className="text-lg font-semibold text-gray-900 mb-2">About This Membership</h2>
              <p className="text-gray-600 whitespace-pre-line">{membership.description}</p>
            </div>
          )}

          {membership.inclusions && (
            <div className="mt-6">
              <h2 className="text-lg font-semibold text-gray-900 mb-2">What's Included</h2>
              <div className="text-gray-600 whitespace-pre-line">{membership.inclusions}</div>
            </div>
          )}

          {/* Bonuses section */}
          {bonuses.length > 0 && (
            <div className="mt-8">
              <h2 className="text-lg font-semibold text-gray-900 mb-4">
                Bonuses Included with {currentTerm?.duration_months}-Month Term
              </h2>
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                {bonuses.map((bonus) => (
                  <div key={bonus.id} className="rounded-xl border border-green-200 bg-green-50 p-4">
                    {bonus.image_url && (
                      <img src={bonus.image_url} alt={bonus.title} className="w-full h-32 object-cover rounded-lg mb-3" />
                    )}
                    <h3 className="font-semibold text-gray-900">{bonus.title}</h3>
                    {bonus.description && (
                      <p className="mt-1 text-sm text-gray-600">{bonus.description}</p>
                    )}
                    <p className="mt-2 text-lg font-bold text-green-600">
                      ${bonus.value.toLocaleString()} value
                    </p>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        {/* Sidebar: term selection */}
        <div className="lg:col-span-1">
          <div className="sticky top-24 card">
            <h3 className="text-lg font-semibold text-gray-900 mb-4">Choose Your Term</h3>

            <div className="space-y-3">
              {terms.map((term, idx) => (
                <button
                  key={term.id}
                  onClick={() => setSelectedTermIdx(idx)}
                  className={`w-full text-left rounded-xl border-2 p-4 transition-all min-h-[44px] ${
                    idx === selectedTermIdx
                      ? "border-primary-500 bg-primary-50"
                      : "border-gray-200 hover:border-gray-300"
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <span className="font-semibold text-gray-900">
                      {term.duration_months} Months
                    </span>
                    <span className={`text-xs rounded-full px-2 py-1 ${
                      idx === selectedTermIdx ? "bg-primary-100 text-primary-700" : "bg-gray-100 text-gray-500"
                    }`}>
                      {term.bonuses?.length || 0} bonus{(term.bonuses?.length || 0) !== 1 ? "es" : ""}
                    </span>
                  </div>
                  <div className="mt-1 text-2xl font-bold text-gray-900">
                    ${term.fee_total.toLocaleString()}
                  </div>
                  {term.fee_monthly && (
                    <p className="text-sm text-gray-500">
                      or ${term.fee_monthly.toLocaleString()}/month
                    </p>
                  )}
                </button>
              ))}
            </div>

            {currentTerm && (
              <button
                onClick={() => onSelectTerm(currentTerm)}
                className="btn-primary w-full mt-6 text-base py-3"
              >
                Continue with {currentTerm.duration_months}-Month Term
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
