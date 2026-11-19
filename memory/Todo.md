# Cassy Backend: Antrean (`memory/Todo.md`)

Lihat antrean terpusat lintas repositori pada [BacklogAndDebt.md](../docs/ProjectManagement/BacklogAndDebt.md).

## Owner / Direktur
- [ ] Cek merek "Cassy" di PDKI (kelas 9 dan 42).
- [ ] Kirim koreksi Midtrans: website, foto selfie, Payment Flow ShopeePay.
- [ ] Periksa Surat Pelimpahan Dana, DPA, dan kesiapan UU PDP / PP 33/2026 sebelum pilot.
- [ ] Isi kunci API provider gratis eksternal Hermes di `~/AppData/Local/hermes/.env` bila diperlukan.

## Engineering
- [x] Pisahkan Docker infra ke repo `s9mcqytn4y-sys/cassy-docker`, rename repo menjadi `cassy-backend` dan `cassy-client`, rapikan branch `main`.
- [x] Standarisasi `.editorconfig`, `.gitattributes`, `.vscode/settings.json`, dan `docs/ProjectManagement/` sebagai PM Source of Truth.
- [ ] Terapkan penegakan batas paket langganan SaaS, driver Amazon SES, dan rotasi key per tenant.
- [ ] Aktifkan masking bearer token path `/r/*`, `/receipts/*`, `/bagian/*` di Nginx log sebelum pilot VPS.
- [x] Merge backend PR #152 (void item tab, migrasi 071/072, `GET /api/v1/pos/printers`).
- [ ] Uji end-to-end A1 di tablet dengan backend dan printer LAN sungguhan (tab, void, split rata/per item, printer dari server).
- [ ] Perbaiki setup subtest 13 `TestTabLifecycleIntegration` (buka shift kasir dulu; kini gagal `NO_OPEN_SHIFT`).
- [ ] Cek pembulatan split `by_item`: harga unit `subtotal / qty` integer bisa membuat alokasi kurang dari subtotal baris.
- [ ] Android: tiket dapur tercetak ulang saat tagihan meja dibayar (cart terkirim ikut dirouting lagi).
- [ ] Android: struk per bagian split (kini hanya struk penuh saat bagian terakhir lunas); salinan dan footer printer server belum dipakai.
- [ ] Android: menu KDS dan Meja belum ada di layout HP (hanya nav rail tablet); flag `split_bill` belum dibaca.
- [ ] A2: buka PR client `feat/a2-queue-display` dan backend `feat/a2-display-url` (backend dulu, agar tautan layar terisi).
- [ ] A2: uji RoleGate di tablet sungguhan (ubah peran perangkat ke `kds`, cek penahan saat shift terbuka); peran `backoffice` sementara tetap di kasir sampai layar A3 ada.
- [ ] A2 sisa: KDS sebagai layar utama tidak punya tombol kembali yang berguna; target PRD tiket pos ke kds di bawah 2 detik di LAN belum diukur.
