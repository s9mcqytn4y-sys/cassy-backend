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
- [ ] Merge backend PR #152 (void item tab, migrasi 071/072, `GET /api/v1/pos/printers`): client PR #99 sudah memanggil endpoint ini.
- [ ] Uji end-to-end A1 di tablet dengan backend dan printer LAN sungguhan (tab, void, split rata/per item, printer dari server).
- [ ] Perbaiki setup subtest 13 `TestTabLifecycleIntegration` (buka shift kasir dulu; kini gagal `NO_OPEN_SHIFT`).
- [ ] Cek pembulatan split `by_item`: harga unit `subtotal / qty` integer bisa membuat alokasi kurang dari subtotal baris.
- [ ] Android: tiket dapur tercetak ulang saat tagihan meja dibayar (cart terkirim ikut dirouting lagi).
- [ ] Android: struk per bagian split (kini hanya struk penuh saat bagian terakhir lunas); salinan dan footer printer server belum dipakai.
- [ ] Android: menu KDS dan Meja belum ada di layout HP (hanya nav rail tablet); flag `split_bill` belum dibaca.
- [ ] Lanjut roadmap A2: mode KDS Android, halaman antrean web dengan display token, kelola display token di backoffice.
