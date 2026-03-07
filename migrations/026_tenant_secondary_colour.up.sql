-- Add secondary/accent colour for tenant branding

ALTER TABLE tenants
    ADD COLUMN secondary_colour TEXT;

