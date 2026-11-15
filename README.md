# Cassy Core POS: Backend API

Backend RESTful API dan Realtime Hub untuk platform white-label multi-tenant QR Table Ordering, Point of Sale (POS), Kitchen Display System (KDS), dan Queue Display untuk bisnis F&B. Lihat [Brand Guidelines dan Product Knowledge](docs/Product/BrandAndProductKnowledge.md) serta dokumen desain klien untuk sistem visual **Pandan & Kunyit** (`#1F5F52`, `#E9A23B`, `#F7F5F0`, `#17211E`).

## Tech Stack

- **Language**: Go 1.27.4
- **Web Framework**: Gin Gonic (`v1.12.0`)
- **Database Driver**: `jackc/pgx/v5` dan `jmoiron/sqlx`
- **Database Engine**: PostgreSQL 18-alpine (Native Row-Level Security & Triple-Role Isolation: `postgres`, `cassy_migrator`, `cassy_app` `NOBYPASSRLS`)
- **Infrastructure**: Repo terpisah [`cassy-docker`](https://github.com/s9mcqytn4y-sys/cassy-docker) (`C:\Projects\CassyWorkspace\Docker`) untuk stack dev, demo, prod, dan VPS Gateway
- **Payment Gateway**: Midtrans Core API (`qris`, `bank_transfer`, `gopay`, `shopeepay`, `card`, `cstore`) + Cashier Offline (`cash`, `static_qris`, `edc`)
- **Realtime Hub**: Gorilla WebSocket (`internal/realtime`) dengan outbox relay transaksional
- **Cloudflare R2 Storage**: Triple-bucket untuk media publik (`https://cdn.abbdullaziz.online`), media privat presigned, dan backup terenkripsi
- **Monitoring & Error Tracking**: Sentry Go SDK (panic/5xx recovery & PII scrubbing) + Prometheus `/metrics` (`:9091`) & Grafana Alloy

## Security & Architecture Guarantees

1. **Multi-Tenant Scoping**: Setiap query dan mutasi berjalan melalui `database.WithTenantTx` menggunakan PostgreSQL 18 RLS dengan role runtime non-owner `cassy_app` (`NOBYPASSRLS`).
2. **Payment Integrity**: Harga produk, opsi modifier, diskon, service charge, dan pajak dihitung ulang sepenuhnya di server memakai integer Rupiah (`int64` / `BIGINT`) dan Basis Points (`BPS`).
3. **Order Idempotency & Atomic Stock Locking**: `OrderService` mengunci stok terurut (`FOR UPDATE`), memeriksa `idempotency_key` + `orders.request_hash` (SHA-256), mengalokasikan nomor urut harian, dan menulis `outbox_events` dalam satu transaksi ACID.
4. **Webhook Verification**: Notifikasi Midtrans diverifikasi memakai signature SHA-512 resmi sebelum transisi status bayar.
5. **Fail-Closed Operations**: Startup ditolak bila `DATABASE_URL` kosong atau `JWT_SECRET_KEY` lemah di lingkungan non-dev. Endpoint publik memakai token bertanda tangan dan resolver `SECURITY DEFINER` yang sempit.
6. **Zero In-Place Coding on Production VPS**: Seluruh perubahan wajib melalui branch pendek, PR hijau CI, merge ke `main`, lalu deploy digest image GHCR melalui `cassy-docker`.

## Feature Status

Status per modul dan rilis dipelihara terpusat di [Project Management Source of Truth](docs/ProjectManagement/README.md), [Roadmap Arsitektur](docs/Architecture/Roadmap.md), dan [Kontrak Integrasi Frontend](docs/Architecture/FrontendIntegrationContract.md). Skema database saat ini berada di 65 migrasi (`db/migrations/000001` s.d. `000065`).

## Local Development Setup (Windows)

Prasyarat: Go 1.27.4, Docker Desktop. Perintah harian tersedia di `scripts/dev.ps1` (detail: [WindowsDevEnvironment.md](docs/Setup/WindowsDevEnvironment.md)).

```powershell
./scripts/dev.ps1 setup      # .env dari template, PostgreSQL 18 lokal, migrasi, seed
./scripts/dev.ps1 run        # API di http://localhost:8080
./scripts/dev.ps1 login      # uji login staf dengan PIN dev
./scripts/dev.ps1 test-int   # tes integrasi PostgreSQL 18 (setara CI)
./scripts/dev.ps1 check      # gofmt, vet, mod tidy, govulncheck, tes
./scripts/dev.ps1 demo-up    # demo publik lewat Cloudflare Quick Tunnel
```

## Ekosistem & Dokumentasi

| Topik | Dokumen |
|---|---|
| Indeks Dokumentasi | [docs/README.md](docs/README.md) |
| Project Management (PM SoT) | [docs/ProjectManagement/README.md](docs/ProjectManagement/README.md) |
| Keputusan Owner & Roadmap | [DecisionRegister.md](docs/Architecture/DecisionRegister.md), [Roadmap.md](docs/Architecture/Roadmap.md) |
| Keamanan & Kontrak API | [SecurityRiskRegister.md](docs/Architecture/SecurityRiskRegister.md), [OpenApi.yaml](docs/Api/OpenApi.yaml) |
| Operasional & Setup | [WorkspaceDirectoryMap.md](docs/Setup/WorkspaceDirectoryMap.md), [DemoReadinessChecklist.md](docs/Setup/DemoReadinessChecklist.md) |

Repositori terkait dalam `C:\Projects\CassyWorkspace`:
- **`cassy-client`** (`C:\Projects\CassyWorkspace\CassyClient`): Guest QR PWA (`apps/web`), Cassy POS Android (`apps/android`), iOS Hardware Bridge (`apps/ios`), dan Design Tokens (`packages/design-tokens`).
- **`cassy-docker`** (`C:\Projects\CassyWorkspace\Docker`): Konfigurasi Compose Dev/Demo/Prod, Nginx, PostgreSQL role init, Monitoring, dan VPS Gateway.
- **`AzureDevId`** (`C:\Users\Acer\OneDrive\Documents\AzureDevId`): Dokumen legalitas, merek, recovery, dan payment gateway.

## Git Branching & Release Strategy

- `main`: Branch tunggal terproteksi dengan konvensi Trunk-Based Development. Lihat [GitWorkflowAndReleases.md](docs/Engineering/GitWorkflowAndReleases.md).
- Branch pendek (`feat/*`, `fix/*`, `docs/*`, `chore/*`) digabung melalui Pull Request setelah seluruh gerbang CI hijau.
