ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS sales_tax_name VARCHAR(30) NOT NULL DEFAULT 'GST',
    ADD COLUMN IF NOT EXISTS sales_tax_percent NUMERIC(5,2) NOT NULL DEFAULT 10.00;

ALTER TABLE tenants
    DROP CONSTRAINT IF EXISTS chk_tenants_sales_tax_percent_range;

ALTER TABLE tenants
    ADD CONSTRAINT chk_tenants_sales_tax_percent_range
        CHECK (sales_tax_percent >= 0 AND sales_tax_percent <= 100);
