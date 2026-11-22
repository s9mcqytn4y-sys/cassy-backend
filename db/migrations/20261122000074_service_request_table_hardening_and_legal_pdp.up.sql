-- 20261122000074_service_request_table_hardening_and_legal_pdp.up.sql
-- Adds table-level temporary mute for CallWaiter (service_requests_muted_until on table_sessions)
-- and updates privacy/DPA legal documents with explicit UU PDP No. 27/2022 Controller vs Processor roles & DPO contact.

ALTER TABLE table_sessions
    ADD COLUMN IF NOT EXISTS service_requests_muted_until TIMESTAMPTZ;

-- 1. Guest Privacy Notice (id-ID -> 1.2.0, en-US -> 1.2.0)
INSERT INTO legal_documents (doc_type, version, locale, title, content_markdown, content_sha256, is_active)
SELECT doc_type, '1.2.0', locale, title, body, encode(sha256(convert_to(body, 'UTF8')), 'hex'), true
FROM (
    SELECT doc_type, locale, title,
           content_markdown || E'\n\n## Peran Pengendali dan Pemroses Data (UU PDP No. 27/2022)\n\nGerai tempat Anda memesan bertindak sebagai **Pengendali Data Pribadi** atas data transaksi, pesanan, dan pembayaran Anda. **AzureDevID** (penyedia platform Cassy) bertindak sebagai **Pemroses Data Pribadi** yang memproses data tersebut secara aman atas instruksi gerai.\n\nPertanyaan atau permintaan hak subjek data dapat dikirimkan ke gerai terkait atau kepada Petugas Pelindung Data (DPO) AzureDevID di **privacy@azuredevid.com**.' AS body
    FROM legal_documents
    WHERE doc_type = 'guest_privacy_notice' AND version = '1.1.0' AND locale = 'id-ID'
    UNION ALL
    SELECT doc_type, locale, title,
           content_markdown || E'\n\n## Data Controller and Processor Roles (Indonesia Law No. 27/2022)\n\nThe venue where you place your order acts as the **Personal Data Controller** for your order and transaction data. **AzureDevID** (provider of the Cassy platform) acts as the **Personal Data Processor** processing data securely on the venue''s instructions.\n\nData subject inquiries may be directed to the venue or to AzureDevID''s Data Protection Officer (DPO) at **privacy@azuredevid.com**.' AS body
    FROM legal_documents
    WHERE doc_type = 'guest_privacy_notice' AND version = '1.1.0' AND locale = 'en-US'
) d
ON CONFLICT (doc_type, version, locale) DO NOTHING;

UPDATE legal_documents SET is_active = false
WHERE doc_type = 'guest_privacy_notice' AND version = '1.1.0';

-- 2. Tenant Privacy Policy PDP (id-ID -> 1.1.0, en-US -> 1.1.0)
INSERT INTO legal_documents (doc_type, version, locale, title, content_markdown, content_sha256, is_active)
SELECT doc_type, '1.1.0', locale, title, body, encode(sha256(convert_to(body, 'UTF8')), 'hex'), true
FROM (
    SELECT doc_type, locale, title,
           content_markdown || E'\n\n## Pemisahan Tanggung Jawab & Kontak DPO (UU PDP No. 27/2022)\n\n- **Mitra Gerai (Pengendali Data):** Menentukan tujuan dan dasar pemrosesan data pelanggan gerai, termasuk persetujuan pemasaran atau pengiriman struk.\n- **AzureDevID (Pemroses Data):** Menyediakan infrastruktur terisolasi multi-tenant (PostgreSQL Row-Level Security) dan enkripsi kredensial BYOK.\n- **Kontak Petugas Pelindung Data (DPO):** **privacy@azuredevid.com** (Jakarta, Indonesia).' AS body
    FROM legal_documents
    WHERE doc_type = 'privacy_policy_pdp' AND version = '1.0.0' AND locale = 'id-ID'
    UNION ALL
    SELECT doc_type, locale, title,
           content_markdown || E'\n\n## Role Separation & DPO Contact (Indonesia Law No. 27/2022)\n\n- **Merchant Partner (Data Controller):** Determines the purpose and lawful basis for processing venue customer data.\n- **AzureDevID (Data Processor):** Provides multi-tenant isolated infrastructure (PostgreSQL Row-Level Security) and encrypted BYOK credential storage.\n- **Data Protection Officer (DPO) Contact:** **privacy@azuredevid.com** (Jakarta, Indonesia).' AS body
    FROM legal_documents
    WHERE doc_type = 'privacy_policy_pdp' AND version = '1.0.0' AND locale = 'en-US'
) d
ON CONFLICT (doc_type, version, locale) DO NOTHING;

UPDATE legal_documents SET is_active = false
WHERE doc_type = 'privacy_policy_pdp' AND version = '1.0.0';

-- 3. Tenant DPA (id-ID -> 1.1.0)
INSERT INTO legal_documents (doc_type, version, locale, title, content_markdown, content_sha256, is_active)
SELECT doc_type, '1.1.0', locale, title, body, encode(sha256(convert_to(body, 'UTF8')), 'hex'), true
FROM (
    SELECT doc_type, locale, title,
           content_markdown || E'\n\n## Batas Tanggung Jawab Mitra & Kontak DPO\n\nMitra Gerai bertanggung jawab atas keabsahan dasar pemrosesan data tamu di gerai masing-masing. Segala permintaan terkait kepatuhan UU PDP No. 27/2022 dan insiden keamanan data dapat disampaikan kepada Petugas Pelindung Data (DPO) AzureDevID melalui **privacy@azuredevid.com**.' AS body
    FROM legal_documents
    WHERE doc_type = 'dpa_tenant' AND version = '1.0.0' AND locale = 'id-ID'
) d
ON CONFLICT (doc_type, version, locale) DO NOTHING;

UPDATE legal_documents SET is_active = false
WHERE doc_type = 'dpa_tenant' AND version = '1.0.0';
