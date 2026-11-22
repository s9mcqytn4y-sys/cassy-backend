-- 20261122000075_product_recommendations_and_station_prep.up.sql
-- Proposal B-04 (Batch 2):
-- 1. Optional badge_label and prep_minutes on products.
-- 2. Tenant-scoped many-to-many product_recommendations table for Smart Pairing.
-- 3. Tenant-scoped kitchen_station_configs table for per-station KDS prep time defaults & queue load.
-- 4. Extend track_order_by_token(BYTEA) with server-calculated estimated_prep_minutes.

ALTER TABLE products
    ADD COLUMN IF NOT EXISTS badge_label VARCHAR(40),
    ADD COLUMN IF NOT EXISTS prep_minutes INT CHECK (prep_minutes IS NULL OR (prep_minutes >= 1 AND prep_minutes <= 120));

CREATE TABLE IF NOT EXISTS product_recommendations (
    store_id UUID NOT NULL,
    product_id UUID NOT NULL,
    recommended_product_id UUID NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (store_id, product_id, recommended_product_id),
    CONSTRAINT chk_product_recommendations_not_self CHECK (product_id <> recommended_product_id),
    CONSTRAINT fk_pr_store FOREIGN KEY (store_id) REFERENCES stores(id) ON DELETE CASCADE,
    CONSTRAINT fk_pr_product FOREIGN KEY (store_id, product_id) REFERENCES products(store_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_pr_recommended FOREIGN KEY (store_id, recommended_product_id) REFERENCES products(store_id, id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_product_recommendations_lookup
    ON product_recommendations (store_id, product_id, sort_order);

ALTER TABLE product_recommendations ENABLE ROW LEVEL SECURITY;
ALTER TABLE product_recommendations FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS store_isolation_product_recommendations ON product_recommendations;
CREATE POLICY store_isolation_product_recommendations ON product_recommendations
    USING (store_id = (SELECT current_store_id()))
    WITH CHECK (store_id = (SELECT current_store_id()));

CREATE TABLE IF NOT EXISTS kitchen_station_configs (
    store_id UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    prep_station VARCHAR(32) NOT NULL,
    default_prep_minutes INT NOT NULL DEFAULT 5 CHECK (default_prep_minutes >= 1 AND default_prep_minutes <= 120),
    per_active_order_minutes INT NOT NULL DEFAULT 2 CHECK (per_active_order_minutes >= 0 AND per_active_order_minutes <= 30),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (store_id, prep_station)
);

ALTER TABLE kitchen_station_configs ENABLE ROW LEVEL SECURITY;
ALTER TABLE kitchen_station_configs FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS store_isolation_kitchen_station_configs ON kitchen_station_configs;
CREATE POLICY store_isolation_kitchen_station_configs ON kitchen_station_configs
    USING (store_id = (SELECT current_store_id()))
    WITH CHECK (store_id = (SELECT current_store_id()));

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'cassy_app') THEN
        EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON product_recommendations TO cassy_app';
        EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON kitchen_station_configs TO cassy_app';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'mycaffe_app') THEN
        EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON product_recommendations TO mycaffe_app';
        EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON kitchen_station_configs TO mycaffe_app';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'cassy_backup') THEN
        EXECUTE 'GRANT SELECT ON product_recommendations TO cassy_backup';
        EXECUTE 'GRANT SELECT ON kitchen_station_configs TO cassy_backup';
    END IF;
END $$;

-- Extend track_order_by_token to expose estimated_prep_minutes from item/station config + active queue load.
DROP FUNCTION IF EXISTS track_order_by_token(BYTEA);
CREATE OR REPLACE FUNCTION track_order_by_token(p_token_hash BYTEA)
RETURNS TABLE (
    store_name             VARCHAR,
    order_number           VARCHAR,
    queue_number           VARCHAR,
    status                 VARCHAR,
    payment_status         VARCHAR,
    updated_at             TIMESTAMPTZ,
    expires_at             TIMESTAMPTZ,
    estimated_prep_minutes INT
)
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
    SELECT
        s.name,
        o.order_number,
        o.queue_number,
        o.status,
        o.payment_status,
        o.updated_at,
        o.expires_at,
        CASE
            WHEN o.status IN ('ready', 'served', 'completed', 'cancelled') THEN 0
            ELSE (
                COALESCE((
                    SELECT MAX(COALESCE(p.prep_minutes, ksc.default_prep_minutes, 5))
                    FROM order_items oi
                    LEFT JOIN products p ON p.store_id = oi.store_id AND p.id = oi.product_id
                    LEFT JOIN kitchen_station_configs ksc ON ksc.store_id = oi.store_id AND ksc.prep_station = p.prep_station
                    WHERE oi.store_id = o.store_id AND oi.order_id = o.id
                ), 5)
                + LEAST(30, COALESCE((
                    SELECT COUNT(*)::int * COALESCE((
                        SELECT MAX(k.per_active_order_minutes)
                        FROM kitchen_station_configs k
                        WHERE k.store_id = o.store_id
                    ), 2)
                    FROM orders q
                    WHERE q.store_id = o.store_id
                      AND q.id <> o.id
                      AND q.status IN ('confirmed', 'preparing')
                      AND q.created_at < o.created_at
                ), 0))
            )
        END AS estimated_prep_minutes
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
