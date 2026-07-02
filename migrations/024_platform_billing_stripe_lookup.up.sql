-- SECURITY DEFINER helpers to support webhook routing when no tenant context is set.
-- These functions allow resolving tenant_id from Stripe identifiers.

CREATE OR REPLACE FUNCTION get_tenant_id_by_stripe_customer_id(p_customer_id TEXT)
RETURNS UUID
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = public
AS $$
  SELECT tenant_id
  FROM tenant_subscriptions
  WHERE stripe_customer_id = p_customer_id
  LIMIT 1;
$$;

CREATE OR REPLACE FUNCTION get_tenant_id_by_stripe_subscription_id(p_subscription_id TEXT)
RETURNS UUID
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = public
AS $$
  SELECT tenant_id
  FROM tenant_subscriptions
  WHERE stripe_subscription_id = p_subscription_id
  LIMIT 1;
$$;

