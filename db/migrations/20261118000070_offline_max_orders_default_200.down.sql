-- 20261118000070_offline_max_orders_default_200.down.sql

ALTER TABLE store_settings ALTER COLUMN offline_max_orders SET DEFAULT 300;
