-- 20261122000075_product_recommendations_and_station_prep.down.sql

DROP FUNCTION IF EXISTS track_order_by_token(BYTEA);
CREATE OR REPLACE FUNCTION track_order_by_token(p_token_hash BYTEA)
RETURNS TABLE (
    store_name     VARCHAR,
    order_number   VARCHAR,
    queue_number   VARCHAR,
    status         VARCHAR,
    payment_status VARCHAR,
    updated_at     TIMESTAMPTZ,
    expires_at     TIMESTAMPTZ
)
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
    SELECT s.name, o.order_number, o.queue_number, o.status, o.payment_status, o.updated_at, o.expires_at
    FROM orders o
    JOIN stores s ON s.id = o.store_id AND s.deleted_at IS NULL
    WHERE o.tracking_token_hash = p_token_hash
      AND NOT (o.status IN ('served', 'completed', 'cancelled') AND o.updated_at < NOW() - INTERVAL '24 hours')
    LIMIT 1;
$$;

REVOKE ALL ON FUNCTION track_order_by_token(BYTEA) FROM PUBLIC;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'cassy_app') THEN
        EXECUTE 'GRANT EXECUTE ON FUNCTION track_order_by_token(BYTEA) TO cassy_app';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'mycaffe_app') THEN
        EXECUTE 'GRANT EXECUTE ON FUNCTION track_order_by_token(BYTEA) TO mycaffe_app';
    END IF;
END $$;

DROP TABLE IF EXISTS kitchen_station_configs;
DROP TABLE IF EXISTS product_recommendations;

ALTER TABLE products
    DROP COLUMN IF EXISTS prep_minutes,
    DROP COLUMN IF EXISTS badge_label;
