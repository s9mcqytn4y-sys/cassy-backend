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
- [x] Android: `copies` dan `footer` printer server dipakai (`PosPrintDispatcher`); dibuktikan test live S2 `LiveCashierLifecycleTest`.
- [ ] Android: struk per bagian belum memuat item per orang (split per item).
- [x] S3 OPS-06..08 terbukti live di CI (client #132, backend #170/#171, docker #12); merge #171 dulu agar govulncheck hijau.
- [ ] Android: `MobileBootstrapData` membaca `hardware_config`, server mengirim `hardware`; konfigurasi hardware dari bootstrap tidak pernah terbaca.
- [x] S4: OPS-09 offline, cup label, live PWA-01/02/03/05/06, PWA-10 `check:copy` (client #133..#137, backend #172..#176, docker #13/#14).
- [ ] Android: notifikasi sistem belum ada; `CassyNotificationManager` tanpa pemanggil dan izin `POST_NOTIFICATIONS` tidak pernah diminta (butuh foreground service).
- [ ] Android: dapur offline W18 (tiket dicetak lewat print agent LAN, `printed_locally`) belum dipakai POS.
- [ ] Gate PWA-08 (k6 50 klien satu IP) dan PWA-09 (Lighthouse CI) belum dikerjakan.
- [x] Android: menu KDS dan Meja di layout HP, flag `split_bill` dibaca dari profil perangkat (client PR #102).
- [x] Backend: flag `split_bill` ditegakkan di `dividePayFirstOrderTx` (split kasir non-tab dan tamu) dan `CloseTab` (split meja).
- [x] Role `manager` K-04 (PR #167, migrasi 000078) dan tender EDC Android (client PR #118).
- [x] Seed uji `db/seeds/E2eLifecycle.sql` untuk stack test (PR #168).
- [x] A2: client PR #100 dan backend PR #153 merged.
- [ ] A2: uji RoleGate di tablet sungguhan (ubah peran perangkat ke `kds`, cek penahan saat shift terbuka); peran `backoffice` sementara tetap di kasir sampai layar A3 ada.
- [x] KDS sebagai layar utama: tombol kembali disembunyikan (client PR #100).
- [ ] A2 sisa: target PRD tiket pos ke kds di bawah 2 detik di LAN belum diukur.
- [ ] Lanjut A3 backoffice setelah Ops Close Gate dan PWA Close Gate hijau (`Docs/PRD/PrdOpsAndPwaClose.md`).
