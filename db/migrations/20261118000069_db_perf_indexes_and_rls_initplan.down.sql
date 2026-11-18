DROP FUNCTION IF EXISTS track_order_by_token(BYTEA);
CREATE OR REPLACE FUNCTION track_order_by_token(p_token_hash BYTEA)
RETURNS TABLE (
    store_name     VARCHAR,
    order_number   VARCHAR,
    queue_number   VARCHAR,
    status         VARCHAR,
    payment_status VARCHAR,
    updated_at     TIMESTAMPTZ
)
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
    SELECT s.name, o.order_number, o.queue_number, o.status, o.payment_status, o.updated_at
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

DROP INDEX IF EXISTS idx_guest_sessions_active_table;
DROP INDEX IF EXISTS idx_orders_store_pending_expires;
DROP INDEX IF EXISTS idx_modifier_options_store_active;
DROP INDEX IF EXISTS idx_product_variants_store_active;
DROP INDEX IF EXISTS idx_products_store_active_sort;

ALTER POLICY p_service_requests_tenant_isolation ON service_requests
    USING (store_id = current_store_id())
    WITH CHECK (store_id = current_store_id());
ALTER POLICY p_store_feature_flags_tenant_isolation ON store_feature_flags
    USING (store_id = current_store_id())
    WITH CHECK (store_id = current_store_id());
ALTER POLICY store_isolation_payments ON payments
    USING (store_id = current_store_id())
    WITH CHECK (store_id = current_store_id());
ALTER POLICY store_isolation_order_items ON order_items
    USING (store_id = current_store_id())
    WITH CHECK (store_id = current_store_id());
ALTER POLICY store_isolation_orders ON orders
    USING (store_id = current_store_id())
    WITH CHECK (store_id = current_store_id());
ALTER POLICY p_guest_sessions_tenant_isolation ON guest_sessions
    USING (store_id = current_store_id())
    WITH CHECK (store_id = current_store_id());
ALTER POLICY p_table_sessions_tenant_isolation ON table_sessions
    USING (store_id = current_store_id())
    WITH CHECK (store_id = current_store_id());
ALTER POLICY p_tables_tenant_isolation ON tables
    USING (store_id = current_store_id())
    WITH CHECK (store_id = current_store_id());
ALTER POLICY store_isolation_store_settings ON store_settings
    USING (store_id = current_store_id())
    WITH CHECK (store_id = current_store_id());
ALTER POLICY store_isolation_omg ON option_modifier_groups
    USING (store_id = current_store_id())
    WITH CHECK (store_id = current_store_id());
ALTER POLICY store_isolation_pmg ON product_modifier_groups
    USING (store_id = current_store_id())
    WITH CHECK (store_id = current_store_id());
ALTER POLICY store_isolation_modifier_options ON modifier_options
    USING (store_id = current_store_id())
    WITH CHECK (store_id = current_store_id());
ALTER POLICY store_isolation_modifier_groups ON modifier_groups
    USING (store_id = current_store_id())
    WITH CHECK (store_id = current_store_id());
ALTER POLICY store_isolation_product_variants ON product_variants
    USING (store_id = current_store_id())
    WITH CHECK (store_id = current_store_id());
ALTER POLICY store_isolation_products ON products
    USING (store_id = current_store_id())
    WITH CHECK (store_id = current_store_id());
ALTER POLICY store_isolation_categories ON categories
    USING (store_id = current_store_id())
    WITH CHECK (store_id = current_store_id());
ALTER POLICY store_isolation_users ON users
    USING (store_id = current_store_id())
    WITH CHECK (store_id = current_store_id());
ALTER POLICY store_isolation_stores ON stores
    USING (id = current_store_id())
    WITH CHECK (id = current_store_id());
