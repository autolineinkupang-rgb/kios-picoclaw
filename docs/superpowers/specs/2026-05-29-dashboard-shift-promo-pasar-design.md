# Dashboard: Shift, Promo, Harga Pasar

**Tanggal:** 2026-05-29
**Status:** Disetujui

## Latar Belakang

Bot Telegram sudah mendukung `/shift`, `/promo`, dan `/pasar` tetapi belum ada UI di dashboard web untuk mengelolanya. Owner dan kasir harus pakai perintah teks, yang kurang intuitif untuk data entry harian.

## Lingkup

Tiga fitur baru, terintegrasi ke halaman yang sudah ada:

1. **Shift Management** — tab baru di `/kasir`
2. **Promo Management** — halaman baru `/promo`
3. **Harga Pasar** — kolom baru di tabel produk `/produk`

---

## 1. Shift Management

### Penempatan
Tab **"Shift"** di halaman `/kasir`, sejajar dengan tab kasir yang ada.

### Akses
Kasir dan owner.

### UI

**Saat ada shift buka:**
- Info: nama kasir, waktu buka, saldo awal
- Tombol **Tutup Shift** → form input saldo akhir → submit

**Saat tidak ada shift aktif:**
- Tombol **Buka Shift** → form input nama kasir + saldo awal → submit

**Tabel riwayat shift** di bawah (10 terakhir):
- Kolom: Kasir, Waktu Buka, Waktu Tutup, Saldo Awal, Saldo Akhir, Status

### Data

- **Shift aktif:** `kios:shift` (Redis String, existing) — GET/SET
- **Riwayat shift:** `kios:shift:history` (Redis List, **baru**) — RPUSH saat tutup shift, LRANGE -10 untuk tampil

Saat shift ditutup via dashboard: update `kios:shift` (status = "tutup"), push copy ke `kios:shift:history`, lalu hapus/kosongkan `kios:shift`.

**Keterbatasan:** Shift yang dibuka/ditutup via Telegram tidak masuk `kios:shift:history` dashboard.

### File Baru
- `components/kasir/shift-tab.tsx` — komponen tab shift
- Tambah action `bukaShiftAction`, `tutupShiftAction` ke `app/(app)/kasir/actions.ts`
- Tambah `setShift`, `getShiftHistory`, `pushShiftHistory` ke `lib/kios.ts`
- Tambah `shiftHistory: "kios:shift:history"` ke `lib/redis.ts`

---

## 2. Promo Management

### Penempatan
Halaman baru `/promo` di sidebar (kasir + owner).

### Akses
- **Kasir:** buat promo baru (status default nonaktif), lihat semua promo
- **Owner:** aktifkan/nonaktifkan promo, hapus promo, buat promo

### UI

**Tabel promo** dengan kolom:
- Produk, Tipe (Persen/Fixed), Nilai, Min Qty, Periode, Status, Aksi

**Badge status:**
- `Aktif` (hijau) — `aktif: true` dan dalam periode
- `Menunggu` (kuning) — `aktif: false`, dibuat kasir, belum diaktifkan owner
- `Nonaktif` (abu) — `aktif: false`, dinonaktifkan manual
- `Kedaluwarsa` (merah) — `aktif: true` tapi tanggal `selesai` sudah lewat

**Form buat promo** (modal atau form di atas tabel):
- Pilih produk (dropdown dari daftar produk)
- Tipe: Persen / Nominal
- Nilai diskon
- Min Qty (opsional, default 1)
- Tanggal mulai & selesai
- Catatan

Kasir submit → `aktif: false`. Owner submit → `aktif: true`.

**Aksi per baris:**
- Kasir: tidak ada aksi (hanya lihat)
- Owner: toggle Aktif/Nonaktif, tombol Hapus (konfirmasi)

### Data

- `kios:promo` (Redis Hash, existing) — field = promo ID, value = JSON Promo
- `kios:seq:promo` (Redis String, existing) — INCR untuk generate ID `PROMO-NNNN`

### Tipe Baru

```typescript
// lib/types.ts
export interface Promo {
  id: string;          // PROMO-NNNN
  produk: string;      // nama produk
  produk_id: string;   // ID produk
  tipe: "persen" | "fixed";
  nilai: number;
  min_qty: number;
  aktif: boolean;
  mulai: string;       // YYYY-MM-DD
  selesai: string;     // YYYY-MM-DD
  catatan: string;
}
```

### File Baru
- `app/(app)/promo/page.tsx`
- `app/(app)/promo/actions.ts` — `createPromoAction`, `togglePromoAction`, `deletePromoAction`
- `components/promo/promo-table.tsx`
- `components/promo/promo-form.tsx`
- Tambah `getAllPromo`, `setPromo`, `deletePromo`, `nextPromoId` ke `lib/kios.ts`
- Tambah `promo: "kios:promo"`, `seqPromo: "kios:seq:promo"` ke `lib/redis.ts`
- Tambah entry ke `components/nav-items.tsx`

---

## 3. Harga Pasar di Tabel Produk

### Penempatan
Dua kolom tambahan di tabel produk yang sudah ada di `/produk`.

### Akses
Semua pengguna (read-only). Data dikelola via `/suplier`.

### UI

Kolom baru di tabel produk (setelah kolom harga jual):
- **Pasar Min** — harga terendah dari semua supplier produk tersebut
- **Pasar Max** — harga tertinggi dari semua supplier produk tersebut
- Tampilkan "—" jika belum ada supplier yang punya harga untuk produk ini
- Tooltip saat hover: "N supplier" (jumlah supplier yang punya data harga)

### Data

Dihitung dari `kios:harga_supplier` (Redis Hash, existing):
- Field format: `produkID|supplierName`
- Value: harga (integer)
- Min/Max dihitung di sisi server saat fetch data produk

### Perubahan File
- `components/produk/produk-table.tsx` — tambah 2 kolom + kalkulasi min/max
- `app/(app)/produk/page.tsx` — fetch `hargaSupplier` bersamaan dengan produk
- `lib/kios.ts` — fungsi helper `getHargaRangeByProduk(produkId)` atau kalkulasi inline

---

## Ringkasan Perubahan File

| File | Perubahan |
|------|-----------|
| `lib/types.ts` | Tambah `Promo` interface |
| `lib/redis.ts` | Tambah `promo`, `seqPromo`, `shiftHistory` keys |
| `lib/kios.ts` | Tambah fungsi shift, promo, helper harga pasar |
| `components/kasir/shift-tab.tsx` | **Baru** |
| `app/(app)/kasir/actions.ts` | Tambah shift actions |
| `app/(app)/kasir/page.tsx` | Tambah tab Shift |
| `components/promo/promo-table.tsx` | **Baru** |
| `components/promo/promo-form.tsx` | **Baru** |
| `app/(app)/promo/page.tsx` | **Baru** |
| `app/(app)/promo/actions.ts` | **Baru** |
| `components/nav-items.tsx` | Tambah `/promo` |
| `components/produk/produk-table.tsx` | Tambah 2 kolom harga pasar |
| `app/(app)/produk/page.tsx` | Fetch harga supplier |

---

## Yang Tidak Diubah

- Go backend tidak berubah — dashboard langsung baca/tulis Redis key yang sama
- Halaman `/suplier` tidak berubah (harga supplier tetap dikelola di sana)
- Autentikasi dan middleware tidak berubah
