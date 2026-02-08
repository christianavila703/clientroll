import { useState, useEffect } from "react";
import api from "../../api/client";
import RichTextEditor from "../common/RichTextEditor";

interface Terms {
  id: string;
  title: string;
  content: string;
  version: number;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export default function TermsPage() {
  const [terms, setTerms] = useState<Terms[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<Terms | null>(null);
  const [search, setSearch] = useState("");

  const loadTerms = async () => {
    try {
      const data = (await api.listTerms()) as { terms: Terms[] };
      setTerms(data.terms || []);
    } catch { /* handle */ }
    finally { setLoading(false); }
  };

  useEffect(() => { loadTerms(); }, []);

  const filtered = terms.filter((t) =>
    !search || t.title.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <div>
      {/* Header */}
      <div className="page-header">
        <h1 className="page-title">Terms & Conditions</h1>
        <button onClick={() => { setEditing(null); setShowForm(!showForm); }} className="btn-primary">
          <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" /></svg>
          New terms
        </button>
      </div>

      {showForm && (
        <TermsForm
          initial={editing}
          onSaved={() => { setShowForm(false); setEditing(null); loadTerms(); }}
          onCancel={() => { setShowForm(false); setEditing(null); }}
        />
      )}

      {/* Search + count */}
      <div className="flex items-center gap-3 mb-4">
        <div className="relative max-w-xs w-full">
          <svg className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" /></svg>
          <input
            type="text" value={search} onChange={(e) => setSearch(e.target.value)}
            placeholder="Search..."
            className="w-full rounded-md border border-gray-200 bg-gray-50 py-1.5 pl-8 pr-3 text-[13px] placeholder:text-gray-400 focus:border-gray-300 focus:bg-white focus:outline-none"
          />
        </div>
        <span className="text-[13px] text-gray-400">{filtered.length} results</span>
      </div>

      {/* Table */}
      {loading ? (
        <div className="card px-5 py-12 text-center text-[13px] text-gray-400">Loading...</div>
      ) : filtered.length === 0 ? (
        <div className="card px-5 py-12 text-center">
          <p className="text-[13px] text-gray-500">No terms & conditions found</p>
          {terms.length === 0 && (
            <button onClick={() => setShowForm(true)} className="mt-3 text-[13px] font-medium text-indigo-600 hover:text-indigo-700">
              + Create your first T&C document
            </button>
          )}
        </div>
      ) : (
        <div className="card overflow-hidden">
          <table className="data-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Last Updated</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((t) => (
                <tr
                  key={t.id}
                  className="cursor-pointer"
                  onClick={() => { setEditing(t); setShowForm(true); }}
                >
                  <td>
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-gray-900">{t.title}</span>
                      {t.is_active && (
                        <span className="badge badge-green">Active</span>
                      )}
                      {t.version > 1 && (
                        <span className="text-[11px] text-gray-400">v{t.version}</span>
                      )}
                    </div>
                  </td>
                  <td className="text-gray-500">
                    {new Date(t.updated_at || t.created_at).toLocaleDateString("en-AU", {
                      day: "numeric", month: "long", year: "numeric"
                    })}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function TermsForm({ initial, onSaved, onCancel }: {
  initial: Terms | null;
  onSaved: () => void;
  onCancel: () => void;
}) {
  const [title, setTitle] = useState(initial?.title || "");
  const [content, setContent] = useState(initial?.content || "");
  const [isActive, setIsActive] = useState(initial?.is_active ?? true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!title || !content) { setError("Title and content are required"); return; }
    setSaving(true); setError("");
    try {
      if (initial) {
        await api.updateTerms(initial.id, { title, content, is_active: isActive });
      } else {
        await api.createTerms({ title, content });
      }
      onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save");
    } finally { setSaving(false); }
  };

  return (
    <div className="card mb-5 overflow-hidden">
      <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200">
        <h3 className="text-sm font-semibold text-gray-900">
          {initial ? `Edit: ${initial.title}` : "New Terms & Conditions"}
        </h3>
        <div className="flex items-center gap-2">
          <button type="button" onClick={onCancel} className="btn-secondary text-[12px]">Cancel</button>
          <button type="button" onClick={() => { const f = document.getElementById("terms-form") as HTMLFormElement; f?.requestSubmit(); }} disabled={saving} className="btn-primary text-[12px]">
            {saving ? "Saving..." : "Save"}
          </button>
        </div>
      </div>
      {error && <div className="mx-5 mt-4 rounded-md bg-red-50 border border-red-200 px-3 py-2 text-[13px] text-red-600">{error}</div>}
      <form id="terms-form" onSubmit={handleSubmit} className="px-5 py-4 space-y-4">
        <div>
          <label className="block text-[13px] font-medium text-gray-700 mb-1">Title *</label>
          <input value={title} onChange={(e) => setTitle(e.target.value)} className="input-field" required placeholder="e.g. Standard Membership Agreement" />
        </div>
        <div>
          <label className="block text-[13px] font-medium text-gray-700 mb-1">Content *</label>
          <p className="text-[11px] text-gray-400 mb-1.5">Full terms and conditions text. Clients will be shown this during signup.</p>
          <RichTextEditor
            content={content}
            onChange={setContent}
            placeholder="Enter your terms and conditions here..."
          />
        </div>
        {initial && (
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="tc-active"
              checked={isActive}
              onChange={(e) => setIsActive(e.target.checked)}
              className="h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"
            />
            <label htmlFor="tc-active" className="text-[13px] text-gray-700">Active</label>
            {initial.version > 1 && <span className="text-[11px] text-gray-400 ml-2">Saving will create version {initial.version + 1}</span>}
          </div>
        )}
      </form>
    </div>
  );
}
