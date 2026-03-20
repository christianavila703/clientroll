ALTER TABLE tenants
    DROP CONSTRAINT IF EXISTS chk_tenants_sales_tax_percent_range;

ALTER TABLE tenants
    DROP COLUMN IF EXISTS sales_tax_percent,
    DROP COLUMN IF EXISTS sales_tax_name;
