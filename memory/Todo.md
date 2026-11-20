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
- [x] Perbaiki setup subtest 13 `TestTabLifecycleIntegration` (backend PR #154).
- [x] Pembulatan split `by_item`: alokasi kini pas ke subtotal baris lewat `lineShare` (backend PR #154).
- [x] Android: tiket dapur tidak lagi tercetak ulang saat tagihan meja dibayar (client PR #101).
- [x] Android: struk per bagian split (client PR #102).
- [ ] Android: `copies` dan `footer` printer server belum dipakai; struk per bagian belum memuat item per orang (split per item).
- [x] Android: menu KDS dan Meja di layout HP, flag `split_bill` dibaca dari profil perangkat (client PR #102).
- [ ] Backend: flag `split_bill` belum ditegakkan di endpoint split kasir (hanya disembunyikan di client).
- [x] A2: client PR #100 dan backend PR #153 merged.
- [ ] A2: uji RoleGate di tablet sungguhan (ubah peran perangkat ke `kds`, cek penahan saat shift terbuka); peran `backoffice` sementara tetap di kasir sampai layar A3 ada.
- [x] KDS sebagai layar utama: tombol kembali disembunyikan (client PR #100).
- [ ] A2 sisa: target PRD tiket pos ke kds di bawah 2 detik di LAN belum diukur.
- [ ] Lanjut A3 backoffice.
