export interface MonthlyRevenuePoint {
  month: string;
  collected_amount?: number;
  tax_amount?: number;
  surcharge_amount?: number;
  to_organisation_amount?: number;
  revenue: number;
}

export interface MembershipBreakdownItem {
  type_title: string;
  currency_code?: string;
  revenue: number;
  count: number;
}

export type PaymentStatusMap = Record<string, number>;

export interface CurrencyTotalItem {
  currency_code: string;
  collected_amount?: number;
  tax_amount?: number;
  surcharge_amount?: number;
  to_organisation_amount?: number;
  revenue: number;
  payments: number;
}

export interface ComprehensiveReport {
  active_members: number;
  total_clients: number;
  month_collected_amount?: number;
  prev_month_collected_amount?: number;
  total_collected_amount?: number;
  month_revenue: number;
  prev_month_revenue: number;
  total_revenue: number;
  month_tax_amount?: number;
  prev_month_tax_amount?: number;
  total_tax_amount?: number;
  month_surcharge_amount?: number;
  prev_month_surcharge_amount?: number;
  total_surcharge_amount?: number;
  month_to_organisation_amount?: number;
  prev_month_to_organisation_amount?: number;
  total_to_organisation_amount?: number;
  organisation_month_revenue?: number;
  organisation_prev_month_revenue?: number;
  organisation_total_revenue?: number;
  mrr: number;
  arr: number;
  avg_revenue_per_member: number;
  failed_payments: number;
  new_signups_month: number;
  churned_month: number;
  collection_rate: number;
  avg_membership_days: number;
  monthly_revenue: MonthlyRevenuePoint[];
  monthly_signups: { month: string; count: number }[];
  monthly_churn: { month: string; count: number }[];
  membership_breakdown: MembershipBreakdownItem[];
  revenue_by_currency: CurrencyTotalItem[];
  payment_statuses: PaymentStatusMap;
  frequency_breakdown: { frequency: string; count: number; revenue: number }[];
  top_clients: { client_id: string; client_name: string; total_paid: number; memberships: number }[];
  upcoming_expirations: { client_name: string; membership_type: string; end_date: string; days_left: number }[];
  recent_failures: { client_name: string; amount: number; failure_reason: string; due_date: string }[];
}

export interface DashboardSummary {
  active_members: number;
  month_collected_amount?: number;
  month_tax_amount?: number;
  month_surcharge_amount?: number;
  month_to_organisation_amount?: number;
  month_revenue: number;
  organisation_month_revenue?: number;
  failed_payments: number;
  new_signups: number;
  open_disputes: number;
  active_mandates: number;
  active_subscriptions: number;
  pending_payments: number;
}
