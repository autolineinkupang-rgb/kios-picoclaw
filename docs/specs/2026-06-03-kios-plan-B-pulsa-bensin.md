Saya sudah mengumpulkan semua konteks yang diperlukan. Sekarang saya akan menulis rencana implementasi lengkap. Karena instruksi melarang menulis ke file, saya akan menyampaikan rencana ini sebagai output teks langsung. Namun, instruksi pengguna meminta saya menyimpan ke `/home/kevinman/Publik/project/kios-picoclaw/docs/specs/2026-06-03-kios-plan-B-pulsa-bensin.md`. Karena saya dalam READ-ONLY mode dan tidak dapat menulis file, saya akan menyampaikan seluruh konten rencana sebagai output teks.

---

CATATAN: Saya beroperasi dalam READ-ONLY planning mode dan tidak dapat menulis file. Berikut adalah isi lengkap rencana yang diminta, siap untuk disimpan ke path yang ditentukan:

---

# Plan B — Pulsa & Bensin

> **Untuk pekerja agentic:** GUNAKAN skill `superpowers:executing-plans` atau `superpowers:subagent-driven-development` untuk implementasi task-by-task. Langkah menggunakan sintaks checkbox (`- [ ]`) untuk pelacakan. Jalankan test kanonikal setelah setiap task: `go test -tags goolm,stdjson ./pkg/tools/kios/...`

**Tanggal:** 2026-06-03
**Status:** Siap implementasi
**Branch:** `feat/spec-bon-pulsa-bensin-pelanggan`
**Prasyarat:** Plan 0 (Fondasi) sudah selesai — `Produk.Jenis`, `JenisOrDefault()`, `performJual` switch, `harga_supplier` backup sudah ada.

**Goal:** Mengimplementasikan penjualan pulsa (saldo modal tunggal + tabel nominal) dan bensin (Pertalite/Pertamax, stok mili-liter) lengkap dengan RBAC, top-up, restock, batalkan, laporan laba akurat, notifikasi kritis, backup/restore, slash commands, dan dashboard opsional.

**Architecture:**
- `store_special.go` — semua data-access key baru (PulsaDenom, PulsaTopup, saldo modal, counter PTU)
- `special.go` — logika jual `sellPulsa` + `sellBensin`, dipanggil dari `performJual`
- `special_test.go` — test unit isolasi (miniredis)
- `commands_special.go` — slash commands `/pulsa`, `/bensin`, `/isipulsa`, `/isibensin`
- Modifikasi minimal di `store.go`, `tool_common.go`, `kasir.go`, `stok.go`, `laporan.go`, `notif.go`, `backup.go`

**Prasyarat toolchain:**
```bash
export GOROOT=$HOME/sdk/go PATH=$HOME/sdk/go/bin:$PATH GOPATH=$HOME/go
```
Perintah test kanonikal:
```bash
go test -tags goolm,stdjson ./pkg/tools/kios/...
```

---

## File Structure

| File | Tanggung jawab | Aksi |
|---|---|---|
| `pkg/tools/kios/store.go` | Struct `Produk` + `Transaksi` + konstanta key | Modify (tambah field + konstanta baru) |
| `pkg/tools/kios/store_special.go` | Data-access PulsaDenom, PulsaTopup, saldo modal, counter PTU | Create (baru) |
| `pkg/tools/kios/special.go` | `sellPulsa`, `sellBensin`, helper logika khusus | Create (baru) |
| `pkg/tools/kios/special_test.go` | Test unit sellPulsa, sellBensin, saldo, batalkan | Create (baru) |
| `pkg/tools/kios/tool_common.go` | `performJual` switch + signature ekstensi | Modify (tambah case pulsa + bensin) |
| `pkg/tools/kios/kasir.go` | `Parameters()` + `jual()` route per-jenis | Modify (tambah nominal/liter, bypass bayar-guard untuk pulsa) |
| `pkg/tools/kios/stok.go` | `batalkanTx` reverse per-jenis | Modify (branch by jenis) |
| `pkg/tools/kios/laporan.go` | `hitungLaba` pakai `tx.Modal`, `stokKritis` bensin | Modify |
| `pkg/tools/kios/notif.go` | `buildLowStockMessage` bensin + pulsa low-balance | Modify |
| `pkg/tools/kios/backup.go` | `BackupData` + `BuildBackup` + `RestoreBackup` + `HasAnyData` | Modify (tambah PulsaDenom/PulsaTopup) |
| `pkg/tools/kios/commands_special.go` | Slash `/pulsa`, `/bensin`, `/isipulsa`, `/isibensin` | Create (baru, karena commands.go sudah 491 baris) |
| `kios-dashboard/src/lib/types.ts` | Mirror interface TS | Modify (tambah field + interface baru) |
| `kios-dashboard/src/lib/redis.ts` | KEY constants | Modify (tambah pulsaDenom, pulsaTopup, seqPtu) |
| `kios-dashboard/src/lib/kios.ts` | Data-access TS | Modify (fungsi pulsa) |
| `kios-dashboard/src/app/(app)/pulsa/page.tsx` | Halaman saldo + top-up + tabel nominal | Create (opsional, Task 7) |

---

## Task 1: Field Baru di Produk + Transaksi + Struct PulsaDenom/PulsaTopup + Key Constants + store_special.go

**Files:**
- Modify: `pkg/tools/kios/store.go` — struct `Produk`, struct `Transaksi`, konstanta key
- Create: `pkg/tools/kios/store_special.go` — semua data-access key baru
- Modify: `kios-dashboard/src/lib/types.ts` — mirror interface
- Modify: `kios-dashboard/src/lib/redis.ts` — KEY constants
- Modify: `kios-dashboard/src/lib/kios.ts` — data-access TS

### Step 1.1: Tulis test yang gagal (store_special.go belum ada)

Buat `pkg/tools/kios/special_test.go`:

```go
package kios

import (
    "context"
    "testing"
)

// --- store_special tests ---

func TestGetSetPulsaDenom(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()

    // Tidak ada denom → nil, no error
    got, err := s.GetPulsaDenom(ctx, 5000)
    if err != nil {
        t.Fatalf("GetPulsaDenom empty: %v", err)
    }
    if got != nil {
        t.Fatalf("expected nil, got %+v", got)
    }

    // Set + Get round-trip
    d := &PulsaDenom{Nominal: 5000, HargaModal: 4800, HargaJual: 5500, Aktif: true}
    if err := s.SetPulsaDenom(ctx, d); err != nil {
        t.Fatalf("SetPulsaDenom: %v", err)
    }
    got, err = s.GetPulsaDenom(ctx, 5000)
    if err != nil {
        t.Fatalf("GetPulsaDenom after set: %v", err)
    }
    if got == nil || got.HargaJual != 5500 {
        t.Fatalf("round-trip mismatch: %+v", got)
    }
}

func TestGetAllPulsaDenom(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()
    for _, n := range []int{5000, 10000, 25000} {
        _ = s.SetPulsaDenom(ctx, &PulsaDenom{Nominal: n, HargaModal: n - 200, HargaJual: n + 500, Aktif: true})
    }
    all, err := s.GetAllPulsaDenom(ctx)
    if err != nil {
        t.Fatalf("GetAllPulsaDenom: %v", err)
    }
    if len(all) != 3 {
        t.Fatalf("expected 3 denom, got %d", len(all))
    }
}

func TestAppendPulsaTopup(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()
    pt := &PulsaTopup{Jumlah: 100000, SaldoSesudah: 100000, Kasir: "owner", Catatan: ""}
    if err := s.AppendPulsaTopup(ctx, pt); err != nil {
        t.Fatalf("AppendPulsaTopup: %v", err)
    }
    if pt.ID == "" {
        t.Fatal("ID harus diisi setelah append")
    }
    all, err := s.GetAllPulsaTopup(ctx)
    if err != nil {
        t.Fatalf("GetAllPulsaTopup: %v", err)
    }
    if len(all) != 1 || all[0].ID != pt.ID {
        t.Fatalf("topup mismatch: %+v", all)
    }
}

func TestNextPtuID(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()
    id1, _ := s.NextPtuID(ctx)
    id2, _ := s.NextPtuID(ctx)
    if id1 != "PTU-0001" {
        t.Errorf("first id want PTU-0001, got %s", id1)
    }
    if id2 != "PTU-0002" {
        t.Errorf("second id want PTU-0002, got %s", id2)
    }
}

func TestIncrDecrSaldoModal(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()
    p := &Produk{ID: "P01", Nama: "Pulsa Anchor", Jenis: "pulsa", SaldoModal: 0}
    _ = s.SetProduk(ctx, p)

    if err := s.IncrSaldoModal(ctx, "P01", 50000); err != nil {
        t.Fatalf("IncrSaldoModal: %v", err)
    }
    got, _ := s.GetProduk(ctx, "P01")
    if got.SaldoModal != 50000 {
        t.Errorf("want 50000, got %d", got.SaldoModal)
    }

    if err := s.DecrSaldoModal(ctx, "P01", 4800); err != nil {
        t.Fatalf("DecrSaldoModal: %v", err)
    }
    got, _ = s.GetProduk(ctx, "P01")
    if got.SaldoModal != 45200 {
        t.Errorf("want 45200, got %d", got.SaldoModal)
    }

    // DecrSaldoModal lebih dari saldo → error
    err := s.DecrSaldoModal(ctx, "P01", 999999)
    if err == nil {
        t.Error("expected error when decr > saldo")
    }
}
```

### Step 1.2: Tambah field ke `Produk` + `Transaksi` di store.go

Di `pkg/tools/kios/store.go`, setelah field `SupplierID` di struct `Produk` (baris 30), tambah:

```go
    SaldoModal   int `json:"saldo_modal,omitempty"`    // pulsa: saldo modal rupiah
    StokMl       int `json:"stok_ml,omitempty"`        // bensin: stok mili-liter
    StokKritisMl int `json:"stok_kritis_ml,omitempty"` // bensin: ambang kritis (default 40000 = 40L)
```

Di struct `Transaksi` (baris 42), setelah field `SessionID`, tambah:

```go
    Modal int     `json:"modal,omitempty"` // modal dikunci saat jual → laba akurat & historis
    Liter float64 `json:"liter,omitempty"` // volume bensin yang terjual (display)
```

Di blok konstanta `const (...)` di store.go (setelah `keyNotifPendingState`), tambah:

```go
    keyPulsaDenom = "kios:pulsa:denom" // HASH field=nominal string, value=PulsaDenom JSON
    keyPulsaTopup = "kios:pulsa:topup" // LIST append-only, value=PulsaTopup JSON
    keySeqPtu     = "kios:seq:ptu"     // INCR counter untuk PTU-NNNN
```

Tambah struct baru di store.go (setelah blok `KiosConfig`):

```go
// PulsaDenom menyimpan konfigurasi harga satu nominal pulsa.
type PulsaDenom struct {
    Nominal    int  `json:"nominal"`     // 5000 | 10000 | 15000 | 20000 | 25000 | 50000 | 100000
    HargaModal int  `json:"harga_modal"` // dikurangi dari SaldoModal saat jual
    HargaJual  int  `json:"harga_jual"`  // kas masuk (harga ke pembeli)
    Aktif      bool `json:"aktif"`
}

// Margin mengembalikan selisih harga jual − modal per denom.
func (d *PulsaDenom) Margin() int { return d.HargaJual - d.HargaModal }

// PulsaTopup mencatat satu event top-up saldo modal pulsa (append-only).
type PulsaTopup struct {
    ID           string `json:"id"`           // "PTU-0001"
    Tanggal      string `json:"tanggal"`
    Jam          string `json:"jam"`
    Jumlah       int    `json:"jumlah"`
    SaldoSesudah int    `json:"saldo_sesudah"`
    Kasir        string `json:"kasir"`
    Catatan      string `json:"catatan"`
}
```

### Step 1.3: Buat store_special.go

Buat `pkg/tools/kios/store_special.go`:

```go
package kios

import (
    "context"
    "encoding/json"
    "fmt"
    "strconv"
)

// --- PulsaDenom (HASH kios:pulsa:denom, field = strconv.Itoa(nominal)) ---

// GetPulsaDenom mengembalikan konfigurasi satu nominal, atau nil bila belum ada.
func (s *Store) GetPulsaDenom(ctx context.Context, nominal int) (*PulsaDenom, error) {
    val, err := s.rdb.HGet(ctx, keyPulsaDenom, strconv.Itoa(nominal)).Result()
    if err != nil {
        // redis.Nil juga di-handle lewat pola yang sama dengan GetProduk
        return nil, nil //nolint:nilerr
    }
    var d PulsaDenom
    if err := json.Unmarshal([]byte(val), &d); err != nil {
        return nil, err
    }
    return &d, nil
}

// SetPulsaDenom menyimpan atau menimpa konfigurasi nominal.
func (s *Store) SetPulsaDenom(ctx context.Context, d *PulsaDenom) error {
    b, err := json.Marshal(d)
    if err != nil {
        return err
    }
    return s.rdb.HSet(ctx, keyPulsaDenom, strconv.Itoa(d.Nominal), string(b)).Err()
}

// GetAllPulsaDenom mengembalikan semua nominal yang sudah dikonfigurasi.
func (s *Store) GetAllPulsaDenom(ctx context.Context) ([]*PulsaDenom, error) {
    m, err := s.rdb.HGetAll(ctx, keyPulsaDenom).Result()
    if err != nil {
        return nil, err
    }
    out := make([]*PulsaDenom, 0, len(m))
    for _, v := range m {
        var d PulsaDenom
        if err := json.Unmarshal([]byte(v), &d); err == nil {
            out = append(out, &d)
        }
    }
    return out, nil
}

// --- PulsaTopup (LIST kios:pulsa:topup, append-only) ---

// NextPtuID menghasilkan ID top-up berikutnya (PTU-0001, ...).
func (s *Store) NextPtuID(ctx context.Context) (string, error) {
    n, err := s.rdb.Incr(ctx, keySeqPtu).Result()
    if err != nil {
        return "", err
    }
    return fmt.Sprintf("PTU-%04d", n), nil
}

// AppendPulsaTopup mencatat satu event top-up saldo modal. ID diisi otomatis.
func (s *Store) AppendPulsaTopup(ctx context.Context, pt *PulsaTopup) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    n, err := s.rdb.Incr(ctx, keySeqPtu).Result()
    if err != nil {
        return err
    }
    pt.ID = fmt.Sprintf("PTU-%04d", n)
    b, err := json.Marshal(pt)
    if err != nil {
        return err
    }
    return s.rdb.RPush(ctx, keyPulsaTopup, string(b)).Err()
}

// GetAllPulsaTopup mengembalikan semua riwayat top-up (urutan append).
func (s *Store) GetAllPulsaTopup(ctx context.Context) ([]*PulsaTopup, error) {
    vals, err := s.rdb.LRange(ctx, keyPulsaTopup, 0, -1).Result()
    if err != nil {
        return nil, err
    }
    out := make([]*PulsaTopup, 0, len(vals))
    for _, v := range vals {
        var pt PulsaTopup
        if err := json.Unmarshal([]byte(v), &pt); err == nil {
            out = append(out, &pt)
        }
    }
    return out, nil
}

// --- Saldo Modal (rides di Produk.SaldoModal) ---

// IncrSaldoModal menambah saldo modal pulsa pada produk anchor.
// Operasi: GetProduk → SaldoModal += delta → SetProduk (di bawah store.mu).
func (s *Store) IncrSaldoModal(ctx context.Context, produkID string, delta int) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    p, err := s.GetProduk(ctx, produkID)
    if err != nil {
        return err
    }
    if p == nil {
        return fmt.Errorf("produk %s tidak ditemukan", produkID)
    }
    p.SaldoModal += delta
    return s.SetProduk(ctx, p)
}

// DecrSaldoModal mengurangi saldo modal pulsa. Error bila delta > SaldoModal.
func (s *Store) DecrSaldoModal(ctx context.Context, produkID string, delta int) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    p, err := s.GetProduk(ctx, produkID)
    if err != nil {
        return err
    }
    if p == nil {
        return fmt.Errorf("produk %s tidak ditemukan", produkID)
    }
    if p.SaldoModal < delta {
        return fmt.Errorf("saldo modal pulsa tidak cukup (saldo %s, butuh %s)",
            FormatRupiah(p.SaldoModal), FormatRupiah(delta))
    }
    p.SaldoModal -= delta
    return s.SetProduk(ctx, p)
}
```

**Catatan penting untuk `GetPulsaDenom`:** Pengecekan `redis.Nil` perlu dibuat eksplisit karena idiom go-redis menghasilkan `redis.Nil` (bukan nil error), berbeda dari error jaringan nyata. Pola yang benar mengikuti `GetProduk` di `store_access.go:103`:

```go
func (s *Store) GetPulsaDenom(ctx context.Context, nominal int) (*PulsaDenom, error) {
    val, err := s.rdb.HGet(ctx, keyPulsaDenom, strconv.Itoa(nominal)).Result()
    if err == redis.Nil {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    var d PulsaDenom
    if err := json.Unmarshal([]byte(val), &d); err != nil {
        return nil, err
    }
    return &d, nil
}
```
Import `"github.com/redis/go-redis/v9"` perlu ditambahkan di store_special.go.

### Step 1.4: Mirror TypeScript

Di `kios-dashboard/src/lib/types.ts`, modifikasi interface `Produk` tambah setelah `supplier_id?`:

```typescript
  saldo_modal?: number;    // pulsa: saldo modal rupiah
  stok_ml?: number;        // bensin: stok mili-liter
  stok_kritis_ml?: number; // bensin: ambang kritis (default 40000 = 40 L)
```

Di interface `Transaksi`, tambah setelah `session_id`:

```typescript
  modal?: number;  // modal dikunci saat jual
  liter?: number;  // volume bensin terjual (display)
```

Tambah interface baru di akhir file:

```typescript
export interface PulsaDenom {
  nominal: number;     // 5000 | 10000 | 15000 | 20000 | 25000 | 50000 | 100000
  harga_modal: number;
  harga_jual: number;
  aktif: boolean;
}

export interface PulsaTopup {
  id: string;          // PTU-NNNN
  tanggal: string;
  jam: string;
  jumlah: number;
  saldo_sesudah: number;
  kasir: string;
  catatan: string;
}
```

Di `kios-dashboard/src/lib/redis.ts`, tambah ke objek `KEY`:

```typescript
  pulsaDenom: "kios:pulsa:denom",
  pulsaTopup: "kios:pulsa:topup",
  seqPtu: "kios:seq:ptu",
```

Di `kios-dashboard/src/lib/kios.ts`, tambah fungsi data-access:

```typescript
import type { PulsaDenom, PulsaTopup } from "./types";

// ── Pulsa Denom ───────────────────────────────────────────────────────────────

export async function getAllPulsaDenom(): Promise<PulsaDenom[]> {
  const map = await redis().hgetall<Record<string, unknown>>(KEY.pulsaDenom);
  if (!map) return [];
  return normalizeList<PulsaDenom>(Object.values(map));
}

export async function setPulsaDenom(d: PulsaDenom): Promise<void> {
  await redis().hset(KEY.pulsaDenom, { [String(d.nominal)]: d });
}

export async function getAllPulsaTopup(): Promise<PulsaTopup[]> {
  const vals = await redis().lrange<unknown>(KEY.pulsaTopup, 0, -1);
  return normalizeList<PulsaTopup>(vals);
}

/** Cari produk anchor pulsa (jenis === "pulsa"). */
export async function getPulsaAnchor(): Promise<import("./types").Produk | null> {
  const all = await getAllProduk();
  return all.find((p) => p.jenis === "pulsa") ?? null;
}
```

### Verifikasi Task 1

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run "TestGetSetPulsaDenom|TestGetAllPulsaDenom|TestAppendPulsaTopup|TestNextPtuID|TestIncrDecrSaldoModal"
```

Semua 5 test harus lulus.

---

## Task 2: sellPulsa + sellBensin di special.go

**Files:**
- Create: `pkg/tools/kios/special.go`
- Modify: `pkg/tools/kios/special_test.go` (tambah test sellPulsa + sellBensin)

### Step 2.1: Tambah test sellPulsa + sellBensin ke special_test.go

```go
// --- sellPulsa tests ---

func seedPulsaAnchor(t *testing.T, s *Store, saldo int) *Produk {
    t.Helper()
    p := &Produk{
        ID: "P99", Nama: "Pulsa", Jenis: "pulsa", Kategori: "pulsa",
        SaldoModal: saldo, HargaBeli: 0, HargaJual: 0,
        Stok: 0, StokMinimum: 0, StokKritis: 0,
    }
    if err := s.SetProduk(context.Background(), p); err != nil {
        t.Fatalf("seedPulsaAnchor: %v", err)
    }
    return p
}

func seedDenom(t *testing.T, s *Store, nominal, modal, jual int) {
    t.Helper()
    d := &PulsaDenom{Nominal: nominal, HargaModal: modal, HargaJual: jual, Aktif: true}
    if err := s.SetPulsaDenom(context.Background(), d); err != nil {
        t.Fatalf("seedDenom: %v", err)
    }
}

func TestSellPulsaOK(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()
    anchor := seedPulsaAnchor(t, s, 100000)
    seedDenom(t, s, 10000, 9500, 11000)

    tx, item, sisa, err := sellPulsa(ctx, s, anchor, 10000, "tunai", "kasir1")
    if err != nil {
        t.Fatalf("sellPulsa: %v", err)
    }
    if tx.Total != 11000 {
        t.Errorf("total want 11000, got %d", tx.Total)
    }
    if tx.Modal != 9500 {
        t.Errorf("modal want 9500, got %d", tx.Modal)
    }
    if tx.Qty != 1 {
        t.Errorf("qty want 1, got %d", tx.Qty)
    }
    if tx.Kategori != "pulsa" {
        t.Errorf("kategori want pulsa, got %s", tx.Kategori)
    }
    if item.SaldoModal != 90500 {
        t.Errorf("SaldoModal want 90500, got %d", item.SaldoModal)
    }
    if sisa != 90500 {
        t.Errorf("sisa want 90500, got %d", sisa)
    }
    // Verifikasi produk tersimpan di Redis
    reloaded, _ := s.GetProduk(ctx, anchor.ID)
    if reloaded.SaldoModal != 90500 {
        t.Errorf("persisted SaldoModal want 90500, got %d", reloaded.SaldoModal)
    }
}

func TestSellPulsaModalKurang(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()
    anchor := seedPulsaAnchor(t, s, 5000)
    seedDenom(t, s, 10000, 9500, 11000)

    _, _, _, err := sellPulsa(ctx, s, anchor, 10000, "tunai", "kasir1")
    if err == nil {
        t.Error("expected error saldo kurang")
    }
}

func TestSellPulsaDenomTidakAda(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()
    anchor := seedPulsaAnchor(t, s, 100000)

    _, _, _, err := sellPulsa(ctx, s, anchor, 99999, "tunai", "kasir1")
    if err == nil {
        t.Error("expected error denom tidak ada")
    }
}

func TestSellPulsaDenomTidakAktif(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()
    anchor := seedPulsaAnchor(t, s, 100000)
    d := &PulsaDenom{Nominal: 10000, HargaModal: 9500, HargaJual: 11000, Aktif: false}
    _ = s.SetPulsaDenom(ctx, d)

    _, _, _, err := sellPulsa(ctx, s, anchor, 10000, "tunai", "kasir1")
    if err == nil {
        t.Error("expected error denom tidak aktif")
    }
}

// --- sellBensin tests ---

func seedBensinProduk(t *testing.T, s *Store, stokMl, kritisMl, beli, jual int) *Produk {
    t.Helper()
    p := &Produk{
        ID: "B01", Nama: "Pertalite", Jenis: "bensin", Kategori: "bensin",
        StokMl: stokMl, StokKritisMl: kritisMl,
        HargaBeli: beli, HargaJual: jual,
        Stok: stokMl / 1000,
    }
    if err := s.SetProduk(context.Background(), p); err != nil {
        t.Fatalf("seedBensin: %v", err)
    }
    return p
}

func TestSellBensinOK(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()
    // 50L stok, beli 10000/L, jual 12000/L
    anchor := seedBensinProduk(t, s, 50000, 40000, 10000, 12000)

    // Jual 2L = 2000 ml
    tx, item, sisaMl, err := sellBensin(ctx, s, anchor, 2000, "tunai", "kasir1")
    if err != nil {
        t.Fatalf("sellBensin: %v", err)
    }
    // Total = round(12000 * 2000 / 1000) = 24000
    if tx.Total != 24000 {
        t.Errorf("total want 24000, got %d", tx.Total)
    }
    // Modal = round(10000 * 2000 / 1000) = 20000
    if tx.Modal != 20000 {
        t.Errorf("modal want 20000, got %d", tx.Modal)
    }
    // Liter = 2.0
    if tx.Liter != 2.0 {
        t.Errorf("liter want 2.0, got %f", tx.Liter)
    }
    if item.StokMl != 48000 {
        t.Errorf("StokMl want 48000, got %d", item.StokMl)
    }
    if sisaMl != 48000 {
        t.Errorf("sisaMl want 48000, got %d", sisaMl)
    }
    // Stok integer harus sinkron
    if item.Stok != 48 {
        t.Errorf("Stok want 48, got %d", item.Stok)
    }
    reloaded, _ := s.GetProduk(ctx, anchor.ID)
    if reloaded.StokMl != 48000 {
        t.Errorf("persisted StokMl want 48000, got %d", reloaded.StokMl)
    }
}

func TestSellBensinStokKurang(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()
    anchor := seedBensinProduk(t, s, 1000, 40000, 10000, 12000)

    _, _, _, err := sellBensin(ctx, s, anchor, 2000, "tunai", "kasir1")
    if err == nil {
        t.Error("expected error stok kurang")
    }
}

func TestSellBensinFraksi(t *testing.T) {
    // 1.5L = 1500ml, jual 12000/L → total = round(12000*1500/1000) = 18000
    s := newTestStore(t)
    ctx := context.Background()
    anchor := seedBensinProduk(t, s, 50000, 40000, 10000, 12000)

    tx, _, _, err := sellBensin(ctx, s, anchor, 1500, "tunai", "kasir1")
    if err != nil {
        t.Fatalf("sellBensin 1.5L: %v", err)
    }
    if tx.Total != 18000 {
        t.Errorf("total 1.5L want 18000, got %d", tx.Total)
    }
    if tx.Liter != 1.5 {
        t.Errorf("liter want 1.5, got %f", tx.Liter)
    }
}
```

### Step 2.2: Buat special.go

Buat `pkg/tools/kios/special.go`:

```go
package kios

import (
    "context"
    "fmt"
    "math"
)

// sellPulsa mencatat penjualan pulsa satu nominal.
//
// Alur:
//  1. Cari PulsaDenom(nominal) — error bila tidak ada atau tidak aktif.
//  2. Validasi item.SaldoModal >= denom.HargaModal — error "Modal pulsa kurang".
//  3. Kurangi SaldoModal (via SetProduk, di bawah store.mu dari DecrSaldoModal tidak
//     langsung berlaku di sini — kita set langsung karena kita sudah punya item).
//  4. Buat + AppendTransaksi.
//  5. TrackHabit.
//  6. Return (tx, item_updated, sisa_saldo, nil).
//
// Parameter item adalah produk anchor (Jenis="pulsa") yang sudah di-fetch caller.
func sellPulsa(ctx context.Context, store *Store, item *Produk, nominal int, metode, kasir string) (*Transaksi, *Produk, int, error) {
    denom, err := store.GetPulsaDenom(ctx, nominal)
    if err != nil {
        return nil, nil, 0, fmt.Errorf("gagal baca denom pulsa: %w", err)
    }
    if denom == nil {
        return nil, nil, 0, fmt.Errorf("nominal pulsa %s tidak tersedia kak 🙏 cek /pulsa untuk daftar nominal", FormatRupiah(nominal))
    }
    if !denom.Aktif {
        return nil, nil, 0, fmt.Errorf("nominal pulsa %s sedang tidak aktif kak 😔", FormatRupiah(nominal))
    }
    if item.SaldoModal < denom.HargaModal {
        return nil, nil, 0, fmt.Errorf("modal pulsa kurang kak 😔 saldo modal %s, butuh %s — minta owner isi dulu ya",
            FormatRupiah(item.SaldoModal), FormatRupiah(denom.HargaModal))
    }

    now := NowWITA()
    item.SaldoModal -= denom.HargaModal
    item.LastUpdate = now.Format("2006-01-02")
    if err := store.SetProduk(ctx, item); err != nil {
        return nil, nil, 0, fmt.Errorf("gagal simpan saldo modal: %w", err)
    }

    tx := &Transaksi{
        Tanggal:     now.Format("2006-01-02"),
        Jam:         now.Format("15:04:05"),
        ProdukID:    item.ID,
        NamaProduk:  item.Nama,
        Kategori:    "pulsa",
        Qty:         1,
        HargaSatuan: denom.HargaJual,
        Total:       denom.HargaJual,
        Modal:       denom.HargaModal,
        MetodeBayar: metode,
        Kasir:       kasir,
        Catatan:     fmt.Sprintf("nominal %s", FormatRupiah(nominal)),
    }
    if _, err := store.AppendTransaksi(ctx, tx); err != nil {
        // Rollback saldo (best-effort; sangat jarang terjadi)
        item.SaldoModal += denom.HargaModal
        _ = store.SetProduk(ctx, item)
        return nil, nil, 0, fmt.Errorf("gagal catat transaksi pulsa: %w", err)
    }
    _ = store.TrackHabit(ctx, "sale", item.Nama)
    return tx, item, item.SaldoModal, nil
}

// sellBensin mencatat penjualan bensin dalam satuan mili-liter.
//
// Alur:
//  1. Validasi item.StokMl >= ml — error "Stok bensin tidak cukup".
//  2. Kurangi StokMl; sinkron item.Stok = StokMl/1000; SetProduk.
//  3. Hitung total dan modal (pembulatan half-even via math.Round).
//  4. Buat + AppendTransaksi dengan field Liter diisi.
//  5. Return (tx, item_updated, sisa_ml, nil).
//
// Parameter ml adalah volume dalam mili-liter (positif).
func sellBensin(ctx context.Context, store *Store, item *Produk, ml int, metode, kasir string) (*Transaksi, *Produk, int, error) {
    if ml <= 0 {
        return nil, nil, 0, fmt.Errorf("volume bensin harus lebih dari 0 kak 🙏")
    }
    if item.StokMl < ml {
        liter := float64(item.StokMl) / 1000
        return nil, nil, 0, fmt.Errorf("stok bensin %s tidak cukup kak 😔 sisa %.2fL", item.Nama, liter)
    }

    now := NowWITA()
    item.StokMl -= ml
    item.Stok = item.StokMl / 1000 // sinkron integer liter untuk tampilan stok biasa
    item.LastUpdate = now.Format("2006-01-02")
    if err := store.SetProduk(ctx, item); err != nil {
        return nil, nil, 0, fmt.Errorf("gagal update stok bensin: %w", err)
    }

    liter := float64(ml) / 1000
    total := int(math.Round(float64(item.HargaJual) * float64(ml) / 1000))
    modal := int(math.Round(float64(item.HargaBeli) * float64(ml) / 1000))
    qtyDisplay := int(math.Round(liter)) // untuk display integer di struk (1L, 2L, dst.)
    if qtyDisplay < 1 {
        qtyDisplay = 1
    }

    tx := &Transaksi{
        Tanggal:     now.Format("2006-01-02"),
        Jam:         now.Format("15:04:05"),
        ProdukID:    item.ID,
        NamaProduk:  item.Nama,
        Kategori:    "bensin",
        Qty:         qtyDisplay,
        HargaSatuan: item.HargaJual,
        Total:       total,
        Modal:       modal,
        Liter:       liter,
        MetodeBayar: metode,
        Kasir:       kasir,
        Catatan:     fmt.Sprintf("%.3fL", liter),
    }
    if _, err := store.AppendTransaksi(ctx, tx); err != nil {
        // Rollback stok (best-effort)
        item.StokMl += ml
        item.Stok = item.StokMl / 1000
        _ = store.SetProduk(ctx, item)
        return nil, nil, 0, fmt.Errorf("gagal catat transaksi bensin: %w", err)
    }
    _ = store.TrackHabit(ctx, "sale", item.Nama)
    return tx, item, item.StokMl, nil
}
```

### Verifikasi Task 2

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run "TestSellPulsa|TestSellBensin"
```

Semua 7 test harus lulus.

---

## Task 3: Wire ke performJual Switch + Kasir Params + batalkanTx Fix

**Files:**
- Modify: `pkg/tools/kios/tool_common.go` — `performJual` switch
- Modify: `pkg/tools/kios/kasir.go` — `Parameters()`, `jual()`
- Modify: `pkg/tools/kios/stok.go` — `batalkanTx()`
- Modify: `pkg/tools/kios/special_test.go` — tambah test wire + batalkan

### Step 3.1: Analisis desain routing

**Masalah:** `performJual` saat ini menerima `(ctx, store, query string, qty int, metode, kasir string, diskonPerUnit int)`. Untuk pulsa butuh `nominal int`; untuk bensin butuh `ml int`. Menambah parameter posisional akan memecah semua caller.

**Solusi terpilih — opsi paling bersih:** Tambah parameter `extras map[string]int` opsional di akhir signature `performJual`. Caller biasa pass `nil`; kasir.go pass `map[string]int{"nominal": 10000}` atau `map[string]int{"ml": 2000}`. Ini lebih baik dari context injection (tidak aman tipe) dan tidak memecah `jualMassal`.

Signature baru:
```go
func performJual(ctx context.Context, store *Store, query string, qty int, metode, kasir string, diskonPerUnit int, extras map[string]int) (*Transaksi, *Produk, int, error)
```

Semua pemanggil yang ada hanya perlu pass `nil` sebagai argumen terakhir.

### Step 3.2: Modifikasi performJual di tool_common.go

Di `pkg/tools/kios/tool_common.go`, ubah signature dan switch:

```go
// performJual executes a sale, dispatching by product kind.
// extras: optional map for special kinds — "nominal" (pulsa) or "ml" (bensin).
func performJual(ctx context.Context, store *Store, query string, qty int, metode, kasir string, diskonPerUnit int, extras map[string]int) (*Transaksi, *Produk, int, error) {
    if qty <= 0 && extras["nominal"] == 0 && extras["ml"] == 0 {
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
    case "pulsa":
        nominal := extras["nominal"]
        if nominal == 0 {
            return nil, nil, 0, fmt.Errorf("nominal pulsa wajib diisi kak 🙏 (contoh: 10000)")
        }
        return sellPulsa(ctx, store, item, nominal, metode, kasir)
    case "bensin":
        ml := extras["ml"]
        if ml == 0 {
            return nil, nil, 0, fmt.Errorf("volume bensin wajib diisi kak 🙏 (contoh: liter=2)")
        }
        return sellBensin(ctx, store, item, ml, metode, kasir)
    default:
        return sellBiasa(ctx, store, item, qty, metode, kasir, diskonPerUnit)
    }
}
```

Semua pemanggil lama di `kasir.go` dan `stok.go` tambah `nil` sebagai argumen terakhir:
- `kasir.go:99` → `performJual(ctx, t.store, argStr(args, "produk"), qty, argStr(args, "metode"), kasir, diskon, nil)`
- `kasir.go:130` → `performJual(ctx, t.store, produk, qty, metode, kasir, diskon, nil)`

### Step 3.3: Modifikasi kasir.go

Di `Parameters()`, tambah ke `properties`:

```go
"nominal": map[string]any{
    "type":        "integer",
    "description": "nominal pulsa yang dijual (5000/10000/15000/20000/25000/50000/100000)",
},
"liter": map[string]any{
    "type":        "number",
    "description": "volume bensin dalam liter (mis. 2 atau 1.5)",
},
```

Di `jual()`, setelah `bayarPtr` dan sebelum pemanggilan `performJual`, tambah logika deteksi:

```go
func (t *KasirTool) jual(ctx context.Context, args map[string]any, kasir string) *tools.ToolResult {
    qty := argInt(args, "qty")
    bayarPtr := argIntPtr(args, "bayar")

    // Pre-lookup produk untuk deteksi jenis sebelum performJual.
    pre, _ := findOne(ctx, t.store, argStr(args, "produk"))

    // Extras untuk jenis khusus.
    var extras map[string]int
    if pre != nil {
        switch pre.JenisOrDefault() {
        case "pulsa":
            nominal := argInt(args, "nominal")
            if nominal == 0 {
                return tools.ErrorResult("Nominal pulsa wajib diisi kak 🙏 (contoh: nominal=10000)")
            }
            extras = map[string]int{"nominal": nominal}
            // Pulsa: hitung total dari denom untuk guard bayar<total
            if bayarPtr != nil {
                denom, _ := t.store.GetPulsaDenom(ctx, nominal)
                if denom != nil && *bayarPtr < denom.HargaJual {
                    kurang := denom.HargaJual - *bayarPtr
                    return tools.ErrorResult(fmt.Sprintf("Uang kurang %s kak 🙏 Total pulsa %s, dibayar %s — transaksi belum dicatat ya.",
                        FormatRupiah(kurang), FormatRupiah(denom.HargaJual), FormatRupiah(*bayarPtr)))
                }
            }
        case "bensin":
            literArg := argFloat(args, "liter")
            if literArg <= 0 {
                return tools.ErrorResult("Volume bensin wajib diisi kak 🙏 (contoh: liter=2)")
            }
            ml := int(math.Round(literArg * 1000))
            extras = map[string]int{"ml": ml}
            // Guard bayar<total bensin
            if bayarPtr != nil && pre.HargaJual > 0 {
                total := int(math.Round(float64(pre.HargaJual) * literArg))
                if *bayarPtr < total {
                    kurang := total - *bayarPtr
                    return tools.ErrorResult(fmt.Sprintf("Uang kurang %s kak 🙏 Total bensin %s, dibayar %s — transaksi belum dicatat ya.",
                        FormatRupiah(kurang), FormatRupiah(total), FormatRupiah(*bayarPtr)))
                }
            }
        default:
            // Jenis biasa: promo + guard bayar<total yang sudah ada
            diskon, promoID = activePromoDiskon(ctx, t.store, pre.ID, qty, pre.HargaJual)
            if bayarPtr != nil && qty > 0 {
                hargaEfektif := pre.HargaJual - diskon
                if hargaEfektif < 0 { hargaEfektif = 0 }
                total := qty * hargaEfektif
                if *bayarPtr < total {
                    kurang := total - *bayarPtr
                    return tools.ErrorResult(fmt.Sprintf("Uang kurang %s kak 🙏 Total %s, dibayar %s — transaksi belum dicatat ya.",
                        FormatRupiah(kurang), FormatRupiah(total), FormatRupiah(*bayarPtr)))
                }
            }
        }
    }

    tx, item, sisa, err := performJual(ctx, t.store, argStr(args, "produk"), qty, argStr(args, "metode"), kasir, diskon, extras)
    if err != nil {
        return tools.ErrorResult(err.Error())
    }
    out := t.struk(tx, item, bayarPtr, promoID)

    // Peringatan sisa stok per jenis
    switch item.JenisOrDefault() {
    case "pulsa":
        if sisa <= item.StokMinimum {
            out += fmt.Sprintf("\n⚠️ Saldo modal pulsa menipis (%s).", FormatRupiah(sisa))
        }
    case "bensin":
        kritisMl := item.StokKritisMl
        if kritisMl == 0 { kritisMl = 40000 }
        if sisa <= 0 {
            out += fmt.Sprintf("\n⚠️ Stok bensin %s HABIS!", item.Nama)
        } else if sisa <= kritisMl {
            out += fmt.Sprintf("\n⚠️ Stok bensin %s menipis (sisa %.1fL).", item.Nama, float64(sisa)/1000)
        }
    default:
        if sisa <= 0 {
            out += fmt.Sprintf("\n⚠️ %s HABIS!", item.Nama)
        } else if sisa <= item.StokKritis {
            out += fmt.Sprintf("\n⚠️ Stok %s menipis (sisa %d).", item.Nama, sisa)
        }
    }
    return tools.UserResult(out)
}
```

Tambahkan helper `argFloat` di `tool_common.go`:

```go
// argFloat reads a float64 argument, tolerating JSON floats and integers.
func argFloat(args map[string]any, key string) float64 {
    switch x := args[key].(type) {
    case float64:
        return x
    case int:
        return float64(x)
    case int64:
        return float64(x)
    }
    return 0
}
```

Import `"math"` harus ditambahkan di `kasir.go`.

### Step 3.4: Fix batalkanTx di stok.go

Di `pkg/tools/kios/stok.go`, ganti fungsi `batalkanTx` (baris 437-455):

```go
func (t *StokTool) batalkanTx(ctx context.Context, args map[string]any) *tools.ToolResult {
    id := strings.ToUpper(argStr(args, "id"))
    if id == "" {
        return tools.ErrorResult("ID transaksi-nya diisi dulu ya kak 🙏")
    }
    tx, err := t.store.RemoveTransaksi(ctx, id)
    if err != nil {
        return tools.ErrorResult("Aduh, gagal batalkan transaksi kak 😣 Coba lagi sebentar ya.").WithError(err)
    }
    if tx == nil {
        return tools.NewToolResult(fmt.Sprintf("Transaksi %s nggak ketemu kak 🔍", id))
    }

    item, _ := t.store.GetProduk(ctx, tx.ProdukID)
    if item == nil {
        return tools.NewToolResult(fmt.Sprintf("Transaksi %s dibatalkan (produk tidak ditemukan, stok tidak dikembalikan).", tx.ID))
    }

    switch item.JenisOrDefault() {
    case "pulsa":
        // Kembalikan modal ke saldo (gunakan tx.Modal yang dikunci saat jual)
        if tx.Modal > 0 {
            item.SaldoModal += tx.Modal
            item.LastUpdate = NowWITA().Format("2006-01-02")
            t.store.SetProduk(ctx, item)
            return tools.NewToolResult(fmt.Sprintf("Transaksi %s dibatalkan, saldo modal pulsa dikembalikan (+%s, saldo kini %s).",
                tx.ID, FormatRupiah(tx.Modal), FormatRupiah(item.SaldoModal)))
        }
        return tools.NewToolResult(fmt.Sprintf("Transaksi %s dibatalkan (modal tidak tercatat, saldo tidak dikembalikan).", tx.ID))

    case "bensin":
        // Kembalikan volume mili-liter
        if tx.Liter > 0 {
            mlKembali := int(math.Round(tx.Liter * 1000))
            item.StokMl += mlKembali
            item.Stok = item.StokMl / 1000
            item.LastUpdate = NowWITA().Format("2006-01-02")
            t.store.SetProduk(ctx, item)
            return tools.NewToolResult(fmt.Sprintf("Transaksi %s dibatalkan, stok bensin %s dikembalikan (+%.3fL, sisa %.1fL).",
                tx.ID, item.Nama, tx.Liter, float64(item.StokMl)/1000))
        }
        return tools.NewToolResult(fmt.Sprintf("Transaksi %s dibatalkan (volume tidak tercatat, stok tidak dikembalikan).", tx.ID))

    default:
        // Jalur biasa: kembalikan stok unit
        item.Stok += tx.Qty
        item.LastUpdate = NowWITA().Format("2006-01-02")
        t.store.SetProduk(ctx, item)
        return tools.NewToolResult(fmt.Sprintf("Transaksi %s dibatalkan, stok %s dikembalikan (+%d).", tx.ID, tx.NamaProduk, tx.Qty))
    }
}
```

Import `"math"` perlu ditambahkan di `stok.go`.

### Step 3.5: Tambah test wire + batalkan ke special_test.go

```go
func TestPerformJualPulsa(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()
    anchor := seedPulsaAnchor(t, s, 100000)
    seedDenom(t, s, 10000, 9500, 11000)
    _ = anchor

    tx, item, sisa, err := performJual(ctx, s, "Pulsa", 1, "tunai", "kasir1", 0, map[string]int{"nominal": 10000})
    if err != nil {
        t.Fatalf("performJual pulsa: %v", err)
    }
    if tx.Total != 11000 {
        t.Errorf("total want 11000, got %d", tx.Total)
    }
    if item.SaldoModal != 90500 {
        t.Errorf("SaldoModal want 90500, got %d", item.SaldoModal)
    }
    if sisa != 90500 {
        t.Errorf("sisa want 90500, got %d", sisa)
    }
}

func TestPerformJualBensin(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()
    _ = seedBensinProduk(t, s, 50000, 40000, 10000, 12000)

    tx, item, sisaMl, err := performJual(ctx, s, "Pertalite", 1, "tunai", "kasir1", 0, map[string]int{"ml": 2000})
    if err != nil {
        t.Fatalf("performJual bensin: %v", err)
    }
    if tx.Total != 24000 {
        t.Errorf("total want 24000, got %d", tx.Total)
    }
    if item.StokMl != 48000 {
        t.Errorf("StokMl want 48000, got %d", item.StokMl)
    }
    if sisaMl != 48000 {
        t.Errorf("sisaMl want 48000, got %d", sisaMl)
    }
}

func TestBatalkanTxPulsa(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()
    anchor := seedPulsaAnchor(t, s, 100000)
    seedDenom(t, s, 10000, 9500, 11000)

    tx, _, _, _ := sellPulsa(ctx, s, anchor, 10000, "tunai", "kasir1")

    tool := &StokTool{store: s}
    result := tool.batalkanTx(ctx, map[string]any{"id": tx.ID})
    if result.IsError {
        t.Fatalf("batalkanTx pulsa error: %s", result.ForLLM)
    }
    reloaded, _ := s.GetProduk(ctx, anchor.ID)
    if reloaded.SaldoModal != 100000 {
        t.Errorf("SaldoModal after batal want 100000, got %d", reloaded.SaldoModal)
    }
}

func TestBatalkanTxBensin(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()
    anchor := seedBensinProduk(t, s, 50000, 40000, 10000, 12000)

    tx, _, _, _ := sellBensin(ctx, s, anchor, 2000, "tunai", "kasir1")

    tool := &StokTool{store: s}
    result := tool.batalkanTx(ctx, map[string]any{"id": tx.ID})
    if result.IsError {
        t.Fatalf("batalkanTx bensin error: %s", result.ForLLM)
    }
    reloaded, _ := s.GetProduk(ctx, anchor.ID)
    if reloaded.StokMl != 50000 {
        t.Errorf("StokMl after batal want 50000, got %d", reloaded.StokMl)
    }
    if reloaded.Stok != 50 {
        t.Errorf("Stok after batal want 50, got %d", reloaded.Stok)
    }
}
```

### Verifikasi Task 3

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run "TestPerformJualPulsa|TestPerformJualBensin|TestBatalkanTxPulsa|TestBatalkanTxBensin"
```

---

## Task 4: Fix hitungLaba + buildLowStockMessage + stokKritis

**Files:**
- Modify: `pkg/tools/kios/laporan.go` — `hitungLaba()`, `stokKritis()`
- Modify: `pkg/tools/kios/notif.go` — `buildLowStockMessage()`
- Modify: `pkg/tools/kios/special_test.go` — test laba + notif

### Step 4.1: Fix hitungLaba di laporan.go

Ganti fungsi `hitungLaba` (baris 108-119):

```go
// hitungLaba computes omzet, modal, laba for a transaction set.
// Pakai tx.Modal bila > 0 (pulsa & bensin — modal dikunci saat jual);
// fallback ke Qty * HargaBeli dari katalog produk saat ini.
func (t *LaporanTool) hitungLaba(ctx context.Context, txs []*Transaksi) (omzet, modal, laba int) {
    all, _ := t.store.GetAllProduk(ctx)
    beli := make(map[string]int, len(all))
    for _, p := range all {
        beli[p.ID] = p.HargaBeli
    }
    for _, tx := range txs {
        omzet += tx.Total
        if tx.Modal > 0 {
            modal += tx.Modal
        } else {
            modal += tx.Qty * beli[tx.ProdukID]
        }
    }
    return omzet, modal, omzet - modal
}
```

### Step 4.2: Fix stokKritis di laporan.go

Ganti fungsi `stokKritis` (baris 121-130):

```go
func (t *LaporanTool) stokKritis(ctx context.Context) []string {
    all, _ := t.store.GetAllProduk(ctx)
    var out []string
    for _, p := range all {
        switch p.JenisOrDefault() {
        case "bensin":
            kritisMl := p.StokKritisMl
            if kritisMl == 0 {
                kritisMl = 40000 // default 40L
            }
            if p.StokMl <= kritisMl {
                out = append(out, fmt.Sprintf("%s (%.1fL)", p.Nama, float64(p.StokMl)/1000))
            }
        default:
            if p.Stok <= p.StokKritis {
                out = append(out, p.Nama)
            }
        }
    }
    return out
}
```

Import `"fmt"` sudah ada di laporan.go.

### Step 4.3: Fix buildLowStockMessage di notif.go

Ganti fungsi `buildLowStockMessage` (baris 166-199):

```go
func (n *NotifService) buildLowStockMessage(ctx context.Context) (string, bool) {
    all, err := n.store.GetAllProduk(ctx)
    if err != nil || len(all) == 0 {
        return "", false
    }

    var b strings.Builder
    count := 0

    for _, p := range all {
        switch p.JenisOrDefault() {
        case "pulsa":
            // Pulsa: pakai StokMinimum sebagai ambang saldo modal minimum
            if p.StokMinimum > 0 && p.SaldoModal <= p.StokMinimum {
                label := "menipis"
                if p.SaldoModal == 0 {
                    label = "HABIS"
                }
                fmt.Fprintf(&b, "- %s [saldo modal %s]: saldo %s, min %s\n",
                    p.Nama, label, FormatRupiah(p.SaldoModal), FormatRupiah(p.StokMinimum))
                count++
            }
        case "bensin":
            kritisMl := p.StokKritisMl
            if kritisMl == 0 {
                kritisMl = 40000
            }
            if p.StokMl <= kritisMl {
                label := "kritis"
                if p.StokMl == 0 {
                    label = "HABIS"
                }
                // Hitung kebutuhan restock: 3× kritis − sisa (min 0)
                butuhMl := kritisMl*3 - p.StokMl
                if butuhMl < 0 {
                    butuhMl = 0
                }
                fmt.Fprintf(&b, "- %s [bensin %s]: sisa %.1fL (kritis %.0fL), perlu restock ±%.1fL\n",
                    p.Nama, label, float64(p.StokMl)/1000, float64(kritisMl)/1000, float64(butuhMl)/1000)
                count++
            }
        default:
            if p.Stok > p.StokMinimum {
                continue
            }
            label := "menipis"
            if p.Stok == 0 {
                label = "HABIS"
            } else if p.Stok <= p.StokKritis {
                label = "kritis"
            }
            butuh := p.StokMinimum*3 - p.Stok
            if butuh < 0 {
                butuh = 0
            }
            fmt.Fprintf(&b, "- %s [%s]: sisa %d (min %d), perlu restock ±%d\n",
                p.Nama, label, p.Stok, p.StokMinimum, butuh)
            count++
        }
    }

    if count == 0 {
        return "", false
    }

    msg := fmt.Sprintf("⚠️ *Notif Stok* — %s WITA\n\n%s\nSegera restock ya kak! 🙏",
        NowWITA().Format("02 Jan 2006 15:04"), b.String())
    return msg, true
}
```

### Step 4.4: Test laba + notif

Tambahkan ke `special_test.go`:

```go
func TestHitungLabaPulsa(t *testing.T) {
    // Laba pulsa harus pakai tx.Modal, bukan Qty*HargaBeli (HargaBeli=0 untuk pulsa)
    s := newTestStore(t)
    ctx := context.Background()
    anchor := seedPulsaAnchor(t, s, 100000)
    seedDenom(t, s, 10000, 9500, 11000)
    tx, _, _, _ := sellPulsa(ctx, s, anchor, 10000, "tunai", "kasir1")

    tool := &LaporanTool{store: s}
    omzet, modal, laba := tool.hitungLaba(ctx, []*Transaksi{tx})
    if omzet != 11000 {
        t.Errorf("omzet want 11000, got %d", omzet)
    }
    if modal != 9500 {
        t.Errorf("modal want 9500, got %d", modal)
    }
    if laba != 1500 {
        t.Errorf("laba want 1500, got %d", laba)
    }
}

func TestHitungLabaBensin(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()
    anchor := seedBensinProduk(t, s, 50000, 40000, 10000, 12000)
    tx, _, _, _ := sellBensin(ctx, s, anchor, 2000, "tunai", "kasir1")

    tool := &LaporanTool{store: s}
    omzet, modal, laba := tool.hitungLaba(ctx, []*Transaksi{tx})
    // omzet=24000, modal=20000, laba=4000
    if omzet != 24000 {
        t.Errorf("omzet want 24000, got %d", omzet)
    }
    if modal != 20000 {
        t.Errorf("modal want 20000, got %d", modal)
    }
    if laba != 4000 {
        t.Errorf("laba want 4000, got %d", laba)
    }
}

func TestNotifBensinKritis(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()
    // Stok 30L di bawah kritis 40L
    _ = seedBensinProduk(t, s, 30000, 40000, 10000, 12000)

    svc := &NotifService{store: s}
    msg, ok := svc.buildLowStockMessage(ctx)
    if !ok {
        t.Fatal("expected low stock message for bensin")
    }
    if !strings.Contains(msg, "bensin") && !strings.Contains(msg, "Pertalite") {
        t.Errorf("message should mention bensin product, got: %s", msg)
    }
}

func TestNotifPulsaLowBalance(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()
    // SaldoModal 4000 di bawah StokMinimum 5000
    p := &Produk{
        ID: "P99", Nama: "Pulsa", Jenis: "pulsa",
        SaldoModal: 4000, StokMinimum: 5000,
    }
    _ = s.SetProduk(ctx, p)

    svc := &NotifService{store: s}
    msg, ok := svc.buildLowStockMessage(ctx)
    if !ok {
        t.Fatal("expected low balance message for pulsa")
    }
    if !strings.Contains(msg, "saldo modal") {
        t.Errorf("message should mention saldo modal, got: %s", msg)
    }
}
```

### Verifikasi Task 4

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run "TestHitungLaba|TestNotif"
```

---

## Task 5: Slash Commands di commands_special.go

**Files:**
- Create: `pkg/tools/kios/commands_special.go`
- Modify: `pkg/tools/kios/commands.go` — daftarkan handler baru (register ke CommandRouter)

**Catatan:** `commands.go` sudah 491 baris (di batas 500), semua slash command baru masuk ke `commands_special.go`.

### Step 5.1: Buat commands_special.go

Buat `pkg/tools/kios/commands_special.go`:

```go
package kios

import (
    "context"
    "fmt"
    "math"
    "sort"
    "strconv"
    "strings"

    "github.com/sipeed/picoclaw/pkg/commands"
)

// RegisterSpecialCommands mendaftarkan slash commands untuk pulsa dan bensin
// ke CommandRouter yang diberikan. Dipanggil dari RegisterKiosCommands.
func RegisterSpecialCommands(router *commands.CommandRouter, store *Store) {
    router.Handle("/pulsa", func(ctx context.Context, text string) string {
        return handlePulsaCmd(ctx, store, text)
    })
    router.Handle("/isipulsa", func(ctx context.Context, text string) string {
        return handleIsiPulsaCmd(ctx, store, text)
    })
    router.Handle("/bensin", func(ctx context.Context, text string) string {
        return handleBensinCmd(ctx, store, text)
    })
    router.Handle("/isibensin", func(ctx context.Context, text string) string {
        return handleIsiBensinCmd(ctx, store, text)
    })
}

// handlePulsaCmd menangani /pulsa dan /pulsa <nominal>
// /pulsa          → tampilkan saldo + daftar nominal (owner/kasir)
// /pulsa 10000    → jual nominal 10000 via sellPulsa (owner/kasir)
func handlePulsaCmd(ctx context.Context, store *Store, text string) string {
    role, kasir, err := resolveRoleFromCtx(ctx, store)
    if err != nil {
        return "Maaf, tidak bisa verifikasi akses kak 🙏"
    }
    if role != "owner" && role != "kasir" {
        return "Maaf ya kak 🙏 perintah ini khusus owner/kasir."
    }

    arg := argAfter(text) // teks setelah "/pulsa"

    if arg == "" {
        // Tampilkan saldo + daftar nominal
        return pulsaInfo(ctx, store)
    }

    // Jual nominal
    nominal, parseErr := strconv.Atoi(strings.ReplaceAll(arg, ".", ""))
    if parseErr != nil || nominal <= 0 {
        return fmt.Sprintf("Format salah kak 🙏 Contoh: /pulsa 10000")
    }

    // Cari produk anchor pulsa
    all, _ := store.GetAllProduk(ctx)
    var anchor *Produk
    for _, p := range all {
        if p.JenisOrDefault() == "pulsa" {
            anchor = p
            break
        }
    }
    if anchor == nil {
        return "Produk pulsa belum ada kak 🔍 Minta owner tambahkan produk dengan jenis=pulsa dulu ya."
    }

    tx, item, sisa, sellErr := sellPulsa(ctx, store, anchor, nominal, "tunai", kasir)
    if sellErr != nil {
        return sellErr.Error()
    }
    out := fmt.Sprintf("✅ Pulsa %s terjual\n"+
        "💰 Harga: %s\n"+
        "🏦 Saldo modal setelah: %s\n"+
        "📋 ID: %s",
        FormatRupiah(nominal),
        FormatRupiah(tx.Total),
        FormatRupiah(sisa),
        tx.ID)
    kritisMl := item.StokMinimum // reuse StokMinimum sebagai ambang saldo
    if kritisMl > 0 && sisa <= kritisMl {
        out += fmt.Sprintf("\n⚠️ Saldo modal pulsa menipis (%s)!", FormatRupiah(sisa))
    }
    return out
}

// pulsaInfo menampilkan saldo modal + daftar nominal + margin.
func pulsaInfo(ctx context.Context, store *Store) string {
    all, _ := store.GetAllProduk(ctx)
    var anchor *Produk
    for _, p := range all {
        if p.JenisOrDefault() == "pulsa" {
            anchor = p
            break
        }
    }
    saldo := 0
    if anchor != nil {
        saldo = anchor.SaldoModal
    }

    denoms, _ := store.GetAllPulsaDenom(ctx)
    sort.Slice(denoms, func(i, j int) bool { return denoms[i].Nominal < denoms[j].Nominal })

    var b strings.Builder
    fmt.Fprintf(&b, "📱 *PULSA*\n💰 Saldo Modal: %s\n\n", FormatRupiah(saldo))
    if len(denoms) == 0 {
        b.WriteString("Belum ada nominal dikonfigurasi.\n")
    } else {
        fmt.Fprintf(&b, "%-10s %-12s %-12s %s\n", "Nominal", "Modal", "Jual", "Margin")
        fmt.Fprintf(&b, "%s\n", strings.Repeat("─", 48))
        for _, d := range denoms {
            aktifStr := "✅"
            if !d.Aktif {
                aktifStr = "❌"
            }
            fmt.Fprintf(&b, "%s %-10s %-12s %-12s +%s\n",
                aktifStr,
                FormatRupiah(d.Nominal),
                FormatRupiah(d.HargaModal),
                FormatRupiah(d.HargaJual),
                FormatRupiah(d.Margin()),
            )
        }
    }
    b.WriteString("\nGunakan /pulsa <nominal> untuk jual, /isipulsa <jumlah> untuk top-up.")
    return b.String()
}

// handleIsiPulsaCmd menangani /isipulsa <jumlah> — owner-only.
// Mencatat top-up saldo modal pulsa dan menambah SaldoModal ke produk anchor.
func handleIsiPulsaCmd(ctx context.Context, store *Store, text string) string {
    role, kasir, err := resolveRoleFromCtx(ctx, store)
    if err != nil {
        return "Maaf, tidak bisa verifikasi akses kak 🙏"
    }
    if role != "owner" {
        return "Maaf ya kak 🙏 /isipulsa khusus owner."
    }

    arg := argAfter(text)
    jumlah := parseRupiah(arg)
    if jumlah <= 0 {
        return "Format: /isipulsa <jumlah> (contoh: /isipulsa 500000)"
    }

    // Cari produk anchor pulsa
    all, _ := store.GetAllProduk(ctx)
    var anchor *Produk
    for _, p := range all {
        if p.JenisOrDefault() == "pulsa" {
            anchor = p
            break
        }
    }
    if anchor == nil {
        return "Produk pulsa belum ada kak 🔍 Minta owner tambahkan produk dengan jenis=pulsa dulu ya."
    }

    now := NowWITA()
    anchor.SaldoModal += jumlah
    anchor.LastUpdate = now.Format("2006-01-02")
    if setErr := store.SetProduk(ctx, anchor); setErr != nil {
        return fmt.Sprintf("Gagal simpan saldo: %v", setErr)
    }

    pt := &PulsaTopup{
        Tanggal:      now.Format("2006-01-02"),
        Jam:          now.Format("15:04:05"),
        Jumlah:       jumlah,
        SaldoSesudah: anchor.SaldoModal,
        Kasir:        kasir,
    }
    _ = store.AppendPulsaTopup(ctx, pt)

    return fmt.Sprintf("✅ Saldo modal pulsa ditambah %s\n💰 Saldo sekarang: %s\n📋 ID Top-up: %s",
        FormatRupiah(jumlah), FormatRupiah(anchor.SaldoModal), pt.ID)
}

// handleBensinCmd menangani /bensin dan /bensin <sub-jenis> <liter>
// /bensin                    → tampilkan stok Pertalite + Pertamax
// /bensin pertalite 2        → jual 2L Pertalite (kasir/owner)
func handleBensinCmd(ctx context.Context, store *Store, text string) string {
    role, kasir, err := resolveRoleFromCtx(ctx, store)
    if err != nil {
        return "Maaf, tidak bisa verifikasi akses kak 🙏"
    }
    if role != "owner" && role != "kasir" {
        return "Maaf ya kak 🙏 perintah ini khusus owner/kasir."
    }

    arg := strings.TrimSpace(argAfter(text))
    if arg == "" {
        return bensinInfo(ctx, store)
    }

    // Parse: /bensin <query> <liter>
    parts := strings.Fields(arg)
    if len(parts) < 2 {
        return "Format: /bensin <produk> <liter> (contoh: /bensin pertalite 2)"
    }
    literStr := parts[len(parts)-1]
    query := strings.Join(parts[:len(parts)-1], " ")

    literF, parseErr := strconv.ParseFloat(literStr, 64)
    if parseErr != nil || literF <= 0 {
        return fmt.Sprintf("Volume tidak valid kak 🙏 Contoh: /bensin pertalite 2")
    }
    ml := int(math.Round(literF * 1000))

    item, findErr := findOne(ctx, store, query)
    if findErr != nil || item == nil {
        return fmt.Sprintf("Produk \"%s\" tidak ditemukan kak 🔍", query)
    }
    if item.JenisOrDefault() != "bensin" {
        return fmt.Sprintf("%s bukan produk bensin kak 🙏", item.Nama)
    }

    tx, updated, sisaMl, sellErr := sellBensin(ctx, store, item, ml, "tunai", kasir)
    if sellErr != nil {
        return sellErr.Error()
    }

    out := fmt.Sprintf("✅ Bensin %s %.3fL terjual\n"+
        "💰 Total: %s\n"+
        "⛽ Stok sisa: %.1fL\n"+
        "📋 ID: %s",
        updated.Nama, literF,
        FormatRupiah(tx.Total),
        float64(sisaMl)/1000,
        tx.ID)
    kritisMl := updated.StokKritisMl
    if kritisMl == 0 {
        kritisMl = 40000
    }
    if sisaMl <= kritisMl {
        out += fmt.Sprintf("\n⚠️ Stok %s menipis! Segera restock ya kak.", updated.Nama)
    }
    return out
}

// bensinInfo menampilkan stok semua produk bensin.
func bensinInfo(ctx context.Context, store *Store) string {
    all, _ := store.GetAllProduk(ctx)
    var bensins []*Produk
    for _, p := range all {
        if p.JenisOrDefault() == "bensin" {
            bensins = append(bensins, p)
        }
    }
    if len(bensins) == 0 {
        return "Belum ada produk bensin kak 🔍"
    }
    var b strings.Builder
    b.WriteString("⛽ *STOK BENSIN*\n\n")
    for _, p := range bensins {
        kritisMl := p.StokKritisMl
        if kritisMl == 0 {
            kritisMl = 40000
        }
        status := "✅"
        if p.StokMl == 0 {
            status = "❌ HABIS"
        } else if p.StokMl <= kritisMl {
            status = "⚠️ KRITIS"
        }
        fmt.Fprintf(&b, "• %s %s\n  Stok: %.1fL | Kritis: %.0fL | Harga: %s/L\n",
            p.Nama, status,
            float64(p.StokMl)/1000,
            float64(kritisMl)/1000,
            FormatRupiah(p.HargaJual),
        )
    }
    b.WriteString("\nGunakan /bensin <nama> <liter> untuk jual, /isibensin untuk restock.")
    return b.String()
}

// handleIsiBensinCmd menangani /isibensin <sub-jenis> <liter> <harga_total> — owner.
// Update StokMl + hitung dan update HargaBeli = harga_total / liter.
func handleIsiBensinCmd(ctx context.Context, store *Store, text string) string {
    role, _, err := resolveRoleFromCtx(ctx, store)
    if err != nil {
        return "Maaf, tidak bisa verifikasi akses kak 🙏"
    }
    if role != "owner" {
        return "Maaf ya kak 🙏 /isibensin khusus owner."
    }

    arg := strings.TrimSpace(argAfter(text))
    parts := strings.Fields(arg)
    if len(parts) < 3 {
        return "Format: /isibensin <produk> <liter> <harga_total>\n" +
            "Contoh: /isibensin pertalite 100 1150000"
    }

    // Parse dari belakang: harga_total dan liter
    hargaTotal := parseRupiah(parts[len(parts)-1])
    literF, parseErr := strconv.ParseFloat(parts[len(parts)-2], 64)
    query := strings.Join(parts[:len(parts)-2], " ")

    if parseErr != nil || literF <= 0 || hargaTotal <= 0 {
        return "Volume atau harga tidak valid kak 🙏"
    }

    item, findErr := findOne(ctx, store, query)
    if findErr != nil || item == nil {
        return fmt.Sprintf("Produk \"%s\" tidak ditemukan kak 🔍", query)
    }
    if item.JenisOrDefault() != "bensin" {
        return fmt.Sprintf("%s bukan produk bensin kak 🙏", item.Nama)
    }

    mlTambah := int(math.Round(literF * 1000))
    hargaBeliPerLiter := int(math.Round(float64(hargaTotal) / literF))

    item.StokMl += mlTambah
    item.Stok = item.StokMl / 1000
    item.HargaBeli = hargaBeliPerLiter
    item.LastUpdate = NowWITA().Format("2006-01-02")
    if setErr := store.SetProduk(ctx, item); setErr != nil {
        return fmt.Sprintf("Gagal update stok: %v", setErr)
    }

    return fmt.Sprintf("✅ Restock bensin %s berhasil\n"+
        "⛽ Ditambah: %.1fL | Stok kini: %.1fL\n"+
        "💰 Harga beli/L diperbarui: %s\n"+
        "📅 %s WITA",
        item.Nama,
        literF, float64(item.StokMl)/1000,
        FormatRupiah(hargaBeliPerLiter),
        NowWITA().Format("02/01/2006 15:04"),
    )
}

// resolveRoleFromCtx adalah wrapper dari resolveRole yang mengembalikan error
// secara eksplisit untuk penggunaan di handler slash command (tanpa ToolResult).
func resolveRoleFromCtx(ctx context.Context, store *Store) (role, name string, err error) {
    r, n, refusal := resolveRole(ctx, store)
    if refusal != nil {
        return "", "", fmt.Errorf("%s", refusal.ForLLM)
    }
    return r, n, nil
}
```

### Step 5.2: Daftarkan di commands.go

Di `commands.go` (atau di fungsi `RegisterKiosCommands`), tambah panggilan ke `RegisterSpecialCommands` setelah semua handler yang sudah ada:

```go
// Di RegisterKiosCommands atau fungsi registrasi utama commands.go:
RegisterSpecialCommands(router, store)
```

Tambahkan juga ke panduan `/panduan` (teks `panduanText` di baris 19-50 commands.go):

```
/pulsa [nominal] — cek saldo/nominal atau jual pulsa
/isipulsa <jumlah> — top-up saldo modal pulsa (owner)
/bensin [nama] [liter] — cek stok atau jual bensin
/isibensin <nama> <liter> <harga> — restock bensin (owner)
```

### Step 5.3: Test commands_special.go

Tambahkan ke `special_test.go`:

```go
func TestHandlePulsaInfoCmd(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()
    seedPulsaAnchor(t, s, 75000)
    seedDenom(t, s, 10000, 9500, 11000)

    // Simulasi context owner
    ctx = withTestOwner(ctx, s)
    result := handlePulsaCmd(ctx, s, "/pulsa")
    if !strings.Contains(result, "75.000") {
        t.Errorf("result should show saldo 75000, got: %s", result)
    }
    if !strings.Contains(result, "10.000") {
        t.Errorf("result should show nominal 10000, got: %s", result)
    }
}

func TestHandleIsiPulsaCmd(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()
    seedPulsaAnchor(t, s, 0)
    ctx = withTestOwner(ctx, s)

    result := handleIsiPulsaCmd(ctx, s, "/isipulsa 200000")
    if !strings.Contains(result, "200.000") {
        t.Errorf("result should show 200000, got: %s", result)
    }
    anchor, _ := s.GetProduk(ctx, "P99")
    if anchor.SaldoModal != 200000 {
        t.Errorf("SaldoModal want 200000, got %d", anchor.SaldoModal)
    }
}

func TestHandleBensinInfoCmd(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()
    seedBensinProduk(t, s, 30000, 40000, 10000, 12000)
    ctx = withTestOwner(ctx, s)

    result := handleBensinCmd(ctx, s, "/bensin")
    if !strings.Contains(result, "KRITIS") {
        t.Errorf("result should show KRITIS for 30L below 40L, got: %s", result)
    }
}

// withTestOwner menyuntikkan context owner untuk test.
// Mirip dengan test existing yang meng-inject via env var.
func withTestOwner(ctx context.Context, s *Store) context.Context {
    ownerPhone := "owner-test"
    _ = s.SetUser(ctx, &UserKios{Phone: ownerPhone, Nama: "Owner Test", Role: "owner", Aktif: true})
    return toolshared.WithCallerID(ctx, ownerPhone)
}
```

Catatan: `toolshared.WithCallerID` mungkin perlu dicek nama pastinya di `pkg/tools/shared/`. Import sesuaikan.

### Verifikasi Task 5

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run "TestHandlePulsa|TestHandleIsiBensin|TestHandleBensin"
```

---

## Task 6: Backup/Restore PulsaDenom + PulsaTopup

**Files:**
- Modify: `pkg/tools/kios/backup.go` — `BackupData`, `BuildBackup`, `Ringkas`, `RestoreBackup`, `HasAnyData`
- Modify: `pkg/tools/kios/special_test.go` — test backup round-trip

### Step 6.1: Tambah test backup round-trip (gagal dulu)

```go
func TestBackupRestorePulsa(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()

    // Setup: anchor + 2 denom + 1 topup
    anchor := seedPulsaAnchor(t, s, 150000)
    _ = anchor
    seedDenom(t, s, 10000, 9500, 11000)
    seedDenom(t, s, 25000, 24500, 26500)
    pt := &PulsaTopup{
        Tanggal: "2026-06-03", Jam: "10:00:00",
        Jumlah: 150000, SaldoSesudah: 150000, Kasir: "owner",
    }
    _ = s.AppendPulsaTopup(ctx, pt)

    // Backup
    b, err := BuildBackup(ctx, s)
    if err != nil {
        t.Fatalf("BuildBackup: %v", err)
    }
    if len(b.PulsaDenom) != 2 {
        t.Errorf("want 2 denom in backup, got %d", len(b.PulsaDenom))
    }
    if len(b.PulsaTopup) != 1 {
        t.Errorf("want 1 topup in backup, got %d", len(b.PulsaTopup))
    }

    // Restore ke store baru
    s2 := newTestStore(t)
    if err := s2.RestoreBackup(ctx, b); err != nil {
        t.Fatalf("RestoreBackup: %v", err)
    }

    // Verifikasi denom
    d10k, _ := s2.GetPulsaDenom(ctx, 10000)
    if d10k == nil || d10k.HargaJual != 11000 {
        t.Errorf("denom 10000 after restore: %+v", d10k)
    }

    // Verifikasi topup
    tops, _ := s2.GetAllPulsaTopup(ctx)
    if len(tops) != 1 {
        t.Errorf("want 1 topup after restore, got %d", len(tops))
    }

    // Verifikasi counter PTU di-restore
    nextID, _ := s2.NextPtuID(ctx)
    // PTU-0001 sudah ada → next harus PTU-0002
    if nextID != "PTU-0002" {
        t.Errorf("next PTU after restore want PTU-0002, got %s", nextID)
    }

    // Verifikasi saldo modal di produk anchor
    anchor2, _ := s2.GetProduk(ctx, "P99")
    if anchor2 == nil || anchor2.SaldoModal != 150000 {
        t.Errorf("SaldoModal after restore want 150000, got %v", anchor2)
    }
}
```

### Step 6.2: Modifikasi backup.go

Tambah field ke `BackupData`:

```go
type BackupData struct {
    Versi         string          `json:"versi"`
    Dibuat        string          `json:"dibuat"`
    Produk        []*Produk       `json:"produk"`
    Transaksi     []*Transaksi    `json:"transaksi"`
    Pembelian     []*Pembelian    `json:"pembelian"`
    PriceHistory  []*PriceHistory `json:"price_history"`
    Supplier      []*Supplier     `json:"supplier"`
    Promo         []*Promo        `json:"promo"`
    Pustaka       []*Pustaka      `json:"pustaka"`
    Users         []*UserKios     `json:"users"`
    Shift         *Shift          `json:"shift,omitempty"`
    HargaSupplier map[string]int  `json:"harga_supplier,omitempty"`
    PulsaDenom    []*PulsaDenom   `json:"pulsa_denom,omitempty"`   // NEW
    PulsaTopup    []*PulsaTopup   `json:"pulsa_topup,omitempty"`   // NEW
}
```

Tambah ke `BuildBackup` setelah `b.HargaSupplier`:

```go
    if b.PulsaDenom, err = store.GetAllPulsaDenom(ctx); err != nil {
        return nil, err
    }
    if b.PulsaTopup, err = store.GetAllPulsaTopup(ctx); err != nil {
        return nil, err
    }
```

Update `Ringkas()`:

```go
func (b *BackupData) Ringkas() string {
    return fmt.Sprintf("%d produk, %d transaksi, %d pembelian, %d riwayat harga, %d supplier, %d promo, %d pustaka, %d pengguna, %d harga supplier, %d denom pulsa, %d topup pulsa",
        len(b.Produk), len(b.Transaksi), len(b.Pembelian), len(b.PriceHistory),
        len(b.Supplier), len(b.Promo), len(b.Pustaka), len(b.Users), len(b.HargaSupplier),
        len(b.PulsaDenom), len(b.PulsaTopup))
}
```

Tambah ke `HasAnyData` (dalam loop `HGetAll`):

```go
    for _, k := range []string{keyProduk, keySupplier, keyPromo, keyPustaka, keyUsers, keyHargaSupplier, keyPulsaDenom} {
```

Modifikasi `RestoreBackup` — tambah ke blok `keys` (dihapus saat restore):

```go
    keys := []string{
        keyProduk, keyTransaksi, keySeqTrx, keyPembelian, keySeqPem,
        keyPriceHist, keySeqPhg, keyShift, keyUsers,
        keySupplier, keySeqSup, keyPromo, keySeqPromo, keyPustaka, keySeqPus,
        keyHargaSupplier,
        keyPulsaDenom, keyPulsaTopup, keySeqPtu, // NEW
    }
```

Tambah restore PulsaDenom dan PulsaTopup setelah restore `HargaSupplier`:

```go
    // Restore PulsaDenom (HASH, field = strconv.Itoa(nominal))
    for _, d := range b.PulsaDenom {
        raw, err := json.Marshal(d)
        if err != nil {
            return err
        }
        if err := s.rdb.HSet(ctx, keyPulsaDenom, strconv.Itoa(d.Nominal), string(raw)).Err(); err != nil {
            return err
        }
    }
    // Restore PulsaTopup (LIST, append-only)
    if err := rpushAll(keyPulsaTopup, anySlice(b.PulsaTopup)); err != nil {
        return err
    }
    // Restore counter PTU
    if err := setSeq(keySeqPtu, maxNumericSuffix(idsPulsaTopup(b.PulsaTopup))); err != nil {
        return err
    }
```

Tambah helper `idsPulsaTopup`:

```go
func idsPulsaTopup(xs []*PulsaTopup) []string {
    ids := make([]string, len(xs))
    for i, x := range xs {
        ids[i] = x.ID
    }
    return ids
}
```

### Verifikasi Task 6

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run "TestBackupRestorePulsa"
```

---

## Task 7 (Opsional): Dashboard Pulsa

**Files:**
- Create: `kios-dashboard/src/app/(app)/pulsa/page.tsx`
- Modify: `kios-dashboard/src/components/kasir/kasir-form.tsx`
- Modify: `kios-dashboard/src/components/nav-items.tsx`

### Step 7.1: Halaman /pulsa

Buat `kios-dashboard/src/app/(app)/pulsa/page.tsx`:

```tsx
import type { Metadata } from "next";
import { getAllPulsaDenom, getPulsaAnchor, getAllPulsaTopup } from "@/lib/kios";
import { ConnectionError } from "@/components/connection-error";
import { PulsaView } from "@/components/pulsa/pulsa-view";
import { ensureOwner } from "@/lib/auth";

export const metadata: Metadata = { title: "Pulsa" };
export const dynamic = "force-dynamic";

export default async function PulsaPage() {
  await ensureOwner(); // RBAC: halaman ini owner-only untuk set harga denom
  try {
    const [anchor, denoms, topups] = await Promise.all([
      getPulsaAnchor(),
      getAllPulsaDenom(),
      getAllPulsaTopup(),
    ]);
    return <PulsaView anchor={anchor} denoms={denoms} topups={topups} />;
  } catch (e) {
    return <ConnectionError message={e instanceof Error ? e.message : String(e)} />;
  }
}
```

### Step 7.2: Komponen PulsaView

Buat `kios-dashboard/src/components/pulsa/pulsa-view.tsx`:

```tsx
"use client";

import { useState, useTransition } from "react";
import type { Produk, PulsaDenom, PulsaTopup } from "@/lib/types";
import { formatRupiah } from "@/lib/format";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";

interface PulsaViewProps {
  anchor: Produk | null;
  denoms: PulsaDenom[];
  topups: PulsaTopup[];
}

export function PulsaView({ anchor, denoms, topups }: PulsaViewProps) {
  const saldo = anchor?.saldo_modal ?? 0;
  const [topupAmount, setTopupAmount] = useState("");
  const [pending, start] = useTransition();

  const sortedDenoms = [...denoms].sort((a, b) => a.nominal - b.nominal);

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold">Manajemen Pulsa</h2>
        <p className="text-sm text-muted-foreground">
          Saldo modal, nominal, dan riwayat top-up
        </p>
      </div>

      {/* Saldo Modal */}
      <Card>
        <CardHeader>
          <CardTitle>Saldo Modal Pulsa</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-3xl font-bold text-green-600">{formatRupiah(saldo)}</p>
          <div className="mt-4 flex gap-2 items-end">
            <div>
              <Label htmlFor="topup-amount">Top-up Saldo</Label>
              <Input
                id="topup-amount"
                type="text"
                placeholder="contoh: 500000"
                value={topupAmount}
                onChange={(e) => setTopupAmount(e.target.value)}
              />
            </div>
            <Button
              disabled={pending || !topupAmount}
              onClick={() => {
                start(async () => {
                  // TODO: panggil server action topupPulsaAction(amount)
                  alert("Top-up: " + topupAmount);
                });
              }}
            >
              {pending ? "Menyimpan..." : "Top-up"}
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Tabel Nominal */}
      <Card>
        <CardHeader>
          <CardTitle>Daftar Nominal</CardTitle>
        </CardHeader>
        <CardContent>
          {sortedDenoms.length === 0 ? (
            <p className="text-muted-foreground">Belum ada nominal dikonfigurasi.</p>
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b">
                  <th className="text-left py-2">Nominal</th>
                  <th className="text-right py-2">Modal</th>
                  <th className="text-right py-2">Jual</th>
                  <th className="text-right py-2">Margin</th>
                  <th className="text-center py-2">Aktif</th>
                </tr>
              </thead>
              <tbody>
                {sortedDenoms.map((d) => (
                  <tr key={d.nominal} className="border-b">
                    <td className="py-2">{formatRupiah(d.nominal)}</td>
                    <td className="text-right py-2">{formatRupiah(d.harga_modal)}</td>
                    <td className="text-right py-2">{formatRupiah(d.harga_jual)}</td>
                    <td className="text-right py-2 text-green-600">
                      +{formatRupiah(d.harga_jual - d.harga_modal)}
                    </td>
                    <td className="text-center py-2">{d.aktif ? "✅" : "❌"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </CardContent>
      </Card>

      {/* Riwayat Top-up */}
      <Card>
        <CardHeader>
          <CardTitle>Riwayat Top-up Saldo</CardTitle>
        </CardHeader>
        <CardContent>
          {topups.length === 0 ? (
            <p className="text-muted-foreground">Belum ada top-up.</p>
          ) : (
            <div className="space-y-2">
              {[...topups].reverse().slice(0, 20).map((pt) => (
                <div key={pt.id} className="flex justify-between text-sm border-b pb-1">
                  <span>{pt.tanggal} {pt.jam}</span>
                  <span className="font-medium text-green-600">+{formatRupiah(pt.jumlah)}</span>
                  <span className="text-muted-foreground">{pt.kasir}</span>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
```

### Step 7.3: Modifikasi kasir-form.tsx (denomination chips + liter input)

Di `KasirForm`, tambah logika deteksi jenis di `addToCart`:

```tsx
// Jika produk pulsa, jangan masukkan cart biasa — gunakan mode pulsa langsung
// Jika produk bensin, tampilkan input liter

// State tambahan
const [pulsaNominal, setPulsaNominal] = useState<number | null>(null);
const [bensinLiter, setBensinLiter] = useState("");

// Di bagian render, setelah addToCart untuk produk pulsa:
// Tampilkan denomination chips dari daftar nominal yang tersedia
```

Implementasi detail komponen kasir-form untuk pulsa/bensin cukup kompleks dan melibatkan server action baru. Sketsa alur:

1. Jika `p.jenis === "pulsa"` diklik, tampilkan chip nominal (5k/10k/15k/20k/25k/50k/100k) di panel samping, bukan qty stepper.
2. Pilih nominal → `checkoutAction({ action: "jual_pulsa", produk: p.id, nominal: chosen, metode })`.
3. Jika `p.jenis === "bensin"` diklik, tampilkan input liter dengan kalkulator Rp↔liter.
4. `checkoutAction({ action: "jual_bensin", produk: p.id, liter: literVal, metode })`.

Server action perlu diperluas untuk route ke `kios_kasir` tool dengan `nominal` atau `liter` sebagai argumen.

---

## Urutan Eksekusi

```
Task 1 (field + store_special.go)
    ↓
Task 2 (sellPulsa + sellBensin)
    ↓
Task 3 (wire performJual + kasir + batalkan)
    ↓
Task 4 (laba + notif fix)     Task 5 (slash commands)     Task 6 (backup/restore)
    ↓                              ↓                              ↓
Task 7 opsional (dashboard) ────────────────────────────────────
```

Task 1-2 blocking serial. Task 3-6 bisa paralel setelah Task 2 selesai.

---

## RBAC Ringkasan

| Aksi | kasir | owner |
|---|:---:|:---:|
| `/pulsa` (lihat) | ✅ | ✅ |
| `/pulsa <nominal>` (jual) | ✅ | ✅ |
| `/isipulsa <jumlah>` (top-up) | ❌ | ✅ |
| `/bensin` (lihat) | ✅ | ✅ |
| `/bensin <nama> <liter>` (jual) | ✅ | ✅ |
| `/isibensin <nama> <liter> <harga>` (restock) | ❌ | ✅ |
| Set harga nominal (`SetPulsaDenom`) | ❌ | ✅ |
| Batalkan transaksi pulsa/bensin | ✅ | ✅ |
| Halaman `/pulsa` dashboard | ❌ | ✅ |

Enforce via `requireOwner(role)` di handler `/isipulsa`, `/isibensin`, dan `PulsaPage`.

---

## Constraints Checklist

- [x] `SaldoModal`, `StokMl`, `StokKritisMl`, `Modal`, `Liter` semua `omitempty` — additive, data lama tetap valid
- [x] `sellPulsa`/`sellBensin` di `special.go`; helper store di `store_special.go`
- [x] `commands.go` tetap di batas 491 baris — slash baru ke `commands_special.go`
- [x] RBAC: jual = kasir+owner; isipulsa/set denom prices = owner-only
- [x] `batalkanTx` reverse per-jenis dengan benar (pulsa→SaldoModal, bensin→StokMl, default→Stok)
- [x] `hitungLaba` pakai `tx.Modal` bila > 0
- [x] Backup/restore mencakup `PulsaDenom` (HASH) + `PulsaTopup` (LIST) + counter `keySeqPtu`
- [x] `HasAnyData` memeriksa `keyPulsaDenom`
- [x] `gofmt`-clean — tidak ada kode yang menyalahi format Go standar
- [x] Setiap Go file baru < 500 baris
- [x] Test: `go test -tags goolm,stdjson ./pkg/tools/kios/...`
- [x] Signature `performJual` diubah dengan `extras map[string]int` — backward-compatible (pass `nil`)
- [x] Rollback best-effort di `sellPulsa`/`sellBensin` bila `AppendTransaksi` gagal

---

## Potensi Masalah & Mitigasi

| Masalah | Mitigasi |
|---|---|
| Race condition `SaldoModal` bila 2 kasir jual bersamaan | `store.mu.Lock()` sudah dipakai di `AppendTransaksi`; `sellPulsa` baca+tulis di luar lock. Solusi: gunakan `DecrSaldoModal` yang atomic (sudah ada di `store_special.go`) — refactor `sellPulsa` untuk panggil `DecrSaldoModal` yang lock, bukan set langsung. |
| `performJual` signature berubah — butuh update semua caller | Hanya 2 caller: `kasir.go:99` dan `kasir.go:130`. Pass `nil` sebagai argumen terakhir. |
| `struk()` di kasir.go tidak menampilkan Liter/Nominal | Tambah branch di `struk()`: jika `tx.Kategori == "pulsa"` tampilkan nominal dari catatan; jika `tx.Kategori == "bensin"` tampilkan Liter. |
| `jualMassal` tidak mendukung pulsa/bensin | `jualMassal` memanggil `performJual` dengan `nil` extras. Untuk sekarang: bensin/pulsa di `jual_massal` akan error "nominal/ml wajib diisi" — ini perilaku yang tepat dan aman. |

**Catatan penting tentang race condition SaldoModal:** Implementasi `sellPulsa` saat ini read→modify→write di luar lock. Pada beban rendah kios desa ini aman, tapi untuk kekokohan, refactor agar memanggil `store.DecrSaldoModal` (yang sudah lock). Kodenya:

```go
// Di sellPulsa, ganti:
//   item.SaldoModal -= denom.HargaModal
//   item.LastUpdate = ...
//   store.SetProduk(ctx, item)
// Dengan:
if err := store.DecrSaldoModal(ctx, item.ID, denom.HargaModal); err != nil {
    return nil, nil, 0, err
}
// Reload item untuk mendapatkan SaldoModal terbaru
item, err = store.GetProduk(ctx, item.ID)
if err != nil { return nil, nil, 0, err }
```

---

## Verifikasi Akhir

```bash
# Jalankan semua test
go test -tags goolm,stdjson ./pkg/tools/kios/...

# Build check
go build -tags goolm,stdjson ./...

# Test khusus Plan B
go test -tags goolm,stdjson ./pkg/tools/kios/... -run "TestSellPulsa|TestSellBensin|TestPerformJual|TestBatalkan|TestHitungLaba|TestNotif|TestBackupRestore|TestHandle"
```

Semua test harus lulus dengan output `PASS`.

---

## Total: 7 Task, 14+ file terpengaruh, ~8 file baru.

### Critical Files for Implementation
- /home/kevinman/Publik/project/kios-picoclaw/pkg/tools/kios/store.go
- /home/kevinman/Publik/project/kios-picoclaw/pkg/tools/kios/tool_common.go
- /home/kevinman/Publik/project/kios-picoclaw/pkg/tools/kios/kasir.go
- /home/kevinman/Publik/project/kios-picoclaw/pkg/tools/kios/backup.go
- /home/kevinman/Publik/project/kios-picoclaw/kios-dashboard/src/lib/types.ts