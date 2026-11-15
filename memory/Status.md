# Cassy Backend: Status (`memory/Status.md`)

**Last Updated**: 2026-11-16
**Branch dasar**: `main` (trunk-based across `cassy-backend`, `cassy-client`, `cassy-docker`)
**Skema DB**: 65 migrasi PostgreSQL 18 RLS (`db/migrations/000001` s.d. `000065`)
**Source of Truth PM**: [docs/ProjectManagement/SprintAndReleasePlan.md](../docs/ProjectManagement/SprintAndReleasePlan.md)

| Area | Status | Catatan |
|---|---|---|
| PostgreSQL 18 + RLS | Implemented & Tested | 65 migrasi aktif; role runtime non-owner `cassy_app` (`NOBYPASSRLS`) |
| Midtrans Core API, BYOK per tenant | Implemented & Tested (Sandbox) | Menunggu persetujuan Business Review Midtrans |
| Stok, kanal ojol manual, stamp card, invoice B2B | Implemented & Tested | Layanan backend siap; layar client menyusul bertahap |
| Guest QR PWA & Android POS (`cassy-client`) | Implemented & Tested (`v2.5.0`) | Android ServerConfig mDNS + Web PWA QR Order/Receipt/Legal terhubung OpenAPI |
| Infra Docker & VPS Gateway (`cassy-docker`) | Ready (`main`) | Stack Core & VpsGateway (`CassyMeshNet` `172.29.20.0/24`) terpisah di repo sendiri |
| Email produksi & WhatsApp | Guarded / Planned | Gmail SMTP aktif untuk demo; produksi target SES (AL6) dan Meta Cloud API |
