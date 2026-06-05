# Design: Menu Produk Sampingan

**Tanggal:** 2026-06-05
**Status:** Approved

---

## Ringkasan

Tambah menu **Produk Sampingan** di dashboard untuk mengelola pulsa, bensin, solar, dan minyak tanah secara terpisah dari produk biasa. Terintegrasi dengan kasir (satu transaksi gabungan, satu metode bayar) dan laporan (modal per jenis dipisah).

---

## 1. Data Model

### Go (`pkg/tools/kios/store.go`)

Field `Jenis` sudah ada. Tambah dua nilai valid baru:

| Nilai | Keterangan | Stok |
|-------|-----------|------|
| `""` / `"biasa"` | Produk biasa (default) | `stok` (unit) |
| `"pulsa"` | Pulsa / top-up | `stok` (unit) + `saldo_modal` (Rp) |
| `"bensin"` | Bensin | `stok` (unit) + `stok_ml` (ml) |
| `"solar"` | Solar / diesel | `stok` (unit) |
| `"minyak_tanah"` | Minyak tanah | `stok` (unit) |

Semua field tambahan (`SaldoModal`, `StokMl`, `StokKritisMl`) sudah ada di struct, tinggal dipakai.

### TypeScript (`kios-dashboard/src/lib/types.ts`)

Update komentar saja:
```ts
jenis?: string; // "" | "biasa" | "pulsa" | "bensin" | "solar" | "minyak_tanah"
```

---

## 2. Halaman Produk Biasa — Filter

File: `kios-dashboard/src/app/(app)/produk/page.tsx`

Saat load produk, filter hanya tampilkan:
```ts
produk.filter(p => !p.jenis || p.jenis === "biasa")
```

Produk sampingan tidak muncul di halaman `/produk`.

---

## 3. Halaman Produk Sampingan

### Navigasi (`src/components/nav-items.tsx`)

Tambah item baru setelah "Produk & Stok":
```ts
{ href: "/produk-sampingan", label: "Produk Sampingan", icon: Zap }
```
Visible untuk semua role (bukan `ownerOnly`).

### File baru

```
src/app/(app)/produk-sampingan/
  ├── page.tsx          — Server Component, load + filter produk sampingan
  └── actions.ts        — Server Actions: create / update / delete

src/components/produk-sampingan/
  ├── sampingan-table.tsx   — tabel dengan kolom Jenis + filter dropdown
  └── sampingan-form.tsx    — form kondisional per jenis
```

### `page.tsx`

```ts
const semua = await getAllProduk();
const sampingan = semua.filter(p =>
  ["pulsa", "bensin", "solar", "minyak_tanah"].includes(p.jenis ?? "")
);
```

### `SampinganTable`

- Filter dropdown: "Semua / Pulsa / Bensin / Solar / Minyak Tanah"
- Kolom tambahan: **Jenis** (badge berwarna)
  - Pulsa → badge biru
  - Bensin → badge oranye
  - Solar → badge kuning
  - Minyak Tanah → badge hijau
- Kolom Stok: untuk pulsa tampilkan "Saldo Rp X" di bawah stok unit

### `SampinganForm`

Field standar (sama dengan ProdukForm): nama, kategori, satuan, harga beli, harga jual, stok, stok minimum, stok kritis, supplier, barcode, gambar.

Field tambahan kondisional:

| Jenis | Field tambahan |
|-------|---------------|
| `pulsa` | `saldo_modal` (Rp) — saldo deposit dari agen |
| `bensin` | `stok_ml` (ml) + `stok_kritis_ml` (ml) — 1 liter = 1000 ml |
| `solar` | — (stok biasa, satuan bebas mis. jerigen) |
| `minyak_tanah` | — (stok biasa, satuan bebas) |

Dropdown `jenis` wajib diisi saat tambah produk baru.

### `actions.ts`

Sama dengan `produk/actions.ts` dengan tambahan:
- Validasi: `jenis` wajib diisi dan harus salah satu dari 4 nilai valid
- Simpan `saldo_modal` untuk pulsa
- Simpan `stok_ml` / `stok_kritis_ml` untuk bensin
- `revalidatePath("/produk-sampingan")` + `/dashboard`

---

## 4. Integrasi Kasir

### UI (`src/components/kasir/kasir-form.tsx`)

Kasir menerima **semua produk** (biasa + sampingan) — tidak ada filter di level page.

Perubahan UI:
- Di picker produk: tampilkan badge kecil jenis untuk item sampingan ("Pulsa", "Bensin", "Solar", "Minyak Tanah")
- Di cart: badge yang sama muncul di tiap item sampingan
- Total tetap gabungan semua item (tidak berubah)
- Satu metode bayar untuk seluruh transaksi (tidak berubah)

Struk hasil checkout — tambah catatan khusus jika ada item pulsa:
```
Saldo modal pulsa berkurang Rp X
```

### `SaleLine` (`src/lib/sales.ts`)

Tambah field:
```ts
export interface SaleLine {
  id: string;
  nama: string;
  qty: number;
  harga: number;
  subtotal: number;
  sisa: number;
  jenis?: string;          // untuk badge di UI
  catatan_sampingan?: string; // mis. "saldo modal -Rp 25.000"
}
```

### `recordSale` (`src/lib/sales.ts`)

Saat memproses tiap item:

1. Selalu isi field `modal` di `Transaksi`: `modal = harga_beli × qty`
2. Simpan jenis ke `catatan` dengan format `"via dashboard [<jenis>]"`:
   - Contoh: `"via dashboard [pulsa]"`, `"via dashboard [bensin]"`
   - Produk biasa: `"via dashboard"` (tanpa bracket) — backward compatible
3. Untuk `jenis === "pulsa"`: kurangi `saldo_modal` produk sebesar `harga_beli × qty`, set `catatan_sampingan` di SaleLine
4. Untuk `jenis === "bensin"`: set field `liter` di Transaksi = `qty` (1 unit = 1 liter)
5. Solar / minyak tanah: sama seperti produk biasa

---

## 5. Laporan — Modal Per Jenis

### `laporan-view.tsx`

Tambah tabel breakdown modal di bagian ringkasan finansial:

| Label | Filter transaksi |
|-------|-----------------|
| Modal Produk Biasa | `catatan` tidak mengandung bracket `[...]` |
| Modal Pulsa | `catatan` mengandung `[pulsa]` |
| Modal Bensin | `catatan` mengandung `[bensin]` |
| Modal Solar | `catatan` mengandung `[solar]` |
| Modal Minyak Tanah | `catatan` mengandung `[minyak_tanah]` |
| **Total Modal** | jumlah semua |

Laba kotor = Total Penjualan − Total Modal (tetap satu angka gabungan).

Parser helper di `src/lib/analytics.ts`:
```ts
function jenisFromCatatan(catatan: string): string {
  const m = catatan.match(/\[(\w+)\]/);
  return m ? m[1] : "biasa";
}
```

### Dashboard KPI (`src/app/(app)/dashboard/page.tsx`)

Tambah KPI card "Modal Sampingan" yang breakdown per jenis — hanya tampilkan jenis yang punya transaksi hari ini (tidak noise kalau belum ada).

---

## 6. Batasan & Keputusan

- Data lama (sebelum fitur ini) tidak punya `jenis` di `catatan` transaksi → dianggap `"biasa"` secara default — tidak break laporan lama
- Produk bensin yang dibuat via bot (sudah ada `stok_ml`) tetap bisa diedit via form sampingan baru
- Saldo modal pulsa diupdate langsung di field produk Redis saat checkout — konsisten dengan cara bot bekerja
- Tidak ada halaman baru untuk laporan sampingan — breakdown masuk ke laporan yang sudah ada

---

## 7. File yang Diubah / Dibuat

### Baru
- `src/app/(app)/produk-sampingan/page.tsx`
- `src/app/(app)/produk-sampingan/actions.ts`
- `src/components/produk-sampingan/sampingan-table.tsx`
- `src/components/produk-sampingan/sampingan-form.tsx`

### Diubah
- `src/components/nav-items.tsx` — tambah nav item
- `src/app/(app)/produk/page.tsx` — filter produk biasa saja
- `src/lib/sales.ts` — logika modal + jenis di catatan + SaleLine fields
- `src/components/kasir/kasir-form.tsx` — badge jenis + catatan pulsa di struk
- `src/components/laporan/laporan-view.tsx` — breakdown modal per jenis
- `src/app/(app)/dashboard/page.tsx` — KPI card modal sampingan
- `src/lib/analytics.ts` — helper `jenisFromCatatan`
- `src/lib/types.ts` — update komentar field `jenis`
