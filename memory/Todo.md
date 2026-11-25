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
- [x] Android: struk per bagian memuat item per orang; split rata memuat daftar meja (client #138).
- [x] S3 OPS-06..08 terbukti live di CI (client #132, backend #170/#171, docker #12); merge #171 dulu agar govulncheck hijau.
- [x] Android: field mati `hardware_config` dihapus dari bootstrap (client #138).
- [x] S4: OPS-09 offline, cup label, live PWA-01/02/03/05/06, PWA-10 `check:copy` (client #133..#137, backend #172..#176, docker #13/#14).
- [ ] Android: notifikasi sistem belum ada (butuh foreground service); kode mati `CassyNotificationManager` dihapus di client #138.
- [ ] Android: dapur offline W18 (tiket dicetak lewat print agent LAN, `printed_locally`) belum dipakai POS.
- [x] S5: PWA-08 k6, PWA-09 Lighthouse, OPS-02/04/05 live, archtest, runbook VPS (backend #178/#179, docker #15, client #138..#140).
- [ ] Drill RTO di VPS pilot setelah provisioning; tag rilis sama di `cassy-backend` dan `cassy-docker`.
- [ ] A3 awal (ditahan): upgrade Compose BOM/Kotlin, pecah `PosViewModel`, ekstrak `:core:*` untuk Cassy Manager.
- [x] Android: menu KDS dan Meja di layout HP, flag `split_bill` dibaca dari profil perangkat (client PR #102).
- [x] Backend: flag `split_bill` ditegakkan di `dividePayFirstOrderTx` (split kasir non-tab dan tamu) dan `CloseTab` (split meja).
- [x] Role `manager` K-04 (PR #167, migrasi 000078) dan tender EDC Android (client PR #118).
- [x] Seed uji `db/seeds/E2eLifecycle.sql` untuk stack test (PR #168).
- [x] A2: client PR #100 dan backend PR #153 merged.
- [ ] A2: uji RoleGate di tablet sungguhan (ubah peran perangkat ke `kds`, cek penahan saat shift terbuka); peran `backoffice` sementara tetap di kasir sampai layar A3 ada.
- [x] KDS sebagai layar utama: tombol kembali disembunyikan (client PR #100).
- [ ] A2 sisa: target PRD tiket pos ke kds di bawah 2 detik di LAN belum diukur.
- [x] Ops Close Gate dan PWA Close Gate lulus simulasi (2026-11-25). A3 ditahan Director; gelombang FL1 berjalan (`Docs/PRD/PrdFrontendUxAndLaunch.md`).
- [x] FL1 W1: nama MyCaffe di SMS OTP, reset PIN, jembatan SMS-email (warna Espresso ke Pandan), skrip MikroTik, label hop jaringan diganti Cassy. Identifier teknis (JWT iss/aud, salt OTP, kunci dev secretbox, nama service Windows, email seed) sengaja tetap.
- [x] FL1 W2 gerbang owner POS (client #141). W3 hapus akun + pipeline AAB (backend #183, client #143/#144). W4 harga tahunan 10x, Starter KDS, batas paket, situs pemasaran (backend #182, client #142).
- [ ] FL1 W5 micro-interaction (token motion + checklist komponen).
- [ ] Director: D-U-N-S + akun Play organisasi; konfirmasi upload key di secret repo; masa retensi pajak; aset listing.
- [ ] Data Safety: `network_security_config` prod masih mengizinkan HTTP ke `192.168.10.10`/`localhost` (server lokal LAN); putuskan sebelum menjawab "enkripsi saat transit: ya".
- [ ] Mode baca-saja trial belum memblokir order QR tamu dan API perangkat; `/legal` dirender di klien (SEO lemah); gambar OG 1200x630 belum ada; tombol "Coba 14 hari" masih ke email (belum ada sign-up mandiri).
