export interface TenantBranding {
  name: string;
  slug: string;
  logo_url?: string;
  primary_colour?: string;
  secondary_colour?: string;
  signup_price_display_mode?: "total" | "weekly" | "monthly";
  enabled_payment_frequencies?: string[];
  ask_emergency_contact?: boolean;
  ask_company_information?: boolean;
  ask_birth_date?: boolean;
  google_places_api_key?: string;
  payment_providers?: string[];
}

export interface TenantSettings {
  slug?: string;
  name?: string;
  enabled_currencies?: string[];
  sales_tax_name?: string;
  sales_tax_percent?: number;
}
