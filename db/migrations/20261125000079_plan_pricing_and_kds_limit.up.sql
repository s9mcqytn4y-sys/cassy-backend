-- 20261125000079_plan_pricing_and_kds_limit.up.sql
-- FL-Q4: annual price = 10 x monthly ("pay 10 months, use 12").
-- FL-Q5: Starter gets 1 KDS station + 1 queue TV; multi-station KDS stays on Pro and Enterprise.
-- New quota max_kds_stations counts non-revoked devices with role 'kds'; 0 means unlimited, as for
-- the other max_* columns. Existing devices are never touched: the quota only gates new ones.

ALTER TABLE subscription_plans
    ADD COLUMN IF NOT EXISTS max_kds_stations INT NOT NULL DEFAULT 0;

ALTER TABLE subscription_plans DROP CONSTRAINT IF EXISTS chk_subscription_plans_kds_stations;
ALTER TABLE subscription_plans ADD CONSTRAINT chk_subscription_plans_kds_stations
    CHECK (max_kds_stations >= 0);

ALTER TABLE subscription_plans DROP CONSTRAINT IF EXISTS chk_subscription_plans_annual_ten_months;
ALTER TABLE subscription_plans ADD CONSTRAINT chk_subscription_plans_annual_ten_months
    CHECK (annual_price_idr = monthly_price_idr * 10) NOT VALID;

COMMENT ON COLUMN subscription_plans.max_kds_stations IS 'Non-revoked KDS devices allowed; 0 = unlimited';

UPDATE subscription_plans
SET annual_price_idr = monthly_price_idr * 10, updated_at = NOW()
WHERE annual_price_idr <> monthly_price_idr * 10;

ALTER TABLE subscription_plans VALIDATE CONSTRAINT chk_subscription_plans_annual_ten_months;

UPDATE subscription_plans
SET supports_kds_queue_tv = true, max_kds_stations = 1, updated_at = NOW()
WHERE code IN ('trial_14d', 'starter_umkm');
