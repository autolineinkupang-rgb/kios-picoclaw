# Tugas 2 — Menu Supplier Dashboard + Banding Harga (Implementation Plan)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Menghadirkan supplier sebagai menu CRUD di dashboard untuk owner+kasir (hapus owner-only), dengan perbandingan harga hybrid (otomatis dari riwayat pembelian + override manual), tersinkron dua arah dengan bot lewat Redis Upstash bersama.

**Architecture:** Dashboard (Next.js) & bot (Go) adalah dua klien atas satu hash Redis `kios:supplier`. Field baru `pic` dan store override harga `kios:harga_supplier` ditambahkan di KEDUA sisi agar konsisten. ID supplier dibuat dengan skema sama (`SUP-%03d` via `INCR kios:seq:sup`).

**Tech Stack:** Go 1.25 (`go-redis`, miniredis test) + Next.js/TypeScript dashboard (`@upstash/redis` REST), Vitest/`node --test` untuk util TS.

---

## Struktur berkas

**Go (model bersama):**
- `pkg/tools/kios/store_more.go` — tambah `Pic` ke `Supplier`; tambah store override harga.
- `pkg/tools/kios/supplier.go` — param `pic`; longgarkan role tambah/edit; merge override di `bandingHarga`.
- `pkg/tools/kios/store.go` — konstanta `keyHargaSupplier`.
- `pkg/tools/kios/tool_common.go` — helper `requireStaff`.
- `pkg/tools/kios/kios_test.go` — test pic, override, role kasir.

**Dashboard (`kios-dashboard/`):**
- `src/lib/types.ts` — interface `Supplier`.
- `src/lib/redis.ts` — key `suplier`, `seqSup`, `hargaSupplier`.
- `src/lib/kios.ts` — data-access supplier + override harga.
- `src/components/nav-items.tsx` — item menu Supplier (bukan ownerOnly).
- `src/app/(app)/suplier/page.tsx`, `src/app/(app)/suplier/actions.ts`.
- `src/components/suplier/suplier-table.tsx`, `suplier-form.tsx`, `banding-harga.tsx`.

Referensi pola CRUD dashboard (disalin): `produk/page.tsx`, `produk/actions.ts`, `components/produk/produk-table.tsx`, `produk/produk-form.tsx`, `lib/kios.ts` (fungsi produk), `nav-items.tsx` (`navItemsForRole`, `ownerOnly`).

Kontrak data bersama (Redis hash `kios:supplier`, field=ID):
```json
{ "id":"SUP-001","nama":"","kontak":"","alamat":"","produk_utama":"","pic":"","catatan":"" }
```
Override harga (Redis hash `kios:harga_supplier`, field=`"<produk_id>|<nama_supplier>"`): nilai = harga beli (integer, sebagai string).

---

# Bagian A — Model bersama (Go)

## Task 1: Tambah field `pic` ke Supplier

**Files:** Modify `pkg/tools/kios/store_more.go` (struct `Supplier`, baris 12-19), `pkg/tools/kios/supplier.go` (Parameters, tambah/edit/cari), `pkg/tools/kios/kios_test.go`.

- [ ] **Step 1: Tulis test gagal** (tambahkan ke `kios_test.go`)

```go
func TestSupplierTool_PicField(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	tool := NewSupplierTool(s)
	tool.Execute(ctx, map[string]any{"action": "tambah", "nama": "UD Maju", "pic": "Pak Budi"})
	res := tool.Execute(ctx, map[string]any{"action": "cari", "nama": "UD Maju"})
	if !strings.Contains(res.ForLLM, "Pak Budi") {
		t.Errorf("expected PIC 'Pak Budi' in cari output, got: %s", res.ForLLM)
	}
}
```

- [ ] **Step 2: Jalankan — GAGAL**

Run: `go test ./pkg/tools/kios/ -run TestSupplierTool_PicField -v`
Expected: GAGAL (PIC tidak muncul).

- [ ] **Step 3a: Tambah field di struct** (`store_more.go`)

```go
type Supplier struct {
	ID          string `json:"id"`
	Nama        string `json:"nama"`
	Kontak      string `json:"kontak"`
	Alamat      string `json:"alamat"`
	ProdukUtama string `json:"produk_utama"`
	Pic         string `json:"pic"`
	Catatan     string `json:"catatan"`
}
```

- [ ] **Step 3b: Tambah param + isi di `supplier.go`**

Di `Parameters()` properties, tambahkan:
```go
			"pic": map[string]any{"type": "string", "description": "nama PIC/sales"},
```
Di `tambah()`, isi field saat membuat `&Supplier{...}`:
```go
			ProdukUtama: argStr(args, "produk_utama"), Pic: argStr(args, "pic"), Catatan: argStr(args, "catatan"),
```
Di `edit()`, tambahkan blok setelah blok `produk_utama`:
```go
	if v := argStr(args, "pic"); v != "" {
		sup.Pic = v
		changed = append(changed, "pic")
	}
```
Di `cari()`, ubah baris pesan agar memuat PIC:
```go
	msg := fmt.Sprintf("[%s] %s\nKontak: %s\nAlamat: %s\nPIC: %s\nProduk utama: %s\nCatatan: %s",
		sup.ID, sup.Nama, sup.Kontak, sup.Alamat, sup.Pic, sup.ProdukUtama, sup.Catatan)
```

- [ ] **Step 4: Jalankan — LULUS**

Run: `go test ./pkg/tools/kios/ -run TestSupplierTool_PicField -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/tools/kios/store_more.go pkg/tools/kios/supplier.go pkg/tools/kios/kios_test.go
git commit -m "feat(kios): tambah field PIC pada supplier (model bersama bot+dashboard)"
```

---

## Task 2: Store override harga supplier (Go)

**Files:** Modify `pkg/tools/kios/store.go` (konstanta), `pkg/tools/kios/store_more.go` (metode), `pkg/tools/kios/kios_test.go`.

- [ ] **Step 1: Tulis test gagal**

```go
func TestHargaSupplierOverride(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.SetHargaSupplier(ctx, "P1", "UD Maju", 54000); err != nil {
		t.Fatalf("set: %v", err)
	}
	all, err := s.GetAllHargaSupplier(ctx)
	if err != nil {
		t.Fatalf("getall: %v", err)
	}
	if all["P1|UD Maju"] != 54000 {
		t.Errorf("expected 54000, got %d", all["P1|UD Maju"])
	}
}
```

- [ ] **Step 2: Jalankan — GAGAL**

Run: `go test ./pkg/tools/kios/ -run TestHargaSupplierOverride -v`
Expected: GAGAL — `undefined: SetHargaSupplier`.

- [ ] **Step 3a: Konstanta key** (`store.go`, dekat key lain mis. setelah `keySupplier`)

```go
	keyHargaSupplier = "kios:harga_supplier"
```

- [ ] **Step 3b: Metode** (`store_more.go`, tambahkan; pastikan import `strconv` ada — tambahkan bila belum)

```go
// hargaSupplierField membentuk field hash override: "<produk_id>|<nama_supplier>".
func hargaSupplierField(produkID, supplier string) string {
	return produkID + "|" + supplier
}

// GetAllHargaSupplier mengembalikan override harga manual: field -> harga.
func (s *Store) GetAllHargaSupplier(ctx context.Context) (map[string]int, error) {
	m, err := s.rdb.HGetAll(ctx, keyHargaSupplier).Result()
	if err != nil {
		return nil, err
	}
	out := make(map[string]int, len(m))
	for k, v := range m {
		if n, err := strconv.Atoi(v); err == nil {
			out[k] = n
		}
	}
	return out, nil
}

// SetHargaSupplier menyimpan override harga manual untuk (produk, supplier).
func (s *Store) SetHargaSupplier(ctx context.Context, produkID, supplier string, harga int) error {
	return s.rdb.HSet(ctx, keyHargaSupplier, hargaSupplierField(produkID, supplier), strconv.Itoa(harga)).Err()
}

// DelHargaSupplier menghapus override harga manual untuk (produk, supplier).
func (s *Store) DelHargaSupplier(ctx context.Context, produkID, supplier string) error {
	return s.rdb.HDel(ctx, keyHargaSupplier, hargaSupplierField(produkID, supplier)).Err()
}
```

- [ ] **Step 4: Jalankan — LULUS**

Run: `go test ./pkg/tools/kios/ -run TestHargaSupplierOverride -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/tools/kios/store.go pkg/tools/kios/store_more.go pkg/tools/kios/kios_test.go
git commit -m "feat(kios): store override harga supplier (kios:harga_supplier)"
```

---

## Task 3: `bandingHarga` memperhitungkan override

**Files:** Modify `pkg/tools/kios/supplier.go` (`bandingHarga`, baris 197-255), `pkg/tools/kios/kios_test.go`.

- [ ] **Step 1: Tulis test gagal**

```go
func TestBandingHarga_OverrideWins(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_ = s.SetProduk(ctx, &Produk{ID: "P1", Nama: "Beras Medium 5kg", HargaBeli: 55000, Supplier: "Toko Beta"})
	s.AppendPembelian(ctx, &Pembelian{ProdukID: "P1", NamaProduk: "Beras Medium 5kg", HargaBeli: 55000, Supplier: "Toko Beta"})
	// Override manual: UD Maju menawarkan 50000 (termurah)
	_ = s.SetHargaSupplier(ctx, "P1", "UD Maju", 50000)

	res := NewSupplierTool(s).Execute(ctx, map[string]any{"action": "banding_harga", "produk": "Beras Medium 5kg"})
	if !strings.Contains(res.ForLLM, "UD Maju") || !strings.Contains(res.ForLLM, "termurah") {
		t.Errorf("expected UD Maju as termurah via override, got: %s", res.ForLLM)
	}
}
```

- [ ] **Step 2: Jalankan — GAGAL**

Run: `go test ./pkg/tools/kios/ -run TestBandingHarga_OverrideWins -v`
Expected: GAGAL (override belum diperhitungkan).

- [ ] **Step 3: Sisipkan merge override di `bandingHarga`**

Setelah blok yang menambahkan harga produk saat ini (tepat sebelum `if len(best) == 0 {`), sisipkan:

```go
	// Override harga manual diutamakan (mengalahkan riwayat pembelian).
	if produkID != "" {
		if overrides, err := t.store.GetAllHargaSupplier(ctx); err == nil {
			for field, harga := range overrides {
				parts := strings.SplitN(field, "|", 2)
				if len(parts) == 2 && parts[0] == produkID && harga > 0 {
					best[parts[1]] = harga
				}
			}
		}
	}
```

- [ ] **Step 4: Jalankan — LULUS, lalu seluruh paket**

Run: `go test ./pkg/tools/kios/ -run TestBandingHarga_OverrideWins -v`
Expected: PASS.
Run: `go test ./pkg/tools/kios/`
Expected: ok.

- [ ] **Step 5: Commit**

```bash
git add pkg/tools/kios/supplier.go pkg/tools/kios/kios_test.go
git commit -m "feat(kios): banding harga supplier perhitungkan override manual"
```

---

## Task 4: Selaraskan role — kasir boleh tambah/edit supplier

**Files:** Modify `pkg/tools/kios/tool_common.go` (helper baru), `pkg/tools/kios/supplier.go` (gate + Description), `pkg/tools/kios/kios_test.go`.

- [ ] **Step 1: Tulis test gagal**

```go
func TestSupplierTool_KasirCanAddNotDelete(t *testing.T) {
	t.Setenv("KIOS_DEFAULT_ROLE", "kasir")
	s := newTestStore(t)
	tool := NewSupplierTool(s)
	ctx := context.Background()
	if res := tool.Execute(ctx, map[string]any{"action": "tambah", "nama": "UD Sinar"}); res.IsError {
		t.Fatalf("kasir should add supplier, got: %s", res.ForLLM)
	}
	if res := tool.Execute(ctx, map[string]any{"action": "hapus", "nama": "UD Sinar"}); !res.IsError {
		t.Errorf("kasir should NOT delete supplier")
	}
}
```

- [ ] **Step 2: Jalankan — GAGAL**

Run: `go test ./pkg/tools/kios/ -run TestSupplierTool_KasirCanAddNotDelete -v`
Expected: GAGAL — `tambah` masih ditolak untuk kasir (`requireOwner`).

- [ ] **Step 3a: Tambah `requireStaff` di `tool_common.go`** (setelah `requireOwner`, baris ~152)

```go
// requireStaff returns a refusal when role is neither owner nor kasir.
func requireStaff(role string) *tools.ToolResult {
	if role != "owner" && role != "kasir" {
		return tools.ErrorResult("Maaf ya kak 🙏 aksi ini khusus owner/kasir.")
	}
	return nil
}
```

- [ ] **Step 3b: Ubah gate di `supplier.go` `Execute()`**

Pada `case "tambah":` dan `case "edit":`, ganti `requireOwner(role)` menjadi `requireStaff(role)`. Biarkan `case "hapus":` tetap `requireOwner(role)`. Perbarui `Description()`:

```go
	return "Kelola supplier kios: tambah, edit, daftar, cari, hapus, dan bandingkan harga beli " +
		"sebuah produk antar supplier (riwayat pembelian + override). Tambah/edit owner+kasir; hapus khusus owner."
```

- [ ] **Step 4: Jalankan — LULUS, lalu seluruh paket**

Run: `go test ./pkg/tools/kios/ -run TestSupplierTool_KasirCanAddNotDelete -v`
Expected: PASS.
Run: `go test ./pkg/tools/kios/ && go build ./...`
Expected: ok.

- [ ] **Step 5: Commit**

```bash
git add pkg/tools/kios/tool_common.go pkg/tools/kios/supplier.go pkg/tools/kios/kios_test.go
git commit -m "feat(kios): kasir boleh tambah/edit supplier (hapus tetap owner)"
```

---

# Bagian B — Dashboard (Next.js)

> Jalankan perintah dari dalam `kios-dashboard/`. Pasang dependency: `pnpm install --frozen-lockfile`.

## Task 5: Tipe & key Redis dashboard

**Files:** Modify `kios-dashboard/src/lib/types.ts`, `kios-dashboard/src/lib/redis.ts`.

- [ ] **Step 1: Tambah interface `Supplier`** ke `types.ts` (cocok byte-for-byte dengan JSON Go)

```ts
export interface Supplier {
  id: string;
  nama: string;
  kontak: string;
  alamat: string;
  produk_utama: string;
  pic: string;
  catatan: string;
}
```

- [ ] **Step 2: Tambah key** ke objek `KEY` di `redis.ts` (lihat key produk `kios:produk` sebagai pola)

```ts
  suplier: "kios:supplier",
  seqSup: "kios:seq:sup",
  hargaSupplier: "kios:harga_supplier",
```

- [ ] **Step 3: Typecheck**

Run (dari `kios-dashboard/`): `npx tsc --noEmit`
Expected: tanpa error.

- [ ] **Step 4: Commit**

```bash
git add kios-dashboard/src/lib/types.ts kios-dashboard/src/lib/redis.ts
git commit -m "feat(kios-dashboard): tipe Supplier + key redis supplier/harga"
```

---

## Task 6: Data-access supplier di `lib/kios.ts`

**Files:** Modify `kios-dashboard/src/lib/kios.ts` (ikuti pola `getAllProduk/setProduk/delProduk/nextProdukId`).

- [ ] **Step 1: Tulis test util** (`kios-dashboard/src/lib/kios.suplier.test.ts`)

Pakai harness test yang sudah dipakai repo (lihat `src/lib/service-auth.test.ts` dijalankan via `node --test`). Test fokus pada `nextSuplierId` (format `SUP-NNN`, harus identik dengan Go) memakai klien Redis tiruan:

```ts
import test from "node:test";
import assert from "node:assert/strict";
import { formatSuplierId } from "./kios";

test("formatSuplierId pads to SUP-NNN like Go", () => {
  assert.equal(formatSuplierId(1), "SUP-001");
  assert.equal(formatSuplierId(42), "SUP-042");
  assert.equal(formatSuplierId(123), "SUP-123");
});
```

- [ ] **Step 2: Jalankan — GAGAL**

Run (dari `kios-dashboard/`): `node --test src/lib/kios.suplier.test.ts`
Expected: GAGAL — `formatSuplierId` belum ada.

- [ ] **Step 3: Tambah fungsi** ke `kios.ts`

```ts
export function formatSuplierId(n: number): string {
  return "SUP-" + String(n).padStart(3, "0");
}

export async function getAllSuplier(): Promise<Supplier[]> {
  const m = await redis().hgetall(KEY.suplier);
  return normalizeList<Supplier>(m);
}

export async function getSuplier(id: string): Promise<Supplier | null> {
  const v = await redis().hget(KEY.suplier, id);
  return normalize<Supplier>(v);
}

export async function setSuplier(s: Supplier): Promise<void> {
  await redis().hset(KEY.suplier, { [s.id]: s });
}

export async function delSuplier(id: string): Promise<void> {
  await redis().hdel(KEY.suplier, id);
}

export async function nextSuplierId(): Promise<string> {
  const n = await redis().incr(KEY.seqSup);
  return formatSuplierId(n as number);
}

// Override harga manual: field "<produk_id>|<nama_supplier>" -> harga.
export async function getAllHargaSupplier(): Promise<Record<string, number>> {
  const m = (await redis().hgetall(KEY.hargaSupplier)) ?? {};
  const out: Record<string, number> = {};
  for (const [k, v] of Object.entries(m)) {
    const n = typeof v === "number" ? v : parseInt(String(v), 10);
    if (!Number.isNaN(n)) out[k] = n;
  }
  return out;
}

export async function setHargaSupplier(produkId: string, supplier: string, harga: number): Promise<void> {
  await redis().hset(KEY.hargaSupplier, { [`${produkId}|${supplier}`]: harga });
}
```

> Catatan: gunakan helper `normalize`/`normalizeList` dan `redis()` yang sudah ada di `kios.ts` (pola identik dengan produk). Tambahkan `import { Supplier } from "./types"` bila perlu.

- [ ] **Step 4: Jalankan — LULUS + typecheck**

Run: `node --test src/lib/kios.suplier.test.ts`
Expected: PASS.
Run: `npx tsc --noEmit`
Expected: tanpa error.

- [ ] **Step 5: Commit**

```bash
git add kios-dashboard/src/lib/kios.ts kios-dashboard/src/lib/kios.suplier.test.ts
git commit -m "feat(kios-dashboard): data-access supplier + override harga (ID selaras Go)"
```

---

## Task 7: Item menu Supplier (owner + kasir)

**Files:** Modify `kios-dashboard/src/components/nav-items.tsx`.

- [ ] **Step 1: Tambah entri** ke array `NAV_ITEMS` (TANPA `ownerOnly`, agar kasir melihatnya). Pakai ikon dari lib yang sudah diimpor (mis. `Truck` dari `lucide-react`; tambahkan ke import bila perlu).

```tsx
  { href: "/suplier", label: "Supplier", icon: <Truck className="h-5 w-5" /> },
```

- [ ] **Step 2: Typecheck**

Run: `npx tsc --noEmit`
Expected: tanpa error.

- [ ] **Step 3: Commit**

```bash
git add kios-dashboard/src/components/nav-items.tsx
git commit -m "feat(kios-dashboard): menu Supplier untuk owner & kasir"
```

---

## Task 8: Server actions supplier (ensureStaff / ensureOwner)

**Files:** Create `kios-dashboard/src/app/(app)/suplier/actions.ts` (pola `produk/actions.ts`).

- [ ] **Step 1: Buat actions**

Salin struktur `produk/actions.ts`. Definisikan dua gerbang: `ensureStaff` (owner ATAU kasir) untuk create/update, `ensureOwner` (owner saja) untuk delete. Gunakan `getSession()` (lib auth) dengan konvensi return yang sama seperti `ensureOwner` di `produk/actions.ts`.

```ts
"use server";
import { revalidatePath } from "next/cache";
import { getSession } from "@/lib/auth";
import { getAllSuplier, setSuplier, delSuplier, nextSuplierId } from "@/lib/kios";
import type { Supplier } from "@/lib/types";

async function ensureStaff() {
  const session = await getSession();
  if (!session || (session.role !== "owner" && session.role !== "kasir")) {
    return { ok: false as const, error: "Akses ditolak. Khusus owner/kasir." };
  }
  return { ok: true as const, session };
}

async function ensureOwner() {
  const session = await getSession();
  if (!session || session.role !== "owner") {
    return { ok: false as const, error: "Akses ditolak. Khusus owner." };
  }
  return { ok: true as const, session };
}

function sanitize(input: Partial<Supplier>): Supplier | { error: string } {
  const nama = (input.nama ?? "").trim();
  if (!nama) return { error: "Nama supplier wajib diisi." };
  return {
    id: input.id ?? "",
    nama,
    kontak: (input.kontak ?? "").trim(),
    alamat: (input.alamat ?? "").trim(),
    produk_utama: (input.produk_utama ?? "").trim(),
    pic: (input.pic ?? "").trim(),
    catatan: (input.catatan ?? "").trim(),
  };
}

export async function createSuplierAction(input: Partial<Supplier>) {
  const gate = await ensureStaff();
  if (!gate.ok) return gate;
  const s = sanitize(input);
  if ("error" in s) return { ok: false as const, error: s.error };
  s.id = await nextSuplierId();
  await setSuplier(s);
  revalidatePath("/suplier");
  return { ok: true as const };
}

export async function updateSuplierAction(input: Partial<Supplier>) {
  const gate = await ensureStaff();
  if (!gate.ok) return gate;
  if (!input.id) return { ok: false as const, error: "ID supplier tidak ada." };
  const s = sanitize(input);
  if ("error" in s) return { ok: false as const, error: s.error };
  s.id = input.id;
  await setSuplier(s);
  revalidatePath("/suplier");
  return { ok: true as const };
}

export async function deleteSuplierAction(id: string) {
  const gate = await ensureOwner();
  if (!gate.ok) return gate;
  await delSuplier(id);
  revalidatePath("/suplier");
  return { ok: true as const };
}
```

> Jika `getSession` di repo punya nama/return berbeda, samakan dengan yang dipakai `produk/actions.ts` (`ensureOwner`).

- [ ] **Step 2: Typecheck**

Run: `npx tsc --noEmit`
Expected: tanpa error.

- [ ] **Step 3: Commit**

```bash
git add "kios-dashboard/src/app/(app)/suplier/actions.ts"
git commit -m "feat(kios-dashboard): server actions supplier (staff add/edit, owner delete)"
```

---

## Task 9: Halaman + tabel + form supplier

**Files:** Create `kios-dashboard/src/app/(app)/suplier/page.tsx`, `kios-dashboard/src/components/suplier/suplier-table.tsx`, `kios-dashboard/src/components/suplier/suplier-form.tsx`.

- [ ] **Step 1: Salin pola produk**
  - `page.tsx`: salin `produk/page.tsx`. Ganti `getAllProduk()` → `getAllSuplier()`, komponen `<ProdukTable>` → `<SuplierTable>`. Set `canManage = session?.role === "owner" || session?.role === "kasir"` (owner+kasir bisa kelola); `canDelete = session?.role === "owner"`. Teruskan keduanya sebagai props.
  - `suplier-table.tsx`: salin `produk/produk-table.tsx`. Kolom: Nama, Kontak, Alamat, PIC, Produk utama, Catatan. Tombol Edit muncul bila `canManage`; tombol Hapus muncul bila `canDelete`. Hapus memanggil `deleteSuplierAction(id)`.
  - `suplier-form.tsx`: salin `produk/produk-form.tsx`. Field input: `nama` (wajib), `kontak`, `alamat`, `pic`, `produk_utama`, `catatan`. Submit memanggil `createSuplierAction`/`updateSuplierAction`.

- [ ] **Step 2: Typecheck + lint**

Run: `npx tsc --noEmit`
Expected: tanpa error.

- [ ] **Step 3: Verifikasi di browser (golden path)**

Run: `pnpm dev` lalu buka `/suplier`.
- Sebagai owner: tambah, edit, hapus supplier → tersimpan.
- Sebagai kasir: tambah & edit muncul/berfungsi; tombol Hapus TIDAK ada.
- Reload → data persisten.

- [ ] **Step 4: Commit**

```bash
git add "kios-dashboard/src/app/(app)/suplier/page.tsx" kios-dashboard/src/components/suplier/
git commit -m "feat(kios-dashboard): halaman CRUD supplier (owner+kasir, hapus owner-only)"
```

---

## Task 10: View perbandingan harga (hybrid + override manual)

**Files:** Create `kios-dashboard/src/components/suplier/banding-harga.tsx`; Modify `suplier/page.tsx` (sisipkan view/sub-tab); Modify `suplier/actions.ts` (action set override).

- [ ] **Step 1: Tambah action override** ke `suplier/actions.ts`

```ts
import { setHargaSupplier } from "@/lib/kios";

export async function setHargaOverrideAction(produkId: string, supplier: string, harga: number) {
  const gate = await ensureStaff();
  if (!gate.ok) return gate;
  if (!produkId || !supplier || !(harga >= 0)) {
    return { ok: false as const, error: "Data harga override tidak valid." };
  }
  await setHargaSupplier(produkId, supplier, Math.floor(harga));
  revalidatePath("/suplier");
  return { ok: true as const };
}
```

- [ ] **Step 2: Buat view perbandingan** (`banding-harga.tsx`)

Komponen menerima daftar produk (dari `getAllProduk()`), riwayat pembelian (`getAllPembelian()`), dan override (`getAllHargaSupplier()`). Untuk produk terpilih: hitung harga beli terendah per supplier dari pembelian (logika sama dengan Go `bandingHarga`: cocokkan `produk_id`/nama, ambil minimum), lalu **timpa** dengan override `"<produk_id>|<supplier>"` bila ada, urutkan menaik, tandai termurah. Sediakan input untuk owner/kasir mengisi override (memanggil `setHargaOverrideAction`).

- [ ] **Step 3: Sisipkan ke `page.tsx`** sebagai bagian/sub-tab "Banding Harga" pada halaman `/suplier`.

- [ ] **Step 4: Typecheck + verifikasi browser**

Run: `npx tsc --noEmit`
Expected: tanpa error.
Browser: pilih produk → lihat harga per supplier dari pembelian; isi override → muncul sebagai termurah & konsisten dengan hasil bot `/suplier banding` (Task 3).

- [ ] **Step 5: Commit**

```bash
git add "kios-dashboard/src/app/(app)/suplier/actions.ts" kios-dashboard/src/components/suplier/banding-harga.tsx "kios-dashboard/src/app/(app)/suplier/page.tsx"
git commit -m "feat(kios-dashboard): perbandingan harga supplier hybrid + override manual"
```

---

## Task 11: Verifikasi lintas-klien (bot ⇄ dashboard)

- [ ] **Step 1: Bot menulis, dashboard membaca**

Di Telegram (sebagai owner): `/suplier` → tambah supplier (atau via tool). Buka dashboard `/suplier` → supplier yang sama muncul (key `kios:supplier` bersama).

- [ ] **Step 2: Dashboard menulis, bot membaca**

Tambah supplier baru via dashboard. Di Telegram: minta daftar supplier → supplier dashboard ikut tampil.

- [ ] **Step 3: Override konsisten**

Set override harga via dashboard (Task 10). Di Telegram: `/suplier banding <produk>` → harga override ikut diperhitungkan (Task 3) dan hasilnya sama dengan dashboard.

- [ ] **Step 4: Build/test final**

Run: `go build ./... && go test ./pkg/tools/kios/`
Run (dari `kios-dashboard/`): `npx tsc --noEmit`
Expected: semua sukses.

---

## Self-review (sudah dijalankan penulis)

- **Cakupan spec:** field `pic` (Task 1), override store (Task 2), banding harga hybrid (Task 3 Go + Task 10 dashboard), role kasir add/edit + owner delete (Task 4 Go + Task 8 dashboard), tipe/key (Task 5), data-access (Task 6), menu (Task 7), CRUD UI (Task 9), konsistensi lintas-klien (Task 11). ✓
- **Placeholder:** kode konkret untuk seluruh bagian Go & data/tipe/actions dashboard; komponen UI menyalin pola `produk` dengan daftar field/kolom eksplisit (bukan TODO).
- **Konsistensi tipe:** `Supplier` (id,nama,kontak,alamat,produk_utama,pic,catatan) identik di Go & TS; ID `SUP-%03d` selaras (`formatSuplierId` vs Go `NextSupplierID`); field override `"<produk_id>|<nama_supplier>"` sama di Go (`hargaSupplierField`) & TS (`setHargaSupplier`); gerbang `ensureStaff`/`requireStaff` & `ensureOwner`/`requireOwner` konsisten.
