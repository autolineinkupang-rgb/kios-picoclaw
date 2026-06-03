# Plan A — Bon/Hutang (Piutang & Hutang Supplier) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Membangun dua buku bon/hutang: piutang (pembeli ngutang ke kios, terhubung registry Pelanggan) dan hutang supplier (kios bayar belakangan), lengkap dengan cicilan, lunas, write-off, slash commands 0-token, dan backup/restore.

**Architecture:** Tool `kios_bon` (`bon.go`) + CRUD `store_bon.go` + slash commands `commands_bon.go` (file baru, agar `commands.go` tidak melebihi 500 baris). Semua penjualan kredit tetap lewat `performJual` (`metode="bon"`), piutang dibuka setelah transaksi berhasil. Hutang supplier dibuka dari `pembelian_id` yang ada. Backup/restore via `BackupData` fields baru.

**Tech Stack:** Go (`pkg/tools/kios`), Redis/miniredis, test table-driven. Mirror TypeScript di `kios-dashboard/src/lib/`.

**Referensi spec:** `docs/specs/2026-06-03-kios-bon-pulsa-bensin-pelanggan-design.md` §3.2, §4.1, §5, §6, §9 Fase 2 Track A.

**Prasyarat:** Plan 0 + Plan 1 sudah landing (branch `feat/spec-bon-pulsa-bensin-pelanggan`).

**Toolchain:**
```bash
export GOROOT=$HOME/sdk/go PATH=$HOME/sdk/go/bin:$PATH GOPATH=$HOME/go
go test -tags goolm,stdjson ./pkg/tools/kios/...
```

---

## File Structure

| File | Tanggung jawab | Aksi |
|---|---|---|
| `pkg/tools/kios/store.go` | Tambah struct Piutang/Hutang/Pembayaran + key constants | Modify |
| `pkg/tools/kios/store_bon.go` | **BARU** — CRUD + counter Piutang/Hutang/Pembayaran + helpers | Create |
| `pkg/tools/kios/bon.go` | **BARU** — tool kios_bon (8 aksi) | Create |
| `pkg/tools/kios/commands_bon.go` | **BARU** — slash /utang /hutang /bayar /jualutang | Create |
| `pkg/tools/kios/store_pelanggan.go` | Tambah DelPelangganSafe | Modify |
| `pkg/tools/kios/stok.go` | Ekstensi batalkanTx untuk bon | Modify |
| `pkg/tools/kios/kasir.go` | Bypass guard bayar<total untuk metode=="bon" | Modify |
| `pkg/tools/kios/backup.go` | Tambah Piutang/Hutang/Pembayaran ke snapshot | Modify |
| `pkg/tools/kios/register.go` | Daftarkan NewBonTool | Modify |
| `pkg/tools/kios/commands.go` | Merge CommandsBon ke CommandsWithNotif | Modify |
| `pkg/tools/kios/bon_test.go` | **BARU** — semua test Plan A | Create |
| `kios-dashboard/src/lib/types.ts` | Interface Piutang/Hutang/Pembayaran | Modify |
| `kios-dashboard/src/lib/redis.ts` | KEY entries | Modify |
| `kios-dashboard/src/lib/kios.ts` | Data-access functions | Modify |

---

## Task 1: Struct + Key Constants + store_bon.go + TS Mirror

**Files:**
- Modify: `pkg/tools/kios/store.go`
- Create: `pkg/tools/kios/store_bon.go`
- Create: `pkg/tools/kios/bon_test.go`
- Modify: `kios-dashboard/src/lib/types.ts`, `redis.ts`, `kios.ts`

- [ ] **Step 1: Tulis failing tests**

Buat `pkg/tools/kios/bon_test.go`:

```go
package kios

import (
	"context"
	"strings"
	"testing"
)

func TestNextPiutangID(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	id1, err := s.NextPiutangID(ctx)
	if err != nil || id1 != "PIU-0001" {
		t.Fatalf("NextPiutangID: %v %q", err, id1)
	}
	id2, _ := s.NextPiutangID(ctx)
	if id2 != "PIU-0002" {
		t.Errorf("second id=%q want PIU-0002", id2)
	}
}

func TestSetGetPiutang(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	p := &Piutang{ID: "PIU-0001", Phone: "628123456789", Pokok: 50000, Sisa: 50000, Status: "terbuka"}
	if err := s.SetPiutang(ctx, p); err != nil {
		t.Fatalf("SetPiutang: %v", err)
	}
	got, err := s.GetPiutang(ctx, "PIU-0001")
	if err != nil || got == nil || got.Pokok != 50000 {
		t.Fatalf("GetPiutang: %v %+v", err, got)
	}
	all, _ := s.GetAllPiutang(ctx)
	if len(all) != 1 {
		t.Errorf("GetAllPiutang len=%d want 1", len(all))
	}
}

func TestAppendGetPembayaran(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	id, _ := s.NextPayID(ctx)
	if err := s.AppendPembayaran(ctx, &Pembayaran{ID: id, LedgerID: "PIU-0001", Jumlah: 10000}); err != nil {
		t.Fatalf("AppendPembayaran: %v", err)
	}
	all, err := s.GetAllPembayaran(ctx)
	if err != nil || len(all) != 1 || all[0].Jumlah != 10000 {
		t.Fatalf("GetAllPembayaran: %v %+v", err, all)
	}
}
```

- [ ] **Step 2: Jalankan, pastikan GAGAL**

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run 'TestNextPiutangID|TestSetGetPiutang|TestAppendGetPembayaran'
```
Expected: FAIL — `Piutang undefined`.

- [ ] **Step 3: Tambah struct + key constants ke store.go**

Di `pkg/tools/kios/store.go`, setelah struct `Pelanggan`:

```go
// Piutang adalah catatan kredit pembeli (pembeli ngutang ke kios).
type Piutang struct {
	ID          string `json:"id"`                      // "PIU-0001"
	PelangganID string `json:"pelanggan_id"`
	Phone       string `json:"phone"`                   // WA ternormalisasi
	TransaksiID string `json:"transaksi_id,omitempty"`  // TRX-xxxx dari jual bon
	Pokok       int    `json:"pokok"`
	Dibayar     int    `json:"dibayar"`
	Sisa        int    `json:"sisa"`
	Status      string `json:"status"`                  // "terbuka" | "lunas" | "dihapus"
	Tanggal     string `json:"tanggal"`
	Jam         string `json:"jam"`
	Kasir       string `json:"kasir"`
	Catatan     string `json:"catatan"`
}

// Hutang adalah catatan kios berutang ke supplier.
type Hutang struct {
	ID          string `json:"id"`                      // "HUT-0001"
	SupplierID  string `json:"supplier_id"`
	PembelianID string `json:"pembelian_id,omitempty"`  // PEM-xxxx dari restock
	Pokok       int    `json:"pokok"`
	Dibayar     int    `json:"dibayar"`
	Sisa        int    `json:"sisa"`
	Status      string `json:"status"`                  // "terbuka" | "lunas" | "dihapus"
	JatuhTempo  string `json:"jatuh_tempo,omitempty"`
	Tanggal     string `json:"tanggal"`
	Catatan     string `json:"catatan"`
}

// Pembayaran adalah satu event cicilan/lunas terhadap Piutang atau Hutang.
type Pembayaran struct {
	ID       string `json:"id"`        // "PAY-0001"
	LedgerID string `json:"ledger_id"` // PIU-xxxx atau HUT-xxxx
	Jenis    string `json:"jenis"`     // "piutang" | "hutang"
	Jumlah   int    `json:"jumlah"`
	Metode   string `json:"metode"`    // tunai | transfer | qris
	Tanggal  string `json:"tanggal"`
	Jam      string `json:"jam"`
	Kasir    string `json:"kasir"`
	Catatan  string `json:"catatan"`
}
```

Di blok `const` (tambahkan setelah `keyPelanggan`):

```go
	keyPiutang  = "kios:piutang"
	keyHutang   = "kios:hutang"
	keyPembayaran = "kios:pembayaran"
	keySeqPiu   = "kios:seq:piu"
	keySeqHut   = "kios:seq:hut"
	keySeqPay   = "kios:seq:pay"
```

- [ ] **Step 4: Buat store_bon.go**

Buat `pkg/tools/kios/store_bon.go`:

```go
package kios

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// --- Piutang ---

func (s *Store) NextPiutangID(ctx context.Context) (string, error) {
	n, err := s.rdb.Incr(ctx, keySeqPiu).Result()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("PIU-%04d", n), nil
}

func (s *Store) SetPiutang(ctx context.Context, p *Piutang) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return s.rdb.HSet(ctx, keyPiutang, p.ID, string(b)).Err()
}

func (s *Store) GetPiutang(ctx context.Context, id string) (*Piutang, error) {
	val, err := s.rdb.HGet(ctx, keyPiutang, id).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p Piutang
	if err := json.Unmarshal([]byte(val), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) GetAllPiutang(ctx context.Context) ([]*Piutang, error) {
	m, err := s.rdb.HGetAll(ctx, keyPiutang).Result()
	if err != nil {
		return nil, err
	}
	out := make([]*Piutang, 0, len(m))
	for _, v := range m {
		var p Piutang
		if json.Unmarshal([]byte(v), &p) == nil {
			out = append(out, &p)
		}
	}
	return out, nil
}

func (s *Store) DelPiutang(ctx context.Context, id string) error {
	return s.rdb.HDel(ctx, keyPiutang, id).Err()
}

// --- Hutang ---

func (s *Store) NextHutangID(ctx context.Context) (string, error) {
	n, err := s.rdb.Incr(ctx, keySeqHut).Result()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("HUT-%04d", n), nil
}

func (s *Store) SetHutang(ctx context.Context, h *Hutang) error {
	b, err := json.Marshal(h)
	if err != nil {
		return err
	}
	return s.rdb.HSet(ctx, keyHutang, h.ID, string(b)).Err()
}

func (s *Store) GetHutang(ctx context.Context, id string) (*Hutang, error) {
	val, err := s.rdb.HGet(ctx, keyHutang, id).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var h Hutang
	if err := json.Unmarshal([]byte(val), &h); err != nil {
		return nil, err
	}
	return &h, nil
}

func (s *Store) GetAllHutang(ctx context.Context) ([]*Hutang, error) {
	m, err := s.rdb.HGetAll(ctx, keyHutang).Result()
	if err != nil {
		return nil, err
	}
	out := make([]*Hutang, 0, len(m))
	for _, v := range m {
		var h Hutang
		if json.Unmarshal([]byte(v), &h) == nil {
			out = append(out, &h)
		}
	}
	return out, nil
}

func (s *Store) DelHutang(ctx context.Context, id string) error {
	return s.rdb.HDel(ctx, keyHutang, id).Err()
}

// --- Pembayaran ---

func (s *Store) NextPayID(ctx context.Context) (string, error) {
	n, err := s.rdb.Incr(ctx, keySeqPay).Result()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("PAY-%04d", n), nil
}

func (s *Store) AppendPembayaran(ctx context.Context, p *Pembayaran) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return s.rdb.RPush(ctx, keyPembayaran, string(b)).Err()
}

func (s *Store) GetAllPembayaran(ctx context.Context) ([]*Pembayaran, error) {
	vals, err := s.rdb.LRange(ctx, keyPembayaran, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	out := make([]*Pembayaran, 0, len(vals))
	for _, v := range vals {
		var p Pembayaran
		if json.Unmarshal([]byte(v), &p) == nil {
			out = append(out, &p)
		}
	}
	return out, nil
}

// --- Helpers untuk bon tool ---

// GetPembelianByID scans the pembelian list for a matching ID.
func (s *Store) GetPembelianByID(ctx context.Context, id string) (*Pembelian, error) {
	all, err := s.GetAllPembelian(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range all {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, nil
}

// GetSupplierByID returns the supplier with the given ID, or nil.
func (s *Store) GetSupplierByID(ctx context.Context, id string) (*Supplier, error) {
	all, err := s.GetAllSupplier(ctx)
	if err != nil {
		return nil, err
	}
	for _, sup := range all {
		if sup.ID == id {
			return sup, nil
		}
	}
	return nil, nil
}
```

- [ ] **Step 5: Jalankan test, pastikan LULUS**

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run 'TestNextPiutangID|TestSetGetPiutang|TestAppendGetPembayaran'
```

- [ ] **Step 6: TS mirror**

`kios-dashboard/src/lib/types.ts` — tambahkan setelah interface `Pelanggan`:

```ts
export type StatusBon = "terbuka" | "lunas" | "dihapus";

export interface Piutang {
  id: string;
  pelanggan_id: string;
  phone: string;
  transaksi_id?: string;
  pokok: number;
  dibayar: number;
  sisa: number;
  status: StatusBon;
  tanggal: string;
  jam: string;
  kasir: string;
  catatan: string;
}

export interface Hutang {
  id: string;
  supplier_id: string;
  pembelian_id?: string;
  pokok: number;
  dibayar: number;
  sisa: number;
  status: StatusBon;
  jatuh_tempo?: string;
  tanggal: string;
  catatan: string;
}

export interface Pembayaran {
  id: string;
  ledger_id: string;
  jenis: "piutang" | "hutang";
  jumlah: number;
  metode: string;
  tanggal: string;
  jam: string;
  kasir: string;
  catatan: string;
}
```

`kios-dashboard/src/lib/redis.ts` — tambahkan di KEY map:

```ts
  piutang: "kios:piutang",
  hutang: "kios:hutang",
  pembayaran: "kios:pembayaran",
  seqPiu: "kios:seq:piu",
  seqHut: "kios:seq:hut",
  seqPay: "kios:seq:pay",
```

`kios-dashboard/src/lib/kios.ts` — tambahkan bagian Bon setelah Pelanggan:

```ts
// ── Bon / Hutang ──────────────────────────────────────────────────────────────

export async function getAllPiutang(): Promise<Piutang[]> {
  const map = await redis().hgetall<Record<string, unknown>>(KEY.piutang);
  if (!map) return [];
  return normalizeList<Piutang>(Object.values(map));
}
export async function getPiutang(id: string): Promise<Piutang | null> {
  return normalize<Piutang>(await redis().hget<unknown>(KEY.piutang, id));
}
export async function setPiutang(p: Piutang): Promise<void> {
  await redis().hset(KEY.piutang, { [p.id]: p });
}
export async function getAllHutang(): Promise<Hutang[]> {
  const map = await redis().hgetall<Record<string, unknown>>(KEY.hutang);
  if (!map) return [];
  return normalizeList<Hutang>(Object.values(map));
}
export async function getHutang(id: string): Promise<Hutang | null> {
  return normalize<Hutang>(await redis().hget<unknown>(KEY.hutang, id));
}
export async function setHutang(h: Hutang): Promise<void> {
  await redis().hset(KEY.hutang, { [h.id]: h });
}
export async function getAllPembayaran(): Promise<Pembayaran[]> {
  const list = await redis().lrange<unknown>(KEY.pembayaran, 0, -1);
  if (!list) return [];
  return normalizeList<Pembayaran>(list);
}
export async function appendPembayaran(p: Pembayaran): Promise<void> {
  await redis().rpush(KEY.pembayaran, p);
}
export async function nextPayId(): Promise<string> {
  const n = await redis().incr(KEY.seqPay);
  return `PAY-${String(n).padStart(4, "0")}`;
}
```

- [ ] **Step 7: Jalankan seluruh paket + typecheck + gofmt**

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/...
gofmt -l pkg/tools/kios/store.go pkg/tools/kios/store_bon.go
cd kios-dashboard && npm run typecheck 2>&1 | tail -5; cd ..
```

- [ ] **Step 8: Commit**

```bash
git add pkg/tools/kios/store.go pkg/tools/kios/store_bon.go pkg/tools/kios/bon_test.go \
  kios-dashboard/src/lib/types.ts kios-dashboard/src/lib/redis.ts kios-dashboard/src/lib/kios.ts
git commit -m "feat(kios): struct Piutang/Hutang/Pembayaran + store_bon CRUD + TS mirror"
```

---

## Task 2: Tool kios_bon + register.go

**Files:**
- Create: `pkg/tools/kios/bon.go`
- Modify: `pkg/tools/kios/register.go`
- Modify: `pkg/tools/kios/bon_test.go`

- [ ] **Step 1: Tulis failing tests**

Tambahkan di akhir `bon_test.go`:

```go
func TestJualBon(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "001", "Mie Goreng", 20, 2000, 3000, 3)
	if _, err := s.UpsertPelanggan(ctx, "Budi", "08123456789"); err != nil {
		t.Fatalf("upsert pelanggan: %v", err)
	}

	bon := NewBonTool(s)
	result := bon.Execute(ctx, map[string]any{
		"action": "jual_bon", "produk": "mie", "qty": float64(2), "pelanggan": "08123456789",
	})
	if result.IsError {
		t.Fatalf("jual_bon error: %s", result.ForLLM)
	}
	all, _ := s.GetAllPiutang(ctx)
	if len(all) != 1 || all[0].Pokok != 6000 || all[0].Status != "terbuka" {
		t.Fatalf("piutang: %+v", all)
	}
	p, _ := s.GetPelanggan(ctx, "628123456789")
	if p == nil || p.TotalBelanja != 6000 {
		t.Errorf("pelanggan total_belanja=%d want 6000", p.TotalBelanja)
	}
}

func TestBayarPiutang(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "001", "Mie Goreng", 20, 2000, 3000, 3)
	s.UpsertPelanggan(ctx, "Budi", "08123456789") //nolint:errcheck

	bon := NewBonTool(s)
	bon.Execute(ctx, map[string]any{"action": "jual_bon", "produk": "mie", "qty": float64(2), "pelanggan": "08123456789"})
	all, _ := s.GetAllPiutang(ctx)
	piuID := all[0].ID

	// Cicil 3000 dari 6000
	r := bon.Execute(ctx, map[string]any{"action": "bayar", "id": piuID, "jumlah": float64(3000), "metode": "tunai"})
	if r.IsError {
		t.Fatalf("bayar error: %s", r.ForLLM)
	}
	piu, _ := s.GetPiutang(ctx, piuID)
	if piu.Dibayar != 3000 || piu.Sisa != 3000 || piu.Status != "terbuka" {
		t.Errorf("setelah cicil: %+v", piu)
	}

	// Lunas
	bon.Execute(ctx, map[string]any{"action": "bayar", "id": piuID, "jumlah": float64(3000), "metode": "tunai"})
	piu, _ = s.GetPiutang(ctx, piuID)
	if piu.Status != "lunas" {
		t.Errorf("setelah lunas status=%q want lunas", piu.Status)
	}
}

func TestBayarOverpaymentDitolak(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "001", "Mie Goreng", 20, 2000, 3000, 3)
	s.UpsertPelanggan(ctx, "Budi", "08123456789") //nolint:errcheck

	bon := NewBonTool(s)
	bon.Execute(ctx, map[string]any{"action": "jual_bon", "produk": "mie", "qty": float64(1), "pelanggan": "08123456789"})
	all, _ := s.GetAllPiutang(ctx)

	r := bon.Execute(ctx, map[string]any{"action": "bayar", "id": all[0].ID, "jumlah": float64(99999), "metode": "tunai"})
	if !r.IsError {
		t.Error("overpayment harus ditolak")
	}
}

func TestHapusPiutangOwnerOnly(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "001", "Mie Goreng", 20, 2000, 3000, 3)
	s.UpsertPelanggan(ctx, "Budi", "08123456789") //nolint:errcheck

	bon := NewBonTool(s)
	bon.Execute(ctx, map[string]any{"action": "jual_bon", "produk": "mie", "qty": float64(1), "pelanggan": "08123456789"})
	all, _ := s.GetAllPiutang(ctx)

	// kasir tidak bisa hapus
	ctxKasir := withRole(ctx, "kasir")
	r := bon.Execute(ctxKasir, map[string]any{"action": "hapus", "id": all[0].ID})
	if !r.IsError {
		t.Error("kasir tidak boleh hapus piutang")
	}
	// owner bisa hapus
	ctxOwner := withRole(ctx, "owner")
	r2 := bon.Execute(ctxOwner, map[string]any{"action": "hapus", "id": all[0].ID})
	if r2.IsError {
		t.Errorf("owner harus bisa hapus: %s", r2.ForLLM)
	}
}
```

*Catatan:* `withRole` adalah helper test:
```go
func withRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, contextKeyRole{}, role)
}
type contextKeyRole struct{}
```
Jika helper ini sudah ada di kios_test.go (cek dulu), pakai yang sudah ada. Jika belum, tambahkan di `bon_test.go`.

- [ ] **Step 2: Jalankan, pastikan GAGAL**

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run 'TestJualBon|TestBayarPiutang|TestBayarOverpayment|TestHapusPiutang'
```

- [ ] **Step 3: Buat bon.go**

Buat `pkg/tools/kios/bon.go`:

```go
package kios

import (
	"context"
	"fmt"
	"strings"

	tools "github.com/sipeed/picoclaw/pkg/tools/shared"
)

// BonTool handles credit sales (piutang) and supplier payables (hutang).
type BonTool struct{ store *Store }

func NewBonTool(store *Store) *BonTool { return &BonTool{store: store} }

func (t *BonTool) Name() string { return "kios_bon" }

func (t *BonTool) Description() string {
	return "Kelola bon & hutang kios: jual_bon (kredit pembeli), catat_hutang_supplier, bayar, lunasi, hapus, daftar_piutang, daftar_hutang, detail."
}

func (t *BonTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action":      map[string]any{"type": "string", "description": "Aksi: jual_bon|catat_hutang_supplier|bayar|lunasi|hapus|daftar_piutang|daftar_hutang|detail"},
			"produk":      map[string]any{"type": "string"},
			"qty":         map[string]any{"type": "integer"},
			"pelanggan":   map[string]any{"type": "string", "description": "nomor WA atau nama pelanggan"},
			"id":          map[string]any{"type": "string", "description": "PIU-xxxx atau HUT-xxxx"},
			"jumlah":      map[string]any{"type": "integer"},
			"metode":      map[string]any{"type": "string"},
			"supplier_id": map[string]any{"type": "string"},
			"pembelian_id": map[string]any{"type": "string"},
			"pokok":       map[string]any{"type": "integer"},
			"filter":      map[string]any{"type": "string"},
			"jatuh_tempo": map[string]any{"type": "string"},
			"catatan":     map[string]any{"type": "string"},
		},
		"required": []string{"action"},
	}
}

func (t *BonTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	role := resolveRole(ctx)
	switch argStr(args, "action") {
	case "jual_bon":
		return t.jualBon(ctx, args, role)
	case "catat_hutang_supplier":
		if r := requireOwner(role); r != nil { return r }
		return t.catatHutangSupplier(ctx, args)
	case "bayar":
		if r := requireStaff(role); r != nil { return r }
		return t.bayar(ctx, args, role)
	case "lunasi":
		if r := requireStaff(role); r != nil { return r }
		return t.lunasi(ctx, args, role)
	case "hapus":
		if r := requireOwner(role); r != nil { return r }
		return t.hapus(ctx, args)
	case "daftar_piutang":
		return t.daftarPiutang(ctx, args)
	case "daftar_hutang":
		return t.daftarHutang(ctx, args)
	case "detail":
		return t.detail(ctx, args)
	default:
		return tools.ErrorResult("Aksi bon belum dikenal kak 🤔")
	}
}

func (t *BonTool) jualBon(ctx context.Context, args map[string]any, kasir string) *tools.ToolResult {
	qty := argInt(args, "qty")
	if qty <= 0 {
		return tools.ErrorResult("Qty harus > 0 kak 🙏")
	}
	phone := NormalizePhone(argStr(args, "pelanggan"))
	if phone == "" {
		return tools.ErrorResult("Nomor WA pelanggan tidak valid kak 🙏 (contoh: 08123456789)")
	}
	pelanggan, err := t.store.UpsertPelanggan(ctx, argStr(args, "pelanggan"), argStr(args, "pelanggan"))
	if err != nil {
		// If pelanggan arg is a name, not a phone — try lookup by phone directly
		phone2 := NormalizePhone(argStr(args, "pelanggan"))
		if phone2 != "" {
			pelanggan, err = t.store.UpsertPelanggan(ctx, argStr(args, "pelanggan"), argStr(args, "pelanggan"))
		}
		if err != nil {
			return tools.ErrorResult("Gagal daftarkan pelanggan kak 😣").WithError(err)
		}
	}
	tx, _, _, err := performJual(ctx, t.store, argStr(args, "produk"), qty, "bon", kasir, 0)
	if err != nil {
		return tools.ErrorResult(err.Error())
	}
	piuID, err := t.store.NextPiutangID(ctx)
	if err != nil {
		return tools.ErrorResult("Gagal buat ID piutang kak 😣").WithError(err)
	}
	now := NowWITA()
	piu := &Piutang{
		ID: piuID, PelangganID: pelanggan.ID, Phone: pelanggan.Phone,
		TransaksiID: tx.ID, Pokok: tx.Total, Dibayar: 0, Sisa: tx.Total,
		Status: "terbuka", Tanggal: now.Format("2006-01-02"), Jam: now.Format("15:04:05"),
		Kasir: kasir,
	}
	if err := t.store.SetPiutang(ctx, piu); err != nil {
		return tools.ErrorResult("Gagal simpan piutang kak 😣").WithError(err)
	}
	// Update pelanggan totals
	pelanggan.TotalBelanja += tx.Total
	pelanggan.TotalUtang += tx.Total
	_ = t.store.SetPelanggan(ctx, pelanggan)

	return tools.NewToolResult(fmt.Sprintf(
		"Bon kredit dicatat kak ✅\n%s [%s] %s × %d = %s\nPiutang: %s (sisa %s). Pembayaran nanti ya.",
		tx.NamaProduk, tx.ID, FormatRupiah(tx.HargaSatuan), qty, FormatRupiah(tx.Total),
		piuID, FormatRupiah(piu.Sisa)))
}

func (t *BonTool) catatHutangSupplier(ctx context.Context, args map[string]any) *tools.ToolResult {
	supID := argStr(args, "supplier_id")
	pokok := argInt(args, "pokok")
	pemID := argStr(args, "pembelian_id")
	if supID == "" {
		return tools.ErrorResult("supplier_id wajib diisi kak 🙏")
	}
	if pokok <= 0 {
		return tools.ErrorResult("Pokok hutang harus > 0 kak 🙏")
	}
	sup, err := t.store.GetSupplierByID(ctx, supID)
	if err != nil || sup == nil {
		return tools.ErrorResult(fmt.Sprintf("Supplier %q tidak ditemukan kak 🔍", supID))
	}
	hutID, err := t.store.NextHutangID(ctx)
	if err != nil {
		return tools.ErrorResult("Gagal buat ID hutang kak 😣").WithError(err)
	}
	now := NowWITA()
	hut := &Hutang{
		ID: hutID, SupplierID: supID, PembelianID: pemID,
		Pokok: pokok, Dibayar: 0, Sisa: pokok, Status: "terbuka",
		JatuhTempo: argStr(args, "jatuh_tempo"),
		Tanggal: now.Format("2006-01-02"), Catatan: argStr(args, "catatan"),
	}
	if err := t.store.SetHutang(ctx, hut); err != nil {
		return tools.ErrorResult("Gagal simpan hutang kak 😣").WithError(err)
	}
	return tools.NewToolResult(fmt.Sprintf(
		"Hutang ke %s dicatat: %s — %s\nID: %s. Bayar nanti sesuai jadwal.",
		sup.Nama, hutID, FormatRupiah(pokok), hutID))
}

func (t *BonTool) bayar(ctx context.Context, args map[string]any, kasir string) *tools.ToolResult {
	id := strings.ToUpper(argStr(args, "id"))
	jumlah := argInt(args, "jumlah")
	if jumlah <= 0 {
		return tools.ErrorResult("Jumlah pembayaran harus > 0 kak 🙏")
	}
	metode := argStr(args, "metode")
	if metode == "" { metode = "tunai" }

	if strings.HasPrefix(id, "PIU-") {
		return t.bayarPiutang(ctx, id, jumlah, metode, kasir)
	} else if strings.HasPrefix(id, "HUT-") {
		return t.bayarHutang(ctx, id, jumlah, metode, kasir)
	}
	return tools.ErrorResult("ID harus PIU-xxxx atau HUT-xxxx kak 🙏")
}

func (t *BonTool) bayarPiutang(ctx context.Context, id string, jumlah int, metode, kasir string) *tools.ToolResult {
	piu, err := t.store.GetPiutang(ctx, id)
	if err != nil { return tools.ErrorResult("Gagal baca piutang kak 😣").WithError(err) }
	if piu == nil { return tools.ErrorResult(fmt.Sprintf("Piutang %s tidak ditemukan kak 🔍", id)) }
	if piu.Status != "terbuka" { return tools.ErrorResult(fmt.Sprintf("Piutang %s sudah %s kak.", id, piu.Status)) }
	if jumlah > piu.Sisa { return tools.ErrorResult(fmt.Sprintf("Overpayment kak 🙏 Sisa hanya %s.", FormatRupiah(piu.Sisa))) }

	piu.Dibayar += jumlah
	piu.Sisa -= jumlah
	if piu.Sisa == 0 { piu.Status = "lunas" }
	if err := t.store.SetPiutang(ctx, piu); err != nil {
		return tools.ErrorResult("Gagal update piutang kak 😣").WithError(err)
	}

	payID, _ := t.store.NextPayID(ctx)
	now := NowWITA()
	_ = t.store.AppendPembayaran(ctx, &Pembayaran{
		ID: payID, LedgerID: id, Jenis: "piutang", Jumlah: jumlah,
		Metode: metode, Tanggal: now.Format("2006-01-02"), Jam: now.Format("15:04:05"), Kasir: kasir,
	})

	// Update pelanggan total_utang cache
	if p, _ := t.store.GetPelanggan(ctx, piu.Phone); p != nil {
		if p.TotalUtang >= jumlah {
			p.TotalUtang -= jumlah
		} else {
			p.TotalUtang = 0
		}
		_ = t.store.SetPelanggan(ctx, p)
	}

	msg := fmt.Sprintf("Pembayaran %s diterima: %s (%s)\nDibayar: %s | Sisa: %s",
		payID, FormatRupiah(jumlah), metode, FormatRupiah(piu.Dibayar), FormatRupiah(piu.Sisa))
	if piu.Status == "lunas" { msg += "\n✅ Piutang LUNAS!" }
	return tools.NewToolResult(msg)
}

func (t *BonTool) bayarHutang(ctx context.Context, id string, jumlah int, metode, kasir string) *tools.ToolResult {
	hut, err := t.store.GetHutang(ctx, id)
	if err != nil { return tools.ErrorResult("Gagal baca hutang kak 😣").WithError(err) }
	if hut == nil { return tools.ErrorResult(fmt.Sprintf("Hutang %s tidak ditemukan kak 🔍", id)) }
	if hut.Status != "terbuka" { return tools.ErrorResult(fmt.Sprintf("Hutang %s sudah %s kak.", id, hut.Status)) }
	if jumlah > hut.Sisa { return tools.ErrorResult(fmt.Sprintf("Overpayment kak 🙏 Sisa hanya %s.", FormatRupiah(hut.Sisa))) }

	hut.Dibayar += jumlah
	hut.Sisa -= jumlah
	if hut.Sisa == 0 { hut.Status = "lunas" }
	if err := t.store.SetHutang(ctx, hut); err != nil {
		return tools.ErrorResult("Gagal update hutang kak 😣").WithError(err)
	}

	payID, _ := t.store.NextPayID(ctx)
	now := NowWITA()
	_ = t.store.AppendPembayaran(ctx, &Pembayaran{
		ID: payID, LedgerID: id, Jenis: "hutang", Jumlah: jumlah,
		Metode: metode, Tanggal: now.Format("2006-01-02"), Jam: now.Format("15:04:05"), Kasir: kasir,
	})

	msg := fmt.Sprintf("Bayar hutang %s: %s (%s)\nDibayar: %s | Sisa: %s",
		payID, FormatRupiah(jumlah), metode, FormatRupiah(hut.Dibayar), FormatRupiah(hut.Sisa))
	if hut.Status == "lunas" { msg += "\n✅ Hutang LUNAS!" }
	return tools.NewToolResult(msg)
}

func (t *BonTool) lunasi(ctx context.Context, args map[string]any, kasir string) *tools.ToolResult {
	id := strings.ToUpper(argStr(args, "id"))
	if strings.HasPrefix(id, "PIU-") {
		piu, _ := t.store.GetPiutang(ctx, id)
		if piu == nil { return tools.ErrorResult("Piutang tidak ditemukan kak 🔍") }
		args["jumlah"] = float64(piu.Sisa)
	} else if strings.HasPrefix(id, "HUT-") {
		hut, _ := t.store.GetHutang(ctx, id)
		if hut == nil { return tools.ErrorResult("Hutang tidak ditemukan kak 🔍") }
		args["jumlah"] = float64(hut.Sisa)
	}
	return t.bayar(ctx, args, kasir)
}

func (t *BonTool) hapus(ctx context.Context, args map[string]any) *tools.ToolResult {
	id := strings.ToUpper(argStr(args, "id"))
	if strings.HasPrefix(id, "PIU-") {
		piu, _ := t.store.GetPiutang(ctx, id)
		if piu == nil { return tools.ErrorResult("Piutang tidak ditemukan kak 🔍") }
		piu.Status = "dihapus"
		_ = t.store.SetPiutang(ctx, piu)
		return tools.NewToolResult(fmt.Sprintf("Piutang %s dihapus (write-off).", id))
	} else if strings.HasPrefix(id, "HUT-") {
		hut, _ := t.store.GetHutang(ctx, id)
		if hut == nil { return tools.ErrorResult("Hutang tidak ditemukan kak 🔍") }
		hut.Status = "dihapus"
		_ = t.store.SetHutang(ctx, hut)
		return tools.NewToolResult(fmt.Sprintf("Hutang %s dihapus (write-off).", id))
	}
	return tools.ErrorResult("ID harus PIU-xxxx atau HUT-xxxx kak 🙏")
}

func (t *BonTool) daftarPiutang(ctx context.Context, args map[string]any) *tools.ToolResult {
	all, err := t.store.GetAllPiutang(ctx)
	if err != nil { return tools.ErrorResult("Gagal baca piutang kak 😣").WithError(err) }
	filter := strings.ToLower(argStr(args, "filter"))
	var open []*Piutang
	for _, p := range all {
		if p.Status != "terbuka" { continue }
		if filter != "" && !strings.Contains(strings.ToLower(p.Phone), filter) { continue }
		open = append(open, p)
	}
	if len(open) == 0 { return tools.NewToolResult("Tidak ada piutang terbuka kak.") }
	var sb strings.Builder
	total := 0
	for _, p := range open {
		fmt.Fprintf(&sb, "- %s [%s] sisa %s (dari %s)\n", p.ID, p.Phone, FormatRupiah(p.Sisa), p.Tanggal)
		total += p.Sisa
	}
	return tools.NewToolResult(fmt.Sprintf("%d piutang terbuka — total %s:\n%s", len(open), FormatRupiah(total), sb.String()))
}

func (t *BonTool) daftarHutang(ctx context.Context, args map[string]any) *tools.ToolResult {
	all, err := t.store.GetAllHutang(ctx)
	if err != nil { return tools.ErrorResult("Gagal baca hutang kak 😣").WithError(err) }
	filter := strings.ToLower(argStr(args, "filter"))
	var open []*Hutang
	for _, h := range all {
		if h.Status != "terbuka" { continue }
		if filter != "" && !strings.Contains(strings.ToLower(h.SupplierID), filter) { continue }
		open = append(open, h)
	}
	if len(open) == 0 { return tools.NewToolResult("Tidak ada hutang supplier terbuka kak.") }
	var sb strings.Builder
	total := 0
	for _, h := range open {
		jt := ""
		if h.JatuhTempo != "" { jt = " (jatuh " + h.JatuhTempo + ")" }
		fmt.Fprintf(&sb, "- %s [%s] sisa %s%s\n", h.ID, h.SupplierID, FormatRupiah(h.Sisa), jt)
		total += h.Sisa
	}
	return tools.NewToolResult(fmt.Sprintf("%d hutang terbuka — total %s:\n%s", len(open), FormatRupiah(total), sb.String()))
}

func (t *BonTool) detail(ctx context.Context, args map[string]any) *tools.ToolResult {
	id := strings.ToUpper(argStr(args, "id"))
	var ledgerInfo string
	if strings.HasPrefix(id, "PIU-") {
		p, _ := t.store.GetPiutang(ctx, id)
		if p == nil { return tools.ErrorResult(fmt.Sprintf("Piutang %s tidak ditemukan kak 🔍", id)) }
		ledgerInfo = fmt.Sprintf("Piutang %s | %s\nPokok: %s | Dibayar: %s | Sisa: %s | Status: %s",
			p.ID, p.Phone, FormatRupiah(p.Pokok), FormatRupiah(p.Dibayar), FormatRupiah(p.Sisa), p.Status)
	} else if strings.HasPrefix(id, "HUT-") {
		h, _ := t.store.GetHutang(ctx, id)
		if h == nil { return tools.ErrorResult(fmt.Sprintf("Hutang %s tidak ditemukan kak 🔍", id)) }
		ledgerInfo = fmt.Sprintf("Hutang %s | %s\nPokok: %s | Dibayar: %s | Sisa: %s | Status: %s",
			h.ID, h.SupplierID, FormatRupiah(h.Pokok), FormatRupiah(h.Dibayar), FormatRupiah(h.Sisa), h.Status)
	} else {
		return tools.ErrorResult("ID harus PIU-xxxx atau HUT-xxxx kak 🙏")
	}
	// Histori pembayaran
	pays, _ := t.store.GetAllPembayaran(ctx)
	var hist strings.Builder
	for _, pay := range pays {
		if pay.LedgerID == id {
			fmt.Fprintf(&hist, "  • %s %s (%s) — %s\n", pay.ID, FormatRupiah(pay.Jumlah), pay.Metode, pay.Tanggal)
		}
	}
	if hist.Len() == 0 {
		return tools.NewToolResult(ledgerInfo + "\n(belum ada pembayaran)")
	}
	return tools.NewToolResult(ledgerInfo + "\nHistori pembayaran:\n" + hist.String())
}
```

- [ ] **Step 4: Daftarkan di register.go**

Di `pkg/tools/kios/register.go`, tambahkan `NewBonTool(store)` ke slice `AllTools`:

```go
func AllTools(store *Store) []toolshared.Tool {
    return []toolshared.Tool{
        NewStokTool(store),
        NewKasirTool(store),
        // ...existing tools...
        NewBonTool(store),  // Plan A
    }
}
```

- [ ] **Step 5: Jalankan test, pastikan LULUS**

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run 'TestJualBon|TestBayarPiutang|TestBayarOverpayment|TestHapusPiutang'
```

- [ ] **Step 6: Jalankan seluruh paket + gofmt + build**

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/...
gofmt -l pkg/tools/kios/bon.go pkg/tools/kios/register.go
make build 2>&1 | tail -2
```

- [ ] **Step 7: Commit**

```bash
git add pkg/tools/kios/bon.go pkg/tools/kios/register.go pkg/tools/kios/bon_test.go
git commit -m "feat(kios): tool kios_bon — jual_bon, bayar, hutang_supplier, hapus, daftar + register"
```

---

## Task 3: Transaksi.PiutangID + bypass guard bayar<total untuk bon

**Files:**
- Modify: `pkg/tools/kios/store.go` (Transaksi + `types.ts`)
- Modify: `pkg/tools/kios/kasir.go`
- Modify: `pkg/tools/kios/bon_test.go`

- [ ] **Step 1: Tulis failing test**

Tambahkan di `bon_test.go`:

```go
func TestJualBonBypassBayarGuard(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "001", "Mie Goreng", 20, 2000, 3000, 3)
	s.UpsertPelanggan(ctx, "Budi", "08123456789") //nolint:errcheck

	// Jual bon tanpa arg bayar harus tetap berhasil (bypass guard bayar<total)
	bon := NewBonTool(s)
	r := bon.Execute(ctx, map[string]any{
		"action": "jual_bon", "produk": "mie", "qty": float64(2), "pelanggan": "08123456789",
	})
	if r.IsError {
		t.Errorf("jual_bon harus bypass guard bayar<total: %s", r.ForLLM)
	}
}
```

- [ ] **Step 2: Tambah PiutangID ke Transaksi (store.go)**

Di struct `Transaksi` di `store.go`, tambahkan (additive, omitempty):

```go
	PiutangID string `json:"piutang_id,omitempty"` // diisi saat jual bon
```

Di `kios-dashboard/src/lib/types.ts` interface `Transaksi`, tambahkan:
```ts
  piutang_id?: string;
```

- [ ] **Step 3: Bypass guard di kasir.go**

Di `pkg/tools/kios/kasir.go`, fungsi `jual()`, bagian guard `bayar<total` (sekitar baris 83-96), tambahkan kondisi:

```go
// Bypass bayar<total guard untuk bon/kredit — pembeli bayar nanti
if argStr(args, "metode") != "bon" {
    if bayarPtr != nil && qty > 0 {
        // ... existing guard code ...
    }
}
```

Bungkus blok `if bayarPtr != nil && qty > 0 {` yang sudah ada dengan `if argStr(args, "metode") != "bon" {`.

- [ ] **Step 4: Jalankan test + seluruh paket**

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run TestJualBonBypassBayarGuard
go test -tags goolm,stdjson ./pkg/tools/kios/...
gofmt -l pkg/tools/kios/store.go pkg/tools/kios/kasir.go
```

- [ ] **Step 5: Commit**

```bash
git add pkg/tools/kios/store.go pkg/tools/kios/kasir.go kios-dashboard/src/lib/types.ts pkg/tools/kios/bon_test.go
git commit -m "feat(kios): Transaksi.PiutangID + bypass guard bayar<total untuk bon"
```

---

## Task 4: batalkanTx ekstensi bon

**Files:**
- Modify: `pkg/tools/kios/stok.go` (batalkanTx)
- Modify: `pkg/tools/kios/bon_test.go`

- [ ] **Step 1: Tulis failing tests**

Tambahkan di `bon_test.go`:

```go
func TestBatalkanTxBonTanpaCicilan(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "001", "Mie Goreng", 20, 2000, 3000, 3)
	s.UpsertPelanggan(ctx, "Budi", "08123456789") //nolint:errcheck

	bon := NewBonTool(s)
	bon.Execute(ctx, map[string]any{"action": "jual_bon", "produk": "mie", "qty": float64(2), "pelanggan": "08123456789"})
	allTx, _ := s.GetAllTransaksi(ctx)
	txID := allTx[0].ID
	allPiu, _ := s.GetAllPiutang(ctx)
	piuID := allPiu[0].ID

	// Batal tanpa cicilan → piutang harus void (Status="dihapus")
	stok := NewStokTool(s)
	r := stok.Execute(ctx, map[string]any{"action": "batalkan_tx", "id": txID})
	if r.IsError {
		t.Fatalf("batalkan_tx error: %s", r.ForLLM)
	}
	piu, _ := s.GetPiutang(ctx, piuID)
	if piu == nil || piu.Status != "dihapus" {
		t.Errorf("piutang harus dihapus, got: %+v", piu)
	}
}

func TestBatalkanTxBonDenganCicilan(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "001", "Mie Goreng", 20, 2000, 3000, 3)
	s.UpsertPelanggan(ctx, "Budi", "08123456789") //nolint:errcheck

	bon := NewBonTool(s)
	bon.Execute(ctx, map[string]any{"action": "jual_bon", "produk": "mie", "qty": float64(2), "pelanggan": "08123456789"})
	allTx, _ := s.GetAllTransaksi(ctx)
	txID := allTx[0].ID
	allPiu, _ := s.GetAllPiutang(ctx)

	// Cicil dulu
	bon.Execute(ctx, map[string]any{"action": "bayar", "id": allPiu[0].ID, "jumlah": float64(1000), "metode": "tunai"})

	// Batal dengan cicilan → harus error
	stok := NewStokTool(s)
	r := stok.Execute(ctx, map[string]any{"action": "batalkan_tx", "id": txID})
	if !r.IsError {
		t.Error("batalkan dengan cicilan harus error")
	}
}
```

- [ ] **Step 2: Jalankan, pastikan GAGAL**

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run 'TestBatalkanTxBon'
```

- [ ] **Step 3: Ekstensi batalkanTx di stok.go**

Di `pkg/tools/kios/stok.go`, fungsi `batalkanTx()`, setelah blok `if item, _ := t.store.GetProduk(...)`, tambahkan:

```go
	// Bon/kredit: void piutang bila belum ada cicilan, tolak bila sudah ada
	if tx.MetodeBayar == "bon" {
		allPiu, _ := t.store.GetAllPiutang(ctx)
		for _, piu := range allPiu {
			if piu.TransaksiID == tx.ID {
				if piu.Dibayar > 0 {
					// Kembalikan transaksi — batalkanTx sudah RemoveTransaksi, tapi kita perlu re-append
					// Lebih sederhana: return error sebelum RemoveTransaksi
					// Lihat catatan di bawah — ini sudah terlambat, kita perlu cek SEBELUM remove.
					return tools.ErrorResult(fmt.Sprintf(
						"Transaksi %s tidak bisa dibatalkan kak — piutang %s sudah ada cicilan %s. Gunakan write-off.",
						tx.ID, piu.ID, FormatRupiah(piu.Dibayar)))
				}
				// Belum ada cicilan → void piutang
				piu.Status = "dihapus"
				_ = t.store.SetPiutang(ctx, piu)
			}
		}
	}
```

**Catatan penting:** `RemoveTransaksi` sudah dipanggil sebelum cek ini. Perbaiki struktur agar cek piutang terjadi SEBELUM `RemoveTransaksi`. Ubah `batalkanTx` menjadi:

```go
func (t *StokTool) batalkanTx(ctx context.Context, args map[string]any) *tools.ToolResult {
	id := strings.ToUpper(argStr(args, "id"))
	if id == "" {
		return tools.ErrorResult("ID transaksi-nya diisi dulu ya kak 🙏")
	}

	// Cek dahulu sebelum remove (irreversible)
	allTx, err := t.store.GetAllTransaksi(ctx)
	if err != nil {
		return tools.ErrorResult("Gagal baca transaksi kak 😣").WithError(err)
	}
	var tx *Transaksi
	for _, t2 := range allTx {
		if t2.ID == id {
			tx = t2
			break
		}
	}
	if tx == nil {
		return tools.NewToolResult(fmt.Sprintf("Transaksi %s nggak ketemu kak 🔍", id))
	}

	// Guard: bon dengan cicilan tidak bisa dibatalkan
	if tx.MetodeBayar == "bon" {
		allPiu, _ := t.store.GetAllPiutang(ctx)
		for _, piu := range allPiu {
			if piu.TransaksiID == id && piu.Dibayar > 0 {
				return tools.ErrorResult(fmt.Sprintf(
					"Transaksi %s tidak bisa dibatalkan kak 🙏 — piutang %s sudah ada cicilan %s. Gunakan write-off.",
					id, piu.ID, FormatRupiah(piu.Dibayar)))
			}
		}
	}

	// Sekarang aman untuk remove
	removed, err := t.store.RemoveTransaksi(ctx, id)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal batalkan transaksi kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	if removed == nil {
		return tools.NewToolResult(fmt.Sprintf("Transaksi %s nggak ketemu kak 🔍", id))
	}

	// Restore stok
	if item, _ := t.store.GetProduk(ctx, removed.ProdukID); item != nil {
		item.Stok += removed.Qty
		item.LastUpdate = NowWITA().Format("2006-01-02")
		t.store.SetProduk(ctx, item)
	}

	// Void piutang bila bon dan belum ada cicilan
	if removed.MetodeBayar == "bon" {
		allPiu, _ := t.store.GetAllPiutang(ctx)
		for _, piu := range allPiu {
			if piu.TransaksiID == id {
				piu.Status = "dihapus"
				_ = t.store.SetPiutang(ctx, piu)
			}
		}
	}

	return tools.NewToolResult(fmt.Sprintf("Transaksi %s dibatalkan, stok %s dikembalikan (+%d).", removed.ID, removed.NamaProduk, removed.Qty))
}
```

- [ ] **Step 4: Jalankan test, pastikan LULUS**

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run 'TestBatalkanTxBon'
go test -tags goolm,stdjson ./pkg/tools/kios/...
gofmt -l pkg/tools/kios/stok.go
```

- [ ] **Step 5: Commit**

```bash
git add pkg/tools/kios/stok.go pkg/tools/kios/bon_test.go
git commit -m "feat(kios): batalkanTx ekstensi bon — void piutang tanpa cicilan, blokir jika ada cicilan"
```

---

## Task 5: DelPelangganSafe + guard hapus supplier dengan hutang terbuka

**Files:**
- Modify: `pkg/tools/kios/store_pelanggan.go`
- Modify: `pkg/tools/kios/supplier.go`
- Modify: `pkg/tools/kios/bon_test.go`

- [ ] **Step 1: Tulis failing tests**

Tambahkan di `bon_test.go`:

```go
func TestDelPelangganSafeDenganPiutangDiblokir(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "001", "Mie Goreng", 20, 2000, 3000, 3)
	s.UpsertPelanggan(ctx, "Budi", "08123456789") //nolint:errcheck

	bon := NewBonTool(s)
	bon.Execute(ctx, map[string]any{"action": "jual_bon", "produk": "mie", "qty": float64(1), "pelanggan": "08123456789"})

	err := s.DelPelangganSafe(ctx, "628123456789")
	if err == nil {
		t.Error("harus error — pelanggan masih punya piutang terbuka")
	}
}

func TestDelPelangganSafeTanpaPiutangOK(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	s.UpsertPelanggan(ctx, "Budi", "08123456789") //nolint:errcheck

	err := s.DelPelangganSafe(ctx, "628123456789")
	if err != nil {
		t.Errorf("hapus tanpa piutang harus OK: %v", err)
	}
	got, _ := s.GetPelanggan(ctx, "628123456789")
	if got != nil {
		t.Error("pelanggan harus sudah terhapus")
	}
}
```

- [ ] **Step 2: Jalankan, pastikan GAGAL**

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run 'TestDelPelangganSafe'
```

- [ ] **Step 3: Tambah DelPelangganSafe ke store_pelanggan.go**

```go
// DelPelangganSafe menghapus pelanggan hanya bila tidak ada piutang terbuka.
func (s *Store) DelPelangganSafe(ctx context.Context, phone string) error {
	allPiu, err := s.GetAllPiutang(ctx)
	if err != nil {
		return err
	}
	for _, p := range allPiu {
		if p.Phone == phone && p.Status == "terbuka" {
			return fmt.Errorf("pelanggan masih punya piutang terbuka %s (%s) — lunasi atau write-off dulu kak",
				p.ID, FormatRupiah(p.Sisa))
		}
	}
	return s.DelPelanggan(ctx, phone)
}
```

Pastikan `store_pelanggan.go` sudah import `"fmt"`.

- [ ] **Step 4: Tambah guard ke supplier.go hapus**

Di `pkg/tools/kios/supplier.go`, fungsi `hapus()`, sebelum `DelSupplier`:

```go
	// Guard: blokir hapus bila ada hutang terbuka ke supplier ini
	allHut, _ := t.store.GetAllHutang(ctx)
	for _, h := range allHut {
		if h.SupplierID == sup.ID && h.Status == "terbuka" {
			return tools.ErrorResult(fmt.Sprintf(
				"Supplier %s masih punya hutang terbuka %s (%s) — lunasi atau write-off dulu kak.",
				sup.Nama, h.ID, FormatRupiah(h.Sisa)))
		}
	}
```

- [ ] **Step 5: Jalankan test + seluruh paket**

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run 'TestDelPelangganSafe'
go test -tags goolm,stdjson ./pkg/tools/kios/...
gofmt -l pkg/tools/kios/store_pelanggan.go pkg/tools/kios/supplier.go
```

- [ ] **Step 6: Commit**

```bash
git add pkg/tools/kios/store_pelanggan.go pkg/tools/kios/supplier.go pkg/tools/kios/bon_test.go
git commit -m "feat(kios): DelPelangganSafe + guard hapus supplier dengan hutang terbuka"
```

---

## Task 6: Slash commands commands_bon.go

**Files:**
- Create: `pkg/tools/kios/commands_bon.go`
- Modify: `pkg/tools/kios/commands.go`
- Modify: `pkg/tools/kios/bon_test.go`

- [ ] **Step 1: Tulis failing test**

Tambahkan di `bon_test.go`:

```go
func TestSlashUtangCommand(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "001", "Mie Goreng", 20, 2000, 3000, 3)
	s.UpsertPelanggan(ctx, "Budi", "08123456789") //nolint:errcheck
	NewBonTool(s).Execute(ctx, map[string]any{
		"action": "jual_bon", "produk": "mie", "qty": float64(2), "pelanggan": "08123456789",
	})

	defs := CommandsBon(s)
	found := false
	for _, d := range defs {
		if d.Name == "utang" {
			found = true
			var out string
			req := commands.Request{
				Channel: "telegram", SenderID: "owner1", Text: "/utang",
				Reply: func(s string) error { out = s; return nil },
			}
			if err := d.Handler(ctx, req, nil); err != nil {
				t.Fatalf("/utang error: %v", err)
			}
			if !strings.Contains(out, "PIU-") {
				t.Errorf("/utang output: %q", out)
			}
		}
	}
	if !found {
		t.Fatal("command /utang tidak ditemukan di CommandsBon")
	}
}
```

- [ ] **Step 2: Jalankan, pastikan GAGAL**

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run TestSlashUtangCommand
```

- [ ] **Step 3: Buat commands_bon.go**

Buat `pkg/tools/kios/commands_bon.go`:

```go
package kios

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/sipeed/picoclaw/pkg/commands"
)

// CommandsBon returns the 0-token Telegram slash commands for bon/hutang.
// Slash commands are rule-based and do not consume LLM tokens.
func CommandsBon(store *Store) []commands.Definition {
	bon := NewBonTool(store)

	reply := func(req commands.Request, text string) error {
		if strings.TrimSpace(text) == "" {
			text = "(tidak ada data)"
		}
		return req.Reply(text)
	}

	return []commands.Definition{
		{
			Name:        "utang",
			Description: "Daftar piutang pembeli terbuka (0-token)",
			Usage:       "/utang [nama pelanggan]",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				filter := strings.TrimSpace(strings.TrimPrefix(req.Text, "/utang"))
				args := map[string]any{"action": "daftar_piutang"}
				if filter != "" {
					args["filter"] = filter
				}
				return reply(req, bon.Execute(ctx, args).ForLLM)
			},
		},
		{
			Name:        "hutang",
			Description: "Daftar hutang kios ke supplier (0-token)",
			Usage:       "/hutang [nama supplier]",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				filter := strings.TrimSpace(strings.TrimPrefix(req.Text, "/hutang"))
				args := map[string]any{"action": "daftar_hutang"}
				if filter != "" {
					args["filter"] = filter
				}
				return reply(req, bon.Execute(ctx, args).ForLLM)
			},
		},
		{
			Name:        "bayar",
			Description: "Catat pembayaran piutang atau hutang (0-token)",
			Usage:       "/bayar <PIU-xxxx|HUT-xxxx> <jumlah>",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				parts := strings.Fields(req.Text)
				if len(parts) < 3 {
					return reply(req, "Format: /bayar <PIU-xxxx|HUT-xxxx> <jumlah>\nContoh: /bayar PIU-0001 15000")
				}
				id := strings.ToUpper(parts[1])
				jumlah, err := strconv.Atoi(parts[2])
				if err != nil || jumlah <= 0 {
					return reply(req, fmt.Sprintf("Jumlah %q tidak valid kak 🙏 Contoh: /bayar %s 15000", parts[2], id))
				}
				return reply(req, bon.Execute(ctx, map[string]any{
					"action": "bayar", "id": id,
					"jumlah": float64(jumlah), "metode": "tunai",
				}).ForLLM)
			},
		},
		{
			Name:        "jualutang",
			Description: "Catat penjualan kredit/bon ke pelanggan (0-token)",
			Usage:       "/jualutang <produk> <qty> <nomor-WA>",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				parts := strings.Fields(req.Text)
				// /jualutang <produk> <qty> <phone>
				if len(parts) < 4 {
					return reply(req, "Format: /jualutang <produk> <qty> <nomor-WA>\nContoh: /jualutang mie 2 08123456789")
				}
				phone := parts[len(parts)-1]
				qtyStr := parts[len(parts)-2]
				produk := strings.Join(parts[1:len(parts)-2], " ")
				qty, err := strconv.Atoi(qtyStr)
				if err != nil || qty <= 0 {
					return reply(req, fmt.Sprintf("Qty %q tidak valid kak 🙏", qtyStr))
				}
				return reply(req, bon.Execute(ctx, map[string]any{
					"action": "jual_bon", "produk": produk,
					"qty": float64(qty), "pelanggan": phone,
				}).ForLLM)
			},
		},
	}
}
```

- [ ] **Step 4: Merge ke CommandsWithNotif di commands.go**

Di `pkg/tools/kios/commands.go`, fungsi `CommandsWithNotif`, ubah `return []commands.Definition{...}` menjadi:

```go
func CommandsWithNotif(store *Store, notifSvc *NotifService) []commands.Definition {
    defs := []commands.Definition{
        // ... existing definitions unchanged ...
    }
    defs = append(defs, CommandsBon(store)...)
    return defs
}
```

(Ubah `return` menjadi `defs :=`, tambahkan `append`, ganti `return` di akhir.)

- [ ] **Step 5: Jalankan test + seluruh paket**

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run TestSlashUtangCommand
go test -tags goolm,stdjson ./pkg/tools/kios/...
gofmt -l pkg/tools/kios/commands_bon.go pkg/tools/kios/commands.go
```

- [ ] **Step 6: Commit**

```bash
git add pkg/tools/kios/commands_bon.go pkg/tools/kios/commands.go pkg/tools/kios/bon_test.go
git commit -m "feat(kios): slash /utang /hutang /bayar /jualutang — 0-token bon commands"
```

---

## Task 7: Backup/restore Piutang/Hutang/Pembayaran

**Files:**
- Modify: `pkg/tools/kios/backup.go`
- Modify: `pkg/tools/kios/bon_test.go`

- [ ] **Step 1: Tulis failing test**

Tambahkan di `bon_test.go`:

```go
func TestBackupRestoreBonRoundtrip(t *testing.T) {
	ctx := context.Background()
	src := newTestStore(t)
	seedProduct(t, src, "001", "Mie Goreng", 20, 2000, 3000, 3)
	src.UpsertPelanggan(ctx, "Budi", "08123456789") //nolint:errcheck

	bon := NewBonTool(src)
	bon.Execute(ctx, map[string]any{"action": "jual_bon", "produk": "mie", "qty": float64(2), "pelanggan": "08123456789"})
	allPiu, _ := src.GetAllPiutang(ctx)
	piuID := allPiu[0].ID
	bon.Execute(ctx, map[string]any{"action": "bayar", "id": piuID, "jumlah": float64(1000), "metode": "tunai"})

	b, err := BuildBackup(ctx, src)
	if err != nil {
		t.Fatalf("BuildBackup: %v", err)
	}
	if len(b.Piutang) != 1 {
		t.Fatalf("piutang dalam backup=%d want 1", len(b.Piutang))
	}
	if len(b.Pembayaran) != 1 {
		t.Fatalf("pembayaran dalam backup=%d want 1", len(b.Pembayaran))
	}

	dst := newTestStore(t)
	if err := dst.RestoreBackup(ctx, b); err != nil {
		t.Fatalf("RestoreBackup: %v", err)
	}

	piu, err := dst.GetPiutang(ctx, piuID)
	if err != nil || piu == nil || piu.Dibayar != 1000 {
		t.Fatalf("piutang setelah restore: %+v err=%v", piu, err)
	}
	// Seq counter harus tidak collision
	nextID, _ := dst.NextPiutangID(ctx)
	if nextID == "PIU-0001" {
		t.Errorf("seq counter tidak dipulihkan: NextPiutangID=%q", nextID)
	}
}
```

- [ ] **Step 2: Jalankan, pastikan GAGAL**

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run TestBackupRestoreBonRoundtrip
```

- [ ] **Step 3: Tambah field ke BackupData**

Di `backup.go` struct `BackupData`, setelah `Pelanggan`:

```go
	Piutang    []*Piutang    `json:"piutang,omitempty"`
	Hutang     []*Hutang     `json:"hutang,omitempty"`
	Pembayaran []*Pembayaran `json:"pembayaran,omitempty"`
```

- [ ] **Step 4: BuildBackup + Ringkas + HasAnyData + RestoreBackup**

`BuildBackup` — tambahkan setelah `GetAllPelanggan`:
```go
	if b.Piutang, err = store.GetAllPiutang(ctx); err != nil { return nil, err }
	if b.Hutang, err = store.GetAllHutang(ctx); err != nil { return nil, err }
	if b.Pembayaran, err = store.GetAllPembayaran(ctx); err != nil { return nil, err }
```

`Ringkas()` — tambahkan `, %d piutang, %d hutang, %d pembayaran` di format string dan `len(b.Piutang), len(b.Hutang), len(b.Pembayaran)` di args.

`HasAnyData` — tambahkan `keyPiutang, keyHutang` ke HASH loop; `keyPembayaran` ke LIST loop.

`RestoreBackup` keys slice — tambahkan:
```go
		keyPiutang, keySeqPiu, keyHutang, keySeqHut,
		keyPembayaran, keySeqPay,
```

Restore block (setelah Pelanggan restore):
```go
	if err := hsetAll(keyPiutang, hashItemsPiutang(b.Piutang)); err != nil { return err }
	if err := hsetAll(keyHutang, hashItemsHutang(b.Hutang)); err != nil { return err }
	if err := rpushAll(keyPembayaran, anySlice(b.Pembayaran)); err != nil { return err }
```

setSeq map — tambahkan:
```go
		keySeqPiu: idsPiutang(b.Piutang),
		keySeqHut: idsHutang(b.Hutang),
		keySeqPay: idsPembayaran(b.Pembayaran),
```

Helper functions (tambahkan di akhir `backup.go`):
```go
func hashItemsPiutang(xs []*Piutang) []hashItem {
	out := make([]hashItem, len(xs))
	for i, x := range xs { out[i] = hashItem{field: x.ID, val: x} }
	return out
}
func hashItemsHutang(xs []*Hutang) []hashItem {
	out := make([]hashItem, len(xs))
	for i, x := range xs { out[i] = hashItem{field: x.ID, val: x} }
	return out
}
func idsPiutang(xs []*Piutang) []string {
	ids := make([]string, len(xs)); for i, x := range xs { ids[i] = x.ID }; return ids
}
func idsHutang(xs []*Hutang) []string {
	ids := make([]string, len(xs)); for i, x := range xs { ids[i] = x.ID }; return ids
}
func idsPembayaran(xs []*Pembayaran) []string {
	ids := make([]string, len(xs)); for i, x := range xs { ids[i] = x.ID }; return ids
}
```

- [ ] **Step 5: Jalankan test + seluruh paket + build**

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run TestBackupRestoreBonRoundtrip
go test -tags goolm,stdjson ./pkg/tools/kios/...
gofmt -l pkg/tools/kios/backup.go
make build 2>&1 | tail -2
```

- [ ] **Step 6: Commit**

```bash
git add pkg/tools/kios/backup.go pkg/tools/kios/bon_test.go
git commit -m "feat(kios): backup/restore Piutang/Hutang/Pembayaran + seq counters + HasAnyData"
```

---

## Task 8 (Dashboard — opsional): Halaman /hutang

**Files:**
- Create: `kios-dashboard/src/app/(app)/hutang/page.tsx`
- Create: `kios-dashboard/src/app/(app)/hutang/actions.ts`
- Modify: `kios-dashboard/src/components/nav-items.tsx`

- [ ] **Step 1: actions.ts**

```ts
"use server";
import { revalidatePath } from "next/cache";
import { getSession } from "@/lib/auth";
import { getPiutang, setPiutang, getHutang, setHutang, nextPayId, appendPembayaran } from "@/lib/kios";
import type { Pembayaran } from "@/lib/types";

function nowWita() {
  const d = new Date(Date.now() + 8 * 3600_000);
  return { tanggal: d.toISOString().slice(0, 10), jam: d.toISOString().slice(11, 19) };
}

export async function recordPaymentAction(ledgerId: string, jumlah: number, metode: string)
  : Promise<{ ok: boolean; error?: string }> {
  const session = await getSession();
  if (!session) return { ok: false, error: "Belum login." };
  if (jumlah <= 0) return { ok: false, error: "Jumlah harus > 0." };

  const isPiu = ledgerId.startsWith("PIU-");
  if (isPiu) {
    const piu = await getPiutang(ledgerId);
    if (!piu) return { ok: false, error: "Tidak ditemukan." };
    if (piu.status !== "terbuka") return { ok: false, error: `Sudah ${piu.status}.` };
    if (jumlah > piu.sisa) return { ok: false, error: "Overpayment." };
    piu.dibayar += jumlah; piu.sisa -= jumlah;
    if (piu.sisa === 0) piu.status = "lunas";
    await setPiutang(piu);
  } else {
    const hut = await getHutang(ledgerId);
    if (!hut) return { ok: false, error: "Tidak ditemukan." };
    if (hut.status !== "terbuka") return { ok: false, error: `Sudah ${hut.status}.` };
    if (jumlah > hut.sisa) return { ok: false, error: "Overpayment." };
    hut.dibayar += jumlah; hut.sisa -= jumlah;
    if (hut.sisa === 0) hut.status = "lunas";
    await setHutang(hut);
  }
  const payId = await nextPayId();
  const { tanggal, jam } = nowWita();
  const pay: Pembayaran = {
    id: payId, ledger_id: ledgerId, jenis: isPiu ? "piutang" : "hutang",
    jumlah, metode: metode || "tunai", tanggal, jam,
    kasir: session.nama, catatan: "",
  };
  await appendPembayaran(pay);
  revalidatePath("/hutang");
  return { ok: true };
}

export async function writeOffAction(ledgerId: string): Promise<{ ok: boolean; error?: string }> {
  const session = await getSession();
  if (!session) return { ok: false, error: "Belum login." };
  if (session.role !== "owner") return { ok: false, error: "Khusus owner." };
  if (ledgerId.startsWith("PIU-")) {
    const piu = await getPiutang(ledgerId);
    if (!piu) return { ok: false, error: "Tidak ditemukan." };
    piu.status = "dihapus"; await setPiutang(piu);
  } else if (ledgerId.startsWith("HUT-")) {
    const hut = await getHutang(ledgerId);
    if (!hut) return { ok: false, error: "Tidak ditemukan." };
    hut.status = "dihapus"; await setHutang(hut);
  } else {
    return { ok: false, error: "ID tidak dikenal." };
  }
  revalidatePath("/hutang");
  return { ok: true };
}
```

- [ ] **Step 2: page.tsx (server component)**

```tsx
export const metadata = { title: "Bon & Hutang" };
import { getAllPiutang, getAllHutang } from "@/lib/kios";
import { getSession } from "@/lib/auth";
import { redirect } from "next/navigation";
import { formatRupiah } from "@/lib/format";

export default async function HutangPage() {
  const session = await getSession();
  if (!session) redirect("/login");
  const [piutang, hutang] = await Promise.all([getAllPiutang(), getAllHutang()]);
  const open = (xs: typeof piutang) => xs.filter((x) => x.status === "terbuka");
  const total = (xs: typeof piutang) => xs.reduce((s, x) => s + x.sisa, 0);
  const openPiu = open(piutang); const openHut = open(hutang);
  return (
    <div className="space-y-8 p-4">
      <h1 className="text-2xl font-bold">Bon & Hutang</h1>
      <section>
        <h2 className="text-lg font-semibold">Piutang Pembeli ({openPiu.length}) — {formatRupiah(total(openPiu))}</h2>
        {openPiu.length === 0 ? <p className="text-sm text-muted-foreground">Tidak ada piutang terbuka.</p> : (
          <ul className="mt-2 space-y-1 text-sm">
            {openPiu.map((p) => (
              <li key={p.id} className="flex gap-4">
                <span className="font-mono">{p.id}</span>
                <span>{p.phone}</span>
                <span>sisa {formatRupiah(p.sisa)}</span>
                <span className="text-muted-foreground">{p.tanggal}</span>
              </li>
            ))}
          </ul>
        )}
      </section>
      <section>
        <h2 className="text-lg font-semibold">Hutang ke Supplier ({openHut.length}) — {formatRupiah(total(openHut))}</h2>
        {openHut.length === 0 ? <p className="text-sm text-muted-foreground">Tidak ada hutang terbuka.</p> : (
          <ul className="mt-2 space-y-1 text-sm">
            {openHut.map((h) => (
              <li key={h.id} className="flex gap-4">
                <span className="font-mono">{h.id}</span>
                <span>{h.supplier_id}</span>
                <span>sisa {formatRupiah(h.sisa)}</span>
                <span className="text-muted-foreground">{h.tanggal}</span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
```

- [ ] **Step 3: nav-items.tsx**

Tambahkan setelah `/pesanan`:
```ts
{ href: "/hutang", label: "Bon & Hutang", icon: BookOpen },
```
Import `BookOpen` dari lucide-react.

- [ ] **Step 4: Typecheck**

```bash
cd kios-dashboard && npm run typecheck 2>&1 | tail -5; cd ..
```

- [ ] **Step 5: Commit**

```bash
git add kios-dashboard/src/app/(app)/hutang/ kios-dashboard/src/components/nav-items.tsx
git commit -m "feat(dashboard): halaman Bon & Hutang — piutang+hutang terbuka, recordPaymentAction"
```

---

## Self-Review

### 1. Spec Coverage (§4.1)
| Requirement | Task |
|---|---|
| Tool kios_bon: jual_bon, bayar, catat_hutang_supplier, hapus, daftar_piutang, daftar_hutang, detail | Task 2 |
| Slash /utang, /hutang, /bayar, /jualutang | Task 6 |
| Jual kredit via performJual metode="bon" + buka Piutang + bump TotalUtang | Task 2 |
| Bypass guard bayar<total untuk bon | Task 3 |
| Overpayment ditolak | Task 2 (bayarPiutang/bayarHutang) |
| batalkanTx bon: tanpa cicilan void, ada cicilan → tolak | Task 4 |
| Hapus pelanggan dengan piutang terbuka → diblokir | Task 5 |
| Hapus supplier dengan hutang terbuka → diblokir | Task 5 |
| RBAC: jual_bon+bayar = kasir+owner; hapus+hutang = owner | Task 2 |
| Backup/restore + seq counters | Task 7 |
| TS mirror | Task 1 |
| Dashboard | Task 8 (opsional) |

### 2. Placeholder Scan
Tidak ada TBD/TODO. Semua fungsi punya body, semua test punya assertion.

### 3. Type Consistency
- `NextPiutangID` → "PIU-%04d"; dikonsumsi di `jualBon`, test, dan backup setSeq — konsisten.
- `NextPayID` → "PAY-%04d"; dikonsumsi di `bayarPiutang`, `bayarHutang`, dashboard actions.
- `DelPelangganSafe` vs `DelPelanggan` — plan 1 memiliki `DelPelanggan`; `DelPelangganSafe` adalah additive override untuk caller yang peduli safety. Konsisten.
- `withRole(ctx, role)` helper dibutuhkan di `bon_test.go` — cek dulu apakah sudah ada di `kios_test.go`; jika belum, tambahkan di `bon_test.go`.
- `hashItemsPelanggan` dibutuhkan oleh Plan 1 Task 2 (backup pelanggan). Plan A Task 7 menambahkan `hashItemsPiutang`/`hashItemsHutang` — tidak konflik.
