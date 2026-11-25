-- 20261125000080_account_deletion_requests.up.sql
-- FL1 W3 / FL-Q9: account deletion from the public web page /hapus-akun. Extends the existing
-- tenant_clearance_requests flow (POST /store/clearance) instead of a parallel table:
--   status cooling_off = pending, cancelled = cancelled, pii_anonymized_ledger_retained = completed,
--   created_at = requested_at, cooling_off_ends_at = execute_after (now 30 days).
-- Adds the request channel, an email OTP challenge table for the public flow, and a SECURITY DEFINER
-- lister so the anonymization worker can find due requests before entering the tenant transaction.

ALTER TABLE tenant_clearance_requests
    ADD COLUMN IF NOT EXISTS channel VARCHAR(10) NOT NULL DEFAULT 'pos';
ALTER TABLE tenant_clearance_requests DROP CONSTRAINT IF EXISTS chk_clearance_channel;
ALTER TABLE tenant_clearance_requests ADD CONSTRAINT chk_clearance_channel CHECK (channel IN ('pos', 'web'));
ALTER TABLE tenant_clearance_requests ALTER COLUMN cooling_off_ends_at SET DEFAULT (NOW() + INTERVAL '30 days');
CREATE UNIQUE INDEX IF NOT EXISTS uq_tenant_clearance_one_pending
    ON tenant_clearance_requests(store_id) WHERE status = 'cooling_off';

COMMENT ON COLUMN tenant_clearance_requests.channel IS 'pos = owner in the POS app, web = public /hapus-akun with email OTP';

CREATE TABLE IF NOT EXISTS account_deletion_challenges (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    store_id    UUID NOT NULL REFERENCES stores(id) ON DELETE RESTRICT,
    user_id     UUID NOT NULL,
    code_hash   CHAR(64) NOT NULL,
    attempts    SMALLINT NOT NULL DEFAULT 0,
    expires_at  TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_account_deletion_challenges_store_id UNIQUE (store_id, id),
    CONSTRAINT fk_account_deletion_challenges_user FOREIGN KEY (store_id, user_id) REFERENCES users(store_id, id) ON DELETE RESTRICT,
    CONSTRAINT chk_account_deletion_challenges_hash CHECK (code_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT chk_account_deletion_challenges_attempts CHECK (attempts BETWEEN 0 AND 10),
    CONSTRAINT chk_account_deletion_challenges_ttl CHECK (expires_at > created_at AND expires_at <= created_at + INTERVAL '15 minutes')
);
CREATE INDEX IF NOT EXISTS idx_account_deletion_challenges_user
    ON account_deletion_challenges(store_id, user_id, created_at);

ALTER TABLE account_deletion_challenges ENABLE ROW LEVEL SECURITY;

-- Due requests only: store and request ids, nothing else, for the anonymization worker.
CREATE OR REPLACE FUNCTION list_due_account_clearances()
RETURNS TABLE (store_id UUID, clearance_id UUID)
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
    SELECT tcr.store_id, tcr.id
    FROM tenant_clearance_requests tcr
    WHERE tcr.status = 'cooling_off' AND tcr.cooling_off_ends_at <= NOW()
    ORDER BY tcr.cooling_off_ends_at
    LIMIT 50
$$;
REVOKE ALL ON FUNCTION list_due_account_clearances() FROM PUBLIC;

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
    EXECUTE 'DROP POLICY IF EXISTS p_account_deletion_challenges_tenant_isolation ON account_deletion_challenges';
    EXECUTE format('CREATE POLICY p_account_deletion_challenges_tenant_isolation ON account_deletion_challenges FOR ALL TO %s
        USING (store_id = (SELECT current_store_id())) WITH CHECK (store_id = (SELECT current_store_id()))', roles);
    EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON account_deletion_challenges TO %s', roles);
    EXECUTE format('GRANT EXECUTE ON FUNCTION list_due_account_clearances() TO %s', roles);
END $$;

-- Audit actions for the public flow and the worker (governance_audit_logs keeps the 000051 list plus these).
ALTER TABLE governance_audit_logs DROP CONSTRAINT IF EXISTS chk_gov_audit_action;
ALTER TABLE governance_audit_logs ADD CONSTRAINT chk_gov_audit_action CHECK (
    action IN (
        'LEGAL_ACCEPTED', 'KYB_UPDATED', 'KYB_REVIEWED',
        'BYOK_MIDTRANS_UPDATED', 'BYOK_MIDTRANS_VERIFIED', 'BYOK_MIDTRANS_DISABLED',
        'STORE_CMS_UPDATED', 'NOTIFICATION_CONFIG_UPDATED',
        'NETWORK_NODE_UPSERTED', 'NETWORK_SIMULATION_RUN',
        'CLEARANCE_REQUESTED', 'CLEARANCE_CANCELLED', 'CLEARANCE_EXECUTED',
        'PLATFORM_INVOICE_CREATED', 'PLATFORM_INVOICE_SETTLED',
        'PLATFORM_INVOICE_WEBHOOK_SETTLED', 'PLATFORM_INVOICE_WEBHOOK_CLOSED',
        'MIKROTIK_PROFILE_UPDATED', 'MIKROTIK_RSC_GENERATED',
        'ONBOARDING_STORE_PROVISIONED', 'MOBILE_PROFILE_UPDATED',
        'CLEARANCE_OTP_ISSUED', 'CLEARANCE_PII_ANONYMIZED'
    )
);
