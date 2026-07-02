import { useEffect, useMemo, useRef, useState } from "react";

interface AddressParts {
  unit_number: string;
  address: string;
  suburb: string;
  state: string;
  postcode: string;
}

interface Props {
  value: string;
  onChange: (value: string) => void;
  onAddressSelect: (parts: AddressParts) => void;
  apiKey?: string;
  placeholder?: string;
  className?: string;
}

interface Suggestion {
  placeId: string;
  text: string;
}

interface PlacesAutocompleteResponse {
  suggestions?: Array<{
    placePrediction?: {
      placeId?: string;
      text?: { text?: string };
    };
  }>;
}

interface PlaceDetailsResponse {
  formattedAddress?: string;
  addressComponents?: Array<{
    longText?: string;
    shortText?: string;
    types?: string[];
  }>;
}

function parseAddressParts(place: PlaceDetailsResponse): AddressParts {
  const components = place.addressComponents || [];

  const getByType = (type: string) =>
    components.find((c) => Array.isArray(c.types) && c.types.includes(type));

  const streetNumber = getByType("street_number")?.longText || "";
  const route = getByType("route")?.longText || "";
  const unitNumber = getByType("subpremise")?.longText || "";
  const locality =
    getByType("locality")?.longText ||
    getByType("sublocality_level_1")?.longText ||
    getByType("postal_town")?.longText ||
    "";
  const state = getByType("administrative_area_level_1")?.shortText || "";
  const postcode = getByType("postal_code")?.longText || "";

  const street = [streetNumber, route].filter(Boolean).join(" ").trim();
  const fallbackAddress = (place.formattedAddress || "").split(",")[0]?.trim() || "";

  return {
    unit_number: unitNumber,
    address: street || fallbackAddress,
    suburb: locality,
    state,
    postcode,
  };
}

export default function AddressAutocompleteInput({
  value,
  onChange,
  onAddressSelect,
  apiKey,
  placeholder = "Start typing an address...",
  className = "input-field",
}: Props) {
  const [suggestions, setSuggestions] = useState<Suggestion[]>([]);
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const sessionTokenRef = useRef<string>("");
  const rootRef = useRef<HTMLDivElement>(null);
  const debounceRef = useRef<number | null>(null);

  useEffect(() => {
    if (!sessionTokenRef.current) {
      sessionTokenRef.current = crypto.randomUUID();
    }
  }, []);

  useEffect(() => {
    const onClick = (e: MouseEvent) => {
      if (!rootRef.current) return;
      if (!rootRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", onClick);
    return () => document.removeEventListener("mousedown", onClick);
  }, []);

  const canLookup = useMemo(() => Boolean(apiKey && apiKey.trim()), [apiKey]);

  const fetchSuggestions = async (input: string) => {
    if (!canLookup || input.trim().length < 3) {
      setSuggestions([]);
      return;
    }
    setLoading(true);
    setError("");
    try {
      const res = await fetch("https://places.googleapis.com/v1/places:autocomplete", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Goog-Api-Key": apiKey || "",
          "X-Goog-FieldMask":
            "suggestions.placePrediction.placeId,suggestions.placePrediction.text.text",
        },
        body: JSON.stringify({
          input,
          includedRegionCodes: ["au"],
          languageCode: "en",
          sessionToken: sessionTokenRef.current,
        }),
      });
      if (!res.ok) throw new Error("autocomplete_failed");
      const data = (await res.json()) as PlacesAutocompleteResponse;
      const next =
        (data.suggestions || [])
          .map((s) => ({
            placeId: s.placePrediction?.placeId || "",
            text: s.placePrediction?.text?.text || "",
          }))
          .filter((s) => s.placeId && s.text) || [];
      setSuggestions(next);
    } catch {
      setSuggestions([]);
      setError("Address lookup unavailable");
    } finally {
      setLoading(false);
    }
  };

  const handleChange = (next: string) => {
    onChange(next);
    setOpen(true);
    if (debounceRef.current) {
      window.clearTimeout(debounceRef.current);
    }
    debounceRef.current = window.setTimeout(() => {
      void fetchSuggestions(next);
    }, 250);
  };

  const handleSelect = async (suggestion: Suggestion) => {
    setOpen(false);
    onChange(suggestion.text);
    if (!canLookup) return;
    try {
      const res = await fetch(
        `https://places.googleapis.com/v1/places/${encodeURIComponent(suggestion.placeId)}?languageCode=en`,
        {
          headers: {
            "X-Goog-Api-Key": apiKey || "",
            "X-Goog-FieldMask": "addressComponents,formattedAddress",
          },
        }
      );
      if (!res.ok) return;
      const data = (await res.json()) as PlaceDetailsResponse;
      onAddressSelect(parseAddressParts(data));
      // New token per completed place session.
      sessionTokenRef.current = crypto.randomUUID();
    } catch {
      // Keep manually selected text even if details fail.
    }
  };

  return (
    <div className="relative" ref={rootRef}>
      <input
        value={value}
        onChange={(e) => handleChange(e.target.value)}
        onFocus={() => setOpen(true)}
        className={className}
        placeholder={placeholder}
        autoComplete="off"
      />
      {open && canLookup && (suggestions.length > 0 || loading || error) && (
        <div className="absolute z-50 mt-1 max-h-64 w-full overflow-y-auto rounded-md border border-gray-200 bg-white shadow-lg">
          {loading && <div className="px-3 py-2 text-[12px] text-gray-500">Looking up addresses...</div>}
          {!loading && error && <div className="px-3 py-2 text-[12px] text-amber-600">{error}</div>}
          {!loading &&
            !error &&
            suggestions.map((s) => (
              <button
                key={s.placeId}
                type="button"
                onClick={() => void handleSelect(s)}
                className="block w-full px-3 py-2 text-left text-[13px] text-gray-700 hover:bg-gray-50"
              >
                {s.text}
              </button>
            ))}
        </div>
      )}
    </div>
  );
}
