-- 20261115000064_receipt_short_codes.down.sql

REVOKE ALL ON FUNCTION resolve_receipt_short_code(TEXT) FROM mycaffe_app;
DROP FUNCTION IF EXISTS resolve_receipt_short_code(TEXT);
DROP INDEX IF EXISTS uq_receipt_tokens_short_code;
ALTER TABLE receipt_tokens DROP COLUMN IF EXISTS short_code;
