# Roadmap Kios Cerdas

> Fitur-fitur berikutnya untuk bot Telegram + dashboard kios desa Rote Ndao.
> Diurutkan dari yang paling dirasakan manfaatnya oleh pengguna.

---

## Status Saat Ini (v0.2.x)

**Sudah jalan:**
- Bot Telegram: kasir, stok, laporan, harga, supplier, promo, belajar, pasar, pustaka, backup/restore, import Excel
- Dashboard: kasir, produk, pesanan online, supplier (banding harga), laporan, pengguna, pengaturan
- Storefront publik `/toko` dengan pembayaran QRIS + konfirmasi WhatsApp
- Notifikasi stok menipis + laporan harian otomatis + alert pesanan menumpuk
- RBAC owner/kasir

---

## Milestone 1 — Kelengkapan Operasional Harian

> **Target:** Semua kebutuhan operasional kios sehari-hari bisa dilakukan tanpa keluar dari sistem.

### F1. Hutang Pelanggan (Kredit Kios)

Realitas kios desa: banyak pembeli ambil barang dulu, bayar belakangan ("bon/utang"). Saat ini tidak ada pencatatan.

**Bot (Go — `pkg/tools/kios/`):**
- Tool baru `kios_hutang`
- Actions: `catat` (tambah hutang baru), `bayar` (catat pembayaran), `daftar` (semua hutang aktif), `detail` (per pelanggan), `lunas` (tandai lunas)
- Redis key: `kios:hutang` (HASH) + `kios:seq:hut` (counter)
- Struct: `Hutang {id, pelanggan_nama, pelanggan_kontak, items[], total, terbayar, sisa, tanggal, kasir, catatan, status}`

**Slash command:** `/hutang [nama]` — lihat hutang aktif

**Dashboard:** Halaman `/hutang` — tabel hutang + form bayar + total piutang

---

### F2. Multi-Item per Transaksi (Keranjang)

Saat ini setiap `/jual` hanya 1 produk. Kasir sering jual 3–5 item sekaligus dan harus panggil beberapa kali.

**Bot:** `kios_kasir` action `jual_keranjang` — terima array items `[{produk, qty}, ...]`, proses atomik, 1 struk untuk semua item. Kode TRX-xxxx per transaksi (bukan per item).

**Dashboard kasir:** Tambah tombol "+ Item" di form kasir, hitung total live, 1 klik submit.

---

### F3. Stok Opname (Cek Fisik Stok)

Rekonsiliasi stok sistem vs fisik — wajib untuk kios yang peduli akurasi.

**Bot:**
- `kios_stok` action `opname_mulai` → export daftar stok ke pesan, user reply dengan koreksi
- `kios_stok` action `opname_simpan` `{produk, stok_fisik}` → catat selisih, update stok, log ke `kios:opname`

**Dashboard:** Halaman `/opname` — tabel stok sistem vs input fisik, highlight selisih, tombol konfirmasi

---

### F4. Halaman Promo & Pasar di Dashboard

Dua tool penting (`kios_promo`, `kios_pasar`) sudah ada di bot tapi belum ada UI di dashboard.

**Dashboard:**
- `/promo` — daftar promo aktif + form buat promo (persen/nominal, produk, tanggal berlaku)
- `/pasar` — tabel harga pasar per produk, grafik tren harga kios vs pasar, form input harga pasar baru

Nav item sudah ada ruang — tinggal tambah `{ href: "/promo", label: "Promo", icon: Tag }` dan `{ href: "/pasar", label: "Harga Pasar", icon: TrendingUp }`.

---

### F5. Ekspor Laporan ke Excel

Owner butuh laporan bulanan dalam format Excel untuk catatan/audit manual.

**Dashboard:** Tombol "Ekspor Excel" di halaman `/laporan` dan `/penjualan`:
- Gunakan library `xlsx` atau `SheetJS` di sisi client
- Export: transaksi (filter periode), produk (stok snapshot), laporan laba

**Bot:** `/laporan ekspor` — bot kirim file `.xlsx` ke chat langsung

---

## Milestone 2 — Pengalaman Pembeli & Pertumbuhan

> **Target:** Pembeli bisa berinteraksi lebih mandiri, mengurangi beban kasir.

### F6. Pelanggan Tetap & Riwayat Pembelian

Catat data pelanggan yang sering beli — berguna untuk hutang, promo, dan analitik.

**Bot:**
- Tool baru `kios_pelanggan` — CRUD data pelanggan `{id, nama, kontak, alamat, total_pembelian, terakhir_beli}`
- Saat jual: opsional link ke pelanggan (`kasir`, `pelanggan_id`)
- Action `riwayat` — 10 transaksi terakhir pelanggan ini

**Dashboard:** Halaman `/pelanggan` — tabel pelanggan, total belanja, riwayat transaksi

---

### F7. Produk Barcode / QR Scan

Input produk lewat scan barcode jauh lebih cepat daripada ketik nama. HP Android sudah punya kamera.

**Bot:** Saat user kirim foto barcode/QR → OCR / ZXing decode → cari produk di `kios:produk` by field `barcode`. Field `barcode` sudah ada di struct `Produk`.

**Dashboard kasir:** Tombol kamera → Web QR scanner (library `@zxing/browser`) → auto-fill field produk

**Syarat:** Isi field `barcode` saat tambah produk; tambah action `set_barcode` di `kios_stok`.

---

### F8. Flash Sale & Promo Terjadwal

Promo dengan waktu terbatas — cocok untuk menggerakkan stok mendekati kedaluwarsa.

**Bot/Go:** Extend `kios_promo`:
- Tambah field `mulai_jam` + `selesai_jam` di struct promo
- Background goroutine cek setiap 15 menit: aktifkan/nonaktifkan promo sesuai jadwal
- Notif ke owner: "🔥 Flash sale [nama promo] dimulai!"

---

### F9. Notifikasi Kedaluwarsa Produk

Produk dengan `has_exp=true` yang mendekati exp date harus diingatkan sebelum jadi kerugian.

**Bot — `notif.go`:** Tambah `tryNotifyExpiring()` di loop notif:
- Cek produk `has_exp=true` dimana `exp_date` dalam 7 hari
- Kirim alert ke owner: "⚠️ [Nama produk] kedaluwarsa [tanggal] — sisa stok: X"
- Konfigurasi via `config_set notif_exp_hari 7` (default 7 hari)

---

## Milestone 3 — Stabilitas & Skala

> **Target:** Siap dipakai lebih luas, tahan banting, mudah di-maintain.

### F10. Integration Test E2E

Gap terbesar saat ini (dari REVIEW.md): `integration/suites/` kosong.

**Prioritas test:**
1. Skenario jual → cek stok berkurang → laporan terupdate (pakai miniredis)
2. RBAC: kasir coba aksi owner → ditolak
3. Backup → restore → verifikasi data identik
4. Import Excel → produk tersimpan → stok benar

**Framework:** Go `testing` + `miniredis` (sudah ada di `go.mod`). Buat `pkg/tools/kios/integration_test.go`.

---

### F11. Redis Pipeline Batching

Beberapa operasi (list stok, laporan bulanan) melakukan banyak HGET/LRANGE terpisah. Pada Upstash (TCP via TLS), latensi per-call bisa 30–100ms.

**Refaktor `store.go`:** Gunakan `redis.Pipeline()` / `MGET` untuk:
- `GetAllProduk()` — 1 HGETALL vs N HGET
- `GetTransaksiByPeriode()` — cursor LRANGE batch

---

### F12. Laporan Keuangan Sederhana (Neraca Bulanan)

Dari data yang sudah ada, bisa hitung ringkasan keuangan bulanan: omzet, modal, laba bersih, hutang tertagih, hutang outstanding.

**Bot:** `kios_laporan` action `keuangan` `{bulan}` → ringkasan 1 halaman format tabel

**Dashboard:** Widget baru di `/laporan` — kartu neraca bulanan dengan grafik tren 6 bulan

---

### F13. Multi-Kios (Opsional, Jangka Panjang)

Untuk owner yang punya lebih dari 1 kios/cabang.

**Konsep:** Namespace Redis per kios (`kios:A:produk`, `kios:B:produk`). User terikat ke kios tertentu. Bot bisa switch kios aktif: `/kios ganti B`.

**Scope:** Desain ulang `Store` struct dengan field `namespace`. Backward-compatible: kios tunggal = namespace kosong.

---

## Urutan Pengerjaan yang Disarankan

```
Milestone 1 (operasional)
  F4 Promo & Pasar dashboard    ← paling cepat (UI only, tools sudah ada)
  F5 Ekspor Excel               ← high value, sering diminta owner
  F2 Multi-item per transaksi   ← langsung dirasakan kasir
  F1 Hutang pelanggan           ← kebutuhan khas kios desa
  F3 Stok opname                ← penting untuk akurasi

Milestone 2 (pertumbuhan)
  F9 Notif kedaluwarsa          ← cepat (extend notif.go)
  F7 Barcode scan               ← UX besar, field sudah ada
  F6 Pelanggan tetap            ← data berharga jangka panjang
  F8 Flash sale terjadwal       ← extend promo yang sudah ada

Milestone 3 (stabilitas)
  F10 Integration test E2E      ← harus sebelum user lebih banyak
  F11 Redis pipeline batching   ← saat mulai terasa lambat
  F12 Laporan keuangan          ← value tinggi, data sudah tersedia
  F13 Multi-kios                ← hanya jika ada kebutuhan nyata
```

---

## Checklist Teknis Sebelum "Production-Ready"

- [ ] Isi `integration/suites/` dengan minimal 4 skenario E2E (F10)
- [ ] Verifikasi healthcheck `/health` selalu hijau di Railway setelah redeploy
- [ ] Test failover: Groq down → fallback ke Gemini berjalan otomatis
- [ ] Load test: 5 user Telegram kirim pesan bersamaan tidak deadlock
- [ ] Dokumentasi `KIOS_OWNER_IDS` di Railway env var sebelum serahkan ke pengguna
- [ ] Backup otomatis aktif dan file `.json` terkirim ke chat owner setiap malam
