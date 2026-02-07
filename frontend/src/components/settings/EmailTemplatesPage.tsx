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
    id: "welcome", name: "Welcome Email", subject: "Welcome to {{company_name}}, {{first_name}}!",
    body: "<p>Hi {{first_name}},</p><p>Your account for <strong>{{company_name}}</strong> has been created successfully. You can now start your wellness journey with us.</p><p>Visit your dashboard to get started.</p>",
    category: "signup", description: "Sent after a new tenant registers",
    variables: ["first_name", "company_name", "dashboard_url"],
    lastUpdated: "7 February 2026",
  },
  {
    id: "membership_confirmation", name: "Membership Confirmation", subject: "Your {{membership_title}} membership is confirmed",
    body: "<p>Hi {{client_name}},</p><p>Your membership has been successfully set up:</p><ul><li><strong>Membership:</strong> {{membership_title}}</li><li><strong>Term:</strong> {{term_months}} months</li><li><strong>Total Fee:</strong> ${{total_fee}}</li></ul><p>If you have any questions, please contact us.</p>",
    category: "signup", description: "Sent to client after completing membership signup",
    variables: ["client_name", "membership_title", "term_months", "total_fee", "company_name"],
    lastUpdated: "7 February 2026",
  },
  {
    id: "verify_email", name: "Verify Email", subject: "Verify your email — {{company_name}}",
    body: "<p>Hi {{first_name}},</p><p>Please confirm your email address by clicking the button below. This link expires in 24 hours.</p>",
    category: "signup", description: "Sent during tenant registration for email verification",
    variables: ["first_name", "verify_url"],
    lastUpdated: "7 February 2026",
  },
  {
    id: "staff_invitation", name: "Staff Invitation", subject: "You're invited to join {{company_name}} on ClientRoll",
    body: "<p>Hi{{invitee_name}},</p><p>{{inviter_name}} has invited you to join <strong>{{company_name}}</strong> on ClientRoll. Click below to set up your account. This link expires in 48 hours.</p>",
    category: "signup", description: "Sent when a tenant admin invites a new staff member",
    variables: ["invitee_name", "inviter_name", "company_name", "invite_url"],
    lastUpdated: "7 February 2026",
  },
  {
    id: "password_reset", name: "Password Reset", subject: "Reset your ClientRoll password",
    body: "<p>Hi {{first_name}},</p><p>We received a request to reset your password. Click the button below to choose a new one. This link expires in 1 hour.</p><p>If you didn't request this, you can safely ignore this email.</p>",
    category: "signup", description: "Sent when a user requests a password reset",
    variables: ["first_name", "reset_url"],
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
    id: "payment_failed_alert", name: "Payment Failed (Admin Alert)", subject: "Payment failed: {{client_name}} — ${{amount}}",
    body: "<p>Hi {{admin_name}},</p><p>A payment has failed:</p><ul><li><strong>Client:</strong> {{client_name}}</li><li><strong>Amount:</strong> ${{amount}}</li><li><strong>Membership:</strong> {{membership_title}}</li><li><strong>Reason:</strong> {{failure_reason}}</li></ul><p>Review this in your dashboard under Payments.</p>",
    category: "payments", description: "Sent to tenant admin when a client's payment fails",
    variables: ["admin_name", "client_name", "amount", "membership_title", "failure_reason"],
    lastUpdated: "7 February 2026",
  },
  {
    id: "dispute_alert", name: "Dispute Alert", subject: "Dispute opened: {{client_name}} — ${{amount}}",
    body: "<p>Hi {{admin_name}},</p><p>A payment dispute has been opened:</p><ul><li><strong>Client:</strong> {{client_name}}</li><li><strong>Amount:</strong> ${{amount}}</li><li><strong>Reason:</strong> {{reason}}</li></ul><p>Please review and respond in your dashboard.</p>",
    category: "payments", description: "Sent to tenant admin when a dispute is opened",
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
  { key: "signup", label: "Signup & Account" },
  { key: "payments", label: "Payments" },
  { key: "reminders", label: "Reminders" },
];

export default function EmailTemplatesPage() {
  const [tab, setTab] = useState("signup");
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
        {tab === "signup" && "Emails sent during registration, verification, and membership signup."}
        {tab === "payments" && "Emails sent for payment receipts, failures, and disputes."}
        {tab === "reminders" && "Emails sent for membership renewals and expiry reminders."}
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
  const [subject, setSubject] = useState(template.subject);
  const [body, setBody] = useState(template.body);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");

  const handleSave = () => {
    setSaving(true);
    // In future: POST to API to save custom template overrides per tenant
    setTimeout(() => {
      setSaving(false);
      setMessage("Template saved");
      setTimeout(() => setMessage(""), 2000);
    }, 500);
  };

  return (
    <div>
      {/* Breadcrumb header */}
      <div className="flex items-center justify-between mb-5">
        <div className="flex items-center gap-2">
          <button onClick={onBack} className="text-[13px] text-indigo-600 hover:text-indigo-700 font-medium">Email Templates</button>
          <svg className="h-4 w-4 text-gray-300" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M8.25 4.5l7.5 7.5-7.5 7.5" /></svg>
          <span className="text-[13px] font-medium text-gray-900">Editing "{template.name}"</span>
        </div>
        <div className="flex items-center gap-2">
          {message && <span className="text-[12px] text-emerald-600">{message}</span>}
          <button onClick={handleSave} disabled={saving} className="btn-primary">
            {saving ? "Saving..." : "Save"}
          </button>
        </div>
      </div>

      <div className="card max-w-4xl">
        <div className="card-header">
          <h3>Template details</h3>
        </div>
        <div className="card-body space-y-4">
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Template name</label>
            <input value={template.name} className="input-field bg-gray-50" readOnly />
          </div>

          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Subject line</label>
            <input value={subject} onChange={(e) => setSubject(e.target.value)} className="input-field" />
          </div>

          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Email body</label>
            <RichTextEditor content={body} onChange={setBody} placeholder="Write your email content..." />
          </div>

          {/* Variables */}
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-2">Available variables</label>
            <div className="flex flex-wrap gap-1.5">
              {template.variables.map((v) => (
                <button
                  key={v}
                  type="button"
                  onClick={() => navigator.clipboard.writeText(`{{${v}}}`)}
                  className="rounded-md bg-indigo-50 border border-indigo-100 px-2 py-1 text-[11px] font-mono text-indigo-700 hover:bg-indigo-100 transition-colors"
                  title={`Click to copy {{${v}}}`}
                >
                  {`{{${v}}}`}
                </button>
              ))}
            </div>
            <p className="text-[11px] text-gray-400 mt-1.5">Click a variable to copy it, then paste into the subject or body.</p>
          </div>
        </div>
      </div>
    </div>
  );
}
