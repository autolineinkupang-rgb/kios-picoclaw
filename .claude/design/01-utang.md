# Desain Fitur: Hutang Pelanggan (kios_hutang)

> Dokumen desain untuk F1 dari Milestone 1 Roadmap Kios Cerdas.
> Status: DRAFT — belum ada kode produksi yang ditulis.

---

## 1. Tujuan dan Ruang Lingkup

### Masalah

Kios desa di Rote Ndao beroperasi dengan sistem "bon" — pembeli mengambil barang dan membayar
kemudian. Saat ini tidak ada pencatatan digital. Akibatnya:

- Owner tidak tahu total piutang secara agregat.
- Pelanggan dan kasir mengandalkan memori atau catatan kertas yang rawan hilang.
- Tidak ada riwayat cicilan atau audit kapan hutang dilunasi.

### Tujuan Fitur

Menyediakan tool `kios_hutang` yang memungkinkan kasir dan owner mencatat hutang baru,
menerima pembayaran, dan memantau piutang aktif — baik via Telegram bot maupun dashboard web.

### Batasan Desain

- Fitur ini ADITIF: tidak mengubah `kasir.go`, `store.go`, atau `Transaksi` yang sudah ada.
- Integrasi dengan `kios_kasir` bersifat opsional dan soft (lihat seksi 4.2).
- Cakupan: hutang perorangan per pelanggan, bukan "akun kredit" atau faktur.
- Tidak ada interest/bunga — fitur ini adalah pencatatan bon, bukan sistem kredit formal.

---

## 2. Data Model

### 2.1 Struct Go — `Hutang`

Lokasi: `pkg/tools/kios/store.go` (blok struct baru, tambahkan setelah `Pesanan`).

```go
// HutangItem adalah satu baris barang dalam catatan hutang.
type HutangItem struct {
    NamaProduk  string `json:"nama_produk"`
    Qty         int    `json:"qty"`
    HargaSatuan int    `json:"harga_satuan"`
    Subtotal    int    `json:"subtotal"`
}

// Hutang merepresentasikan catatan kredit pelanggan (bon/utang).
type Hutang struct {
    ID              string       `json:"id"`                // "HUT-0001"
    PelangganNama   string       `json:"pelanggan_nama"`    // wajib
    PelangganKontak string       `json:"pelanggan_kontak"`  // opsional (HP/alamat)
    Items           []HutangItem `json:"items"`             // daftar barang yang dibon
    Total           int          `json:"total"`             // total nominal hutang
    Terbayar        int          `json:"terbayar"`          // total yang sudah dibayar
    Sisa            int          `json:"sisa"`              // = Total - Terbayar (redundan tapi denormalized untuk query mudah)
    Tanggal         string       `json:"tanggal"`           // YYYY-MM-DD (WITA) saat hutang dicatat
    Kasir           string       `json:"kasir"`             // nama kasir yang mencatat
    Catatan         string       `json:"catatan"`           // keterangan bebas (opsional)
    Status          string       `json:"status"`            // "aktif" | "lunas"
    TanggalLunas    string       `json:"tanggal_lunas"`     // YYYY-MM-DD saat lunas, kosong jika masih aktif
    TransaksiID     string       `json:"transaksi_id"`      // opsional: link ke TRX-xxxx jika dari kasir
    RiwayatBayar    []Bayaran    `json:"riwayat_bayar"`     // history setiap pembayaran
}

// Bayaran merepresentasikan satu event pembayaran hutang.
type Bayaran struct {
    Tanggal string `json:"tanggal"` // YYYY-MM-DD (WITA)
    Jam     string `json:"jam"`     // HH:mm:ss
    Jumlah  int    `json:"jumlah"`  // nominal yang dibayar saat itu
    Kasir   string `json:"kasir"`   // siapa yang menerima pembayaran
    Catatan string `json:"catatan"` // opsional
}
```

**Keputusan desain:**
- `Sisa` disimpan redundan (= `Total - Terbayar`) agar query dashboard tidak perlu kalkulasi.
  Kasir bayar selalu update ketiganya secara atomik di layer Store.
- `Items` disimpan sebagai snapshot saat bon dicatat (tidak referensi ke `kios:produk`).
  Ini mencegah inkonsistensi jika nama produk diubah kemudian.
- `RiwayatBayar` embedded dalam struct Hutang (bukan key terpisah) karena volume per hutang kecil
  dan atomisitas update lebih mudah dijaga dengan single HSET.

### 2.2 Redis Keys

| Key | Tipe Redis | Isi |
|-----|-----------|-----|
| `kios:hutang` | HASH | field=id (mis. `HUT-0001`), value=JSON `Hutang` |
| `kios:seq:hut` | STRING | Counter auto-increment → `HUT-0001` dst. |

Tidak ada key baru lain. Format ID mengikuti pola yang ada: `INCR kios:seq:hut` → format `HUT-%04d`.

Kedua key ini wajib ditambahkan ke:
- Konstanta di `store.go`: `keyHutang = "kios:hutang"` dan `keySeqHut = "kios:seq:hut"`
- Objek KEY di `kios-dashboard/src/lib/redis.ts`: `hutang: "kios:hutang"` dan `seqHut: "kios:seq:hut"`

### 2.3 Mirror TypeScript

Lokasi: `kios-dashboard/src/lib/types.ts` (tambahkan di akhir file).

```typescript
export interface HutangItem {
  nama_produk: string;
  qty: number;
  harga_satuan: number;
  subtotal: number;
}

export interface Bayaran {
  tanggal: string;   // YYYY-MM-DD
  jam: string;       // HH:mm:ss
  jumlah: number;
  kasir: string;
  catatan: string;
}

export interface Hutang {
  id: string;                  // "HUT-0001"
  pelanggan_nama: string;
  pelanggan_kontak: string;
  items: HutangItem[];
  total: number;
  terbayar: number;
  sisa: number;
  tanggal: string;             // YYYY-MM-DD
  kasir: string;
  catatan: string;
  status: "aktif" | "lunas";
  tanggal_lunas: string;       // "" jika masih aktif
  transaksi_id: string;        // "" jika tidak terhubung ke transaksi
  riwayat_bayar: Bayaran[];
}
```

---

## 3. API Tool `kios_hutang`

### 3.1 Registrasi dan Struktur

File baru: `pkg/tools/kios/hutang.go` (< 400 baris, split ke `hutang_store.go` jika mendekati 500).

```go
type HutangTool struct{ store *Store }

func (t *HutangTool) Name() string { return "kios_hutang" }
```

### 3.2 JSON Schema Parameters

```go
func (t *HutangTool) Parameters() map[string]any {
    return map[string]any{
        "type": "object",
        "properties": map[string]any{
            "action": map[string]any{
                "type": "string",
                "enum": []string{"catat", "bayar", "daftar", "detail", "lunas"},
            },
            "id":               map[string]any{"type": "string", "description": "ID hutang (HUT-xxxx)"},
            "pelanggan":        map[string]any{"type": "string", "description": "nama atau kontak pelanggan"},
            "kontak":           map[string]any{"type": "string"},
            "items": map[string]any{
                "type": "array",
                "description": "daftar barang yang dibon [{nama_produk, qty, harga_satuan}]",
                "items": map[string]any{
                    "type": "object",
                    "properties": map[string]any{
                        "nama_produk":  map[string]any{"type": "string"},
                        "qty":          map[string]any{"type": "integer"},
                        "harga_satuan": map[string]any{"type": "integer"},
                    },
                },
            },
            "total":     map[string]any{"type": "integer", "description": "total hutang (opsional jika items diisi)"},
            "jumlah":    map[string]any{"type": "integer", "description": "nominal pembayaran (action: bayar)"},
            "catatan":   map[string]any{"type": "string"},
            "transaksi_id": map[string]any{"type": "string", "description": "opsional link ke TRX-xxxx"},
        },
        "required": []string{"action"},
    }
}
```

### 3.3 Actions Detail

#### `catat` — Hutang Baru

| Aspek | Detail |
|-------|--------|
| RBAC | kasir + owner |
| Param wajib | `pelanggan` (nama), dan minimal salah satu: `items` atau `total` |
| Param opsional | `kontak`, `catatan`, `transaksi_id` |
| Validasi | `total > 0`; jika `items` diisi, hitung `subtotal` tiap item dan `total` = sum; jika `total` juga diisi dan berbeda dari sum items, gunakan total manual + catat catatan ketidakcocokan |
| Proses | `INCR kios:seq:hut` → ID; buat `Hutang{Status:"aktif", Terbayar:0, Sisa:total}`; `HSET kios:hutang` |
| Output | Konfirmasi: ID, nama pelanggan, total hutang, daftar items |

**Contoh respons sukses:**
```
Hutang dicatat: [HUT-0042] Ibu Mariana
Total: Rp 85.000 (3 item)
- Beras 5kg x2 = Rp 60.000
- Gula 1kg x1 = Rp 15.000  
- Sabun x2 = Rp 10.000
Kasir: Yuli | 30/05/2026 09:45 WITA
```

#### `bayar` — Catat Pembayaran

| Aspek | Detail |
|-------|--------|
| RBAC | kasir + owner |
| Param wajib | `id` (HUT-xxxx) ATAU `pelanggan` (nama, akan cari hutang aktif pertama); `jumlah` |
| Validasi | `jumlah > 0`; hutang harus berstatus `aktif`; `jumlah` tidak boleh melebihi `Sisa` (warn, tapi izinkan overpayment? — lihat edge case) |
| Proses | Tambah `Bayaran` ke `RiwayatBayar`; update `Terbayar += jumlah`, `Sisa -= jumlah`; jika `Sisa <= 0`, otomatis set `Status = "lunas"` dan `TanggalLunas = today` |
| Output | Struk pembayaran: sisa hutang setelah bayar, atau konfirmasi lunas |

**Catatan:** Jika `jumlah >= Sisa`, action `bayar` otomatis melunasi (tidak perlu call `lunas` terpisah).

#### `daftar` — Semua Hutang Aktif

| Aspek | Detail |
|-------|--------|
| RBAC | kasir + owner |
| Param opsional | (tidak ada) |
| Filter | Hanya tampil yang `Status == "aktif"` |
| Output | Tabel ringkas: ID, nama pelanggan, total, terbayar, sisa, tanggal |
| Aggregasi | Tampilkan total piutang (sum `Sisa` semua aktif) di footer |

#### `detail` — Rincian Per Pelanggan

| Aspek | Detail |
|-------|--------|
| RBAC | kasir + owner |
| Param wajib | `id` ATAU `pelanggan` |
| Jika pakai `pelanggan` | Tampilkan SEMUA hutang (aktif dan lunas) milik pelanggan tersebut |
| Output | Header info pelanggan + items + riwayat pembayaran lengkap |

#### `lunas` — Tandai Lunas Manual

| Aspek | Detail |
|-------|--------|
| RBAC | owner saja |
| Param wajib | `id` |
| Validasi | Hutang harus `aktif` |
| Proses | Set `Status = "lunas"`, `TanggalLunas = today`, `Terbayar = Total`, `Sisa = 0`; tambah `Bayaran` fiktif senilai sisa (untuk audit trail) dengan catatan "dilunasi manual oleh owner" |
| Justifikasi RBAC | Tindakan ini mungkin digunakan untuk write-off (menghapus hutang tak tertagih), bukan pembayaran nyata — hak owner saja |

### 3.4 Store Methods (Go)

Tambahkan di `store.go` atau file baru `hutang_store.go`:

```go
// NextHutangID generates the next hutang ID (HUT-0001 format).
func (s *Store) NextHutangID(ctx context.Context) (string, error)

// SetHutang stores or updates a hutang record.
func (s *Store) SetHutang(ctx context.Context, h *Hutang) error

// GetHutang returns a hutang by ID or nil.
func (s *Store) GetHutang(ctx context.Context, id string) (*Hutang, error)

// GetAllHutang returns all hutang records (active + settled).
func (s *Store) GetAllHutang(ctx context.Context) ([]*Hutang, error)

// DelHutang removes a hutang by ID (untuk admin/cleanup saja, tidak diexpose ke tool).
func (s *Store) DelHutang(ctx context.Context, id string) error
```

Semua method mengikuti pola `GetAllSupplier`/`SetSupplier` yang sudah ada (HGETALL, HSET, HDel).

---

## 4. Slash Command dan Integrasi Kasir

### 4.1 Slash Command `/hutang`

Lokasi: `pkg/tools/kios/commands.go`, di dalam `CommandsWithNotif()`.

```
/hutang [nama]
```

Perilaku:
- `/hutang` (tanpa argumen): tampilkan semua hutang aktif + total piutang
- `/hutang Mariana`: tampilkan semua hutang aktif milik pelanggan bernama "Mariana"

Implementasi menggunakan `hutang.Execute(ctx, ...)` langsung, sama dengan pola `/suplier`:

```go
{
    Name:        "hutang",
    Description: "Lihat hutang aktif pelanggan (tanpa AI)",
    Usage:       "/hutang [nama pelanggan]",
    Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
        args := map[string]any{"action": "daftar"}
        if arg := argAfter(req.Text); arg != "" {
            args = map[string]any{"action": "detail", "pelanggan": arg}
        }
        return reply(req, hutang.Execute(ctx, args).ForLLM)
    },
},
```

Tambahkan juga ke `panduanText` di commands.go:
```
/hutang [nama] — lihat hutang aktif / cari per pelanggan
```

### 4.2 Integrasi Kasir (Opsional, Soft)

**Pendekatan yang dipilih: TIDAK memodifikasi `kasir.go`.**

Alasan: Memodifikasi `performJual()` atau action `jual` di kasir menambah risiko regresi pada
fitur inti yang sudah stabil. Pemisahan concern juga lebih bersih.

**Alur hutang via kasir yang diusulkan (soft integration):**

Kasir tetap menggunakan `kios_kasir` action `jual` dengan `metode: "hutang"` (enum baru).
Jika `metode == "hutang"`, `kios_kasir` mencatat transaksi seperti biasa TAPI juga:
1. Mengembalikan pesan yang menyarankan kasir untuk juga mencatat hutang dengan `kios_hutang` action `catat`.
2. Mengisi `transaksi_id` secara manual saat call `kios_hutang catat`.

**Trade-off:**

| Opsi | Pro | Kontra |
|------|-----|--------|
| Modifikasi `kasir.go` (auto-catat hutang saat metode=hutang) | UX mulus, satu call | Risiko regresi kasir; coupling kuat; kasir.go mendekati batas baris |
| Soft integration (dua call terpisah) | Zero risk ke kasir; mudah test; jelas separation of concern | Dua langkah untuk kasir; LLM perlu mengarahkan |
| Tambah metode "hutang" ke enum kasir + hint tanpa auto-catat | Minim risiko; kasir struk mencantumkan "HUTANG" | Pencatatan hutang tetap manual |

**Rekomendasi:** Implementasikan dulu sebagai dua call terpisah. Setelah `kios_hutang` stabil dan
ada test, pertimbangkan auto-catat hutang di kasir dalam iterasi berikutnya (tambah flag
`auto_hutang: true` di `jual` tanpa mengubah default behavior).

**Catatan untuk implementor:** Jika suatu saat `metode: "hutang"` ditambahkan ke enum kasir,
pastikan `performJual` tidak mengurangi stok untuk transaksi hutang — atau stok tetap dikurangi
tapi piutang dicatat. Kebijakan bisnis ini perlu dikonfirmasi ke pengguna (lihat seksi 9).

---

## 5. Dashboard Web

### 5.1 Halaman Baru: `/hutang`

Lokasi: `kios-dashboard/src/app/(app)/hutang/` (direktori baru, sejajar dengan `suplier/`).

File yang dibutuhkan:
- `page.tsx` — server component, load data, render tabel + form
- `actions.ts` — server actions (catat, bayar, lunas)

### 5.2 Komponen UI

Komponen baru di `kios-dashboard/src/components/hutang/`:

| Komponen | Fungsi |
|----------|--------|
| `hutang-table.tsx` | Tabel hutang aktif dengan kolom: ID, Pelanggan, Total, Terbayar, Sisa, Tanggal, Aksi |
| `hutang-form.tsx` | Form catat hutang baru (nama, kontak, items dinamis, catatan) |
| `bayar-form.tsx` | Modal/inline form bayar cicilan (pilih hutang, isi jumlah, kasir) |
| `hutang-detail.tsx` | Panel detail: info pelanggan + daftar items + riwayat pembayaran |

Pola mengikuti `SuplierTable` + `BandingHarga` di `src/components/suplier/`.

### 5.3 Data Access (kios.ts)

Tambahkan ke `kios-dashboard/src/lib/kios.ts`:

```typescript
export async function getAllHutang(): Promise<Hutang[]>
export async function getHutang(id: string): Promise<Hutang | null>
export async function setHutang(h: Hutang): Promise<void>
export async function nextHutangId(): Promise<string>  // INCR kios:seq:hut -> "HUT-0001"
```

### 5.4 Server Actions (actions.ts)

```typescript
// Catat hutang baru — kasir + owner
export async function createHutangAction(input: Partial<Hutang>): Promise<ActionResult>

// Catat pembayaran — kasir + owner
export async function bayarHutangAction(
  id: string,
  jumlah: number,
  kasir: string,
  catatan?: string
): Promise<ActionResult>

// Tandai lunas manual — owner saja
export async function lunasHutangAction(id: string): Promise<ActionResult>
```

Setiap action mengikuti pola `createSuplierAction` di `suplier/actions.ts`:
1. `ensureStaff()` atau `ensureOwner()` berdasarkan RBAC.
2. Validasi input (pelanggan wajib, jumlah > 0, dst.).
3. Operasi Redis via `kios.ts`.
4. `revalidatePath("/hutang")`.

### 5.5 Layout Halaman

```
/hutang
  ├── Header: "Hutang Pelanggan" + subtitle
  ├── KPI cards: Total Piutang Aktif | Jumlah Pelanggan Berhutang | Rata-rata Hutang
  ├── [Tombol "Catat Hutang Baru"] — hanya muncul jika canManage
  ├── HutangTable (hanya aktif by default, toggle "Tampilkan Lunas")
  └── HutangDetail (panel slide-in atau row expand saat klik baris)
```

### 5.6 Navigasi

Tambahkan nav item di `kios-dashboard/src/app/(app)/layout.tsx` (atau file nav terpisah):

```typescript
{ href: "/hutang", label: "Hutang", icon: CreditCard }
```

---

## 6. Titik Integrasi (File dan Fungsi Persis)

### 6.1 Go Bot

| File | Perubahan |
|------|-----------|
| `pkg/tools/kios/store.go` | Tambah const `keyHutang`, `keySeqHut`; tambah struct `Hutang`, `HutangItem`, `Bayaran`; tambah Store methods `NextHutangID`, `SetHutang`, `GetHutang`, `GetAllHutang`, `DelHutang` |
| `pkg/tools/kios/hutang.go` | File baru — struct `HutangTool`, method `Name()`, `Description()`, `Parameters()`, `Execute()`, dan 5 private action methods |
| `pkg/tools/kios/register.go` | Tambah `NewHutangTool(store)` constructor; tambah `NewHutangTool(store)` ke slice di `AllTools()` |
| `pkg/tools/kios/commands.go` | Tambah `hutang` ke slice `CommandsWithNotif()`; update `panduanText`; inisialisasi `hutang := NewHutangTool(store)` di dalam fungsi |

### 6.2 Dashboard

| File | Perubahan |
|------|-----------|
| `kios-dashboard/src/lib/types.ts` | Tambah interface `HutangItem`, `Bayaran`, `Hutang` |
| `kios-dashboard/src/lib/redis.ts` | Tambah `hutang: "kios:hutang"` dan `seqHut: "kios:seq:hut"` ke object `KEY` |
| `kios-dashboard/src/lib/kios.ts` | Tambah fungsi `getAllHutang`, `getHutang`, `setHutang`, `nextHutangId` |
| `kios-dashboard/src/app/(app)/hutang/page.tsx` | File baru |
| `kios-dashboard/src/app/(app)/hutang/actions.ts` | File baru |
| `kios-dashboard/src/components/hutang/*.tsx` | Komponen baru (hutang-table, hutang-form, bayar-form, hutang-detail) |
| `kios-dashboard/src/app/(app)/layout.tsx` | Tambah nav item "Hutang" |

### 6.3 Dependency Graph

```
hutang.go
  → store.go (Hutang, HutangItem, Bayaran structs + Store methods)
  → tool_common.go (resolveRole, requireOwner, requireStaff, argStr, argInt, argItems)
  → pkg/tools/shared/result.go (ToolResult)

register.go
  → hutang.go (NewHutangTool, HutangTool)

commands.go
  → hutang.go (NewHutangTool)
```

---

## 7. Rencana Test (Table-Driven, miniredis)

Lokasi: `pkg/tools/kios/hutang_test.go` (file baru), mengikuti pola `kios_test.go`.

### 7.1 Setup

```go
func setupTestStore(t *testing.T) *Store {
    mr := miniredis.RunT(t)
    opt, _ := redis.ParseURL("redis://" + mr.Addr())
    return NewStoreWithClient(redis.NewClient(opt))
}
```

### 7.2 Test Cases Kritis

#### TestHutangCatat

```
name                          | input                              | expected
"catat dengan items"          | items=[{beras,2,10000}], total=0   | HUT-0001, sisa=20000, status=aktif
"catat dengan total manual"   | total=50000, items=[]              | HUT-0001, sisa=50000
"catat tanpa pelanggan"       | pelanggan=""                       | error
"catat total nol"             | pelanggan="A", total=0, items=[]  | error
"id sekuensial"               | dua catat berurutan                | HUT-0001, HUT-0002
```

#### TestHutangBayar

```
name                          | setup          | input       | expected
"cicilan parsial"             | hutang 100000  | bayar 30000 | terbayar=30000, sisa=70000, status=aktif
"pelunasan tepat"             | hutang 50000   | bayar 50000 | terbayar=50000, sisa=0, status=lunas, tanggal_lunas != ""
"pelunasan via sisa kurang"   | hutang 50000 terbayar 30000 | bayar 20000 | status=lunas
"overpayment"                 | hutang 50000   | bayar 60000 | lihat edge case — warn tapi terima
"bayar hutang tidak ada"      | id="HUT-9999"  | jumlah=1000 | error not found
"bayar hutang sudah lunas"    | hutang lunas   | jumlah=1000 | error sudah lunas
"riwayat_bayar bertambah"     | 2x bayar       | —           | len(riwayat_bayar) == 2
```

#### TestHutangDaftar

```
name                          | setup                    | expected
"filter hanya aktif"          | 2 aktif + 1 lunas        | len = 2
"total piutang di output"     | 2 aktif (30k + 50k)      | "Total piutang: Rp 80.000"
"tidak ada hutang"            | —                        | pesan kosong (bukan error)
```

#### TestHutangDetail

```
name                          | input             | expected
"cari by id"                  | id="HUT-0001"     | data lengkap + riwayat
"cari by nama pelanggan"      | pelanggan="Ani"   | semua hutang Ani (aktif + lunas)
"pelanggan tidak ada"         | pelanggan="XYZ"   | pesan tidak ditemukan
```

#### TestHutangLunas

```
name                          | RBAC  | expected
"owner bisa lunas manual"     | owner | status=lunas, audit trail di riwayat
"kasir tidak bisa lunas"      | kasir | error RBAC
"lunas hutang sudah lunas"    | owner | error sudah lunas
```

#### TestHutangSekuensial (integration)

Skenario E2E dalam satu test:
1. Catat hutang Ibu Sari Rp 120.000 (3 item)
2. `/hutang` → muncul di daftar aktif
3. Bayar Rp 50.000 → sisa Rp 70.000, status aktif
4. Bayar Rp 70.000 → otomatis lunas
5. `/hutang` → Ibu Sari tidak muncul di daftar aktif
6. `detail Sari` → muncul dengan status lunas + 2 entri riwayat_bayar

---

## 8. Edge Cases dan Risiko

### 8.1 Overpayment

Jika `jumlah > Sisa`: sistem menerima pembayaran, set `status = "lunas"`, tapi `Terbayar`
tidak melebihi `Total` (kelebihan tidak disimpan). Kasir diberi peringatan di output.
Justifikasi: ini cukup untuk kios desa; implementasi "kembalian hutang" terlalu kompleks.

### 8.2 Pelanggan Sama, Banyak Hutang

`detail` dengan nama pelanggan mengembalikan SEMUA hutang matching (bukan hanya yang pertama).
`bayar` dengan nama pelanggan (bukan ID): jika ada >1 hutang aktif, tool mengembalikan error
"lebih dari satu hutang aktif, gunakan ID HUT-xxxx" untuk menghindari ambiguitas.

### 8.3 Nama Pelanggan Ambigu

Pencarian nama menggunakan substring case-insensitive (konsisten dengan `CariSupplier`).
Jika ada "Ibu Ani" dan "Ibu Anita", keduanya cocok untuk query "ani". Untuk `bayar`, ini
menyebabkan error ambiguitas (lihat 8.2). Untuk `detail` dan `daftar`, semua ditampilkan.

### 8.4 Atomisitas Update Bayaran

Redis tidak mendukung transaksi atomik `WATCH/MULTI` yang mudah dengan struct embedded.
Pendekatan: `Store.mu.Lock()` (mutex yang sudah ada di struct `Store`) sebelum read-modify-write
`kios:hutang`. Ini konsisten dengan pola `AppendTransaksi` yang sudah ada.

### 8.5 Volume Data

`GetAllHutang` membaca semua hutang (HGETALL). Untuk kios desa dengan ~50-200 pelanggan,
ini tidak masalah. Jika volume tumbuh > 1000 hutang, pertimbangkan pagination atau
mengarsip hutang lunas ke key terpisah (`kios:hutang:arsip`).

### 8.6 Backup dan Restore

Struct `Hutang` harus dimasukkan ke snapshot backup (`backup.go`) dan restore (`restore.go`).
Ini perlu diverifikasi — jika backup menggunakan `HGETALL` atas semua key `kios:*` yang
diketahui, maka `kios:hutang` dan `kios:seq:hut` perlu ditambahkan ke list backup.

### 8.7 Keamanan Dashboard

Halaman `/hutang` di dashboard mengandung data finansial sensitif (nama pelanggan, nominal).
Pastikan halaman ini ada di dalam `(app)/` group yang sudah dilindungi auth (middleware Next.js).
Tidak ada halaman hutang di `/toko` (storefront publik).

---

## 9. Pertanyaan Terbuka

1. **Stok saat jual hutang:** Apakah saat kasir mencatat bon, stok harus langsung dikurangi?
   Jika ya, integrasi dengan `kios_kasir` menjadi wajib (bukan opsional). Jika tidak, berarti
   stok tidak mencerminkan barang yang sudah "keluar" lewat bon. Kebijakan ini perlu konfirmasi
   pengguna sebelum implementasi.

2. **Notifikasi hutang jatuh tempo:** Apakah perlu reminder otomatis ke Telegram jika hutang
   sudah lebih dari X hari? Jika ya, ini extend `notif.go` (mirip `tryNotifyExpiring`).
   Tidak dimasukkan ke scope saat ini.

3. **Hutang tidak berbatas waktu:** Saat ini tidak ada field `jatuh_tempo`. Apakah perlu?
   Menambahkan field ini mudah (tambah `JatuhTempo string` ke struct), tapi business logic
   (apa yang terjadi jika jatuh tempo) belum ada.

4. **Pelanggan sebagai entitas terpisah:** F6 di Roadmap mendefinisikan `kios_pelanggan`.
   Jika F6 diimplementasikan lebih dulu, `Hutang` bisa punya field `pelanggan_id` (referensi
   ke entitas pelanggan). Apakah F1 harus menunggu F6, atau jalan sendiri dengan nama saja?
   Rekomendasi: jalan sendiri dulu dengan nama + kontak string. Migrasi ke `pelanggan_id`
   bisa dilakukan kemudian (field opsional, backward-compatible).

5. **Hapus hutang:** Apakah owner perlu bisa menghapus catatan hutang sepenuhnya (misalnya
   salah catat)? Saat ini hanya `lunas` manual yang disediakan. `DelHutang` ada di Store layer
   tapi tidak diexpose ke tool. Perlu ditambahkan action `hapus` (owner-only)?

6. **Format output Telegram:** Hutang dengan banyak items bisa menghasilkan pesan panjang.
   Apakah perlu membatasi tampilan items di `daftar` (hanya total, tanpa breakdown items)?
   Saat ini desain menampilkan ringkasan di `daftar` dan detail penuh di `detail`.

---

*Dokumen ini ditulis oleh agen desain Ruflo-swarm. Tidak ada kode produksi yang diubah.*
*Versi: 1.0 — 2026-05-30*
