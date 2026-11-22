-- 20261122000074_service_request_table_hardening_and_legal_pdp.down.sql

DELETE FROM legal_documents WHERE doc_type = 'guest_privacy_notice' AND version = '1.2.0';
UPDATE legal_documents SET is_active = true WHERE doc_type = 'guest_privacy_notice' AND version = '1.1.0';

DELETE FROM legal_documents WHERE doc_type = 'privacy_policy_pdp' AND version = '1.1.0';
UPDATE legal_documents SET is_active = true WHERE doc_type = 'privacy_policy_pdp' AND version = '1.0.0';

DELETE FROM legal_documents WHERE doc_type = 'dpa_tenant' AND version = '1.1.0';
UPDATE legal_documents SET is_active = true WHERE doc_type = 'dpa_tenant' AND version = '1.0.0';

ALTER TABLE table_sessions
    DROP COLUMN IF EXISTS service_requests_muted_until;
