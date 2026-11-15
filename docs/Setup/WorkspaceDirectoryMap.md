# Peta Distribusi Folder Workspace & Ekosistem Cassy

Dokumen ini menjadi acuan tunggal AI Agent (Antigravity, Claude Code, Codex, Hermes, OpenCode) dan Engineer manusia di workstation Windows 11.

## 1. Direktori Aktif Ekosistem Cassy (`C:\Projects\CassyWorkspace`)

| Path Absolut | Repo GitHub | Peran & Cakupan | Status |
|---|---|---|---|
| `C:\Projects\CassyWorkspace\CassyBackend` | `s9mcqytn4y-sys/cassy-backend` | Backend Go 1.27, PostgreSQL 18 RLS (65 migrasi), Midtrans Core API, `wa-bridge`, `print-agent` | Aktif (`main`) |
| `C:\Projects\CassyWorkspace\CassyClient` | `s9mcqytn4y-sys/cassy-client` | Monorepo client: Guest QR PWA (`apps/web`), Android POS (`apps/android`), iOS Bridge (`apps/ios`), `packages/design-tokens` | Aktif (`main`) |
| `C:\Projects\CassyWorkspace\Docker` | `s9mcqytn4y-sys/cassy-docker` (private) | Docker Compose Dev/Demo/Prod, `VpsGateway` (`CassyVpsRedis`, `CassyVpsN8n`, `CassyVpsHermesGateway`), Nginx, PostgreSQL init, Monitoring | Aktif (`main`) |
| `C:\Projects\CassyWorkspace\Worktrees` | Sub-worktree lokal | Worktree terpisah (`cassy-backend-agy`, `cassy-backend-claude`, `cassy-client-agy`, `cassy-client-claude`) untuk eksperimen dan verifikasi paralel | Aktif |
| `C:\Projects\CassyWorkspace\Scripts` | Lokal | Skrip utilitas sinkronisasi workspace dan worktree | Aktif |

## 2. Brankas Bisnis & Identitas (`C:\Users\Acer\OneDrive\Documents\AzureDevId`)

Direktori ini berisi aset merek resmi (`BrandAssetsLogo`, `LegalAndBrandGuidelines`), legalitas (`LegalitasPerizinan`), analisis produk/kompetitor (`ProductSpecCompetitor`), serta dokumen gateway/keamanan (`MerchantPaymentGateway`, `SecurityRecovery`). Semua agent memiliki akses baca untuk keperluan audit dan pengembangan, dengan larangan mutlak mencetak, menyalin ke log, atau meng-commit kredensial dan kode pemulihan.
