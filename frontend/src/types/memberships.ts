export interface Bonus {
  id: string;
  title: string;
  description?: string;
  image_url?: string;
  value: number;
}

export interface TermOption {
  id: string;
  duration_months: number;
  fee_total: number;
  fee_weekly?: number;
  fee_fortnightly?: number;
  fee_monthly?: number;
  fee_quarterly?: number;
  allow_upfront: boolean;
  num_payments?: number;
  continue_after_term?: boolean;
  custom_schedule_enabled?: boolean;
  custom_schedule_label?: string;
  custom_schedule_amount?: number;
  custom_schedule_interval_unit?: "week" | "month";
  custom_schedule_interval_count?: number;
  custom_schedule_num_payments?: number;
  bonuses?: Bonus[];
}

export interface MembershipType {
  id: string;
  membership_name?: string;
  membership_level?: string;
  title: string;
  slug: string;
  description?: string;
  summary?: string;
  inclusions?: string;
  benefits?: string;
  currency_code?: string;
  enabled_payment_frequencies?: string[];
  apply_sales_tax: boolean;
  is_active: boolean;
  is_most_popular?: boolean;
  sort_order: number;
  top_image_url?: string;
  term_options?: TermOption[];
  created_at: string;
  updated_at: string;
}

export interface MembershipRecord {
  membership_id: string;
  membership_type: string;
  membership_type_id: string;
  term_months: number;
  start_date: string;
  end_date: string;
  payment_method?: string;
  payment_frequency?: string;
  payment_interval_unit?: string;
  payment_interval_count?: number;
  payment_amount?: number;
  num_payments?: number;
  continue_after_term?: boolean;
  status: string;
  mandate_id?: string;
  subscription_id?: string;
  has_agreement?: boolean;
  total_paid: number;
  total_expected: number;
  failed_count: number;
  last_payment_date?: string;
  last_payment_status?: string;
  payment_currency_code?: string;
  last_payment_amount?: number;
  last_base_amount?: number;
  last_tax_amount?: number;
  last_processing_fee_amount?: number;
}
