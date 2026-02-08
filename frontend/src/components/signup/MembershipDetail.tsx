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
    <div className="max-w-6xl mx-auto">
      <button onClick={onBack} className="mb-6 text-sm text-primary-600 hover:text-primary-700 font-medium flex items-center gap-1">
        <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
        </svg>
        Back to memberships
      </button>

      {/* Hero image - full width, prominent */}
      {membership.top_image_url && (
        <div className="mb-8 rounded-2xl overflow-hidden shadow-lg">
          <img
            src={membership.top_image_url}
            alt={membership.title}
            className="w-full h-64 sm:h-80 md:h-96 object-cover"
          />
        </div>
      )}

      <div className="grid grid-cols-1 gap-8 lg:grid-cols-3">
        {/* Main content - walkthrough/selling section */}
        <div className="lg:col-span-2 space-y-8">
          {/* Title and value */}
          <div>
            <h1 className="text-4xl sm:text-5xl font-bold text-gray-900 mb-3">
              {membership.title}
            </h1>
            <div className="flex items-baseline gap-2">
              <span className="text-4xl font-bold text-primary-600 tabular-nums">
                ${membership.total_value.toLocaleString()}
              </span>
              <span className="text-lg text-gray-500">total membership value</span>
            </div>
          </div>

          {/* Description - selling copy */}
          {membership.description && (
            <div className="prose prose-lg max-w-none">
              <h2 className="text-2xl font-bold text-gray-900 mb-3">About This Membership</h2>
              <div className="text-gray-700 text-lg leading-relaxed whitespace-pre-line">
                {membership.description}
              </div>
            </div>
          )}

          {/* Inclusions - what's included */}
          {membership.inclusions && (
            <div>
              <h2 className="text-2xl font-bold text-gray-900 mb-4">What's Included</h2>
              <div className="text-gray-700 text-lg leading-relaxed whitespace-pre-line bg-gray-50 rounded-xl p-6 border border-gray-200">
                {membership.inclusions}
              </div>
            </div>
          )}

          {/* Bonuses section - visual cards */}
          {bonuses.length > 0 && (
            <div>
              <h2 className="text-2xl font-bold text-gray-900 mb-4">
                Exclusive Bonuses with {currentTerm?.duration_months}-Month Term
              </h2>
              <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">
                {bonuses.map((bonus) => (
                  <div key={bonus.id} className="rounded-2xl border-2 border-green-300 bg-gradient-to-br from-green-50 to-green-100 p-5 shadow-md hover:shadow-lg transition-shadow">
                    {bonus.image_url && (
                      <img 
                        src={bonus.image_url} 
                        alt={bonus.title} 
                        className="w-full h-40 object-cover rounded-xl mb-4 shadow-sm" 
                      />
                    )}
                    <h3 className="text-xl font-bold text-gray-900 mb-2">{bonus.title}</h3>
                    {bonus.description && (
                      <p className="text-gray-700 mb-3 leading-relaxed">{bonus.description}</p>
                    )}
                    <div className="flex items-center justify-between pt-3 border-t border-green-200">
                      <span className="text-sm text-gray-600">Bonus Value</span>
                      <span className="text-2xl font-bold text-green-700 tabular-nums">
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
            <div className="card shadow-lg border-2 border-gray-200">
              <h3 className="text-xl font-bold text-gray-900 mb-5">Choose Your Term</h3>

              <div className="space-y-3 mb-6">
                {terms.map((term, idx) => (
                  <button
                    key={term.id}
                    onClick={() => setSelectedTermIdx(idx)}
                    className={`w-full text-left rounded-xl border-2 p-5 transition-all min-h-[60px] ${
                      idx === selectedTermIdx
                        ? "border-primary-600 bg-primary-50 shadow-md"
                        : "border-gray-200 hover:border-gray-300 hover:shadow-sm"
                    }`}
                  >
                    <div className="flex items-center justify-between mb-2">
                      <span className="font-bold text-gray-900 text-lg">
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
                    <div className="mt-1 text-3xl font-bold text-gray-900 tabular-nums">
                      ${term.fee_total.toLocaleString()}
                    </div>
                    {term.fee_monthly && (
                      <p className="text-sm text-gray-500 mt-1">
                        or ${term.fee_monthly.toLocaleString()}/month
                      </p>
                    )}
                  </button>
                ))}
              </div>

              {currentTerm && (
                <button
                  onClick={() => onSelectTerm(currentTerm)}
                  className="btn-primary w-full text-lg py-4 font-semibold shadow-md hover:shadow-lg transition-all"
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
