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
| PM-E1 | SaaS Billing | Penegakan batas paket langganan (limit SKU, outlet, staf) di service layer | Tinggi | Sebelum pilot berbayar |
| PM-E2 | Device Pairing | Endpoint `pairing_token` single-use TTL 5 menit (`SecurityQrPairingAdr`) | Sedang | Saat modul generator QR Owner dibangun |
| PM-E3 | Observability Log | Samarkan bearer token pada path `/receipts/*`, `/r/*`, `/bagian/*` di `access.log` Nginx | Sedang | Sebelum pengiriman log ke pihak ketiga |
| PM-E4 | Security CI Client | Penambahan secret scanner (Gitleaks) pada workflow `cassy-client` dan `cassy-docker` | Rendah | Selesai v2.5.0 |
| PM-E5 | Database Hygiene | `CassyPostgresDev` dibersihkan dengan `./scripts/Invoke-CassyDev.ps1 reset-db` (migration + clean dev seed); integration tests memakai `cassy-pg-integration` | Sedang | Selesai v2.5.0 |
| PM-E6 | Performance / Quote | Audit latensi endpoint `GET /catalog` & `POST /orders/quote` saat konkurensi multi-browser via Next.js rewrite | Sedang | Next Sprint |
| PM-E7 | OpenAPI Parity | Rekonsiliasi `cmd/api/routes.go` dengan `docs/Api/OpenApi.yaml` (`/health/live`, `/api/v1/health`, `/api/v1/mobile/bootstrap`, `/public/receipts/{token}/pdf`, `short/{code}`) + regenerasi `codegen:api` | Sedang | Selesai v2.5.0 |
| PM-E8 | Docker & Cron Naming | Pembaruan path & binary `cassy-*` di `Docker/Backup/CassyBackup*.cron` (`/opt/cassy`, `/app/cassy-migrate`), `Docker/Core/*.yml`, dan `cassy-api` / `cassy-web` / `cassy-wa-bridge` GHCR | Rendah | Selesai v2.5.0 |
