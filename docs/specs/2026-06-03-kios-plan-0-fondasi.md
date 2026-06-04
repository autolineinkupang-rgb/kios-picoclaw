# Plan 0 — Fondasi (Fase 0) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Menyiapkan fondasi data & corong penjualan agar fitur Fase 2 (bon, pulsa/bensin, satuan-beli, pelanggan) bisa menumpang tanpa memecah build atau perilaku yang ada.

**Architecture:** Tambah discriminator `Produk.Jenis` (default lazy `"biasa"`) + `Produk.SupplierID`; refactor `performJual` menjadi switch-by-jenis dengan cabang `sellBiasa` yang byte-identik dengan perilaku sekarang; perbaiki bug backup (`harga_supplier` belum ikut) + bump versi; pecah `store.go` (616 baris) agar di bawah batas 500.

**Tech Stack:** Go (package `pkg/tools/kios`), Redis (Upstash) via go-redis, test table-driven + `miniredis`. Mirror TypeScript di `kios-dashboard/src/lib/types.ts`.

**Referensi spec:** `docs/specs/2026-06-03-kios-bon-pulsa-bensin-pelanggan-design.md` (§1, §3.1, §6, §9 Fase 0).

**Prasyarat toolchain (jalankan di setiap sesi terminal sebelum perintah Go):**
```bash
export GOROOT=$HOME/sdk/go PATH=$HOME/sdk/go/bin:$PATH GOPATH=$HOME/go
```
Perintah test kanonik (jangan pakai `go test ./...` polos — gagal karena CGO libolm):
```bash
go test -tags goolm,stdjson ./pkg/tools/kios/...
```

---

## File Structure

| File | Tanggung jawab | Aksi |
|---|---|---|
| `pkg/tools/kios/store.go` | Definisi tipe `Produk` & struct lain, konstanta key, `KiosConfig` | Modify (tambah field + helper; lalu dipangkas saat split) |
| `pkg/tools/kios/store_access.go` | **BARU** — semua method data-access `*Store` + helper package-level | Create (Task 4) |
| `pkg/tools/kios/tool_common.go` | `performJual`, `findOne`, helper jual | Modify (refactor switch + `sellBiasa`) |
| `pkg/tools/kios/backup.go` | `BackupData`, build/restore/ringkas/HasAnyData | Modify (tambah `harga_supplier`) |
| `pkg/tools/kios/kios_test.go` | Test paket kios | Modify (tambah test) |
| `kios-dashboard/src/lib/types.ts` | Mirror TS dari struct Go | Modify (mirror field `Produk`) |

---

## Task 1: Tambah `Jenis` + `SupplierID` ke `Produk` + helper `JenisOrDefault`

**Files:**
- Modify: `pkg/tools/kios/store.go:21-37` (struct `Produk`) dan tambah method setelahnya
- Test: `pkg/tools/kios/kios_test.go`
- Modify: `kios-dashboard/src/lib/types.ts` (mirror)

- [ ] **Step 1: Tulis test yang gagal**

Tambahkan di akhir `pkg/tools/kios/kios_test.go`:

```go
func TestJenisOrDefault(t *testing.T) {
	cases := map[string]string{"": "biasa", "biasa": "biasa", "pulsa": "pulsa", "bensin": "bensin"}
	for in, want := range cases {
		p := &Produk{Jenis: in}
		if got := p.JenisOrDefault(); got != want {
			t.Errorf("JenisOrDefault(%q)=%q want %q", in, got, want)
		}
	}
}
```

- [ ] **Step 2: Jalankan test, pastikan GAGAL (kompilasi)**

Run: `go test -tags goolm,stdjson ./pkg/tools/kios/... -run TestJenisOrDefault`
Expected: FAIL — `p.JenisOrDefault undefined` dan/atau `unknown field Jenis`.

- [ ] **Step 3: Tambah field ke struct `Produk`**

Di `pkg/tools/kios/store.go`, dalam struct `Produk` (setelah baris `ImageURL string json:"image_url"`, sebelum `}`), tambahkan:

```go
	Jenis      string `json:"jenis,omitempty"`       // "" | "biasa" | "pulsa" | "bensin"
	SupplierID string `json:"supplier_id,omitempty"` // FK stabil ke Supplier.ID
```

- [ ] **Step 4: Tambah method helper**

Tepat setelah penutup `}` struct `Produk` di `store.go`, tambahkan:

```go
// JenisOrDefault returns the product kind, defaulting to "biasa" when unset so
// pre-existing products (which have no jenis field) behave as ordinary stock items.
func (p *Produk) JenisOrDefault() string {
	if p.Jenis == "" {
		return "biasa"
	}
	return p.Jenis
}
```

- [ ] **Step 5: Jalankan test, pastikan LULUS**

Run: `go test -tags goolm,stdjson ./pkg/tools/kios/... -run TestJenisOrDefault`
Expected: PASS.

- [ ] **Step 6: Mirror ke TypeScript**

Di `kios-dashboard/src/lib/types.ts`, pada interface `Produk`, tambahkan dua field opsional (cocokkan dengan field lain yang sudah ada di interface itu):

```ts
  jenis?: string;        // "" | "biasa" | "pulsa" | "bensin"
  supplier_id?: string;
```

Verifikasi: `cd kios-dashboard && npm run typecheck` → tidak ada error baru. (Kembali ke root setelahnya.)

- [ ] **Step 7: Commit**

```bash
git add pkg/tools/kios/store.go pkg/tools/kios/kios_test.go kios-dashboard/src/lib/types.ts
git commit -m "feat(kios): tambah Produk.Jenis + SupplierID + JenisOrDefault (fondasi)"
```

---

## Task 2: Refactor `performJual` menjadi switch-by-jenis dengan cabang `sellBiasa`

Tujuan: memindahkan logika penjualan biasa ke fungsi `sellBiasa`, dan membuat `performJual` mendispatch berdasarkan `JenisOrDefault()`. Untuk Plan 0 hanya ada cabang `"biasa"` (default) → perilaku **identik**. Ini menyiapkan titik sisip pulsa/bensin di Plan B.

**Files:**
- Modify: `pkg/tools/kios/tool_common.go:186-236` (`performJual`)
- Test: `pkg/tools/kios/kios_test.go`

- [ ] **Step 1: Tulis test regresi yang menegaskan dispatch**

Tambahkan di akhir `pkg/tools/kios/kios_test.go`:

```go
func TestPerformJualJenisBiasaRoute(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	// Produk dengan Jenis eksplisit "biasa" harus berperilaku sama dgn produk lama.
	if err := s.SetProduk(ctx, &Produk{
		ID: "010", Nama: "Minyak Goreng 1L", Kategori: "umum", Satuan: "pcs",
		Jenis: "biasa", Stok: 6, HargaBeli: 16000, HargaJual: 18000, StokMinimum: 5, StokKritis: 2,
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	tx, _, sisa, err := performJual(ctx, s, "minyak", 2, "tunai", "ken", 0)
	if err != nil {
		t.Fatalf("performJual: %v", err)
	}
	if tx.Total != 36000 || sisa != 4 {
		t.Errorf("total=%d sisa=%d want 36000/4", tx.Total, sisa)
	}
}
```

- [ ] **Step 2: Jalankan test, pastikan LULUS dulu (baseline hijau)**

Run: `go test -tags goolm,stdjson ./pkg/tools/kios/... -run 'TestPerformJual'`
Expected: PASS (test lama `TestPerformJual` + test baru). Ini baseline sebelum refactor — refactor tidak boleh mengubah hasil.

- [ ] **Step 3: Ganti badan `performJual` + tambah `sellBiasa`**

Di `pkg/tools/kios/tool_common.go`, ganti SELURUH fungsi `performJual` (baris 186-236, dari `func performJual(` sampai `}` penutupnya) dengan dua fungsi berikut:

```go
// performJual executes a sale. It validates inputs, then dispatches by product
// kind. Ordinary unit-stock products go through sellBiasa (the historical path);
// special kinds (pulsa/bensin) are added in later plans.
func performJual(ctx context.Context, store *Store, query string, qty int, metode, kasir string, diskonPerUnit int) (*Transaksi, *Produk, int, error) {
	if qty <= 0 {
		return nil, nil, 0, fmt.Errorf("jumlahnya harus lebih dari 0 ya kak 🙏")
	}
	item, err := findOne(ctx, store, query)
	if err != nil {
		return nil, nil, 0, err
	}
	if item == nil {
		return nil, nil, 0, fmt.Errorf("produk \"%s\" nggak ketemu kak 🔍 coba ketik /stok buat lihat daftarnya ya", query)
	}
	if metode == "" {
		metode = "tunai"
	}
	switch item.JenisOrDefault() {
	default: // "biasa"
		return sellBiasa(ctx, store, item, qty, metode, kasir, diskonPerUnit)
	}
}

// sellBiasa records a sale of an ordinary unit-stock product: it validates stock,
// decrements it, and appends the transaction at the effective unit price
// (list price minus diskonPerUnit). Returns the transaction, updated product, and
// remaining stock.
func sellBiasa(ctx context.Context, store *Store, item *Produk, qty int, metode, kasir string, diskonPerUnit int) (*Transaksi, *Produk, int, error) {
	if item.Stok < qty {
		return nil, nil, 0, fmt.Errorf("yah, stok %s tinggal %d kak 😅 nggak cukup buat jual segitu", item.Nama, item.Stok)
	}
	hargaEfektif := item.HargaJual - diskonPerUnit
	if hargaEfektif < 0 {
		hargaEfektif = 0
	}
	catatan := ""
	if diskonPerUnit > 0 {
		catatan = fmt.Sprintf("diskon %s/unit", FormatRupiah(diskonPerUnit))
	}
	now := NowWITA()
	item.Stok -= qty
	item.LastUpdate = now.Format("2006-01-02")
	if err := store.SetProduk(ctx, item); err != nil {
		return nil, nil, 0, err
	}
	tx := &Transaksi{
		Tanggal:     now.Format("2006-01-02"),
		Jam:         now.Format("15:04:05"),
		ProdukID:    item.ID,
		NamaProduk:  item.Nama,
		Kategori:    item.Kategori,
		Qty:         qty,
		HargaSatuan: hargaEfektif,
		Total:       qty * hargaEfektif,
		MetodeBayar: metode,
		Kasir:       kasir,
		Catatan:     catatan,
	}
	if _, err := store.AppendTransaksi(ctx, tx); err != nil {
		return nil, nil, 0, err
	}
	// Automatic learning: record the sale habit (peak hour + top product).
	_ = store.TrackHabit(ctx, "sale", item.Nama)
	return tx, item, item.Stok, nil
}
```

- [ ] **Step 4: Jalankan SEMUA test paket, pastikan tetap hijau**

Run: `go test -tags goolm,stdjson ./pkg/tools/kios/...`
Expected: PASS semua (refactor netral-perilaku — `TestPerformJual`, `TestJualMassal`, `TestPerformJualJenisBiasaRoute`, dll. lulus).

- [ ] **Step 5: Pastikan build penuh tidak rusak**

Run: `make build`
Expected: build sukses, binary `build/picoclaw` terbentuk.

- [ ] **Step 6: Commit**

```bash
git add pkg/tools/kios/tool_common.go pkg/tools/kios/kios_test.go
git commit -m "refactor(kios): performJual dispatch by jenis + sellBiasa (netral-perilaku)"
```

---

## Task 3: Backup/restore `harga_supplier` + bump versi backup

Bug: `kios:harga_supplier` (override harga per produk-suplier) tidak ikut backup → hilang saat restore / wipe Upstash. Perbaiki + naikkan `Versi` ke `"1.1"`.

**Files:**
- Modify: `pkg/tools/kios/backup.go` (struct, BuildBackup, Ringkas, HasAnyData, RestoreBackup)
- Test: `pkg/tools/kios/kios_test.go`

- [ ] **Step 1: Tulis test round-trip yang gagal**

Tambahkan di akhir `pkg/tools/kios/kios_test.go`:

```go
func TestBackupRestoreHargaSupplier(t *testing.T) {
	ctx := context.Background()
	src := newTestStore(t)
	if err := src.SetHargaSupplier(ctx, "002", "Toko Jaya", 54000); err != nil {
		t.Fatalf("set harga supplier: %v", err)
	}
	b, err := BuildBackup(ctx, src)
	if err != nil {
		t.Fatalf("build backup: %v", err)
	}
	if b.Versi != "1.1" {
		t.Errorf("versi=%q want 1.1", b.Versi)
	}
	if len(b.HargaSupplier) != 1 {
		t.Fatalf("harga_supplier in backup=%d want 1", len(b.HargaSupplier))
	}
	// Restore ke store kosong dan pastikan override pulih.
	dst := newTestStore(t)
	if err := dst.RestoreBackup(ctx, b); err != nil {
		t.Fatalf("restore: %v", err)
	}
	m, err := dst.GetAllHargaSupplier(ctx)
	if err != nil {
		t.Fatalf("get harga supplier: %v", err)
	}
	if m["002|Toko Jaya"] != 54000 {
		t.Errorf("restored harga=%d want 54000 (map=%v)", m["002|Toko Jaya"], m)
	}
}
```

- [ ] **Step 2: Jalankan test, pastikan GAGAL**

Run: `go test -tags goolm,stdjson ./pkg/tools/kios/... -run TestBackupRestoreHargaSupplier`
Expected: FAIL — `b.HargaSupplier undefined` (kompilasi) lalu, setelah field ditambah, versi `1.0`≠`1.1` / map kosong.

- [ ] **Step 3: Tambah field ke `BackupData`**

Di `pkg/tools/kios/backup.go` struct `BackupData` (setelah baris `Shift *Shift json:"shift,omitempty"`), tambahkan:

```go
	HargaSupplier map[string]int `json:"harga_supplier,omitempty"`
```

- [ ] **Step 4: Isi di `BuildBackup` + bump versi**

Di `backup.go` fungsi `BuildBackup`:
- Ganti baris `b := &BackupData{Versi: "1.0", ...}` menjadi `Versi: "1.1"`:

```go
	b := &BackupData{Versi: "1.1", Dibuat: NowWITA().Format("2006-01-02 15:04:05")}
```

- Sebelum `return b, nil`, tambahkan:

```go
	if b.HargaSupplier, err = store.GetAllHargaSupplier(ctx); err != nil {
		return nil, err
	}
```

- [ ] **Step 5: Tampilkan di `Ringkas`**

Di `backup.go` method `Ringkas`, ganti string & argumen agar menyertakan harga supplier:

```go
func (b *BackupData) Ringkas() string {
	return fmt.Sprintf("%d produk, %d transaksi, %d pembelian, %d riwayat harga, %d supplier, %d promo, %d pustaka, %d pengguna, %d harga supplier",
		len(b.Produk), len(b.Transaksi), len(b.Pembelian), len(b.PriceHistory),
		len(b.Supplier), len(b.Promo), len(b.Pustaka), len(b.Users), len(b.HargaSupplier))
}
```

- [ ] **Step 6: Pulihkan di `RestoreBackup`**

Di `backup.go` fungsi `RestoreBackup`:
- Tambahkan `keyHargaSupplier` ke slice `keys` yang di-`Del` (di akhir slice, sebelum `}`):

```go
		keySupplier, keySeqSup, keyPromo, keySeqPromo, keyPustaka, keySeqPus,
		keyHargaSupplier,
```

- Setelah blok `rpushAll(keyPriceHist, ...)` (dan sebelum blok `if b.Shift != nil {`), tambahkan:

```go
	for field, harga := range b.HargaSupplier {
		if err := s.rdb.HSet(ctx, keyHargaSupplier, field, strconv.Itoa(harga)).Err(); err != nil {
			return err
		}
	}
```

(Paket `strconv` sudah di-import di `backup.go` — dipakai `setSeq`. Bila build mengeluh, pastikan import ada.)

- [ ] **Step 7: Sertakan di `HasAnyData`**

Di `backup.go` method `HasAnyData`, tambahkan `keyHargaSupplier` ke slice HASH pertama:

```go
	for _, k := range []string{keyProduk, keySupplier, keyPromo, keyPustaka, keyUsers, keyHargaSupplier} {
```

- [ ] **Step 8: Jalankan test, pastikan LULUS**

Run: `go test -tags goolm,stdjson ./pkg/tools/kios/... -run TestBackupRestoreHargaSupplier`
Expected: PASS.

- [ ] **Step 9: Jalankan seluruh paket + mirror TS BackupData (bila ada)**

Run: `go test -tags goolm,stdjson ./pkg/tools/kios/...`
Expected: PASS semua.

Cek apakah ada mirror `BackupData` di dashboard:
Run: `grep -rn "harga_supplier\|BackupData\|interface Backup" kios-dashboard/src/lib/ || echo "tidak ada mirror — lewati"`
Bila ada interface backup di TS, tambahkan `harga_supplier?: Record<string, number>;`. Bila tidak, lewati.

- [ ] **Step 10: Commit**

```bash
git add pkg/tools/kios/backup.go pkg/tools/kios/kios_test.go
git commit -m "fix(kios): sertakan harga_supplier di backup/restore + versi backup 1.1"
```

---

## Task 4: Pecah `store.go` di bawah 500 baris (hygiene)

`store.go` ±628 baris (setelah Task 1). Pindahkan semua method data-access `*Store` dan helper package-level ke file baru `store_access.go` (paket sama `kios` → tidak ada perubahan pemanggil). Definisi tipe + konstanta key tetap di `store.go`.

**Files:**
- Create: `pkg/tools/kios/store_access.go`
- Modify: `pkg/tools/kios/store.go`

- [ ] **Step 1: Catat baseline hijau**

Run: `go test -tags goolm,stdjson ./pkg/tools/kios/...`
Expected: PASS (baseline sebelum memindah kode).

- [ ] **Step 2: Buat file baru dengan header paket**

Buat `pkg/tools/kios/store_access.go` berisi:

```go
package kios

// Data-access methods for *Store and package-level helpers. Split out of
// store.go to keep each file under the 500-line project limit. Type definitions,
// Redis key constants, and KiosConfig remain in store.go.
```

- [ ] **Step 3: Pindahkan method & helper**

Potong (cut) dari `store.go` dan tempel (paste) ke `store_access.go` SEMUA blok berikut (gunakan daftar simbol ini sebagai panduan — pindahkan definisi fungsi utuhnya, bukan tipe):

- Bagian `--- Login dashboard ---`: `CreateLoginCode` (biarkan `loginCodeTTL const` di store.go).
- `GetConfig`, `SaveConfig`, `IsAutoLearnEnabled`, `defaultConfig`.
- Konstruktor/koneksi: `NewStore`, `NewStoreWithClient`, `Ping`.
- Produk: `GetProduk`, `GetAllProduk`, `SetProduk`, `DelProduk`, `NextProdukID`, `CariProduk`.
- Transaksi: `AppendTransaksi`, `GetAllTransaksi`, `RemoveTransaksi`.
- Pembelian/PriceHistory: `AppendPembelian`, `AppendPriceHistory`, `GetAllPriceHistory`.
- Shift: `GetShift`, `SetShift`.
- Users: `GetUser`, `SetUser`, `GetAllUsers`.
- Pesanan: `GetAllPesanan`.
- Seed: `IsSeedDone`, `MarkSeedDone`, `ResetSeed`.
- Helper waktu/format: `NowWITA`, `FormatRupiah`.

TETAP di `store.go`: semua deklarasi `type ... struct`, blok `const ( keyProduk ... )`, `const loginCodeTTL`, `type Store struct`, dan `type KiosConfig struct` + komentarnya.

> Catatan: `type Store struct { rdb *redis.Client; mu sync.Mutex }` (atau serupa) TETAP di store.go karena ia definisi tipe; method-nya pindah.

- [ ] **Step 4: Rapikan import di kedua file**

Jalankan formatter+import fixer:
```bash
gofmt -w pkg/tools/kios/store.go pkg/tools/kios/store_access.go
```
Lalu build untuk menemukan import yang kurang/lebih:
```bash
make build
```
Perbaiki import di tiap file sampai build bersih (mis. `store.go` mungkin hanya butuh `time` untuk `loginCodeTTL`; `store_access.go` butuh `context`, `encoding/json`, `fmt`, `sort`, `strconv`, `strings`, `time`, `sync`, dan paket redis sesuai pemakaian). Biarkan compiler memandu: hapus import yang dilaporkan "imported and not used".

- [ ] **Step 5: Verifikasi ukuran file**

Run: `wc -l pkg/tools/kios/store.go pkg/tools/kios/store_access.go`
Expected: `store.go` < 500 baris.

- [ ] **Step 6: Jalankan seluruh test + build**

Run: `go test -tags goolm,stdjson ./pkg/tools/kios/... && make build`
Expected: PASS + build sukses (kode identik, hanya berpindah file).

- [ ] **Step 7: Commit**

```bash
git add pkg/tools/kios/store.go pkg/tools/kios/store_access.go
git commit -m "refactor(kios): pisah data-access ke store_access.go (store.go <500 baris)"
```

---

## Self-Review (sudah dijalankan saat penyusunan)

- **Spec coverage (Fase 0):** `Produk.Jenis`+helper (Task 1) ✓; `SupplierID` (Task 1) ✓; refactor `performJual` netral-perilaku (Task 2) ✓; fix backup `harga_supplier` + bump versi (Task 3) ✓; pecah file >500 (Task 4) ✓. Catatan: pemecahan `commands.go`/`stok.go` ditunda ke plan yang benar-benar menambah kode ke file itu (Plan B/C menambah file BARU, bukan menumbuhkan file ini), jadi tidak diblok di Fase 0.
- **Placeholder scan:** tidak ada TBD/TODO; setiap step Go menyertakan kode lengkap.
- **Type consistency:** `JenisOrDefault()` dipakai konsisten di Task 1 & 2; `sellBiasa` signature konsisten; `BackupData.HargaSupplier` (`map[string]int`) cocok dengan `GetAllHargaSupplier` (`store_more.go`).

---

## Catatan untuk plan berikutnya
- **Plan 1 (Pelanggan):** registry `Pelanggan` (key `kios:pelanggan`, field=no.WA) + CRUD + backup wiring + normalisasi phone — fondasi untuk piutang (Plan A) & storefront (Plan D).
- **Plan A/B/C/D** menyusul setelah Plan 1 (lihat §9 spec). Plan B menyentuh `performJual` lewat cabang switch baru yang sudah disiapkan di Task 2.
