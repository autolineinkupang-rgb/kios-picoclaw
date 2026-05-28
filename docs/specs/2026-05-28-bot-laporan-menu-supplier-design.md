# Spec: Laporan Bot Otomatis/On-demand/Alert + Menu Supplier Dashboard

- **Tanggal:** 2026-05-28
- **Status:** Disetujui (siap tahap implementation plan)
- **Penulis:** Claude Code (sesi brainstorming superpowers)

## Konteks

Permintaan terdiri dari dua tugas:

1. **Tugas 1** — bot PicoClaw (server Railway) dapat mengakses data dashboard
   untuk memantau & membaca informasi, lalu mengirim laporan ke Telegram.
2. **Tugas 2** — owner dan kasir mendapat menu tambahan di dashboard untuk
   mengedit supplier, termasuk perbandingan harga produk antar-supplier.

### Temuan kunci dari analisa kode (mengubah lingkup)

Eksplorasi (5 agen + pembacaan terarah) menemukan bahwa sebagian besar Tugas 1
**sudah ada**, dan inti Tugas 2 **sudah ada di sisi bot tetapi belum di dashboard**:

- Bot Go dan dashboard Next.js memakai **database Upstash yang sama**
  (`kios-dashboard/src/lib/redis.ts:3-5` menyatakan eksplisit). Bot Go memakai
  `go-redis` via `UPSTASH_REDIS_URL` (`pkg/tools/kios/store.go:245`); dashboard
  memakai `@upstash/redis` REST via `UPSTASH_REDIS_REST_URL`/`_TOKEN`.
- Bot **sudah** membaca key `kios:*` langsung (`GetAllProduk`, `GetAllTransaksi`,
  `GetAllPembelian`, `GetAllPesanan`, dst). Karena itu **login kode verifikasi
  Telegram untuk bot tidak diperlukan** — akses baca sudah dimiliki via Redis
  bersama. `/api/summary` (HMAC, `pkg/tools/kios/dashboard_summary.go`) dipakai
  hanya untuk ringkasan **terhitung** (omzet, laba, terlaris, jam ramai, stok).
- Laporan **terjadwal** sudah ada: cron `kios-laporan-harian`
  (`pkg/tools/kios/report.go`), aktif bila env `KIOS_REPORT_CHAT` diisi
  (default `0 18 * * *`, TZ `Asia/Makassar`).
- Laporan **on-demand** sudah ada via tool `kios_laporan` (`laporan.go`).
- **Alert** sudah sebagian: `notif.go` (`NotifService`, tick 2 menit) mengirim
  notif **stok menipis** (harian, gated `notif_jam`) dan **pesanan baru**
  (real-time) ke owner. Belum ada alert **stok kritis/habis** dan **pesanan
  pending menumpuk**.
- Supplier **sudah jadi entitas di bot**: key `kios:supplier`
  (`pkg/tools/kios/store.go:140`), struct `Supplier` (`store_more.go:12`),
  tool `kios_supplier` (`supplier.go`) dengan aksi tambah/edit/daftar/cari/hapus
  dan **banding_harga** (dari `kios:pembelian`). Dashboard **belum** punya
  entitas supplier — `supplier` di dashboard hanya field teks pada produk.

## Tujuan

- Memastikan & menyempurnakan alur laporan/alert bot ke Telegram (Tugas 1).
- Menghadirkan supplier sebagai menu CRUD di dashboard untuk owner+kasir,
  dengan perbandingan harga hybrid (Tugas 2), tersinkron dua arah dengan bot
  lewat Redis bersama.

## Non-tujuan (YAGNI)

- Tidak membuat mekanisme login kode verifikasi untuk bot (tidak diperlukan).
- Tidak menambah autentikasi/role baru di luar owner/kasir yang sudah ada.
- Tidak mengubah skema produk/transaksi/pesanan yang sudah ada.

## Arsitektur (prinsip "dua klien, satu data")

Dashboard (Next.js) dan bot (Go) adalah dua klien atas satu model data Redis
Upstash. Implikasi yang mengikat seluruh desain:

- Setiap field baru pada entitas bersama (mis. `pic` pada Supplier) harus
  ditambahkan di **kedua** sisi (struct Go + tipe TS) agar bermakna.
- ID entitas harus dibuat dengan skema yang sama di kedua sisi untuk mencegah
  tabrakan/duplikasi (`kios:seq:sup` INCR + format `SUP-%03d`).
- Fitur turunan (perbandingan harga) harus konsisten: sumber & override yang
  sama dipakai bot maupun dashboard.

---

## Tugas 1 — Laporan & Alert (lingkup: aktifkan + sempurnakan)

### 1a. Aktivasi (operasional, env Railway)

Set/verifikasi env pada service `picoclaw gateway`:

- `KIOS_REPORT_CHAT` — chat Telegram tujuan laporan harian (mengaktifkan cron).
- `KIOS_REPORT_CRON` — opsional, override jadwal (default `0 18 * * *`).
- `KIOS_DASHBOARD_URL` — base URL dashboard untuk `/api/summary`.
- `KIOS_SERVICE_SECRET` — HMAC, **harus sama** dengan `KIOS_SERVICE_SECRET`
  di dashboard.
- Config kios: `notif_enabled = true`, `notif_jam` sesuai keinginan.

### 1b. Penyempurnaan kode

1. **Laporan harian kaya.** Upgrade `DailyReportText` (`report.go:16`) agar
   menyusun laporan dari ringkasan `/api/summary` (omzet, laba, transaksi,
   produk terlaris, jam ramai, status stok) memakai jalur HMAC yang sudah ada
   di `dashboard_summary.go`. Fallback ke `laporan ringkas` Go bila `/api/summary`
   tidak terjangkau (dashboard mati) agar laporan tetap terkirim.
2. **Alert stok kritis/habis.** Tambah di `notif.go`: selain `Stok <= StokMinimum`,
   beri peringatan terpisah untuk `Stok <= StokKritis` dan `Stok == 0` (habis),
   dengan penanda berbeda. Gating harian tetap dipakai agar tidak spam.
3. **Alert pesanan pending menumpuk.** Tambah pemeriksaan: bila jumlah pesanan
   ber-status `pending` melewati ambang (default mis. `KIOS_PENDING_ALERT_THRESHOLD`,
   fallback 5), kirim peringatan ke owner. Gunakan key state agar tidak mengirim
   berulang untuk kondisi yang sama (mirip `keyNotifPesananLast`).

### 1c. Error handling

- Kegagalan `/api/summary` (timeout/non-200) → log warn + fallback laporan Go.
- Kegagalan kirim Telegram → log warn per penerima, lanjut penerima lain
  (pola `sendToOwners` yang sudah ada dipertahankan).

### 1d. Testing

- Unit test deterministik builder pesan: low-stock, kritis/habis, pesanan
  menumpuk (ambang).
- Test `DailyReportText` dengan summary tiruan (sukses) dan dengan `/api/summary`
  gagal (fallback).

---

## Tugas 2 — Menu Supplier Dashboard (build utama)

### 2a. Perubahan model bersama (Go + TS)

Tambah field `pic` (PIC/nama sales) ke entitas Supplier di **kedua** sisi:

- Go `Supplier` (`pkg/tools/kios/store_more.go:12`): tambah
  `Pic string \`json:"pic"\``.
- Go `SupplierTool` (`supplier.go`): tambah parameter `pic` pada `tambah`/`edit`
  dan tampilkan pada `cari`/`daftar`.
- TS `Supplier` (dashboard `lib/types.ts`): `{ id, nama, kontak, alamat,
  produk_utama, pic, catatan }` — cocok byte-for-byte dengan JSON Go.

Kontrak data final (Redis hash `kios:supplier`, field = ID):

```json
{ "id": "SUP-001", "nama": "", "kontak": "", "alamat": "",
  "produk_utama": "", "pic": "", "catatan": "" }
```

### 2b. CRUD dashboard (salin pola `produk`)

Berkas baru/diubah di `kios-dashboard/`:

- `src/components/nav-items.tsx` — item menu `Supplier` (`href:"/suplier"`),
  **bukan** `ownerOnly` (kasir juga melihatnya).
- `src/app/(app)/suplier/page.tsx` — daftar supplier; `canManage` untuk
  owner+kasir.
- `src/app/(app)/suplier/actions.ts` — `create/update` digerbang `ensureStaff`
  (owner ATAU kasir); `delete` digerbang `ensureOwner`.
- `src/components/suplier/suplier-table.tsx` + `suplier-form.tsx` — list + modal
  tambah/edit (pola `produk-table`/`produk-form`).
- `src/lib/kios.ts` — `getAllSuplier/getSuplier/setSuplier/delSuplier/nextSuplierId`
  (hash `kios:supplier`, ID via INCR `kios:seq:sup` → `SUP-%03d`).
- `src/lib/redis.ts` — tambah `suplier: "kios:supplier"` dan `seqSup: "kios:seq:sup"`.
- `src/lib/types.ts` — tipe `Supplier` (lihat 2a).

### 2c. Role model

| Aksi | Owner | Kasir |
|---|---|---|
| Lihat daftar supplier | ✓ | ✓ |
| Tambah / edit supplier | ✓ | ✓ (`ensureStaff`) |
| Hapus supplier | ✓ | ✗ (`ensureOwner`) |

Penyelarasan bot: longgarkan `SupplierTool` (`supplier.go`) `tambah`/`edit`
dari `requireOwner` → izinkan owner+kasir (helper baru `requireStaff`/sejenis);
`hapus` tetap owner-only. Tujuan: aturan konsisten di bot & dashboard.

### 2d. Perbandingan harga (hybrid, key bersama)

- **Sumber otomatis:** dari `kios:pembelian` — per produk, harga_beli terendah
  per supplier (logika `bandingHarga` di `supplier.go:197` sebagai acuan).
- **Override manual:** key Redis **baru bersama**, mis. `kios:harga_supplier`
  (hash, field = `<produk_id>|<supplier_id>` → harga). Dipakai oleh:
  - Dashboard: view perbandingan menampilkan harga otomatis + override (override
    diutamakan bila ada), tandai termurah.
  - Bot: `bandingHarga` di `supplier.go` diperluas agar ikut memperhitungkan
    override dari `kios:harga_supplier` sehingga hasil bot = dashboard.
- UI dashboard: pada halaman supplier (atau sub-tab "Banding Harga"), pilih
  produk → tabel harga per supplier; owner+kasir dapat mengisi/override harga.

### 2e. Error handling

- Validasi input form (nama wajib; harga override numerik ≥ 0).
- Operasi Redis gagal → pesan ramah Bahasa Indonesia + tidak menutup modal.
- Gerbang role di server action (bukan hanya UI) untuk mencegah bypass.

### 2f. Testing

- Go: perluas `kios_test.go` untuk field `pic`, role kasir pada tambah/edit,
  dan `bandingHarga` dengan override manual.
- Dashboard: unit test util data-access supplier + util perbandingan (otomatis
  vs override). Verifikasi UI golden-path di browser sebelum dianggap selesai.

---

## Inventarisasi berkas

**Tugas 1 (Go):** `pkg/tools/kios/report.go`, `pkg/tools/kios/notif.go`,
`pkg/tools/kios/dashboard_summary.go` (reuse), test terkait. Env Railway.

**Tugas 2 (Go):** `pkg/tools/kios/store_more.go`, `pkg/tools/kios/supplier.go`,
`pkg/tools/kios/store.go` (key seq bila perlu), `pkg/tools/kios/kios_test.go`.

**Tugas 2 (dashboard):** `src/lib/types.ts`, `src/lib/redis.ts`, `src/lib/kios.ts`,
`src/components/nav-items.tsx`, `src/app/(app)/suplier/{page,actions}.tsx`,
`src/components/suplier/{suplier-table,suplier-form}.tsx`, gerbang `ensureStaff`.

## Risiko & catatan

- **Konsistensi schema lintas-bahasa:** JSON tag Go harus persis sama dengan
  tipe TS; uji baca-tulis silang (bot menulis, dashboard membaca, dan sebaliknya).
- **HMAC secret** harus identik di bot & dashboard, jika tidak `/api/summary` 401.
- **Ambang alert** sebaiknya dapat dikonfigurasi env agar tidak spam.

## Pertanyaan terbuka

Tidak ada — semua keputusan desain sudah dikonfirmasi pengguna.
