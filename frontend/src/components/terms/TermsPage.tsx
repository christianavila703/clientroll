import { useState, useEffect, useRef } from "react";
import api from "../../api/client";
import RichTextEditor from "../common/RichTextEditor";
import type { Editor } from "@tiptap/react";

const TC_VARIABLES = [
  { key: "client_name", label: "Client Name", desc: "Full name of the member" },
  { key: "client_email", label: "Client Email", desc: "Member's email address" },
  { key: "client_phone", label: "Client Phone", desc: "Member's phone number" },
  { key: "client_address", label: "Client Address", desc: "Member's full address" },
  { key: "client_dob", label: "Date of Birth", desc: "Member's date of birth" },
  { key: "client_gender", label: "Gender", desc: "Member's gender" },
  { key: "emergency_contact_name", label: "Emergency Contact Name", desc: "Member's emergency contact name" },
  { key: "emergency_contact_phone", label: "Emergency Contact Phone", desc: "Member's emergency contact phone" },
  { key: "membership_title", label: "Membership Title", desc: "Name of the membership type" },
  { key: "term_months", label: "Term (Months)", desc: "Duration in months (e.g. 6, 12)" },
  { key: "term_weeks", label: "Term (Weeks)", desc: "Duration in weeks (e.g. 26, 52)" },
  { key: "total_fee", label: "Total Fee", desc: "Total contract value" },
  { key: "payment_amount", label: "Payment Amount", desc: "Per-period payment amount" },
  { key: "payment_frequency", label: "Payment Frequency", desc: "Weekly, fortnightly, or monthly" },
  { key: "start_date", label: "Start Date", desc: "When the membership begins" },
  { key: "end_date", label: "End Date", desc: "When the minimum term ends" },
  { key: "payment_method", label: "Payment Method", desc: "Card, direct debit, etc." },
  { key: "company_name", label: "Company Name", desc: "Your business name" },
  { key: "company_trading_name", label: "Trading Name", desc: "Your business trading name" },
  { key: "company_abn", label: "ABN", desc: "Australian Business Number" },
  { key: "company_unit_number", label: "Company Unit Number", desc: "Unit or suite (member business address)" },
  { key: "company_address", label: "Company Street Address", desc: "Street line only (member business address)" },
  { key: "company_email", label: "Company Email", desc: "Your business email" },
  { key: "company_phone", label: "Company Phone", desc: "Your business phone" },
  { key: "today_date", label: "Today's Date", desc: "Date the agreement is signed" },
];

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

  const handleDelete = async (id: string, title: string) => {
    const ok = window.confirm(`Delete "${title}"?\n\nThis will remove it from any memberships it’s attached to.`);
    if (!ok) return;
    try {
      await api.deleteTerms(id);
      await loadTerms();
    } catch (e) {
      console.error(e);
      alert(e instanceof Error ? e.message : "Unable to delete terms. Please try again.");
    }
  };

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
              Create your first T&C document
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
                <th className="w-12"></th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((t) => (
                <tr
                  key={t.id}
                  className="cursor-pointer"
                  onClick={() => {
                    if (showForm && editing?.id === t.id) { setShowForm(false); setEditing(null); }
                    else { setEditing(t); setShowForm(true); }
                  }}
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
                  <td onClick={(e) => e.stopPropagation()}>
                    <button
                      onClick={() => handleDelete(t.id, t.title)}
                      className="rounded p-1 text-gray-400 hover:text-red-500 hover:bg-red-50"
                      title="Delete"
                    >
                      <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" /></svg>
                    </button>
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
  const editorRef = useRef<Editor | null>(null);

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
      setError(err instanceof Error ? err.message : "Unable to save terms.");
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
          <button type="button" onClick={() => { const f = document.getElementById("terms-form") as HTMLFormElement; f?.requestSubmit(); }} disabled={saving} className="btn-primary">
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

        {/* Variables panel */}
        <div className="rounded-lg bg-indigo-50 border border-indigo-100 px-4 py-3">
          <div className="flex items-center gap-2 mb-2">
            <svg className="h-4 w-4 text-indigo-500" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M17.25 6.75L22.5 12l-5.25 5.25m-10.5 0L1.5 12l5.25-5.25m7.5-3l-4.5 16.5" /></svg>
            <p className="text-[12px] font-semibold text-indigo-800">Variables</p>
            <span className="text-[11px] text-indigo-500">— Click to insert into content</span>
          </div>
          <div className="flex flex-wrap gap-1.5">
            {TC_VARIABLES.map((v) => (
              <button
                key={v.key}
                type="button"
                onClick={() => {
                  const tag = `{{${v.key}}}`;
                  const editor = editorRef.current;
                  if (editor) {
                    editor.chain().focus().insertContent(tag).run();
                    return;
                  }
                  // Fallback: append to end.
                  setContent((prev) => (prev ? prev.replace(/<\/p>$/, ` ${tag}</p>`) : `<p>${tag}</p>`));
                }}
                className="rounded-md bg-white border border-indigo-200 px-2.5 py-1 text-[11px] font-mono text-indigo-700 hover:border-indigo-400 hover:bg-indigo-100 transition-colors shadow-sm cursor-pointer"
                title={v.desc}
              >
                {"{{"}
                {v.key}
                {"}}"}
              </button>
            ))}
          </div>
          <p className="text-[10px] text-indigo-400 mt-2">These will be replaced with actual values when the T&C is shown to a client during signup.</p>
        </div>

        <div>
          <label className="block text-[13px] font-medium text-gray-700 mb-1">Content *</label>
          <p className="text-[11px] text-gray-400 mb-1.5">Full terms and conditions text. Use the variables above to personalise the document.</p>
          <RichTextEditor
            content={content}
            onChange={setContent}
            placeholder="Enter your terms and conditions here..."
            onReady={(ed) => {
              editorRef.current = ed;
            }}
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
