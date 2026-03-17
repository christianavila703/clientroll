import { useEffect, useMemo, useRef, useState } from "react";
import type { Editor } from "@tiptap/react";
import api, { ApiError, type EmailTemplateConfig } from "../../api/client";
import RichTextEditor from "../common/RichTextEditor";

type Template = EmailTemplateConfig;

const CATEGORIES = [
  { key: "client", label: "Client Communications", description: "Emails sent to your clients for membership confirmations and welcomes." },
  { key: "payments", label: "Payment Notifications", description: "Emails sent to clients for payment receipts and failed payment notifications." },
  { key: "internal", label: "Staff Alerts", description: "Alerts sent to your staff when payments fail or disputes are opened." },
  { key: "reminders", label: "Reminders", description: "Emails sent to clients when their membership is nearing expiry." },
];

export default function EmailTemplatesPage() {
  const [tab, setTab] = useState("client");
  const [editing, setEditing] = useState<Template | null>(null);
  const [search, setSearch] = useState("");
  const [templates, setTemplates] = useState<Template[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [rowBusy, setRowBusy] = useState<string | null>(null);
  const [bulkBusy, setBulkBusy] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError("");
    api.getEmailTemplates()
      .then((res) => {
        if (!cancelled) setTemplates(res.templates || []);
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof ApiError ? e.message : "Failed to load email templates");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const filtered = templates.filter(
    (t) => t.category === tab && (!search || t.name.toLowerCase().includes(search.toLowerCase()))
  );
  const allEnabled = useMemo(
    () => filtered.length > 0 && filtered.every((t) => t.enabled),
    [filtered]
  );

  const setTemplate = (next: Template) => {
    setTemplates((prev) => prev.map((t) => (t.id === next.id ? next : t)));
    setEditing((prev) => (prev && prev.id === next.id ? next : prev));
  };

  const toggleTemplate = async (template: Template, enabled: boolean) => {
    setRowBusy(template.id);
    setMessage("");
    setError("");
    try {
      const updated = await api.updateEmailTemplate(template.id, { enabled });
      setTemplate(updated);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to update template");
    } finally {
      setRowBusy(null);
    }
  };

  const toggleCategoryAll = async (enabled: boolean) => {
    setBulkBusy(true);
    setMessage("");
    setError("");
    try {
      await api.toggleEmailTemplatesCategory(tab, enabled);
      setTemplates((prev) =>
        prev.map((t) => (t.category === tab ? { ...t, enabled } : t))
      );
      setMessage(enabled ? "All templates in this category enabled" : "All templates in this category disabled");
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to update category");
    } finally {
      setBulkBusy(false);
    }
  };

  if (editing) {
    return (
      <TemplateEditor
        template={editing}
        onBack={() => setEditing(null)}
        onSaved={(next) => {
          setTemplate(next);
          setMessage("Template saved");
        }}
      />
    );
  }

  return (
    <div>
      <div className="page-header">
        <h1 className="page-title">Email Templates</h1>
      </div>
      {!!error && <div className="mb-4 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-[13px] text-red-700">{error}</div>}
      {!!message && <div className="mb-4 rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-[13px] text-emerald-700">{message}</div>}

      <div className="flex border-b border-gray-200 mb-4">
        {CATEGORIES.map((c) => (
          <button
            key={c.key}
            onClick={() => setTab(c.key)}
            className={`px-3 py-2.5 text-[13px] font-medium border-b-2 -mb-px transition-colors ${
              tab === c.key ? "border-indigo-600 text-indigo-600" : "border-transparent text-gray-500 hover:text-gray-700"
            }`}
          >
            {c.label}
          </button>
        ))}
      </div>

      <p className="text-[13px] text-gray-500 mb-4">{CATEGORIES.find((c) => c.key === tab)?.description}</p>

      <div className="flex items-center gap-3 mb-4">
        <div className="relative max-w-xs w-full">
          <svg className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" /></svg>
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search..."
            className="w-full rounded-md border border-gray-200 bg-gray-50 py-1.5 pl-8 pr-3 text-[13px] placeholder:text-gray-400 focus:border-gray-300 focus:bg-white focus:outline-none"
          />
        </div>
        <span className="text-[13px] text-gray-400">{filtered.length} results</span>
        <label className="ml-auto inline-flex items-center gap-2 text-[13px] text-gray-600">
          <input
            type="checkbox"
            checked={allEnabled}
            disabled={bulkBusy || filtered.length === 0}
            onChange={(e) => {
              void toggleCategoryAll(e.target.checked);
            }}
            className="rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"
          />
          <span>{bulkBusy ? "Saving..." : "Enable all in this category"}</span>
        </label>
      </div>

      <div className="card overflow-hidden">
        {loading ? (
          <div className="px-4 py-8 text-center text-[13px] text-gray-500">Loading templates...</div>
        ) : (
          <table className="data-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Enabled</th>
                <th>Last Updated</th>
                <th className="w-10"></th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((t) => (
                <tr key={t.id} className="cursor-pointer" onClick={() => setEditing(t)}>
                  <td>
                    <p className="font-medium text-gray-900">{t.name}</p>
                    <p className="text-[11px] text-gray-400 mt-0.5">{t.description}</p>
                  </td>
                  <td onClick={(e) => e.stopPropagation()}>
                    <input
                      type="checkbox"
                      checked={t.enabled}
                      disabled={rowBusy === t.id}
                      onChange={(e) => {
                        void toggleTemplate(t, e.target.checked);
                      }}
                      className="rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"
                    />
                  </td>
                  <td className="text-gray-500">{formatLastUpdated(t.updated_at)}</td>
                  <td>
                    <button className="rounded p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-100" onClick={(e) => { e.stopPropagation(); setEditing(t); }}>
                      <svg className="h-4 w-4" fill="currentColor" viewBox="0 0 20 20"><path d="M10 6a2 2 0 110-4 2 2 0 010 4zM10 12a2 2 0 110-4 2 2 0 010 4zM10 18a2 2 0 110-4 2 2 0 010 4z" /></svg>
                    </button>
                  </td>
                </tr>
              ))}
              {!filtered.length && (
                <tr>
                  <td colSpan={4} className="py-8 text-center text-[13px] text-gray-500">No templates found</td>
                </tr>
              )}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

function TemplateEditor({ template, onBack, onSaved }: { template: Template; onBack: () => void; onSaved: (template: Template) => void }) {
  const [subject, setSubject] = useState(template.subject);
  const [body, setBody] = useState(template.body);
  const [enabled, setEnabled] = useState(template.enabled);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const subjectRef = useRef<HTMLInputElement | null>(null);
  const bodyEditorRef = useRef<Editor | null>(null);
  const lastTargetRef = useRef<"subject" | "body">("subject");

  const handleSave = async () => {
    setSaving(true);
    setError("");
    setMessage("");
    try {
      const updated = await api.updateEmailTemplate(template.id, { enabled, subject, body });
      onSaved(updated);
      setEnabled(updated.enabled);
      setSubject(updated.subject);
      setBody(updated.body);
      setSaving(false);
      setMessage("Template saved");
      setTimeout(() => setMessage(""), 2000);
    } catch (e) {
      setSaving(false);
      setError(e instanceof ApiError ? e.message : "Failed to save template");
    }
  };

  const insertVariable = (v: string) => {
    const tag = `{{${v}}}`;

    if (lastTargetRef.current === "subject") {
      const input = subjectRef.current;
      const start = input?.selectionStart ?? subject.length;
      const end = input?.selectionEnd ?? subject.length;
      const next = subject.slice(0, start) + tag + subject.slice(end);
      setSubject(next);
      queueMicrotask(() => {
        if (!input) return;
        input.focus();
        const caret = start + tag.length;
        input.setSelectionRange(caret, caret);
      });
      return;
    }

    const editor = bodyEditorRef.current;
    if (editor) {
      editor.chain().focus().insertContent(tag).run();
      return;
    }
    setBody((prev) => (prev || "") + tag);
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-5">
        <div className="flex items-center gap-1.5 text-[15px]">
          <button onClick={onBack} className="text-indigo-600 hover:text-indigo-700 font-semibold">Emails</button>
          <span className="text-gray-400">›</span>
          <span className="font-semibold text-gray-900">Editing "{template.name}"</span>
        </div>
        <div className="flex items-center gap-2">
          {message && <span className="text-[12px] text-emerald-600 bg-emerald-50 px-2 py-1 rounded">{message}</span>}
          <button onClick={() => void handleSave()} disabled={saving} className="btn-primary">
            <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
            {saving ? "Saving..." : "Save"}
          </button>
        </div>
      </div>
      {!!error && <div className="mb-4 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-[13px] text-red-700">{error}</div>}

      <div className="card overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-200">
          <h2 className="text-base font-semibold text-gray-900">Template details</h2>
        </div>

        <div className="px-6 py-5 space-y-5">
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Template name</label>
            <input value={template.name} readOnly className="input-field max-w-md bg-gray-50 text-gray-500" />
          </div>

          <div className="flex items-center gap-3">
            <label className="block text-[13px] font-medium text-gray-700">Enabled</label>
            <input
              type="checkbox"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
              className="rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"
            />
          </div>

          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">
              Email subject <span className="text-red-500">*</span>
            </label>
            <div className="relative max-w-lg">
              <input
                ref={subjectRef}
                value={subject}
                onFocus={() => {
                  lastTargetRef.current = "subject";
                }}
                onChange={(e) => setSubject(e.target.value)}
                className="input-field pr-28"
              />
              <div className="absolute right-1 top-1/2 -translate-y-1/2">
                <PlaceholderDropdown
                  variables={template.variables}
                  onInsert={(v) => {
                    lastTargetRef.current = "subject";
                    insertVariable(v);
                  }}
                />
              </div>
            </div>
          </div>

          <div>
            <div className="rounded-md border border-gray-300 overflow-hidden focus-within:border-indigo-500 focus-within:ring-1 focus-within:ring-indigo-500 transition-shadow">
              <div
                onMouseDown={() => {
                  lastTargetRef.current = "body";
                }}
              >
                <RichTextEditor
                  content={body}
                  onChange={setBody}
                  placeholder="Write your email content..."
                  onReady={(ed) => {
                    bodyEditorRef.current = ed;
                  }}
                />
              </div>
            </div>
          </div>

          <div className="rounded-md bg-gray-50 border border-gray-200 px-4 py-3">
            <p className="text-[12px] font-medium text-gray-600 mb-2">Available placeholders</p>
            <div className="flex flex-wrap gap-1.5">
              {template.variables.map((v) => (
                <button
                  key={v}
                  type="button"
                  onClick={() => insertVariable(v)}
                  className="rounded bg-white border border-gray-200 px-2 py-0.5 text-[11px] font-mono text-gray-700 hover:border-indigo-300 hover:bg-indigo-50 hover:text-indigo-700 transition-colors shadow-sm"
                  title={`Insert {{${v}}} at cursor`}
                >
                  {v}
                </button>
              ))}
            </div>
            <p className="text-[11px] text-gray-400 mt-2">Click a placeholder to insert it at your cursor.</p>
          </div>
        </div>
      </div>
    </div>
  );
}

function PlaceholderDropdown({ variables, onInsert }: { variables: string[]; onInsert: (v: string) => void }) {
  const [open, setOpen] = useState(false);

  return (
    <div className="relative">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="flex items-center gap-1 rounded border border-gray-200 bg-white px-2 py-1 text-[11px] font-medium text-gray-500 hover:bg-gray-50 transition-colors"
      >
        Placeholders
        <svg className={`h-3 w-3 transition-transform ${open ? "rotate-180" : ""}`} fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" /></svg>
      </button>
      {open && (
        <div className="absolute right-0 top-full mt-1 w-48 rounded-lg border border-gray-200 bg-white py-1 shadow-lg z-50 max-h-48 overflow-y-auto">
          {variables.map((v) => (
            <button
              key={v}
              onClick={() => { onInsert(v); setOpen(false); }}
              className="flex w-full px-3 py-1.5 text-left text-[12px] font-mono text-gray-700 hover:bg-indigo-50 hover:text-indigo-700"
            >
              {`{{${v}}}`}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

function formatLastUpdated(value: string | null | undefined) {
  if (!value) return "Default";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "Default";
  return parsed.toLocaleDateString(undefined, {
    day: "numeric",
    month: "long",
    year: "numeric",
  });
}
