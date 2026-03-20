export interface PaymentRecord {
  payment_id?: string;
  id?: string;
  client_id?: string;
  client_name?: string;
  client_email?: string;
  membership_type?: string;
  membership_id?: string;
  payment_source?: "membership" | "service";
  description?: string;
  term_months?: number;
  amount: number;
  base_amount?: number;
  tax_amount?: number;
  processing_fee_amount?: number;
  currency_code?: string;
  status: string;
  due_date: string;
  paid_date?: string;
  failure_reason?: string;
  retry_count?: number;
  provider_reference?: string;
  payment_method?: string;
  total_paid?: number;
  total_expected?: number;
  membership_end?: string;
  membership_status?: string;
  payment_frequency?: string;
  _type?: "membership" | "service";
  _description?: string;
}

export interface AdminPaymentRecord extends PaymentRecord {
  payment_id: string;
  client_id: string;
  client_name: string;
  membership_type: string;
  membership_id: string;
  term_months: number;
  retry_count: number;
  total_paid: number;
  total_expected: number;
  membership_end: string;
  membership_status: string;
}
