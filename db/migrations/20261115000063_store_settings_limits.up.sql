-- 20261115000063_store_settings_limits.up.sql
-- Store-configurable offline sync caps (OF7) and inventory count/waste auto-approval threshold (K2).

ALTER TABLE store_settings
    ADD COLUMN IF NOT EXISTS offline_max_hours INT NOT NULL DEFAULT 12,
    ADD COLUMN IF NOT EXISTS offline_max_orders INT NOT NULL DEFAULT 300,
    ADD COLUMN IF NOT EXISTS stock_approval_limit BIGINT NOT NULL DEFAULT 50000;

ALTER TABLE store_settings DROP CONSTRAINT IF EXISTS chk_store_settings_offline_hours;
ALTER TABLE store_settings ADD CONSTRAINT chk_store_settings_offline_hours
    CHECK (offline_max_hours BETWEEN 1 AND 72);

ALTER TABLE store_settings DROP CONSTRAINT IF EXISTS chk_store_settings_offline_orders;
ALTER TABLE store_settings ADD CONSTRAINT chk_store_settings_offline_orders
    CHECK (offline_max_orders BETWEEN 10 AND 5000);

ALTER TABLE store_settings DROP CONSTRAINT IF EXISTS chk_store_settings_stock_approval_limit;
ALTER TABLE store_settings ADD CONSTRAINT chk_store_settings_stock_approval_limit
    CHECK (stock_approval_limit >= 0);

ALTER TABLE invoice_payments DROP CONSTRAINT IF EXISTS chk_invoice_payments_method;
ALTER TABLE invoice_payments ADD CONSTRAINT chk_invoice_payments_method
    CHECK (method IN ('transfer', 'cash', 'qris', 'credit'));
