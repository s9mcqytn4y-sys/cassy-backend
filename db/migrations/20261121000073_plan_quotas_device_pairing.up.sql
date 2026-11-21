-- 20261121000073_plan_quotas_device_pairing.up.sql
-- PM-E1: per-plan quotas for SKUs, staff and POS devices (0 means unlimited, same as max_tables and
-- max_printers). PM-E2: single-use device pairing tokens with a 5-minute TTL (SecurityQrPairingAdr).

ALTER TABLE subscription_plans
    ADD COLUMN IF NOT EXISTS max_products INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS max_staff INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS max_devices INT NOT NULL DEFAULT 0;

ALTER TABLE subscription_plans DROP CONSTRAINT IF EXISTS chk_subscription_plans_quota_limits;
ALTER TABLE subscription_plans ADD CONSTRAINT chk_subscription_plans_quota_limits
    CHECK (max_products >= 0 AND max_staff >= 0 AND max_devices >= 0);

COMMENT ON COLUMN subscription_plans.max_products IS 'Active (non-archived) products allowed; 0 = unlimited';
COMMENT ON COLUMN subscription_plans.max_staff IS 'Active staff accounts allowed, owners included; 0 = unlimited';
COMMENT ON COLUMN subscription_plans.max_devices IS 'Non-revoked POS devices allowed; 0 = unlimited';

-- Starter and the 14-day trial are capped; Pro and Enterprise stay unlimited.
UPDATE subscription_plans
SET max_products = 100, max_staff = 5, max_devices = 2, updated_at = NOW()
WHERE code IN ('trial_14d', 'starter_umkm');

CREATE TABLE IF NOT EXISTS device_pairing_tokens (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    store_id      UUID NOT NULL REFERENCES stores(id) ON DELETE RESTRICT,
    token_hash    CHAR(64) NOT NULL,
    device_name   VARCHAR(50) NOT NULL,
    device_prefix VARCHAR(4) NOT NULL,
    issued_by     UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at    TIMESTAMPTZ NOT NULL,
    consumed_at   TIMESTAMPTZ,
    device_id     UUID,
    CONSTRAINT uq_device_pairing_tokens_store_id UNIQUE (store_id, id),
    CONSTRAINT fk_device_pairing_tokens_issued_by FOREIGN KEY (store_id, issued_by) REFERENCES users(store_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_device_pairing_tokens_device FOREIGN KEY (store_id, device_id) REFERENCES devices(store_id, id) ON DELETE RESTRICT,
    CONSTRAINT chk_device_pairing_tokens_name CHECK (length(btrim(device_name)) > 0),
    CONSTRAINT chk_device_pairing_tokens_prefix CHECK (device_prefix ~ '^[A-Z][A-Z0-9]{0,3}$'),
    CONSTRAINT chk_device_pairing_tokens_ttl CHECK (expires_at > created_at AND expires_at <= created_at + INTERVAL '5 minutes'),
    CONSTRAINT chk_device_pairing_tokens_consumed CHECK ((consumed_at IS NULL) = (device_id IS NULL))
);
CREATE INDEX IF NOT EXISTS idx_device_pairing_tokens_open
    ON device_pairing_tokens(store_id, expires_at) WHERE consumed_at IS NULL;

ALTER TABLE device_pairing_tokens ENABLE ROW LEVEL SECURITY;

-- Grant to whichever runtime roles exist (cassy_app now, mycaffe_app on clusters not yet rebranded).
DO $$
DECLARE
    roles TEXT;
BEGIN
    SELECT string_agg(quote_ident(rolname), ', ' ORDER BY rolname) INTO roles
    FROM pg_roles WHERE rolname IN ('cassy_app', 'mycaffe_app');
    IF roles IS NULL THEN
        RAISE EXCEPTION 'no runtime role (cassy_app or mycaffe_app) exists';
    END IF;
    EXECUTE 'DROP POLICY IF EXISTS p_device_pairing_tokens_tenant_isolation ON device_pairing_tokens';
    EXECUTE format('CREATE POLICY p_device_pairing_tokens_tenant_isolation ON device_pairing_tokens FOR ALL TO %s
        USING (store_id = (SELECT current_store_id())) WITH CHECK (store_id = (SELECT current_store_id()))', roles);
    EXECUTE format('GRANT SELECT, INSERT, UPDATE ON device_pairing_tokens TO %s', roles);
END $$;

ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS chk_audit_logs_action;
ALTER TABLE audit_logs ADD CONSTRAINT chk_audit_logs_action CHECK (
    action IN (
        'VOID_ORDER', 'VOID_ORDER_ITEM', 'REFUND', 'STOCK_OVERRIDE', 'PRICE_CHANGE',
        'SHIFT_OPEN', 'SHIFT_CLOSE', 'SHIFT_FORCE_CLOSE',
        'ROLE_CHANGE', 'STATIC_QRIS_RECONCILE', 'CASH_PAYMENT_RECORDED', 'EDC_RECONCILE',
        'ORDER_CREATE', 'ORDER_CANCEL', 'PAYMENT_SETTLED',
        'STAFF_LOGIN_LOCKED', 'STAFF_UNLOCK', 'STAFF_PIN_RESET', 'STAFF_DEACTIVATED',
        'STAFF_SESSION_REVOKED', 'STAFF_INVITED', 'STAFF_PIN_SET', 'STAFF_TEMP_PIN_ISSUED',
        'STORE_BOOTSTRAP', 'STORE_SETTINGS_CHANGED',
        'CASH_MOVEMENT', 'DRAWER_CREATE', 'DRAWER_UPDATE',
        'TAB_OPEN', 'ROUND_SUBMIT', 'TAB_CLOSE', 'TAB_REOPEN',
        'OWNER_PIN_FAILED',
        'TABLE_CREATE', 'TABLE_UPDATE', 'TABLE_QR_ROTATE', 'TABLE_SESSION_CLOSE',
        'PRODUCT_IMAGE_CHANGED',
        'APPROVAL_REQUESTED', 'APPROVAL_APPROVED', 'APPROVAL_REJECTED',
        'STAFF_PHONE_SET', 'CUSTOMER_PHONE_VERIFIED',
        'OUTLET_CASH_MOVEMENT',
        'ORDER_DISCOUNT',
        'STAFF_BADGE_ISSUED', 'STAFF_BADGE_REVOKED',
        'TABLE_MOVE', 'TABLE_MERGE', 'TABLE_SPLIT',
        'TABLE_TIMER_EXTEND',
        'DISPLAY_TOKEN_CREATED', 'DISPLAY_TOKEN_REVOKED',
        'PRINTER_CREATED', 'PRINTER_UPDATED', 'PRINTER_TOKEN_ROTATED', 'PRINT_REPRINT',
        'RECEIPT_SENT',
        'DEVICE_REGISTERED', 'DEVICE_RENAMED', 'DEVICE_REVOKED', 'DEVICE_ROLE_CHANGED', 'FEATURE_FLAG_CHANGED', 'DEVICE_PIN_VERIFIER_ISSUED',
        'OFFLINE_ORDER_SYNCED', 'OFFLINE_ORDER_REJECTED', 'OFFLINE_ORDER_RESOLVED', 'DEVICE_PIN_FAILED',
        'PROMOTION_CREATED', 'PROMOTION_UPDATED',
        'BUNDLE_CHANGED',
        'INGREDIENT_CREATED', 'INGREDIENT_UPDATED', 'RECIPE_CHANGED',
        'SUPPLIER_CHANGED', 'PURCHASE_ORDER_CHANGED', 'GOODS_RECEIVED',
        'PAYABLE_PAID',
        'STOCK_SESSION_SUBMITTED', 'STOCK_SESSION_APPROVED', 'STOCK_SESSION_REJECTED',
        'SALES_CHANNEL_CHANGED', 'CHANNEL_PRICES_CHANGED', 'PLATFORM_PAYMENT_RECORDED',
        'STAMP_PROGRAM_CHANGED', 'STAMP_REWARD_REDEEMED',
        'BUSINESS_CUSTOMER_CHANGED', 'INVOICE_CHARGED', 'INVOICE_ISSUED', 'INVOICE_PAYMENT_RECORDED',
        'DEVICE_PAIRING_TOKEN_ISSUED', 'DEVICE_PAIRED'
    )
);
