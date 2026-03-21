import { fmtMoney } from "../../utils/paymentLabels";

/** Snapshot merged from Stripe webhooks (invoice, payment_intent, charge). */
export type StripeDetailsRecord = Record<string, string | number | boolean | undefined>;

function parseStripeDetails(raw: unknown): StripeDetailsRecord | null {
  if (!raw || typeof raw !== "object") return null;
  const o = raw as Record<string, unknown>;
  if (Object.keys(o).length === 0) return null;
  const out: StripeDetailsRecord = {};
  for (const [k, v] of Object.entries(o)) {
    if (v === null || v === undefined) continue;
    if (typeof v === "string" || typeof v === "number" || typeof v === "boolean") {
      out[k] = v;
    }
  }
  return Object.keys(out).length ? out : null;
}

function fmtWhen(iso?: string) {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString("en-AU", {
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

type RowProps = { label: string; value?: string; mono?: boolean };
function Row({ label, value, mono }: RowProps) {
  if (!value) return null;
  return (
    <div className="flex flex-col sm:flex-row sm:items-start sm:gap-3 py-1.5 border-b border-gray-100 last:border-0">
      <span className="text-[11px] text-gray-500 shrink-0 sm:w-40">{label}</span>
      <span className={`text-[12px] text-gray-900 break-all ${mono ? "font-mono text-[11px]" : ""}`}>{value}</span>
    </div>
  );
}

interface Props {
  stripeDetails: unknown;
  currencyCode?: string;
  amount?: number;
  status?: string;
  clientName?: string;
  clientEmail?: string;
}

/**
 * Dashboard-style breakdown for Stripe-backed payments (BECS, cards via Billing).
 */
export default function StripeTransactionPanel({
  stripeDetails,
  currencyCode = "AUD",
  amount,
  status,
  clientName,
  clientEmail,
}: Props) {
  const s = parseStripeDetails(stripeDetails);
  if (!s) return null;

  const cur = (s.currency as string) || currencyCode.toLowerCase();
  const piStatus = String(s.payment_intent_status || "");
  const timeline: { t: string; label: string }[] = [];
  if (s.payment_intent_created_at) {
    timeline.push({ t: String(s.payment_intent_created_at), label: "Payment started" });
  }
  if (s.invoice_finalized_at) {
    timeline.push({ t: String(s.invoice_finalized_at), label: "Invoice finalized" });
  }
  if (piStatus === "processing" || (status === "pending" && piStatus)) {
    if (s.last_stripe_event_at) {
      timeline.push({
        t: String(s.last_stripe_event_at),
        label: piStatus === "processing" ? "Payment pending (with bank)" : "Last Stripe update",
      });
    }
  } else if (s.last_stripe_event_at && !s.payment_intent_created_at && !s.invoice_finalized_at) {
    timeline.push({ t: String(s.last_stripe_event_at), label: "Stripe activity" });
  }
  timeline.sort((a, b) => new Date(a.t).getTime() - new Date(b.t).getTime());

  const desc =
    (s.stripe_line_description as string) ||
    (s.stripe_description as string) ||
    "";

  const displayStatus =
    status === "pending" && piStatus === "processing"
      ? "Pending (direct debit)"
      : status === "pending"
        ? "Pending"
        : status || "—";

  return (
    <div className="rounded-lg border border-gray-200 bg-gray-50/80 p-4 text-left">
      <div className="flex flex-wrap items-end justify-between gap-2 mb-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-wide text-gray-500">Stripe transaction</p>
          {typeof amount === "number" && (
            <p className="text-[18px] font-bold text-gray-900 tabular-nums mt-1">
              ${fmtMoney(amount)}{" "}
              <span className="text-[13px] font-semibold text-gray-500 uppercase">{String(cur)}</span>
            </p>
          )}
        </div>
        <span
          className={`text-[11px] font-semibold uppercase tracking-wide px-2 py-0.5 rounded ${
            status === "succeeded" || status === "confirmed"
              ? "bg-emerald-100 text-emerald-800"
              : status === "failed"
                ? "bg-red-100 text-red-800"
                : "bg-amber-100 text-amber-800"
          }`}
        >
          {displayStatus}
        </span>
      </div>

      {desc && (
        <div className="mb-4">
          <p className="text-[11px] font-medium text-gray-600 mb-1">Subscription / line item</p>
          <p className="text-[12px] text-gray-800">{desc}</p>
        </div>
      )}

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mb-4">
        {(s.stripe_invoice_status || s.payment_intent_status || s.stripe_charge_status) && (
          <div>
            <p className="text-[11px] text-gray-500">Stripe states</p>
            <p className="text-[13px] font-medium text-gray-800">
              {[s.stripe_invoice_status && `Invoice: ${s.stripe_invoice_status}`, s.payment_intent_status, s.stripe_charge_status]
                .filter(Boolean)
                .join(" · ")}
            </p>
          </div>
        )}
      </div>

      {(s.payment_method_label || s.payment_method_type) && (
        <div className="mb-4">
          <p className="text-[11px] font-medium text-gray-600 mb-1">Payment method</p>
          <p className="text-[12px] text-gray-900">{String(s.payment_method_label || s.payment_method_type)}</p>
          {(s.bank_bsb || s.bank_last4) && (
            <p className="text-[11px] text-gray-500 mt-1 font-mono">
              {[s.bank_bsb && `BSB ${s.bank_bsb}`, s.bank_last4 && `Account ···· ${s.bank_last4}`].filter(Boolean).join(" · ")}
            </p>
          )}
        </div>
      )}

      <div className="mb-4">
        <p className="text-[11px] font-medium text-gray-600 mb-1">Customer (Stripe)</p>
        <Row label="Name" value={(s.stripe_customer_name as string) || clientName} />
        <Row label="Email" value={(s.stripe_customer_email as string) || clientEmail} />
        <Row label="Customer ID" value={s.stripe_customer_id as string} mono />
      </div>

      <div className="mb-4">
        <p className="text-[11px] font-medium text-gray-600 mb-1">References</p>
        <Row
          label="Invoice"
          value={
            s.invoice_number && s.invoice_id
              ? `${s.invoice_number} · ${s.invoice_id}`
              : (s.invoice_id as string) || (s.invoice_number as string)
          }
          mono
        />
        <Row label="Subscription" value={s.stripe_subscription_id as string} mono />
        <Row label="PaymentIntent" value={s.payment_intent_id as string} mono />
        <Row label="Charge" value={s.stripe_charge_id as string} mono />
        <Row label="Mandate" value={s.stripe_mandate_id as string} mono />
        <Row label="Payment method" value={s.stripe_payment_method_id as string} mono />
      </div>

      {timeline.length > 0 && (
        <div>
          <p className="text-[11px] font-medium text-gray-600 mb-2">Recent activity</p>
          <ul className="space-y-2">
            {timeline.map((e, i) => (
              <li key={i} className="flex gap-3 text-[12px]">
                <span className="text-gray-400 shrink-0 w-36 tabular-nums">{fmtWhen(e.t)}</span>
                <span className="text-gray-800">{e.label}</span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

export function hasStripeDetails(raw: unknown): boolean {
  return parseStripeDetails(raw) !== null;
}
