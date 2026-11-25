-- 20261125000080_account_deletion_requests.down.sql
-- Restore the 000051 governance audit action list. NOT VALID keeps rollback possible after rows
-- with the new actions exist; those rows stay as history.
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
        'ONBOARDING_STORE_PROVISIONED', 'MOBILE_PROFILE_UPDATED'
    )
) NOT VALID;

DROP FUNCTION IF EXISTS list_due_account_clearances();
DROP TABLE IF EXISTS account_deletion_challenges;

DROP INDEX IF EXISTS uq_tenant_clearance_one_pending;
ALTER TABLE tenant_clearance_requests ALTER COLUMN cooling_off_ends_at SET DEFAULT (NOW() + INTERVAL '7 days');
ALTER TABLE tenant_clearance_requests DROP CONSTRAINT IF EXISTS chk_clearance_channel;
ALTER TABLE tenant_clearance_requests DROP COLUMN IF EXISTS channel;
