# Design: Hub Impor/Ekspor + Bon Dashboard + Notifikasi Piutang

**Tanggal:** 2026-06-05  
**Status:** Approved

---

## Ringkasan

Tiga fitur digabung dalam satu spec karena semuanya berputar di data piutang/pelanggan:

1. **Hub Impor/Ekspor** — halaman `/impor` jadi hub bertab lengkap: import + export produk, piutang, hutang, dan penjualan (CSV + Excel)
2. **Bon di Dashboard Kasir** — kasir dashboard bisa jual kredit (bon), otomatis buat Piutang + daftarkan Pelanggan
3. **Notifikasi Piutang** — dashboard banner + pesan Telegram harian ke owner jika ada piutang terbuka

---

## 1. Hub Impor/Ekspor

### Struktur Tab

| Tab | Role | Impor | Ekspor |
|-----|------|-------|--------|
| Produk & Stok | semua | produk (owner) / stok opname (kasir) — sudah ada | daftar produk (owner) — baru |
| Piutang | owner | bulk CSV/Excel — baru | daftar piutang — baru |
| Hutang | owner | bulk CSV/Excel — baru | daftar hutang — baru |
| Penjualan | owner | — | riwayat transaksi — baru |

Kasir hanya melihat tab "Produk & Stok" — tidak ada perubahan perilaku untuk role kasir.

### Library Export

Tambah `xlsx` (SheetJS) sebagai dependency baru. Digunakan sisi client setelah data di-fetch via Server Action.

Utility baru `src/lib/export-table.ts`:
```ts
export function exportCsv(filename: string, headers: string[], rows: string[][]): void
export async function exportXlsx(filename: string, sheetName: string, headers: string[], rows: string[][]): Promise<void>
```

### Export — Data & Kolom

**Produk** (owner only): `id, barcode, nama, kategori, satuan, harga_beli, harga_jual, stok, stok_minimum, stok_kritis, supplier, jenis, saldo_modal, stok_ml`

**Piutang**: `id, tanggal, pelanggan_id, phone, pokok, dibayar, sisa, status, kasir, catatan`

**Hutang**: `id, tanggal, supplier_id, pokok, dibayar, sisa, status, jatuh_tempo, catatan`

**Penjualan/Transaksi**: `id, tanggal, jam, nama_produk, kategori, qty, harga_satuan, total, metode_bayar, kasir, catatan, modal`  
Filter tanggal: dropdown "Hari Ini / 7 Hari / 30 Hari / Semua" (default: 30 hari)

### Import Piutang

Kolom yang dikenali: `phone` (atau `no_hp`, `wa`), `nama`, `pokok`, `dibayar`, `catatan`, `tanggal`

Logika:
1. `NormalizePhone(phone)` — jika tidak valid, lewati baris + catat error
2. `upsertPelanggan(nama, phone)` — buat/update Pelanggan
3. Hitung `sisa = pokok − dibayar`; `status = "lunas"` jika sisa ≤ 0, else `"terbuka"`
4. Generate `id = PIU-XXXX` via `incr(kios:seq:piu)`
5. Simpan Piutang; update `Pelanggan.total_utang += sisa`

Server Action: `importPiutangAction(rows)` di `src/app/(app)/impor/actions.ts`  
Revalidate: `/pelanggan`, `/dashboard`

### Import Hutang

Kolom yang dikenali: `supplier_id` (atau `supplier`, `nama_supplier`), `pokok`, `dibayar`, `catatan`, `tanggal`, `jatuh_tempo`

Logika:
1. Cari Supplier: cocokkan via `supplier_id`, lalu via nama (case-insensitive)
2. Jika tidak ditemukan, lewati baris + catat error
3. Hitung `sisa = pokok − dibayar`; `status = "lunas"` jika sisa ≤ 0, else `"terbuka"`
4. Generate `id = HUT-XXXX` via `incr(kios:seq:hut)`
5. Simpan Hutang

Server Action: `importHutangAction(rows)` di `src/app/(app)/impor/actions.ts`  
Revalidate: `/dashboard`

### File yang Diubah/Dibuat

Baru:
- `src/lib/export-table.ts` — helper exportCsv + exportXlsx

Diubah:
- `kios-dashboard/package.json` — tambah `xlsx`
- `src/app/(app)/impor/actions.ts` — tambah `importPiutangAction`, `importHutangAction`, `exportProdukAction`, `exportPiutangAction`, `exportHutangAction`, `exportTransaksiAction`
- `src/components/impor/impor-view.tsx` — refactor jadi hub bertab

---

## 2. Bon di Dashboard Kasir

### Kasir Form (`kasir-form.tsx`)

Dropdown metode bayar ditambah opsi **"Bon (Kredit)"**.

Saat "bon" dipilih:
- Muncul field **No HP Pelanggan** (wajib) dengan hint "Format: 08xxx atau 628xxx"
- Field "Bayar" dan tampilan kembalian disembunyikan
- Tombol checkout berubah label jadi "Catat Bon"
- Validasi client-side: `NormalizePhone` — tolak submit jika HP tidak valid

### `recordSale` (`src/lib/sales.ts`)

Tambah parameter opsional: `pelangganPhone?: string`

Validasi awal: jika `metode === "bon"` dan `pelangganPhone` kosong/invalid → return error.

Setelah semua transaksi dicatat:
- `upsertPelanggan` (function baru di `src/lib/kios.ts` — mirror dari Go)
- `incr(kios:seq:piu)` → generate `PIU-XXXX`
- Simpan `Piutang` via `setPiutang`
- Update `Pelanggan.total_utang += total`, `total_belanja += total`
- Return tambahan field `piutang_id` dan `pelanggan` di `SaleResult`

### `upsertPelanggan` di TypeScript (`src/lib/kios.ts`)

Mirror dari `store_pelanggan.go`:
```ts
export async function upsertPelanggan(nama: string, rawPhone: string): Promise<Pelanggan>
```
- `NormalizePhone`: strip `+`, konversi `08x` → `628x`
- Key: `PLG-<phone>`
- Jika belum ada: buat baru; jika sudah: update nama jika lebih panjang

### `nextPiutangId` di TypeScript (`src/lib/kios.ts`)

```ts
export async function nextPiutangId(): Promise<string>
// incr(kios:seq:piu) → "PIU-0001"
```

### Struk Bon

Setelah checkout bon, struk menampilkan:
```
Bon kredit dicatat ✓
Pelanggan: 08123456789
Piutang: PIU-0042 · Rp 150.000 (belum dibayar)
```

### RBAC

Bon tersedia untuk **kasir dan owner** — konsisten dengan bot Telegram.

### File yang Diubah

- `src/lib/sales.ts` — tambah parameter `pelangganPhone`, logika bon
- `src/lib/kios.ts` — tambah `upsertPelanggan`, `nextPiutangId`, `setPiutang` (jika belum ada export)
- `src/components/kasir/kasir-form.tsx` — field HP + label bon + struk bon

---

## 3. Notifikasi Piutang

### Dashboard — Halaman `/pelanggan`

Card ringkasan kondisional di atas tabel, ditampilkan hanya jika ada piutang `status = "terbuka"`:

```
⚠️  3 pelanggan belum bayar · Total Rp 450.000
```

Data diambil sisi server (Server Component): `getAllPiutang()` difilter `status === "terbuka"`, agregat per `pelanggan_id`.

### Dashboard — Halaman `/dashboard`

KPI card "Piutang Terbuka" kondisional (hanya jika ada):
- Nilai: total Rp piutang terbuka
- Sub-label: "X pelanggan"
- Klik → navigasi ke `/pelanggan`

Data dihitung di `dashboard/page.tsx` bersamaan dengan KPI lain.

### Telegram — Go (`pkg/tools/kios/notif.go`)

Field baru di `KiosConfig` (Go + TypeScript):
```go
// Go
NotifPiutangEnabled bool `json:"notif_piutang_enabled"`
```
```ts
// TypeScript
notif_piutang_enabled?: boolean; // default true
```

Default: `true` (on). Toggle via form `/pengaturan`.

Pengiriman menumpang loop notif yang sudah ada (`tryNotify`), berjalan bersamaan dengan notif stok di jam `notif_jam`. Gating identik: hanya kirim sekali per hari (via key `kios:notif:piutang_last_date`).

Format pesan Telegram:
```
📋 Piutang terbuka: 3 pelanggan

• 08123456789 — Rp 50.000 (sejak 2026-06-01)
• 08987654321 — Rp 120.000 (sejak 2026-05-28)
• 08555000111 — Rp 280.000 (sejak 2026-05-20)

Total: Rp 450.000
```

Hanya tampilkan maks 10 entri; jika lebih → "...dan X lainnya."

### File yang Diubah

Go:
- `pkg/tools/kios/store.go` — tambah `NotifPiutangEnabled` ke `KiosConfig`
- `pkg/tools/kios/notif.go` — tambah `tryNotifyPiutang`, panggil di loop

Dashboard:
- `src/lib/types.ts` — tambah `notif_piutang_enabled` ke `KiosConfig`
- `src/lib/kios.ts` — update default config
- `src/app/(app)/pengaturan/actions.ts` — simpan field baru
- `src/components/pengaturan/pengaturan-form.tsx` — toggle UI baru
- `src/app/(app)/pelanggan/page.tsx` — fetch piutang, hitung ringkasan
- `src/components/pelanggan/pelanggan-list.tsx` (atau komponen baru) — card ringkasan
- `src/app/(app)/dashboard/page.tsx` — KPI piutang terbuka

---

## 4. Batasan & Keputusan

- Import piutang/hutang bersifat **additive** — tidak menimpa record yang sudah ada (match by ID tidak dilakukan, selalu buat baru)
- Dashboard bon hanya membuat **satu Piutang per checkout** (semua item dalam satu sesi = satu piutang, konsisten dengan total transaksi)
- Export transaksi dibatasi default 30 hari untuk mencegah payload besar
- `NormalizePhone` di TypeScript harus konsisten 100% dengan Go agar `pelanggan_id` tidak split
- Notif Telegram piutang tidak dikirim jika `notif_enabled = false` (mengikuti flag utama)

---

## 5. Ringkasan File

### Baru
- `kios-dashboard/src/lib/export-table.ts`

### Diubah — Dashboard
- `package.json` (kios-dashboard) — tambah `xlsx`
- `src/lib/kios.ts` — `upsertPelanggan`, `nextPiutangId`
- `src/lib/sales.ts` — parameter bon + logika piutang
- `src/lib/types.ts` — `notif_piutang_enabled`
- `src/app/(app)/impor/actions.ts` — import piutang/hutang + export actions
- `src/components/impor/impor-view.tsx` — hub bertab
- `src/components/kasir/kasir-form.tsx` — field HP bon + struk bon
- `src/app/(app)/pelanggan/page.tsx` — fetch piutang untuk banner
- `src/components/pelanggan/pelanggan-list.tsx` — card ringkasan piutang
- `src/app/(app)/dashboard/page.tsx` — KPI piutang terbuka
- `src/app/(app)/pengaturan/actions.ts` — simpan `notif_piutang_enabled`
- `src/components/pengaturan/pengaturan-form.tsx` — toggle notif piutang

### Diubah — Go
- `pkg/tools/kios/store.go` — `NotifPiutangEnabled` di `KiosConfig`
- `pkg/tools/kios/notif.go` — `tryNotifyPiutang`
