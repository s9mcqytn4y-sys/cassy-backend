# Backlog and Technical Debt (Cassy Backend)

Daftar terpadu utang teknis dan item kerja terbuka yang masih relevan setelah pembersihan ekosistem. Item audit v0.5.0 yang sudah lunas tercatat di [AuditBacklog.md](../Engineering/AuditBacklog.md).

## Backlog Operasional & Owner

| ID | Area | Deskripsi | Prioritas | Status |
|---|---|---|---|---|
| PM-O1 | Midtrans | Kirim koreksi dokumen Business Review (website, selfie, alur ShopeePay) | Tinggi | Menunggu Owner |
| PM-O2 | Legal & Merek | Pengecekan merek "Cassy" di PDKI dan tinjauan DPA / PP 33/2026 | Sedang | Menunggu Owner |
| PM-O3 | Email Produksi | Migrasi pengiriman email produksi dari Gmail SMTP ke Amazon SES (AL6) | Sedang | Planned |
| PM-O4 | WhatsApp | Verifikasi bisnis Meta dan persetujuan template `struk_digital` & `tagihan_invoice` | Sedang | Blocked by Meta |

## Backlog Engineering

| ID | Area | Deskripsi | Prioritas | Pemicu / Target |
|---|---|---|---|---|
| PM-E1 | SaaS Billing | Penegakan batas paket langganan (limit SKU, outlet, staf) di service layer | Tinggi | Implemented & Tested (migrasi 000066, `ensurePlanQuota`, 402 `PLAN_LIMIT_REACHED`): SKU, staf, perangkat, meja, printer. Kuota outlet Blocked by Dependency (model data satu toko per tenant) |
| PM-E2 | Device Pairing | Endpoint `pairing_token` single-use TTL 5 menit (`SecurityQrPairingAdr`) | Sedang | Implemented & Tested (backend): `POST /api/v1/pos/devices/pairing-token` + `POST /api/v1/pos/devices/pair`. UI QR Owner Dashboard dan sambungan Android masih Planned |
| PM-E3 | Observability Log | Samarkan bearer token pada path `/receipts/*`, `/r/*`, `/bagian/*` di `access.log` Nginx | Sedang | Implemented (repo `cassy-docker`): `map $request_uri $masked_request_uri` + `$masked_http_referer` di `NginxProd.conf` dan `NginxDemo.conf` |
| PM-E4 | Security CI Client | Penambahan secret scanner (Gitleaks) pada workflow `cassy-client` dan `cassy-docker` | Rendah | Selesai v2.5.0 |
| PM-E5 | Database Hygiene | `CassyPostgresDev` dibersihkan dengan `./scripts/Invoke-CassyDev.ps1 reset-db` (migration + clean dev seed); integration tests memakai `cassy-pg-integration` | Sedang | Selesai v2.5.0 |
| PM-E6 | Performance / Quote | Audit latensi endpoint `GET /catalog` & `POST /orders/quote` saat konkurensi multi-browser via Next.js rewrite | Sedang | Implemented & Tested: singleflight per slug untuk katalog publik, `stale-while-revalidate=30`, quote selalu `no-store`. Pengukuran latensi di demo masih Planned |
| PM-E7 | OpenAPI Parity | Rekonsiliasi `cmd/api/routes.go` dengan `docs/Api/OpenApi.yaml` (`/health/live`, `/api/v1/health`, `/api/v1/mobile/bootstrap`, `/public/receipts/{token}/pdf`, `short/{code}`) + regenerasi `codegen:api` | Sedang | Selesai v2.5.0 |
| PM-E8 | Docker & Cron Naming | Pembaruan path & binary `cassy-*` di `Docker/Backup/CassyBackup*.cron` (`/opt/cassy`, `/app/cassy-migrate`), `Docker/Core/*.yml`, dan `cassy-api` / `cassy-web` / `cassy-wa-bridge` GHCR | Rendah | Selesai v2.5.0 |
