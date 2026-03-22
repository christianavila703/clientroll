ALTER TABLE clients
DROP COLUMN IF EXISTS company_country,
DROP COLUMN IF EXISTS company_postcode,
DROP COLUMN IF EXISTS company_state_region,
DROP COLUMN IF EXISTS company_city,
DROP COLUMN IF EXISTS company_address,
DROP COLUMN IF EXISTS company_website,
DROP COLUMN IF EXISTS company_phone,
DROP COLUMN IF EXISTS company_number,
DROP COLUMN IF EXISTS tax_number;

ALTER TABLE tenants
DROP COLUMN IF EXISTS ask_company_information,
DROP COLUMN IF EXISTS ask_emergency_contact;
