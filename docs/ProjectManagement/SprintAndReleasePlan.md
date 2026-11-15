# Sprint and Release Plan (Cassy Backend)

## Status Ekosistem Saat Ini

| Komponen | Versi / Baseline | Status | Repositori |
|---|---|---|---|
| Core API & PostgreSQL 18 RLS | 65 migrasi (`000001`-`000065`), `v0.12.x` -> `v2.5.0` aligned | Implemented & Tested | `s9mcqytn4y-sys/cassy-backend` |
| Guest QR PWA & Android POS | Next.js 15 / React 19 + Kotlin Compose (`v2.5.0`) | Implemented & Tested | `s9mcqytn4y-sys/cassy-client` |
| Infra Docker & VPS Gateway | Triple-Role PG18, Nginx, Monitoring, MeshNet `172.29.20.0/24` | Config Ready (`main`) | `s9mcqytn4y-sys/cassy-docker` |

## Milestone Aktif

1. **Stabilisasi Workspace & CI**: Standarisasi `.editorconfig`, `.gitattributes`, `.vscode/settings.json`, workflow GitHub Actions minimal-privilege, dan dokumentasi `ProjectManagement/`.
2. **Verifikasi E2E Live Backend**: Memastikan alur Guest QR PWA (`/order/[token]`, checkout, tracker, `/r/[code]`, `/legal`) teruji terhadap PostgreSQL 18 ter-seed dengan akun uji Android.
3. **Persiapan Pilot & Produksi VPS**: Penegakan paket langganan SaaS, aktivasi Midtrans Business Review, dan pembaruan halaman publik `abbdullaziz.online`.
