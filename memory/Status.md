# Cassy Backend: Status (`memory/Status.md`)

**Last Updated**: 2026-11-20
**Branch dasar**: `main` (trunk-based across `cassy-backend`, `cassy-client`, `cassy-docker`)
**Skema DB**: 72 migrasi PostgreSQL 18 RLS (`db/migrations/000001` s.d. `20261119000072`)
**Source of Truth PM**: [docs/ProjectManagement/SprintAndReleasePlan.md](../docs/ProjectManagement/SprintAndReleasePlan.md)

| Area | Status | Catatan |
|---|---|---|
| PostgreSQL 18 + RLS | Implemented & Tested | 72 migrasi aktif; role runtime non-owner `cassy_app` (`NOBYPASSRLS`) |
| Midtrans Core API, BYOK per tenant | Implemented & Tested (Sandbox) | Menunggu persetujuan Business Review Midtrans |
| Stok, kanal ojol manual, stamp card, invoice B2B | Implemented & Tested | Layanan backend siap; layar client menyusul bertahap |
| Guest QR PWA & Android POS (`cassy-client`) | Implemented & Tested (`v2.5.0`) | Android ServerConfig mDNS + Web PWA QR Order/Receipt/Legal terhubung OpenAPI |
| Android POS A1 (kasir lengkap) | Implemented & Tested (unit + screenshot CI), belum e2e | Client PR #99 merged 2026-11-19 (`8990d85`): meja + open bill, panggilan pelayan, void item tab dengan PIN/approval owner, bagi tagihan meja rata dan per item, printer dari server. Backend pendukung (migrasi 071 void, 072 `printers.lan_address`, `GET /pos/printers`) PR #152 merged |
| A2 Dapur dan antrean | Implemented & Tested (unit + e2e web + screenshot CI), belum diuji di tablet/TV | Client PR #100 merged: halaman web `/antrean` (display token di fragmen URL, polling 5 s + WS, wake lock), tab Android "Layar antrean", fix header `Device` untuk sync offline, RoleGate dari `GET /device/catalog`. Backend PR #153 merged: `display_url` saat token dibuat |
| Gelombang bugfix A1/A2 (2026-11-20) | Merged, CI hijau | Client #101 (tiket dapur tidak dicetak ulang), #102 (menu HP KDS/Meja, flag `split_bill`, struk per bagian, test RoleGate yang membuat CI hang 6 jam + `timeout-minutes: 30`). Backend #154 (`by_item` pas ke rupiah, subtest 13) |
| Infra Docker & VPS Gateway (`cassy-docker`) | Ready (`main`) | Stack Core & VpsGateway (`CassyMeshNet` `172.29.20.0/24`) terpisah di repo sendiri |
| Email produksi & WhatsApp | Guarded / Planned | Gmail SMTP aktif untuk demo; produksi target SES (AL6) dan Meta Cloud API |
