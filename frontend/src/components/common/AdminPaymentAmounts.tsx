import { fmtMoney, customerCharged, processingSurcharge, shouldShowPaymentSplit, type PaymentLike } from "../../utils/paymentLabels";

type Props = {
  p: PaymentLike;
  /** "compact" = one column; "stacked" = labeled lines */
  layout?: "compact" | "stacked";
  className?: string;
};

/**
 * Shows payment line-items in plain language for admin screens.
 */
export default function AdminPaymentAmounts({ p, layout = "compact", className = "" }: Props) {
  const cur = p.currency_code || "AUD";
  const charged = customerCharged(p);
  const fee = processingSurcharge(p);
  const tax = Number(p.tax_amount ?? 0) || 0;
  const hasBase = p.base_amount !== undefined && p.base_amount !== null && !Number.isNaN(Number(p.base_amount));
  const baseExTax = hasBase ? Number(p.base_amount) : Math.max(0, charged - fee - tax);
  const split = shouldShowPaymentSplit(p);

  if (!split && layout === "compact") {
    return (
      <div className={className}>
        <p className="tabular-nums font-medium text-gray-900">
          {cur} ${fmtMoney(charged)}
        </p>
      </div>
    );
  }

  if (layout === "stacked") {
    return (
      <div className={`space-y-0.5 ${className}`}>
        <div className="flex justify-between gap-3 text-[12px]">
          <span className="text-gray-500">Payment</span>
          <span className="tabular-nums font-medium text-gray-900">{cur} ${fmtMoney(baseExTax)}</span>
        </div>
        {tax > 0.004 && (
          <div className="flex justify-between gap-3 text-[11px] text-gray-500">
            <span>Tax</span>
            <span className="tabular-nums">{cur} ${fmtMoney(tax)}</span>
          </div>
        )}
        {fee > 0.004 && (
          <div className="flex justify-between gap-3 text-[11px] text-gray-500">
            <span>Payment processing fee</span>
            <span className="tabular-nums">{cur} ${fmtMoney(fee)}</span>
          </div>
        )}
        <div className="flex justify-between gap-3 text-[12px]">
          <span className="text-gray-600 font-medium">Total</span>
          <span className="tabular-nums font-semibold text-gray-900">{cur} ${fmtMoney(charged)}</span>
        </div>
      </div>
    );
  }

  return (
    <div className={className}>
      <p className="tabular-nums font-medium text-gray-900">
        {cur} ${fmtMoney(charged)}
        <span className="text-[10px] font-normal text-gray-400 block mt-0.5">total</span>
      </p>
      <p className="text-[11px] text-emerald-800 tabular-nums mt-1">
        {cur} ${fmtMoney(baseExTax)}
        <span className="text-[10px] font-normal text-gray-500 block">payment</span>
      </p>
      {tax > 0.004 && (
        <p className="text-[10px] text-gray-400 tabular-nums mt-0.5">
          + {cur} ${fmtMoney(tax)} tax
        </p>
      )}
      {fee > 0.004 && (
        <p className="text-[10px] text-gray-400 tabular-nums mt-0.5">
          + {cur} ${fmtMoney(fee)} payment processing fee
        </p>
      )}
    </div>
  );
}
