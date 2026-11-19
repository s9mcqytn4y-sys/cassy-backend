-- 20261118000070_offline_max_orders_default_200.up.sql
-- OF7 default cap lowered from 300 to 200 offline orders (Director decision K3, 2026-11-18).
-- Only the default for new store_settings rows changes; stores that already saved a value keep it.

ALTER TABLE store_settings ALTER COLUMN offline_max_orders SET DEFAULT 200;
