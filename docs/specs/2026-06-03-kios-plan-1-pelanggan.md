# Plan 1 — Registry Pelanggan Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Membangun registry `Pelanggan` (pembeli) yang menjadi fondasi shared untuk dua fitur: bon/hutang (Plan A, piutang terkait pelanggan) dan storefront (Plan D, checkout wajib nama+WA).

**Architecture:** `Pelanggan` disimpan di Redis HASH `kios:pelanggan` dengan field = no. WA ternormalisasi "62…" (bukan ID counter — WA adalah identitas). ID disimpan sebagai `PLG-<phone>`. Operasi utama adalah `UpsertPelanggan` (find-or-create idempoten saat checkout) + CRUD standar. Backup/restore wajib mencakup key baru. TS mirror di `types.ts`, KEY di `redis.ts`, data-access di `kios.ts`.

**Tech Stack:** Go (`pkg/tools/kios`), Redis (Upstash/miniredis), test table-driven + miniredis (`newTestStore`/`seedProduct` di `kios_test.go`). Mirror TypeScript di `kios-dashboard/src/lib/`.

**Referensi spec:** `docs/specs/2026-06-03-kios-bon-pulsa-bensin-pelanggan-design.md` §3.2 (`Pelanggan`), §4.4 (storefront — kontrak data yang dibutuhkan), §6 (backup), §9 Fase 1.

**Prasyarat:** Plan 0 sudah landing (branch `feat/spec-bon-pulsa-bensin-pelanggan`, commit `11f67b7f`).

**Prasyarat toolchain (jalankan sebelum perintah Go):**
```bash
export GOROOT=$HOME/sdk/go PATH=$HOME/sdk/go/bin:$PATH GOPATH=$HOME/go
```
Perintah test kanonik:
```bash
go test -tags goolm,stdjson ./pkg/tools/kios/...
```

---

## File Structure

| File | Tanggung jawab | Aksi |
|---|---|---|
| `pkg/tools/kios/store.go` | Tambah key constant + struct `Pelanggan` | Modify |
| `pkg/tools/kios/store_pelanggan.go` | **BARU** — CRUD + UpsertPelanggan + NormalizePhone | Create |
| `pkg/tools/kios/backup.go` | Sertakan `Pelanggan` di snapshot | Modify |
| `pkg/tools/kios/pelanggan_test.go` | **BARU** — test unit Plan 1 | Create |
| `kios-dashboard/src/lib/types.ts` | Tambah interface `Pelanggan` | Modify |
| `kios-dashboard/src/lib/redis.ts` | Tambah `KEY.pelanggan` | Modify |
| `kios-dashboard/src/lib/kios.ts` | Tambah data-access `upsertPelanggan`, `getPelanggan`, `getAllPelanggan` | Modify |

---

## Task 1: Struct `Pelanggan` + key constant + NormalizePhone (Go)

**Files:**
- Modify: `pkg/tools/kios/store.go` (struct + key)
- Create: `pkg/tools/kios/store_pelanggan.go` (NormalizePhone + CRUD)
- Create: `pkg/tools/kios/pelanggan_test.go`

- [ ] **Step 1: Tulis test yang gagal**

Buat file `pkg/tools/kios/pelanggan_test.go`:

```go
package kios

import (
	"context"
	"testing"
)

func TestNormalizePhone(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"08123456789", "628123456789"},
		{"+628123456789", "628123456789"},
		{"628123456789", "628123456789"},
		{"8123456789", "628123456789"},
		{"", ""},
		{"abc", ""},
	}
	for _, c := range cases {
		got := NormalizePhone(c.in)
		if got != c.want {
			t.Errorf("NormalizePhone(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestUpsertPelanggan(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Upsert baru → harus membuat record
	p, err := s.UpsertPelanggan(ctx, "Budi", "08123456789")
	if err != nil {
		t.Fatalf("upsert baru: %v", err)
	}
	if p.Phone != "628123456789" {
		t.Errorf("phone=%q want 628123456789", p.Phone)
	}
	if p.ID != "PLG-628123456789" {
		t.Errorf("id=%q want PLG-628123456789", p.ID)
	}
	if p.Nama != "Budi" {
		t.Errorf("nama=%q want Budi", p.Nama)
	}
	if p.TotalPesanan != 1 {
		t.Errorf("TotalPesanan=%d want 1", p.TotalPesanan)
	}

	// Upsert ulang (nomor sama, nama berbeda) → TotalPesanan bertambah, Nama diupdate
	p2, err := s.UpsertPelanggan(ctx, "Budi Santoso", "628123456789")
	if err != nil {
		t.Fatalf("upsert ulang: %v", err)
	}
	if p2.TotalPesanan != 2 {
		t.Errorf("TotalPesanan setelah upsert ulang=%d want 2", p2.TotalPesanan)
	}
	if p2.Nama != "Budi Santoso" {
		t.Errorf("nama setelah update=%q want Budi Santoso", p2.Nama)
	}
	if p2.ID != p.ID {
		t.Errorf("id berubah: %q → %q (tidak boleh)", p.ID, p2.ID)
	}
}

func TestGetPelanggan(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Belum ada → nil
	got, err := s.GetPelanggan(ctx, "628123456789")
	if err != nil {
		t.Fatalf("get kosong: %v", err)
	}
	if got != nil {
		t.Errorf("got non-nil want nil (belum ada)")
	}

	// Setelah upsert → ada
	if _, err := s.UpsertPelanggan(ctx, "Siti", "08123456789"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err = s.GetPelanggan(ctx, "628123456789")
	if err != nil {
		t.Fatalf("get setelah upsert: %v", err)
	}
	if got == nil || got.Nama != "Siti" {
		t.Errorf("got=%v want Siti", got)
	}
}

func TestGetAllPelanggan(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.UpsertPelanggan(ctx, "Adi", "08111111111")   //nolint:errcheck
	s.UpsertPelanggan(ctx, "Budi", "08222222222")  //nolint:errcheck
	s.UpsertPelanggan(ctx, "Cici", "08333333333")  //nolint:errcheck

	all, err := s.GetAllPelanggan(ctx)
	if err != nil {
		t.Fatalf("getall: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("len=%d want 3", len(all))
	}
}

func TestUpsertPelangganInvalidPhone(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, err := s.UpsertPelanggan(ctx, "Ghost", "abc")
	if err == nil {
		t.Error("expected error for invalid phone")
	}
}
```

- [ ] **Step 2: Jalankan test, pastikan GAGAL (kompilasi)**

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run 'TestNormalizePhone|TestUpsertPelanggan|TestGetPelanggan|TestGetAllPelanggan|TestUpsertPelangganInvalidPhone'
```
Expected: FAIL — `NormalizePhone undefined`, `UpsertPelanggan undefined`, dll.

- [ ] **Step 3: Tambah key constant dan struct `Pelanggan` ke `store.go`**

Di `pkg/tools/kios/store.go`, setelah blok `const (...)` key (setelah baris `keyNotifPendingState`), tambahkan:
```go
	keyPelanggan = "kios:pelanggan" // HASH: field = no. WA ternormalisasi; value = Pelanggan JSON
```

Lalu di `store.go`, setelah struct `Pesanan` (sebelum blok `const`), tambahkan:
```go
// Pelanggan adalah pembeli yang terdaftar dari storefront. Diidentifikasi oleh
// nomor WA yang dinormalisasi (format "62..."). Dipakai oleh piutang (bon) dan
// storefront checkout.
type Pelanggan struct {
	ID           string `json:"id"`            // "PLG-<phone>"
	Phone        string `json:"phone"`         // no. WA ternormalisasi "62..." (= HASH field key)
	Nama         string `json:"nama"`
	TotalUtang   int    `json:"total_utang"`   // cache dari piutang terbuka; ditulis oleh bon ledger
	TotalPesanan int    `json:"total_pesanan"` // jumlah order seumur hidup
	TotalBelanja int    `json:"total_belanja"` // total rupiah seumur hidup
	Catatan      string `json:"catatan"`
	CreatedAt    int64  `json:"created_at"`    // unix seconds
	LastOrder    string `json:"last_order"`    // tanggal YYYY-MM-DD terakhir order
}
```

- [ ] **Step 4: Buat `store_pelanggan.go` (NormalizePhone + CRUD + Upsert)**

Buat file `pkg/tools/kios/store_pelanggan.go`:

```go
package kios

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/redis/go-redis/v9"
)

var reDigits = regexp.MustCompile(`\D`)

// NormalizePhone converts an Indonesian mobile number to the canonical "62..."
// format used as the HASH field key in kios:pelanggan.
// Returns "" when the input is empty or doesn't yield a usable number.
func NormalizePhone(raw string) string {
	d := reDigits.ReplaceAllString(strings.TrimSpace(raw), "")
	if d == "" {
		return ""
	}
	if strings.HasPrefix(d, "0") {
		d = "62" + d[1:]
	} else if strings.HasPrefix(d, "8") {
		d = "62" + d
	}
	// Already "62..." or unrecognised format: return as-is.
	// Minimum usable length: 62 (2) + 8 digits = 10 chars.
	if len(d) < 10 {
		return ""
	}
	return d
}

// UpsertPelanggan creates or updates a Pelanggan record keyed by the normalised
// phone number. Calling it on each new order keeps the registry fresh without
// requiring a separate "register" step. Returns the updated record.
func (s *Store) UpsertPelanggan(ctx context.Context, nama, rawPhone string) (*Pelanggan, error) {
	phone := NormalizePhone(rawPhone)
	if phone == "" {
		return nil, fmt.Errorf("nomor WhatsApp tidak valid kak 🙏 (contoh: 08123456789)")
	}

	// Fetch existing or create a new zero-value record.
	existing, err := s.GetPelanggan(ctx, phone)
	if err != nil {
		return nil, err
	}
	now := NowWITA()
	if existing == nil {
		existing = &Pelanggan{
			ID:        "PLG-" + phone,
			Phone:     phone,
			CreatedAt: now.Unix(),
		}
	}

	// Always update with the latest name and timestamp.
	existing.Nama = strings.TrimSpace(nama)
	existing.TotalPesanan++
	existing.LastOrder = now.Format("2006-01-02")

	return existing, s.SetPelanggan(ctx, existing)
}

// SetPelanggan persists a Pelanggan record (field = Phone).
func (s *Store) SetPelanggan(ctx context.Context, p *Pelanggan) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return s.rdb.HSet(ctx, keyPelanggan, p.Phone, string(b)).Err()
}

// GetPelanggan returns the customer keyed by the canonical phone number,
// or nil (not found) when no record exists.
func (s *Store) GetPelanggan(ctx context.Context, phone string) (*Pelanggan, error) {
	val, err := s.rdb.HGet(ctx, keyPelanggan, phone).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p Pelanggan
	if err := json.Unmarshal([]byte(val), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// GetAllPelanggan returns every customer in the registry, unsorted.
func (s *Store) GetAllPelanggan(ctx context.Context) ([]*Pelanggan, error) {
	m, err := s.rdb.HGetAll(ctx, keyPelanggan).Result()
	if err != nil {
		return nil, err
	}
	out := make([]*Pelanggan, 0, len(m))
	for _, v := range m {
		var p Pelanggan
		if err := json.Unmarshal([]byte(v), &p); err == nil {
			out = append(out, &p)
		}
	}
	return out, nil
}

// DelPelanggan removes a customer record (owner-only at the tool layer).
func (s *Store) DelPelanggan(ctx context.Context, phone string) error {
	return s.rdb.HDel(ctx, keyPelanggan, phone).Err()
}
```

- [ ] **Step 5: Jalankan test, pastikan LULUS**

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run 'TestNormalizePhone|TestUpsertPelanggan|TestGetPelanggan|TestGetAllPelanggan|TestUpsertPelangganInvalidPhone'
```
Expected: PASS semua.

- [ ] **Step 6: Jalankan seluruh paket + gofmt**

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/...
gofmt -l pkg/tools/kios/store.go pkg/tools/kios/store_pelanggan.go  # harus tidak ada output
```

- [ ] **Step 7: Commit**

```bash
git add pkg/tools/kios/store.go pkg/tools/kios/store_pelanggan.go pkg/tools/kios/pelanggan_test.go
git commit -m "feat(kios): struct Pelanggan + NormalizePhone + CRUD UpsertPelanggan"
```

---

## Task 2: Backup/restore `kios:pelanggan`

`Pelanggan` adalah data primer (bukan cache) — harus ikut backup atau hilang saat restore/wipe.

**Files:**
- Modify: `pkg/tools/kios/backup.go`
- Modify: `pkg/tools/kios/pelanggan_test.go`

- [ ] **Step 1: Tulis test round-trip yang gagal**

Tambahkan di akhir `pkg/tools/kios/pelanggan_test.go`:

```go
func TestBackupRestorePelanggan(t *testing.T) {
	ctx := context.Background()
	src := newTestStore(t)

	if _, err := src.UpsertPelanggan(ctx, "Rina", "08512345678"); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	b, err := BuildBackup(ctx, src)
	if err != nil {
		t.Fatalf("build backup: %v", err)
	}
	if len(b.Pelanggan) != 1 {
		t.Fatalf("pelanggan in backup=%d want 1", len(b.Pelanggan))
	}

	dst := newTestStore(t)
	if err := dst.RestoreBackup(ctx, b); err != nil {
		t.Fatalf("restore: %v", err)
	}

	phone := NormalizePhone("08512345678")
	got, err := dst.GetPelanggan(ctx, phone)
	if err != nil {
		t.Fatalf("get setelah restore: %v", err)
	}
	if got == nil || got.Nama != "Rina" {
		t.Errorf("restored pelanggan=%v want Rina", got)
	}
}
```

- [ ] **Step 2: Jalankan, pastikan GAGAL**

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run TestBackupRestorePelanggan
```
Expected: FAIL — `b.Pelanggan undefined`.

- [ ] **Step 3: Tambah field ke `BackupData` struct di `backup.go`**

Di `backup.go`, dalam struct `BackupData` (setelah field `HargaSupplier`), tambahkan:

```go
	Pelanggan []*Pelanggan `json:"pelanggan,omitempty"`
```

- [ ] **Step 4: Isi di `BuildBackup`**

Di fungsi `BuildBackup`, sebelum `return b, nil` (setelah blok `GetAllHargaSupplier`), tambahkan:

```go
	if b.Pelanggan, err = store.GetAllPelanggan(ctx); err != nil {
		return nil, err
	}
```

- [ ] **Step 5: Tampilkan di `Ringkas`**

Di method `Ringkas`, tambahkan `, %d pelanggan` di akhir format string + `len(b.Pelanggan)` sebagai argumen terakhir. Hasil:

```go
func (b *BackupData) Ringkas() string {
	return fmt.Sprintf("%d produk, %d transaksi, %d pembelian, %d riwayat harga, %d supplier, %d promo, %d pustaka, %d pengguna, %d harga supplier, %d pelanggan",
		len(b.Produk), len(b.Transaksi), len(b.Pembelian), len(b.PriceHistory),
		len(b.Supplier), len(b.Promo), len(b.Pustaka), len(b.Users), len(b.HargaSupplier), len(b.Pelanggan))
}
```

- [ ] **Step 6: Pulihkan di `RestoreBackup`**

Di `backup.go` fungsi `RestoreBackup`:

a) Tambahkan `keyPelanggan` ke slice `keys` yang di-`Del` (di akhir slice):
```go
		keyHargaSupplier,
		keyPelanggan,
```

b) Setelah loop restore `harga_supplier` dan sebelum `if b.Shift != nil {`, tambahkan:

```go
	for _, p := range b.Pelanggan {
		if err := s.SetPelanggan(ctx, p); err != nil {
			return err
		}
	}
```

- [ ] **Step 7: Tambah `keyPelanggan` ke `HasAnyData`**

Di method `HasAnyData`, tambahkan `keyPelanggan` ke slice HASH pertama:

```go
	for _, k := range []string{keyProduk, keySupplier, keyPromo, keyPustaka, keyUsers, keyHargaSupplier, keyPelanggan} {
```

- [ ] **Step 8: Jalankan test + seluruh paket**

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run TestBackupRestorePelanggan
go test -tags goolm,stdjson ./pkg/tools/kios/...
gofmt -l pkg/tools/kios/backup.go
make build 2>&1 | tail -2
```
Expected: semua PASS, gofmt bersih, build sukses.

- [ ] **Step 9: Commit**

```bash
git add pkg/tools/kios/backup.go pkg/tools/kios/pelanggan_test.go
git commit -m "feat(kios): Pelanggan masuk backup/restore + HasAnyData"
```

---

## Task 3: Mirror TypeScript (types.ts + redis.ts + kios.ts)

**Files:**
- Modify: `kios-dashboard/src/lib/types.ts`
- Modify: `kios-dashboard/src/lib/redis.ts`
- Modify: `kios-dashboard/src/lib/kios.ts`

- [ ] **Step 1: Tambah interface `Pelanggan` ke `types.ts`**

Di `kios-dashboard/src/lib/types.ts`, setelah interface `Pesanan` (sekitar line 130), tambahkan:

```ts
export interface Pelanggan {
  id: string;            // "PLG-<phone>"
  phone: string;         // no. WA ternormalisasi "62..." (= HASH field key)
  nama: string;
  total_utang: number;   // cache dari piutang terbuka; ditulis oleh bon ledger
  total_pesanan: number;
  total_belanja: number;
  catatan: string;
  created_at: number;    // unix seconds
  last_order: string;    // "YYYY-MM-DD"
}
```

- [ ] **Step 2: Tambah key ke `redis.ts`**

Di `kios-dashboard/src/lib/redis.ts`, di dalam `export const KEY = {`, tambahkan setelah `hargaSupplier`:

```ts
  pelanggan: "kios:pelanggan",
```

- [ ] **Step 3: Tambah data-access ke `kios.ts`**

Di `kios-dashboard/src/lib/kios.ts`, tambahkan import `Pelanggan` dari `./types` (sesuai import lain di file yang sudah ada, mis. `import type { ..., Pelanggan } from "./types";`). Lalu tambahkan bagian baru setelah bagian `Pesanan`:

```ts
// ── Pelanggan (customer registry) ────────────────────────────────────────────

export async function getPelanggan(phone: string): Promise<Pelanggan | null> {
  return normalize<Pelanggan>(await redis().hget<unknown>(KEY.pelanggan, phone));
}

export async function getAllPelanggan(): Promise<Pelanggan[]> {
  const map = await redis().hgetall<Record<string, unknown>>(KEY.pelanggan);
  if (!map) return [];
  return normalizeList<Pelanggan>(Object.values(map));
}

export async function setPelanggan(p: Pelanggan): Promise<void> {
  await redis().hset(KEY.pelanggan, { [p.phone]: p });
}

export async function upsertPelanggan(
  nama: string,
  rawPhone: string,
): Promise<Pelanggan> {
  const phone = normalizeWaTs(rawPhone);
  if (!phone) throw new Error("Nomor WhatsApp tidak valid");

  const existing = await getPelanggan(phone);
  const now = Math.floor(Date.now() / 1000);
  const today = new Date().toISOString().slice(0, 10);

  const updated: Pelanggan = existing
    ? {
        ...existing,
        nama: nama.trim(),
        total_pesanan: existing.total_pesanan + 1,
        last_order: today,
      }
    : {
        id: `PLG-${phone}`,
        phone,
        nama: nama.trim(),
        total_utang: 0,
        total_pesanan: 1,
        total_belanja: 0,
        catatan: "",
        created_at: now,
        last_order: today,
      };

  await setPelanggan(updated);
  return updated;
}
```

Catatan: `normalizeWaTs` adalah helper kecil yang mengkonversi ke format "62…" — definisikan di atas fungsi `upsertPelanggan` (di `kios.ts`, bukan di `wa.ts`, agar tidak ada circular dep):

```ts
function normalizeWaTs(raw: string): string {
  const d = (raw || "").replace(/\D/g, "");
  if (!d) return "";
  if (d.startsWith("0")) return "62" + d.slice(1);
  if (d.startsWith("8")) return "62" + d;
  if (d.length >= 10) return d; // already "62..."
  return "";
}
```

- [ ] **Step 4: Jalankan TypeScript typecheck**

```bash
cd kios-dashboard && npm run typecheck 2>&1 | tail -5; cd ..
```
Expected: 0 error (atau hanya pre-existing errors yang tidak terkait perubahanmu).

- [ ] **Step 5: Commit**

```bash
git add kios-dashboard/src/lib/types.ts kios-dashboard/src/lib/redis.ts kios-dashboard/src/lib/kios.ts
git commit -m "feat(dashboard): mirror Pelanggan — types + KEY + data-access"
```

---

## Self-Review (dijalankan saat penyusunan)

- **Spec coverage (Fase 1):** `Pelanggan` struct + CRUD + backup/restore (Task 1+2) ✓; mirror TS: `types.ts` + `redis.ts` + `kios.ts` + `upsertPelanggan` + `normalizeWaTs` (Task 3) ✓. Kontrak yang dibutuhkan Plan A (bon/piutang): `UpsertPelanggan`, `GetPelanggan`, `GetAllPelanggan`, `SetPelanggan`, `DelPelanggan` — semua ada. Kontrak yang dibutuhkan Plan D (storefront): `upsertPelanggan` (TS) + `pelanggan_id` di Pesanan — field `pelanggan_id` di `Pesanan` Go struct + TS interface ditambahkan di Plan D (storefront menjadi consumer pertama yang benar-benar menyimpan `pelanggan_id`); Plan 1 hanya menyiapkan tool.
- **Placeholder scan:** tidak ada TBD/TODO; semua kode lengkap.
- **Type consistency:** `NormalizePhone` (Go) dan `normalizeWaTs` (TS) keduanya menghasilkan `"62..."` untuk input yang sama — konsisten di kedua sisi. `UpsertPelanggan` (Go) menambah `TotalPesanan++`; `upsertPelanggan` (TS) menambah `total_pesanan + 1` — sinkron. Backup field `Pelanggan []*Pelanggan` menggunakan `SetPelanggan` yang sudah ada (bukan loop inline baru) — konsisten dengan pola restore `Supplier`/`Promo`/dll.

---

## Catatan untuk plan berikutnya

- **Plan A (Bon/Hutang):** menggunakan `UpsertPelanggan`/`GetPelanggan` saat jual-bon (credit sale). Perlu tambah field `pelanggan_id` ke `Pesanan` di Plan D juga.
- **Plan D (Storefront):** menggunakan `upsertPelanggan` (TS) di `/api/pesanan` route; menambah `pelanggan_id` ke `Pesanan` struct (Go + TS).
- `TotalUtang` pada `Pelanggan` **hanya ditulis oleh Plan A** (bon ledger). Plan 1 membuat field-nya; Plan A mengisinya. Read-only di sisi storefront.
