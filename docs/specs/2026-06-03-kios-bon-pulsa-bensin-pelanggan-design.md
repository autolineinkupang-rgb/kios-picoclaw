# Desain: Bon/Hutang, Pulsa & Bensin, Satuan Beli, Suplier↔Produk, Registry Pelanggan, Barcode

**Tanggal:** 2026-06-03
**Status:** Disetujui untuk perencanaan implementasi
**Proyek:** kios-picoclaw (Go Telegram bot + Next.js dashboard, Upstash Redis)

Dokumen ini menggabungkan enam fitur baru menjadi satu fondasi data yang konsisten. Disusun dari riset 5 agent perencana terhadap basis kode nyata.

---

## 1. Prinsip Inti (Non-negotiable)

1. **Satu tabel produk.** Pulsa & bensin TETAP baris `Produk` (HASH `kios:produk`), dibedakan oleh field `jenis`. Tidak ada "sistem produk kedua". Alasan: `GetAllProduk` (`store.go:288`), `CariProduk` (`store.go:335`), laporan, notif, backup, dan dashboard semuanya iterasi `Produk` — produk baru harus terlihat oleh semuanya.
2. **Satu corong penjualan.** Semua jual lewat `performJual` (`tool_common.go:186`) → `AppendTransaksi` (`store.go:368`). Penjualan kredit (bon), pulsa, dan bensin MEMPERLUAS corong ini via dispatch `jenis`/`metode`, bukan fork.
3. **Additive.** Kios tools tidak boleh memecah build picoclaw upstream. Semua field baru `omitempty`; data lama tetap dekode valid (`jenis` kosong = `"biasa"`).
4. **Kontrak Go↔TS.** Setiap perubahan struct Go = perubahan mirror di `kios-dashboard/src/lib/types.ts` (+ `redis.ts` KEY, + `kios.ts` data-access bila perlu).
5. **Tanpa SSRF/backdoor.** Tidak ada fetch URL sisi-server dari input pembeli (mempertahankan prinsip perbaikan QRIS sebelumnya). Decode barcode 100% sisi-klien.
6. **Disiplin ukuran file.** Go file < 500 baris. `store.go` (616), `commands.go` (491), `stok.go` (478) sudah di/atas batas → pecah dulu sebelum menambah fitur.

---

## 2. Keputusan Final (dikonfirmasi owner)

| Keputusan | Pilihan |
|---|---|
| Bon/Hutang | DUA buku: piutang (pembeli ngutang) + hutang (kios ke suplier) |
| Model pulsa | Saldo modal rupiah; tabel nominal 5k/10k/15k/20k/25k/50k/100k {modal, jual, margin}; **satu saldo gabungan** (bukan per-operator) |
| BBM | Hanya **bensin** (Pertalite/Pertamax) model liter (mili-liter, kritis 40 L). **Solar & minyak tanah = produk biasa** |
| Satuan beli | Per-produk: daftar kemasan {nama, isi}; harga beli/pcs = harga_kemasan / isi; stok += qty × isi; kebijakan **timpa harga terbaru** |
| Identitas pembeli | Nama + WA **wajib**, backed registry **Pelanggan** (key = no. WA ternormalisasi) |
| Tombol QRIS | Sembunyikan tombol "Kirim Pesanan" generik saat alur QRIS-dinamis menampilkan "Sudah kirim — Buat Pesanan" |
| Barcode | Dashboard kasir scan kamera (`BarcodeDetector`, tanpa dep baru) + Telegram ketik angka barcode. Foto barcode = ditunda |
| Slash bon | `/utang` = piutang pembeli; `/hutang` = utang ke suplier |
| Migrasi suplier | Ke `supplier_id`, **bertahap dual-read** (baca format lama+baru, tulis baru) |
| Akses halaman Pelanggan | Kasir boleh lihat; hapus pelanggan owner-only |

---

## 3. Model Data

### 3.1 Perubahan `Produk` (`store.go:21`, additive)

```go
Jenis        string    `json:"jenis,omitempty"`          // "" | "biasa" | "pulsa" | "bensin"
PackDefs     []Kemasan `json:"pack_defs,omitempty"`      // fitur satuan beli
SupplierID   string    `json:"supplier_id,omitempty"`    // FK stabil ke Supplier.ID
SaldoModal   int       `json:"saldo_modal,omitempty"`    // pulsa: saldo modal rupiah
StokMl       int       `json:"stok_ml,omitempty"`        // bensin: stok mili-liter (int, hindari drift float)
StokKritisMl int       `json:"stok_kritis_ml,omitempty"` // bensin: ambang kritis, default 40000 (40 L)
```

Helper `func (p *Produk) JenisOrDefault() string` → `"biasa"` bila kosong. Pakai helper ini di mana-mana, bukan baca field mentah.

> Catatan unit bensin: **integer mili-liter**, bukan float. Uang sudah integer rupiah; volume integer menghindari akumulasi error float lintas ribuan transaksi & membuat JSON round-trip deterministik. `HargaBeli`/`HargaJual` tetap rupiah/liter; nilai jual = `hargaPerLiter * ml / 1000` dengan pembulatan eksplisit.

### 3.2 Struct baru

```go
type Kemasan struct {
    Nama string `json:"nama"` // "dos","lusin","setengah lusin","box","renteng","ball",...
    Isi  int    `json:"isi"`  // pcs per kemasan
}

type Pelanggan struct {
    ID         string `json:"id"`          // "PLG-<wa>" (no counter; WA = identitas)
    Phone      string `json:"phone"`       // no. WA ternormalisasi "62..." (HASH field key)
    Nama       string `json:"nama"`
    TotalUtang int    `json:"total_utang"` // cache denormalisasi dari piutang terbuka (read-only di sisi storefront)
    TotalPesanan int  `json:"total_pesanan"`
    TotalBelanja int  `json:"total_belanja"`
    Catatan    string `json:"catatan"`
    CreatedAt  int64  `json:"created_at"`
    LastOrder  string `json:"last_order"`
}

type Piutang struct {
    ID          string `json:"id"`           // "PIU-0001"
    PelangganID string `json:"pelanggan_id"`
    Phone       string `json:"phone"`
    TransaksiID string `json:"transaksi_id,omitempty"`
    Pokok       int    `json:"pokok"`
    Dibayar     int    `json:"dibayar"`
    Sisa        int    `json:"sisa"`
    Status      string `json:"status"`       // "terbuka" | "lunas" | "dihapus"
    Tanggal     string `json:"tanggal"`
    Jam         string `json:"jam"`
    Kasir       string `json:"kasir"`
    Catatan     string `json:"catatan"`
}

type Hutang struct {
    ID          string `json:"id"`           // "HUT-0001"
    SupplierID  string `json:"supplier_id"`
    PembelianID string `json:"pembelian_id,omitempty"`
    Pokok       int    `json:"pokok"`
    Dibayar     int    `json:"dibayar"`
    Sisa        int    `json:"sisa"`
    Status      string `json:"status"`       // "terbuka" | "lunas" | "dihapus"
    JatuhTempo  string `json:"jatuh_tempo,omitempty"`
    Tanggal     string `json:"tanggal"`
    Catatan     string `json:"catatan"`
}

type Pembayaran struct {
    ID       string `json:"id"`       // "PAY-0001"
    LedgerID string `json:"ledger_id"`// PIU-* atau HUT-*
    Jenis    string `json:"jenis"`    // "piutang" | "hutang"
    Jumlah   int    `json:"jumlah"`
    Metode   string `json:"metode"`   // tunai|transfer|qris
    Tanggal  string `json:"tanggal"`
    Jam      string `json:"jam"`
    Kasir    string `json:"kasir"`
    Catatan  string `json:"catatan"`
}

type PulsaDenom struct {
    Nominal    int  `json:"nominal"`     // 5000..100000
    HargaModal int  `json:"harga_modal"` // dikurangi dari saldo_modal saat jual
    HargaJual  int  `json:"harga_jual"`  // kas masuk
    Aktif      bool `json:"aktif"`
}
// Margin() = HargaJual - HargaModal

type PulsaTopup struct {
    ID           string `json:"id"`      // "PTU-0001"
    Tanggal      string `json:"tanggal"`
    Jam          string `json:"jam"`
    Jumlah       int    `json:"jumlah"`
    SaldoSesudah int    `json:"saldo_sesudah"`
    Kasir        string `json:"kasir"`
    Catatan      string `json:"catatan"`
}
```

### 3.3 Perluasan struct lama (additive)

```go
// Transaksi (store.go:40)
Modal     int     `json:"modal,omitempty"`      // modal dikunci saat jual → laba pulsa/bensin akurat & historis
Liter     float64 `json:"liter,omitempty"`      // penjualan bensin (display)
PiutangID string  `json:"piutang_id,omitempty"` // diisi bila jual kredit

// Pesanan (store.go:115)
PelangganID string `json:"pelanggan_id,omitempty"`

// Pembelian (store.go:57) — fitur satuan beli
Kemasan    string `json:"kemasan,omitempty"`
Isi        int    `json:"isi,omitempty"`
QtyPack    int    `json:"qty_pack,omitempty"`
HargaPack  int    `json:"harga_pack,omitempty"` // sumber kebenaran; harga/pcs = derivasi
SupplierID string `json:"supplier_id,omitempty"`
```

### 3.4 Inventaris Redis key

| Key | Tipe | Counter | Field/Isi |
|---|---|---|---|
| `kios:pelanggan` | HASH | — | field = no. WA ternormalisasi; value = `Pelanggan` JSON |
| `kios:piutang` | HASH | `kios:seq:piu` | field = ID; mutable (saldo berubah) |
| `kios:hutang` | HASH | `kios:seq:hut` | field = ID; mutable |
| `kios:pembayaran` | LIST | `kios:seq:pay` | append-only (audit) |
| `kios:pulsa:denom` | HASH | — | field = nominal; value = `PulsaDenom` |
| `kios:pulsa:topup` | LIST | `kios:seq:ptu` | append-only |
| `kios:harga_supplier` | HASH | — | **reuse**; migrasi field ke `<produk_id>\|<supplier_id>` (dual-read) |

> HASH untuk record mutable (piutang/hutang/pelanggan), LIST untuk event immutable (pembayaran/topup) — sesuai pola Transaksi/Pembelian yang sudah ada. Saldo pulsa & stok-liter bensin **menumpang di `Produk`** → tidak ada key terpisah, permukaan backup minim.
>
> **Pemisahan piutang dari Pelanggan (anti write-contention):** `Piutang` disimpan di key sendiri (`kios:piutang`), bukan di-embed ke JSON `Pelanggan`. `Pelanggan.TotalUtang` hanyalah cache yang di-recompute. Ini mencegah upsert pesanan (dari storefront) menimpa update piutang (dari kasir).

### 3.5 Counter
Semua entitas bernomor pakai pola INCR `kios:seq:<x>` (seperti `NextSupplierID`, `store_more.go:87`) → otomatis kompatibel dengan `setSeq`/`maxNumericSuffix` di restore. `Pelanggan` tidak butuh counter (ID = `PLG-<phone>`).

---

## 4. Integrasi per Fitur

### 4.1 Bon/Hutang (Fitur 1)
- **Tool `kios_bon`** (file baru `bon.go`) — aksi: `jual_bon`, `bayar`, `lunasi`, `catat_hutang_supplier`, `bayar_hutang`, `hapus`/`write_off`, `daftar_piutang`, `daftar_hutang`, `detail`.
- **Slash (0-token):** `/utang [nama]` (piutang pembeli), `/hutang [suplier]` (utang ke suplier), `/bayar <PIU-id|HUT-id> <jumlah>`, `/jualutang <produk> <qty> <pelanggan>`.
- **Jual kredit:** panggil `performJual` apa adanya (rekam Transaksi normal), set `metode="bon"`, lalu buka `Piutang` dengan `TransaksiID=tx.ID`, `Pokok=tx.Total`; bump `Pelanggan.TotalUtang`. Lewati guard `bayar<total` di `kasir.go:85-96` untuk `metode=="bon"`.
- **Hutang suplier:** dari `pembelian_id` yang ada (`GetAllPembelian`, `store_more.go:39`) → buka `Hutang`.
- **`/batal` transaksi bon** (`batalkanTx`, `stok.go:437`): bila `tx.MetodeBayar=="bon"`, cari piutang via `TransaksiID`; belum ada cicilan → void; sudah ada cicilan → tolak ("gunakan write-off").
- **Overpayment ditolak** (`jumlah > sisa`); `jumlah<=0` ditolak; `sisa` tak boleh negatif.
- **Hapus pelanggan/suplier dengan utang berjalan → diblokir.**

### 4.2 Pulsa & Bensin (Fitur 2)
- **Dispatch di `performJual`** (`tool_common.go:186`) setelah `findOne`:
  - `pulsa`: caller kirim `nominal`; cek `SaldoModal >= denom.HargaModal`; `SaldoModal -= HargaModal`; Transaksi `{Qty:1, HargaSatuan:HargaJual, Total:HargaJual, Modal:HargaModal, Kategori:"pulsa"}`. Tanpa ubah `Stok`. **Kurang saldo → tolak sebelum rekam.**
  - `bensin`: caller kirim `liter` atau `bayar`(rupiah→ml); cek `StokMl>=ml`; `StokMl-=ml`; Transaksi `{Total:hargaPerLiter*ml/1000 (bulat), Modal:hargaBeliPerLiter*ml/1000, Liter:ml/1000}`.
- `KasirTool.Parameters` (+`nominal`, +`liter`, enum metode +`bon`). `jual_massal` ikut otomatis (pakai `performJual` yang sama).
- **`batalkanTx` dikoreksi:** branch `jenis` → pulsa balikin `SaldoModal+=Modal`, bensin balikin `StokMl`. (Saat ini cuma `Stok++` → salah untuk keduanya.)
- **Laba** (`hitungLaba`, `laporan.go:108`): pakai `tx.Modal` bila `>0`, fallback ke `Qty*HargaBeli`. Akurat untuk semua jenis.
- **Notif** (`buildLowStockMessage`, `notif.go:166`): bensin → `StokMl<=StokKritisMl`; pulsa → `SaldoModal<=ambang`. Solar/minyak tanah ikut jalur stok biasa.
- **Pulsa modal** (`kios:pulsa:denom` + `Produk.SaldoModal`), top-up via `/isipulsa` (owner) + screen dashboard.
- **Tanpa barcode** untuk pulsa/bensin (`CariProduk` sudah skip barcode kosong).

### 4.3 Satuan Beli + Suplier↔Produk (Fitur 3)
- **Restock** (`stok.go:167` `tambah`): bila ada `kemasan`+`qty_pack`+`harga_pack` → `isi = lookup(PackDefs)` atau arg `isi`; `qty = qty_pack*isi`; `harga_beli = round(harga_pack/isi)`. Jalur per-pcs lama tetap ada. `recordPembelian` simpan field kemasan + tulis snapshot harga per-suplier.
- **Kebijakan harga beli: timpa terbaru** (rekam `PriceHistory`).
- **`atur_kemasan`** (owner) untuk definisikan PackDefs produk.
- **Suplier↔produk many-to-many** via `supplier_id` + `harga_supplier` (per produk-suplier). `SupplierTool.cari`/`bandingHarga` pakai relasi ID; `hapus` cascade hapus override + snapshot suplier itu.
- **Migrasi `harga_supplier` (dual-read):** `hargaSupplierField` (`store_more.go:229`) coba ID dulu, fallback nama; tulis selalu format ID. Resolver lazy: produk dengan `SupplierID==""` tapi `Supplier!=""` → match nama saat baca, tulis-balik ID opportunistik di `SetProduk` berikutnya. **Tanpa batch rewrite.**

### 4.4 Storefront / Pelanggan / Barcode (Fitur 4)
- **Validasi WA** (`lib/wa.ts`): `isValidWaNumber` pakai `normalizeWaNumber` yang sudah ada → regex `^62\d{8,13}$`. Nama ≥2 char.
- **Storefront** (`storefront-view.tsx`): label hapus "(opsional)", error inline (`role="alert"`, `aria-invalid`), submit di-blok sampai `formOk`. Gate juga tombol "Chat kasir QR dinamis" & tombol submit QRIS.
- **API `/api/pesanan`**: validasi server (jangan percaya klien), normalisasi WA, drop fallback `"Pembeli"`, **upsert Pelanggan** (find-or-create by phone) sebelum `setPesanan`, set `pesanan.pelanggan_id`.
- **Tombol QRIS:** `const qrisChatFlow = qris.enabled && metode==="qris" && !!waNumber;` → tombol "Kirim Pesanan" generik (`storefront-view.tsx:491`) dibungkus `{!qrisChatFlow && (...)}`. Hasil: tunai → 1 tombol; qris+ada WA → hanya "Sudah kirim — Buat Pesanan"; qris tanpa WA → 1 tombol generik.
- **Halaman Pelanggan** (`(app)/pelanggan/`): list (cari nama/WA), detail (riwayat order filter `pelanggan_id` + ringkas piutang + quick WA). Nav item ikon `Contact`/`UserRound` (hindari bentrok `Users` di `/pengguna`). **Kasir boleh lihat; hapus owner-only.**
- **Barcode dashboard** (`kasir-form.tsx`): tombol Scan → komponen `barcode-scanner.tsx` pakai `BarcodeDetector` (formats: `ean_13,ean_8,code_128,qr_code`) + `getUserMedia({facingMode:"environment"})`; hasil → set `query` (pencarian `matchesQuery` sudah cek `barcode`). Fallback: pesan + fokus input (scanner USB ketik ke field). **Tanpa dep baru**; zxing hanya bila iOS Safari diwajibkan (lazy import).
- **Barcode Telegram:** ketik angka sudah jalan via `CariProduk` (`store.go:339`). Foto barcode ditunda (LLM tak andal decode 1D); balas "ketik angka barcode ya kak".

---

## 5. RBAC (tambahan)

Aturan: penangkapan transaksi (jual, terima bayar, daftar pelanggan) = kasir+owner; uang-config & hapus = owner.

| Aksi | kasir | owner |
|---|:---:|:---:|
| Jual pulsa / bensin | ✅ | ✅ |
| Jual kredit / catat bon (piutang) | ✅ | ✅ |
| Catat pembayaran piutang | ✅ | ✅ |
| Hapus / write-off piutang | ❌ | ✅ |
| Catat & bayar hutang ke suplier | ❌ | ✅ |
| Top-up saldo modal pulsa | ❌ | ✅ |
| Set harga nominal pulsa / set kritis-liter / set stok-liter manual | ❌ | ✅ |
| Restock (pulsa/bensin/pack normal) | ✅ | ✅ |
| Definisi kemasan (isi per dos/lusin) | ❌ | ✅ |
| Set harga beli per-suplier (override) | ❌ | ✅ |
| Daftar/edit pelanggan, lihat halaman Pelanggan | ✅ | ✅ |
| Hapus pelanggan | ❌ | ✅ |
| Scan barcode (pencarian) | ✅ | ✅ |

Enforce via `requireOwner`/`requireStaff` (`tool_common.go:152/160`) di Go; `ensureStaff`/`ensureOwner` di dashboard.

---

## 6. Backup / Restore / Seed / Import

- **Bug yang diperbaiki sekalian:** `harga_supplier` saat ini TIDAK ikut backup → masukkan sekarang.
- `BackupData` (`backup.go:20`) + field: `Pelanggan`, `Piutang`, `Hutang`, `Pembayaran`, `PulsaDenom`, `PulsaTopup`, `HargaSupplier`. (Saldo pulsa & liter bensin ikut di `Produk`.)
- `BuildBackup`/`Ringkas`/`RestoreBackup`/`HasAnyData`: tambah key + counter (`keyPiutang/Hutang/Pembayaran/PulsaDenom/PulsaTopup/Pelanggan` + `seq:piu/hut/pay/ptu`). Restore recompute `Sisa`/`Dibayar` dari list pembayaran (self-heal cache).
- Backup `Versi` → `"1.1"` (forward-compatible; backup `1.0` lama tetap restore, slice baru nil→kosong).
- **Seed:** tidak perlu baris pelanggan/piutang/hutang. Opsional: seed 1 anchor pulsa + 7 nominal default + 2 baris bensin (`StokKritisMl=40000`), di belakang `IsSeedDone`.
- **Import:** `ImportProdukRows` terima kolom opsional `jenis`/`isi`/kemasan/`supplier_id`; tambah `ImportPelangganRows` (paralel `ImportSupplierRows`).

---

## 7. Strategi Test

- Paket diuji: `pkg/tools/kios` (CGO-free), table-driven + miniredis via `NewStoreWithClient`.
- **Perintah benar:** `go test -tags goolm,stdjson ./pkg/tools/kios/...` (atau `make test`). `go test ./...` polos GAGAL (jalur CGO libolm di matrix gateway). Export toolchain dulu (CLAUDE.md §Toolchain).
- File test baru (masing-masing <500 baris): `bon_test.go`, `special_produk_test.go`, `pack_test.go`, `pelanggan_test.go`.
- **Test backup round-trip** (paling bernilai): build→restore→assert semua key+counter baru bertahan, termasuk `harga_supplier`.
- Update `TestToolsRegister` (`kios_test.go:513`) untuk jumlah tool baru.

---

## 8. Layout File (semua <500 baris)

**Pecah dulu (Fase 0):** `store.go` (616), `commands.go` (491), `stok.go` (478).

| File baru | Isi |
|---|---|
| `store_pelanggan.go` | `Pelanggan` + CRUD + normalisasi phone |
| `store_bon.go` | `Piutang`/`Hutang`/`Pembayaran` + CRUD + counter |
| `pack.go` | `Kemasan` + konversi pack→pcs (murni, mudah ditest) |
| `special.go` | dispatch pulsa/bensin + helper (dipakai `performJual` & notif) |
| `bon.go` | tool `kios_bon` |
| `pelanggan.go` | tool `kios_pelanggan` (bila terpisah dari kasir) |
| `commands_bon.go` | slash `/utang`,`/hutang`,`/bayar`,`/jualutang` |
| `*_test.go` | per fitur (lihat §7) |
| Dashboard | `(app)/pelanggan/*`, `components/pelanggan/*`, `components/kasir/barcode-scanner.tsx`, `(app)/pulsa/*` |

Struct `Produk` tetap di `store.go` (tipe kanonik) — pengecualian; tambah field di sana.

---

## 9. Urutan Bangun (dependency-ordered)

**Fase 0 — Fondasi (serial, blocking):**
1. Pecah file >500 baris (mekanis, risiko rendah, dulukan).
2. Tambah `Produk.Jenis` + helper + `SupplierID`; wire ke `performJual`/laporan/notif sebagai **refactor netral-perilaku** (penjualan biasa byte-identik, dibuktikan test) sebelum jenis baru ada.
3. Perbaiki gap backup `harga_supplier` + bump versi `1.1`.
   > *Item paling berisiko:* refactor `performJual` (corong tunggal, state uang).

**Fase 1 — Registry Pelanggan (serial, blok fitur 1 & 4):**
4. `Pelanggan` struct/key/CRUD + backup/restore/HasAnyData + normalisasi phone.

**Fase 2 — Paralel (setelah Fase 1):**
- Track A — Bon/Hutang (butuh Pelanggan + Supplier + `metode="bon"`). *Fitur paling berisiko* (state uang) → test terkuat.
- Track B — Pulsa/Bensin (butuh `Jenis` Fase 0). Self-contained di `special.go`.
- Track C — Satuan beli + Suplier↔Produk (butuh `SupplierID`). Risiko: migrasi dual-read `harga_supplier`.
- Track D — Storefront/Pelanggan UX + Barcode + QRIS button (butuh Pelanggan).
- Merge yang menyentuh `performJual` (A & B) **diserialkan** — landing B dulu (risiko lebih rendah), A rebase.

**Fase 3 — Penutup (serial):**
5. `DashboardSummary` + `/api/summary` (+ piutang/hutang/liter/saldo).
6. Test backup round-trip lintas semua fitur — gate rilis terakhir.

**Tiga item paling berisiko (urut):** (1) refactor `performJual` `Jenis`+`bon`; (2) migrasi format key `harga_supplier`; (3) kelengkapan backup/restore (satu-satunya salinan durable — key terlewat = kehilangan data senyap).

---

## 10. Risiko & Catatan Terbuka

- **TS `recordSale` (`lib/sales.ts`) menduplikasi logika jual Go** — logika pulsa/bensin/bon harus diimplement dua kali dan dijaga sinkron (utang teknis yang sudah ada).
- **Konkurensi saldo pulsa & total_utang:** read-modify-write; bot & dashboard proses berbeda. Pakai `s.mu` / DECRBY Redis untuk hindari lost update.
- **Nomor WA typo → pelanggan duplikat / piutang terpecah.** v1 terima (phone = identitas); kasir verifikasi nomor di kasir; aksi "merge" pelanggan = fitur masa depan.
- **Migrasi `harga_supplier`:** jangan hapus field format-nama sampai terverifikasi semua ter-backfill.
