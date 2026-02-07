import { useState } from "react";
import RichTextEditor from "../common/RichTextEditor";

interface Template {
  id: string;
  name: string;
  subject: string;
  body: string;
  category: string;
  description: string;
  variables: string[];
  lastUpdated: string;
}

const DEFAULT_TEMPLATES: Template[] = [
  {
    id: "membership_confirmation", name: "Membership Confirmation", subject: "Your {{membership_title}} membership is confirmed",
    body: "<p>Hi {{client_name}},</p><p>Your membership has been successfully set up:</p><ul><li><strong>Membership:</strong> {{membership_title}}</li><li><strong>Term:</strong> {{term_months}} months</li><li><strong>Total Fee:</strong> ${{total_fee}}</li></ul><p>If you have any questions, please contact us.</p>",
    category: "client", description: "Sent to client after completing membership signup",
    variables: ["client_name", "membership_title", "term_months", "total_fee", "company_name"],
    lastUpdated: "7 February 2026",
  },
  {
    id: "membership_welcome", name: "New Member Welcome", subject: "Welcome to {{company_name}}, {{client_name}}!",
    body: "<p>Hi {{client_name}},</p><p>Welcome to <strong>{{company_name}}</strong>! We're excited to have you as a member.</p><p>Your <strong>{{membership_title}}</strong> membership is now active. Here's what to expect:</p><ul><li>Your first payment of ${{payment_amount}} will be debited {{payment_frequency}}</li><li>Your membership runs until {{end_date}}</li></ul><p>If you have any questions, don't hesitate to reach out.</p>",
    category: "client", description: "Welcome email sent to new clients after signing up",
    variables: ["client_name", "company_name", "membership_title", "payment_amount", "payment_frequency", "end_date"],
    lastUpdated: "7 February 2026",
  },
  {
    id: "payment_receipt", name: "Payment Receipt", subject: "Payment receipt — ${{amount}}",
    body: "<p>Hi {{client_name}},</p><p>We've received your payment:</p><ul><li><strong>Amount:</strong> ${{amount}}</li><li><strong>Membership:</strong> {{membership_title}}</li><li><strong>Date:</strong> {{payment_date}}</li></ul><p>Thank you for your payment.</p>",
    category: "payments", description: "Sent to client after a successful payment",
    variables: ["client_name", "amount", "membership_title", "payment_date", "company_name"],
    lastUpdated: "7 February 2026",
  },
  {
    id: "payment_failed", name: "Payment Failed (Client)", subject: "Payment failed — action required",
    body: "<p>Hi {{client_name}},</p><p>Unfortunately your scheduled payment could not be processed.</p><ul><li><strong>Amount:</strong> ${{amount}}</li><li><strong>Membership:</strong> {{membership_title}}</li><li><strong>Reason:</strong> {{failure_reason}}</li></ul><p>Please ensure your bank details are up to date. We'll automatically retry this payment.</p>",
    category: "payments", description: "Sent to client when their payment fails",
    variables: ["client_name", "amount", "membership_title", "failure_reason", "company_name"],
    lastUpdated: "7 February 2026",
  },
  {
    id: "payment_failed_alert", name: "Payment Failed (Staff Alert)", subject: "Payment failed: {{client_name}} — ${{amount}}",
    body: "<p>Hi {{admin_name}},</p><p>A payment has failed for one of your clients:</p><ul><li><strong>Client:</strong> {{client_name}}</li><li><strong>Amount:</strong> ${{amount}}</li><li><strong>Membership:</strong> {{membership_title}}</li><li><strong>Reason:</strong> {{failure_reason}}</li></ul><p>Review this in your dashboard under Payments.</p>",
    category: "internal", description: "Sent to staff when a client's payment fails",
    variables: ["admin_name", "client_name", "amount", "membership_title", "failure_reason"],
    lastUpdated: "7 February 2026",
  },
  {
    id: "dispute_alert", name: "Dispute Alert", subject: "Dispute opened: {{client_name}} — ${{amount}}",
    body: "<p>Hi {{admin_name}},</p><p>A payment dispute has been opened:</p><ul><li><strong>Client:</strong> {{client_name}}</li><li><strong>Amount:</strong> ${{amount}}</li><li><strong>Reason:</strong> {{reason}}</li></ul><p>Please review and respond in your dashboard.</p>",
    category: "internal", description: "Sent to staff when a client disputes a payment",
    variables: ["admin_name", "client_name", "amount", "reason"],
    lastUpdated: "7 February 2026",
  },
  {
    id: "membership_expiring", name: "Membership Expiring", subject: "Your {{membership_title}} membership expires soon",
    body: "<p>Hi {{client_name}},</p><p>Your <strong>{{membership_title}}</strong> membership is set to expire on <strong>{{expiry_date}}</strong>.</p><p>Please contact us if you'd like to renew or discuss your options.</p>",
    category: "reminders", description: "Sent to client when membership is nearing end of term",
    variables: ["client_name", "membership_title", "expiry_date", "company_name"],
    lastUpdated: "7 February 2026",
  },
];

const CATEGORIES = [
  { key: "client", label: "Client Communications" },
  { key: "payments", label: "Payment Notifications" },
  { key: "internal", label: "Staff Alerts" },
  { key: "reminders", label: "Reminders" },
];

export default function EmailTemplatesPage() {
  const [tab, setTab] = useState("client");
  const [editing, setEditing] = useState<Template | null>(null);
  const [search, setSearch] = useState("");

  const filtered = DEFAULT_TEMPLATES.filter(
    (t) => t.category === tab && (!search || t.name.toLowerCase().includes(search.toLowerCase()))
  );

  if (editing) {
    return <TemplateEditor template={editing} onBack={() => setEditing(null)} />;
  }

  return (
    <div>
      <div className="page-header">
        <h1 className="page-title">Email Templates</h1>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-gray-200 mb-4">
        {CATEGORIES.map((c) => (
          <button key={c.key} onClick={() => setTab(c.key)}
            className={`px-3 py-2.5 text-[13px] font-medium border-b-2 -mb-px transition-colors ${
              tab === c.key ? "border-indigo-600 text-indigo-600" : "border-transparent text-gray-500 hover:text-gray-700"
            }`}>{c.label}</button>
        ))}
      </div>

      <p className="text-[13px] text-gray-500 mb-4">
        {tab === "client" && "Emails sent to your clients for membership confirmations and welcomes."}
        {tab === "payments" && "Emails sent to clients for payment receipts and failed payment notifications."}
        {tab === "internal" && "Alerts sent to your staff when payments fail or disputes are opened."}
        {tab === "reminders" && "Emails sent to clients when their membership is nearing expiry."}
      </p>

      {/* Search + count */}
      <div className="flex items-center gap-3 mb-4">
        <div className="relative max-w-xs w-full">
          <svg className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" /></svg>
          <input type="text" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search..."
            className="w-full rounded-md border border-gray-200 bg-gray-50 py-1.5 pl-8 pr-3 text-[13px] placeholder:text-gray-400 focus:border-gray-300 focus:bg-white focus:outline-none" />
        </div>
        <span className="text-[13px] text-gray-400">{filtered.length} results</span>
      </div>

      {/* Table */}
      <div className="card overflow-hidden">
        <table className="data-table">
          <thead><tr><th>Name</th><th>Last Updated</th><th className="w-10"></th></tr></thead>
          <tbody>
            {filtered.map((t) => (
              <tr key={t.id} className="cursor-pointer" onClick={() => setEditing(t)}>
                <td>
                  <p className="font-medium text-gray-900">{t.name}</p>
                  <p className="text-[11px] text-gray-400 mt-0.5">{t.description}</p>
                </td>
                <td className="text-gray-500">{t.lastUpdated}</td>
                <td>
                  <button className="rounded p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-100" onClick={(e) => { e.stopPropagation(); setEditing(t); }}>
                    <svg className="h-4 w-4" fill="currentColor" viewBox="0 0 20 20"><path d="M10 6a2 2 0 110-4 2 2 0 010 4zM10 12a2 2 0 110-4 2 2 0 010 4zM10 18a2 2 0 110-4 2 2 0 010 4z" /></svg>
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function TemplateEditor({ template, onBack }: { template: Template; onBack: () => void }) {
  const [name, setName] = useState(template.name);
  const [subject, setSubject] = useState(template.subject);
  const [body, setBody] = useState(template.body);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");

  const handleSave = () => {
    setSaving(true);
    setTimeout(() => {
      setSaving(false);
      setMessage("Template saved");
      setTimeout(() => setMessage(""), 2000);
    }, 500);
  };

  const insertVariable = (v: string) => {
    const tag = `{{${v}}}`;
    navigator.clipboard.writeText(tag);
    setMessage(`Copied ${tag}`);
    setTimeout(() => setMessage(""), 1500);
  };

  return (
    <div>
      {/* Header — Ignition style: breadcrumb + actions */}
      <div className="flex items-center justify-between mb-5">
        <div className="flex items-center gap-1.5 text-[15px]">
          <button onClick={onBack} className="text-indigo-600 hover:text-indigo-700 font-semibold">Emails</button>
          <span className="text-gray-400">›</span>
          <span className="font-semibold text-gray-900">Editing "{template.name}"</span>
        </div>
        <div className="flex items-center gap-2">
          {message && <span className="text-[12px] text-emerald-600 bg-emerald-50 px-2 py-1 rounded">{message}</span>}
          <button onClick={handleSave} disabled={saving} className="btn-primary">
            <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
            {saving ? "Saving..." : "Save"}
          </button>
        </div>
      </div>

      {/* Main card — Ignition "Template details" style */}
      <div className="card max-w-4xl overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-200">
          <h2 className="text-base font-semibold text-gray-900">Template details</h2>
        </div>

        <div className="px-6 py-5 space-y-5">
          {/* Template name */}
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">
              Template name <span className="text-red-500">*</span>
            </label>
            <input value={name} onChange={(e) => setName(e.target.value)} className="input-field max-w-md" />
          </div>

          {/* Email subject */}
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">
              Email subject <span className="text-red-500">*</span>
            </label>
            <div className="relative max-w-lg">
              <input value={subject} onChange={(e) => setSubject(e.target.value)} className="input-field pr-28" />
              {/* Inline placeholders dropdown for subject */}
              <div className="absolute right-1 top-1/2 -translate-y-1/2">
                <PlaceholderDropdown variables={template.variables} onInsert={(v) => setSubject(subject + ` {{${v}}}`)} />
              </div>
            </div>
          </div>

          {/* Rich text editor with Placeholders dropdown in toolbar */}
          <div>
            <div className="rounded-md border border-gray-300 overflow-hidden focus-within:border-indigo-500 focus-within:ring-1 focus-within:ring-indigo-500 transition-shadow">
              <RichTextEditor content={body} onChange={setBody} placeholder="Write your email content..." />
            </div>
          </div>

          {/* Variables reference */}
          <div className="rounded-md bg-gray-50 border border-gray-200 px-4 py-3">
            <p className="text-[12px] font-medium text-gray-600 mb-2">Available placeholders</p>
            <div className="flex flex-wrap gap-1.5">
              {template.variables.map((v) => (
                <button
                  key={v}
                  type="button"
                  onClick={() => insertVariable(v)}
                  className="rounded bg-white border border-gray-200 px-2 py-0.5 text-[11px] font-mono text-gray-700 hover:border-indigo-300 hover:bg-indigo-50 hover:text-indigo-700 transition-colors shadow-sm"
                  title={`Click to copy {{${v}}}`}
                >
                  {v}
                </button>
              ))}
            </div>
            <p className="text-[11px] text-gray-400 mt-2">Click to copy a placeholder, then paste it into the subject or body.</p>
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
