-- Rollback: Remove the Sample Studio v4 terms document and its links.
-- Does NOT restore previously detached T&C links (those must be re-attached manually).

DO $$
DECLARE
    v_tenant_id UUID;
    v_terms_id  UUID;
BEGIN
    SELECT id INTO v_tenant_id
    FROM tenants
    WHERE slug = 'sample-studio'
       OR name ILIKE '%sample studio%'
    LIMIT 1;

    IF v_tenant_id IS NULL THEN
        RETURN;
    END IF;

    SELECT id INTO v_terms_id
    FROM terms_and_conditions
    WHERE tenant_id = v_tenant_id
      AND title = 'Membership Agreement & Payment Authority – Version 4'
    ORDER BY created_at DESC
    LIMIT 1;

    IF v_terms_id IS NULL THEN
        RETURN;
    END IF;

    DELETE FROM membership_type_terms
    WHERE tenant_id = v_tenant_id
      AND terms_and_conditions_id = v_terms_id;

    DELETE FROM terms_and_conditions
    WHERE id = v_terms_id
      AND tenant_id = v_tenant_id;
END $$;
