-- 20261115000063_store_settings_limits.down.sql
ALTER TABLE invoice_payments DROP CONSTRAINT IF EXISTS chk_invoice_payments_method;
ALTER TABLE invoice_payments ADD CONSTRAINT chk_invoice_payments_method
    CHECK (method IN ('transfer', 'cash', 'qris'));
ALTER TABLE store_settings DROP CONSTRAINT IF EXISTS chk_store_settings_stock_approval_limit;
ALTER TABLE store_settings DROP CONSTRAINT IF EXISTS chk_store_settings_offline_orders;
ALTER TABLE store_settings DROP CONSTRAINT IF EXISTS chk_store_settings_offline_hours;
ALTER TABLE store_settings DROP COLUMN IF EXISTS stock_approval_limit;
ALTER TABLE store_settings DROP COLUMN IF EXISTS offline_max_orders;
ALTER TABLE store_settings DROP COLUMN IF EXISTS offline_max_hours;
