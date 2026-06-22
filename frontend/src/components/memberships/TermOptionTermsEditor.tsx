import { useEffect, useState } from "react";

import api from "../../api/client";
import type { AttachedTerms, TermOption } from "../../types/memberships";

interface TermsDoc {
  id: string;
  title: string;
  version: number;
  is_active: boolean;
}

interface Row {
  terms_id: string;
  is_required: boolean;
  sort_order: number;
}

interface Props {
  termOption: TermOption;
  allTerms: TermsDoc[];
  onSaved?: () => void;
}

const initialRows = (to: TermOption): Row[] => {
  const attached: AttachedTerms[] = to.attached_terms ?? [];
  return attached
    .slice()
    .sort((a, b) => a.attach_sort_order - b.attach_sort_order)
    .map(a => ({
      terms_id: a.id,
      is_required: a.is_required,
      sort_order: a.attach_sort_order,
    }));
};

export default function TermOptionTermsEditor({ termOption, allTerms, onSaved }: Props) {
  const [rows, setRows] = useState<Row[]>(() => initialRows(termOption));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [dirty, setDirty] = useState(false);

  useEffect(() => {
    setRows(initialRows(termOption));
    setError("");
    setMessage("");
    setDirty(false);
  }, [termOption.id]);

  const activeTerms = allTerms.filter(t => t.is_active);
  const checked = new Set(rows.map(r => r.terms_id));

  const toggle = (id: string) => {
    setDirty(true);
    setError("");
    setMessage("");
    if (checked.has(id)) {
      setRows(rows.filter(r => r.terms_id !== id));
    } else {
      setRows([
        ...rows,
        {
          terms_id: id,
          is_required: true,
          sort_order: rows.length,
        },
      ]);
    }
  };

  const toggleRequired = (id: string, required: boolean) => {
    setDirty(true);
    setRows(rows.map(r => (r.terms_id === id ? { ...r, is_required: required } : r)));
  };

  const move = (id: string, delta: -1 | 1) => {
    const idx = rows.findIndex(r => r.terms_id === id);
    if (idx < 0) return;
    const target = idx + delta;
    if (target < 0 || target >= rows.length) return;
    const next = [...rows];
    [next[idx], next[target]] = [next[target], next[idx]];
    setDirty(true);
    setRows(next.map((r, i) => ({ ...r, sort_order: i })));
  };

  const handleSave = async () => {
    setError("");
    setMessage("");
    setSaving(true);
    try {
      const payload = rows.map((r, i) => ({
        terms_id: r.terms_id,
        is_required: r.is_required,
        sort_order: i,
      }));
      await api.replaceTermOptionTerms(termOption.id, payload);
      setDirty(false);
      setMessage("Saved.");
      onSaved?.();
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : "Failed to save attached terms";
      setError(msg);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="mt-3 border border-emerald-100 rounded-lg p-4 bg-emerald-50/30 space-y-3 text-[13px]">
      <div className="flex items-center justify-between">
        <div>
          <h4 className="font-semibold text-gray-900">Attached Terms & Conditions</h4>
          <p className="text-[12px] text-gray-500">
            Tick every T&C the member must accept before signing this specific term option.
          </p>
        </div>
        <button
          type="button"
          onClick={handleSave}
          disabled={saving || !dirty}
          className="px-3 py-1.5 rounded-md bg-emerald-600 text-white text-[12px] disabled:bg-gray-300"
        >
          {saving ? "Saving..." : "Save terms"}
        </button>
      </div>

      {error && <div className="bg-red-50 text-red-700 text-[12px] px-2 py-1 rounded">{error}</div>}
      {message && <div className="bg-green-50 text-green-700 text-[12px] px-2 py-1 rounded">{message}</div>}

      {activeTerms.length === 0 && (
        <p className="text-[12px] text-gray-500">
          No active terms documents exist yet. Create one in the Terms page first.
        </p>
      )}

      <div className="space-y-1">
        {activeTerms.map(t => {
          const row = rows.find(r => r.terms_id === t.id);
          const isAttached = !!row;
          const orderIdx = isAttached ? rows.findIndex(r => r.terms_id === t.id) : -1;
          return (
            <div
              key={t.id}
              className={`flex items-center gap-2 p-2 rounded-md border ${
                isAttached ? "border-emerald-300 bg-white" : "border-gray-200 bg-white/60"
              }`}
            >
              <input type="checkbox" checked={isAttached} onChange={() => toggle(t.id)} />
              <span className="flex-1 text-gray-800">
                {t.title}{" "}
                <span className="text-[11px] text-gray-400">v{t.version}</span>
              </span>
              {isAttached && (
                <>
                  <label className="flex items-center gap-1 text-[11px] text-gray-600">
                    <input
                      type="checkbox"
                      checked={row?.is_required ?? true}
                      onChange={e => toggleRequired(t.id, e.target.checked)}
                    />
                    Required
                  </label>
                  <div className="flex gap-1">
                    <button
                      type="button"
                      className="text-[11px] px-2 py-0.5 rounded border border-gray-200 disabled:opacity-30"
                      onClick={() => move(t.id, -1)}
                      disabled={orderIdx <= 0}
                    >
                      Up
                    </button>
                    <button
                      type="button"
                      className="text-[11px] px-2 py-0.5 rounded border border-gray-200 disabled:opacity-30"
                      onClick={() => move(t.id, 1)}
                      disabled={orderIdx === rows.length - 1}
                    >
                      Down
                    </button>
                  </div>
                </>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
