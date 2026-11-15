-- 20261115000064_receipt_short_codes.up.sql
-- Short URL support (/r/{shortCode}) for digital receipts in WhatsApp, SMS and email.

ALTER TABLE receipt_tokens
    ADD COLUMN IF NOT EXISTS short_code VARCHAR(12);

CREATE UNIQUE INDEX IF NOT EXISTS uq_receipt_tokens_short_code
    ON receipt_tokens(short_code)
    WHERE short_code IS NOT NULL;

CREATE OR REPLACE FUNCTION resolve_receipt_short_code(p_code TEXT)
RETURNS TABLE (store_id UUID, order_id UUID, expires_at TIMESTAMPTZ)
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
    SELECT r.store_id, r.order_id, r.expires_at
    FROM receipt_tokens r
    WHERE r.short_code = p_code
    LIMIT 1;
$$;

REVOKE ALL ON FUNCTION resolve_receipt_short_code(TEXT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION resolve_receipt_short_code(TEXT) TO mycaffe_app;
