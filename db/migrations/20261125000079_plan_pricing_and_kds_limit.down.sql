-- 20261125000079_plan_pricing_and_kds_limit.down.sql
-- Restores the 9.6 x annual prices and the Starter plan without KDS from migration 051.

ALTER TABLE subscription_plans DROP CONSTRAINT IF EXISTS chk_subscription_plans_annual_ten_months;

UPDATE subscription_plans
SET annual_price_idr = monthly_price_idr * 96 / 10, updated_at = NOW()
WHERE code IN ('starter_umkm', 'pro_cafe', 'enterprise_whitelabel');

UPDATE subscription_plans
SET supports_kds_queue_tv = false, updated_at = NOW()
WHERE code = 'starter_umkm';

ALTER TABLE subscription_plans DROP CONSTRAINT IF EXISTS chk_subscription_plans_kds_stations;
ALTER TABLE subscription_plans DROP COLUMN IF EXISTS max_kds_stations;
