# Produk Sampingan Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Tambah menu Produk Sampingan (pulsa, bensin, solar, minyak tanah) di dashboard dengan CRUD terpisah, integrasi kasir, dan breakdown modal per jenis di laporan.

**Architecture:** Field `jenis` sudah ada di struct `Produk` di Go dan TypeScript. Kita expose field ini via halaman `/produk-sampingan` baru, update `recordSale` agar mencatat jenis di field `catatan` transaksi, lalu parse catatan tersebut di laporan untuk breakdown modal per jenis.

**Tech Stack:** Next.js 15 App Router, Server Actions, TypeScript, Upstash Redis via `@/lib/kios`, Tailwind CSS, lucide-react, node:test untuk unit test.

---

## File Map

**Baru:**
- `kios-dashboard/src/app/(app)/produk-sampingan/page.tsx` — Server Component, filter + render tabel
- `kios-dashboard/src/app/(app)/produk-sampingan/actions.ts` — Server Actions CRUD
- `kios-dashboard/src/components/produk-sampingan/sampingan-table.tsx` — tabel + badge jenis
- `kios-dashboard/src/components/produk-sampingan/sampingan-form.tsx` — form kondisional per jenis

**Diubah:**
- `kios-dashboard/src/lib/analytics.ts` — tambah `jenisFromCatatan`, `ModalPerJenis`, `modalPerJenis`
- `kios-dashboard/src/lib/sales.ts` — `SaleLine` + `jenis` fields + logika pulsa/bensin di `recordSale`
- `kios-dashboard/src/lib/types.ts` — update komentar field `jenis`
- `kios-dashboard/src/components/nav-items.tsx` — tambah nav item Produk Sampingan
- `kios-dashboard/src/app/(app)/produk/page.tsx` — filter hanya produk biasa
- `kios-dashboard/src/components/kasir/kasir-form.tsx` — badge jenis + catatan pulsa di struk
- `kios-dashboard/src/components/laporan/laporan-view.tsx` — breakdown modal per jenis
- `kios-dashboard/src/app/(app)/dashboard/page.tsx` — KPI card modal sampingan

---

## Task 1: analytics.ts — jenisFromCatatan + modalPerJenis

**Files:**
- Modify: `kios-dashboard/src/lib/analytics.ts`
- Create: `kios-dashboard/src/lib/analytics.test.ts`

- [ ] **Step 1.1: Tulis failing test**

Buat file `kios-dashboard/src/lib/analytics.test.ts`:

```ts
import test from "node:test";
import assert from "node:assert/strict";
import { jenisFromCatatan, modalPerJenis } from "./analytics.ts";
import type { Transaksi, Produk } from "./types.ts";

test("jenisFromCatatan: returns jenis from bracket", () => {
  assert.equal(jenisFromCatatan("via dashboard [pulsa]"), "pulsa");
  assert.equal(jenisFromCatatan("via dashboard [bensin]"), "bensin");
  assert.equal(jenisFromCatatan("via dashboard [solar]"), "solar");
  assert.equal(jenisFromCatatan("via dashboard [minyak_tanah]"), "minyak_tanah");
});

test("jenisFromCatatan: returns biasa when no bracket", () => {
  assert.equal(jenisFromCatatan("via dashboard"), "biasa");
  assert.equal(jenisFromCatatan("via bot"), "biasa");
  assert.equal(jenisFromCatatan(""), "biasa");
});

test("modalPerJenis: splits modal by jenis from catatan", () => {
  const produk: Produk[] = [
    { id: "P1", nama: "Beras", kategori: "sembako", satuan: "kg", stok: 10, harga_beli: 10000, harga_jual: 12000, stok_minimum: 5, stok_kritis: 2, supplier: "", last_update: "", has_exp: false, exp_date: "", image_url: "", barcode: "" },
    { id: "P2", nama: "Pulsa", kategori: "pulsa", satuan: "paket", stok: 10, harga_beli: 25000, harga_jual: 27000, stok_minimum: 5, stok_kritis: 2, supplier: "", last_update: "", has_exp: false, exp_date: "", image_url: "", barcode: "", jenis: "pulsa" },
  ];
  const txs: Transaksi[] = [
    { id: "T1", tanggal: "2026-06-05", jam: "10:00:00", produk_id: "P1", nama_produk: "Beras", kategori: "sembako", qty: 2, harga_satuan: 12000, total: 24000, metode_bayar: "tunai", kasir: "admin", catatan: "via dashboard", session_id: "", modal: 20000 },
    { id: "T2", tanggal: "2026-06-05", jam: "10:05:00", produk_id: "P2", nama_produk: "Pulsa", kategori: "pulsa", qty: 1, harga_satuan: 27000, total: 27000, metode_bayar: "transfer", kasir: "admin", catatan: "via dashboard [pulsa]", session_id: "", modal: 25000 },
  ];
  const result = modalPerJenis(txs, produk);
  assert.equal(result.biasa, 20000);
  assert.equal(result.pulsa, 25000);
  assert.equal(result.bensin, 0);
  assert.equal(result.solar, 0);
  assert.equal(result.minyak_tanah, 0);
  assert.equal(result.total, 45000);
});

test("modalPerJenis: falls back to harga_beli when tx.modal not set", () => {
  const produk: Produk[] = [
    { id: "P1", nama: "Beras", kategori: "sembako", satuan: "kg", stok: 10, harga_beli: 10000, harga_jual: 12000, stok_minimum: 5, stok_kritis: 2, supplier: "", last_update: "", has_exp: false, exp_date: "", image_url: "", barcode: "" },
  ];
  const txs: Transaksi[] = [
    { id: "T1", tanggal: "2026-06-05", jam: "10:00:00", produk_id: "P1", nama_produk: "Beras", kategori: "sembako", qty: 3, harga_satuan: 12000, total: 36000, metode_bayar: "tunai", kasir: "admin", catatan: "via bot", session_id: "" },
  ];
  const result = modalPerJenis(txs, produk);
  assert.equal(result.biasa, 30000); // 3 * 10000 fallback
  assert.equal(result.total, 30000);
});
```

- [ ] **Step 1.2: Jalankan test — pastikan FAIL**

```bash
cd kios-dashboard && node --test "src/lib/analytics.test.ts"
```
Expected: error `jenisFromCatatan is not exported` atau `modalPerJenis is not exported`

- [ ] **Step 1.3: Tambah fungsi ke analytics.ts**

Tambahkan di akhir file `kios-dashboard/src/lib/analytics.ts` (setelah `metodeBayarShare`):

```ts
/** Parses the product kind from the catatan field written by recordSale.
 * Format: "via dashboard [<jenis>]" → jenis; anything else → "biasa". */
export function jenisFromCatatan(catatan: string): string {
  const m = catatan.match(/\[(\w+)\]/);
  return m ? m[1] : "biasa";
}

export interface ModalPerJenis {
  biasa: number;
  pulsa: number;
  bensin: number;
  solar: number;
  minyak_tanah: number;
  total: number;
}

/** Modal breakdown per product kind using tx.modal when set (accurate), falling
 * back to qty * current harga_beli for old bot transactions. */
export function modalPerJenis(txs: Transaksi[], produk: Produk[]): ModalPerJenis {
  const beli = new Map<string, number>();
  for (const p of produk) beli.set(p.id, p.harga_beli);

  const result: ModalPerJenis = { biasa: 0, pulsa: 0, bensin: 0, solar: 0, minyak_tanah: 0, total: 0 };
  for (const tx of txs) {
    const m = tx.modal && tx.modal > 0 ? tx.modal : tx.qty * (beli.get(tx.produk_id) ?? 0);
    const jenis = jenisFromCatatan(tx.catatan);
    if (jenis === "pulsa" || jenis === "bensin" || jenis === "solar" || jenis === "minyak_tanah") {
      result[jenis] += m;
    } else {
      result.biasa += m;
    }
    result.total += m;
  }
  return result;
}
```

- [ ] **Step 1.4: Jalankan test — pastikan PASS**

```bash
cd kios-dashboard && node --test "src/lib/analytics.test.ts"
```
Expected: `✔ jenisFromCatatan: returns jenis from bracket`, `✔ modalPerJenis: splits modal by jenis from catatan`, dll.

- [ ] **Step 1.5: Commit**

```bash
git add kios-dashboard/src/lib/analytics.ts kios-dashboard/src/lib/analytics.test.ts
git commit -m "feat(analytics): jenisFromCatatan + modalPerJenis untuk breakdown modal per jenis"
```

---

## Task 2: sales.ts — SaleLine + recordSale

**Files:**
- Modify: `kios-dashboard/src/lib/sales.ts`

- [ ] **Step 2.1: Ganti seluruh isi sales.ts**

Tulis ulang `kios-dashboard/src/lib/sales.ts` dengan konten berikut (logic yang sudah ada dipertahankan, ditambah modal + jenis + saldo_modal untuk pulsa):

```ts
import { getAllProduk, nextTrxId, pushTransaksi, setProduk } from "./kios";
import { timeWITA, todayWITA, formatRupiah } from "./format";
import type { Transaksi } from "./types";

export interface SaleItemInput {
  produkId: string;
  qty: number;
}

export interface SaleLine {
  id: string;
  nama: string;
  qty: number;
  harga: number;
  subtotal: number;
  sisa: number;
  jenis?: string;
  catatan_sampingan?: string;
}

export type SaleResult =
  | { ok: true; total: number; lines: SaleLine[] }
  | { ok: false; error: string };

const METODE = new Set(["tunai", "transfer", "qris"]);

/**
 * Records a multi-item sale: validates all stock first, then commits one
 * Transaksi per product line (matching the bot's per-item model) and decrements
 * stock. Prices are always taken from the server-side product record.
 * Shared by the cashier cart checkout and buyer-order processing.
 */
export async function recordSale(
  rawItems: SaleItemInput[],
  metode: string,
  kasirNama: string,
  catatan = "via dashboard",
): Promise<SaleResult> {
  const wanted = new Map<string, number>();
  for (const it of rawItems) {
    const q = Math.trunc(it.qty);
    if (!it.produkId || !Number.isFinite(q) || q <= 0) continue;
    wanted.set(it.produkId, (wanted.get(it.produkId) ?? 0) + q);
  }
  if (wanted.size === 0) return { ok: false, error: "Keranjang kosong." };

  const all = await getAllProduk();
  const byId = new Map(all.map((p) => [p.id, p]));

  const insufficient: string[] = [];
  for (const [id, q] of wanted) {
    const p = byId.get(id);
    if (!p) return { ok: false, error: `Produk ${id} tidak ditemukan.` };
    if (p.stok < q) insufficient.push(`${p.nama} (minta ${q}, sisa ${p.stok})`);
  }
  if (insufficient.length) {
    return { ok: false, error: `Stok tidak cukup: ${insufficient.join("; ")}.` };
  }

  const m = METODE.has(metode) ? metode : "tunai";
  const today = todayWITA();
  const jam = timeWITA();
  const lines: SaleLine[] = [];
  let total = 0;

  for (const [id, q] of wanted) {
    const p = byId.get(id)!;
    const sub = q * p.harga_jual;
    const modal = p.harga_beli * q;
    const jenis = p.jenis && p.jenis !== "biasa" ? p.jenis : undefined;
    const catatanTx = jenis ? `${catatan} [${jenis}]` : catatan;

    const txId = await nextTrxId();
    const tx: Transaksi = {
      id: txId,
      tanggal: today,
      jam,
      produk_id: p.id,
      nama_produk: p.nama,
      kategori: p.kategori,
      qty: q,
      harga_satuan: p.harga_jual,
      total: sub,
      metode_bayar: m,
      kasir: kasirNama,
      catatan: catatanTx,
      session_id: "",
      modal,
      ...(jenis === "bensin" ? { liter: q } : {}),
    };
    await pushTransaksi(tx);

    p.stok -= q;
    p.last_update = today;

    let catatan_sampingan: string | undefined;
    if (jenis === "pulsa") {
      const debit = p.harga_beli * q;
      p.saldo_modal = Math.max(0, (p.saldo_modal ?? 0) - debit);
      catatan_sampingan = `saldo modal -${formatRupiah(debit)}`;
    } else if (jenis === "bensin") {
      p.stok_ml = Math.max(0, (p.stok_ml ?? 0) - q * 1000);
    }

    await setProduk(p);
    total += sub;
    lines.push({
      id: txId,
      nama: p.nama,
      qty: q,
      harga: p.harga_jual,
      subtotal: sub,
      sisa: p.stok,
      ...(jenis ? { jenis } : {}),
      ...(catatan_sampingan ? { catatan_sampingan } : {}),
    });
  }

  return { ok: true, total, lines };
}
```

- [ ] **Step 2.2: Typecheck**

```bash
cd kios-dashboard && npm run typecheck 2>&1 | head -30
```
Expected: tanpa error di `sales.ts`. Kalau ada error `formatRupiah not found in format`, cek bahwa `formatRupiah` memang ada di `format.ts` (ada di baris 36).

- [ ] **Step 2.3: Commit**

```bash
git add kios-dashboard/src/lib/sales.ts
git commit -m "feat(sales): catat modal + jenis di transaksi, debit saldo_modal pulsa"
```

---

## Task 3: types.ts + nav-items.tsx + produk/page.tsx

**Files:**
- Modify: `kios-dashboard/src/lib/types.ts` (baris 21)
- Modify: `kios-dashboard/src/components/nav-items.tsx`
- Modify: `kios-dashboard/src/app/(app)/produk/page.tsx`

- [ ] **Step 3.1: Update komentar jenis di types.ts**

Di `kios-dashboard/src/lib/types.ts`, baris 21, ubah:
```ts
  jenis?: string;        // "" | "biasa" | "pulsa" | "bensin"
```
Menjadi:
```ts
  jenis?: string;        // "" | "biasa" | "pulsa" | "bensin" | "solar" | "minyak_tanah"
```

- [ ] **Step 3.2: Tambah nav item Produk Sampingan di nav-items.tsx**

Di `kios-dashboard/src/components/nav-items.tsx`, ubah import:
```ts
import {
  LayoutDashboard,
  Package,
  Receipt,
  BarChart3,
  ShoppingCart,
  ClipboardList,
  FileUp,
  Users,
  Settings,
  Truck,
  Contact,
} from "lucide-react";
```
Menjadi:
```ts
import {
  LayoutDashboard,
  Package,
  Receipt,
  BarChart3,
  ShoppingCart,
  ClipboardList,
  FileUp,
  Users,
  Settings,
  Truck,
  Contact,
  Zap,
} from "lucide-react";
```

Lalu di array `NAV_ITEMS`, tambah item setelah `{ href: "/produk", ... }`:
```ts
export const NAV_ITEMS: NavItem[] = [
  { href: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { href: "/kasir", label: "Kasir", icon: ShoppingCart },
  { href: "/pesanan", label: "Pesanan", icon: ClipboardList },
  { href: "/pelanggan", label: "Pelanggan", icon: Contact },
  { href: "/produk", label: "Produk & Stok", icon: Package },
  { href: "/produk-sampingan", label: "Produk Sampingan", icon: Zap },
  { href: "/suplier", label: "Supplier", icon: Truck },
  { href: "/impor", label: "Impor Data", icon: FileUp },
  { href: "/penjualan", label: "Penjualan", icon: Receipt },
  { href: "/laporan", label: "Laporan", icon: BarChart3 },
  { href: "/pengguna", label: "Pengguna", icon: Users, ownerOnly: true },
  { href: "/pengaturan", label: "Pengaturan", icon: Settings, ownerOnly: true },
];
```

- [ ] **Step 3.3: Filter produk biasa di produk/page.tsx**

Di `kios-dashboard/src/app/(app)/produk/page.tsx`, ubah blok try:
```ts
  let produk;
  try {
    produk = await getAllProduk();
  } catch (e) {
```
Menjadi:
```ts
  let produk;
  try {
    const all = await getAllProduk();
    produk = all.filter((p) => !p.jenis || p.jenis === "biasa");
  } catch (e) {
```

- [ ] **Step 3.4: Typecheck**

```bash
cd kios-dashboard && npm run typecheck 2>&1 | head -30
```
Expected: no errors.

- [ ] **Step 3.5: Commit**

```bash
git add kios-dashboard/src/lib/types.ts kios-dashboard/src/components/nav-items.tsx kios-dashboard/src/app/(app)/produk/page.tsx
git commit -m "feat(nav): tambah menu Produk Sampingan, filter produk biasa di halaman /produk"
```

---

## Task 4: produk-sampingan/actions.ts

**Files:**
- Create: `kios-dashboard/src/app/(app)/produk-sampingan/actions.ts`

- [ ] **Step 4.1: Buat actions.ts**

Buat file `kios-dashboard/src/app/(app)/produk-sampingan/actions.ts`:

```ts
"use server";

import { revalidatePath } from "next/cache";
import { getSession } from "@/lib/auth";
import { delProduk, getProduk, nextProdukId, setProduk } from "@/lib/kios";
import { todayWITA } from "@/lib/format";
import type { Produk } from "@/lib/types";

export type JenisSampingan = "pulsa" | "bensin" | "solar" | "minyak_tanah";

export interface SampinganInput {
  id?: string;
  jenis: JenisSampingan;
  nama: string;
  barcode: string;
  kategori: string;
  satuan: string;
  stok: number;
  harga_beli: number;
  harga_jual: number;
  stok_minimum: number;
  stok_kritis: number;
  supplier: string;
  exp_date: string;
  image_url: string;
  pack_defs?: Array<{ nama: string; isi: number }>;
  // pulsa only
  saldo_modal?: number;
  // bensin only
  stok_ml?: number;
  stok_kritis_ml?: number;
}

export type ActionResult = { ok: true; message: string } | { ok: false; error: string };

const VALID_JENIS: JenisSampingan[] = ["pulsa", "bensin", "solar", "minyak_tanah"];

async function ensureOwner(): Promise<ActionResult | null> {
  const session = await getSession();
  if (!session) return { ok: false, error: "Sesi berakhir. Silakan masuk lagi." };
  if (session.role !== "owner") return { ok: false, error: "Aksi ini khusus pemilik (owner)." };
  return null;
}

function num(v: number): number {
  return Number.isFinite(v) ? Math.trunc(v) : 0;
}

function sanitize(input: SampinganInput): ActionResult | null {
  if (!input.nama?.trim()) return { ok: false, error: "Nama produk wajib diisi." };
  if (!VALID_JENIS.includes(input.jenis)) return { ok: false, error: "Jenis produk tidak valid." };
  if (input.harga_jual < 0 || input.harga_beli < 0) return { ok: false, error: "Harga tidak boleh minus." };
  if (input.stok < 0) return { ok: false, error: "Stok tidak boleh minus." };
  const img = input.image_url?.trim() ?? "";
  if (img && !/^(https?:\/\/|data:image\/)/i.test(img)) {
    return { ok: false, error: "Gambar harus berupa URL (http/https) atau hasil unggahan." };
  }
  if (img.length > 600_000) return { ok: false, error: "Gambar terlalu besar." };
  return null;
}

function revalidate() {
  revalidatePath("/produk-sampingan");
  revalidatePath("/dashboard");
  revalidatePath("/kasir");
}

export async function createSampinganAction(input: SampinganInput): Promise<ActionResult> {
  const denied = await ensureOwner();
  if (denied) return denied;
  const invalid = sanitize(input);
  if (invalid) return invalid;

  const id = await nextProdukId();
  const exp = input.exp_date?.trim() ?? "";
  const p: Produk = {
    id,
    barcode: input.barcode?.trim() ?? "",
    nama: input.nama.trim(),
    kategori: input.kategori?.trim() || input.jenis,
    satuan: input.satuan?.trim() || "pcs",
    stok: num(input.stok),
    harga_beli: num(input.harga_beli),
    harga_jual: num(input.harga_jual),
    stok_minimum: num(input.stok_minimum) || 5,
    stok_kritis: num(input.stok_kritis) || 2,
    supplier: input.supplier?.trim() ?? "",
    last_update: todayWITA(),
    has_exp: exp !== "",
    exp_date: exp,
    image_url: input.image_url?.trim() ?? "",
    jenis: input.jenis,
    ...(input.jenis === "pulsa" ? { saldo_modal: num(input.saldo_modal ?? 0) } : {}),
    ...(input.jenis === "bensin"
      ? { stok_ml: num(input.stok_ml ?? 0), stok_kritis_ml: num(input.stok_kritis_ml ?? 40000) }
      : {}),
    ...(input.pack_defs?.length ? { pack_defs: input.pack_defs } : {}),
  };
  await setProduk(p);
  revalidate();
  return { ok: true, message: `Produk sampingan "${p.nama}" ditambahkan (${p.id}).` };
}

export async function updateSampinganAction(input: SampinganInput): Promise<ActionResult> {
  const denied = await ensureOwner();
  if (denied) return denied;
  if (!input.id) return { ok: false, error: "ID produk tidak valid." };
  const invalid = sanitize(input);
  if (invalid) return invalid;

  const existing = await getProduk(input.id);
  if (!existing) return { ok: false, error: "Produk tidak ditemukan." };

  const exp = input.exp_date?.trim() ?? "";
  const p: Produk = {
    ...existing,
    barcode: input.barcode?.trim() ?? "",
    nama: input.nama.trim(),
    kategori: input.kategori?.trim() || input.jenis,
    satuan: input.satuan?.trim() || "pcs",
    stok: num(input.stok),
    harga_beli: num(input.harga_beli),
    harga_jual: num(input.harga_jual),
    stok_minimum: num(input.stok_minimum),
    stok_kritis: num(input.stok_kritis),
    supplier: input.supplier?.trim() ?? "",
    last_update: todayWITA(),
    has_exp: exp !== "",
    exp_date: exp,
    image_url: input.image_url?.trim() ?? "",
    jenis: input.jenis,
    ...(input.jenis === "pulsa" ? { saldo_modal: num(input.saldo_modal ?? existing.saldo_modal ?? 0) } : {}),
    ...(input.jenis === "bensin"
      ? {
          stok_ml: num(input.stok_ml ?? existing.stok_ml ?? 0),
          stok_kritis_ml: num(input.stok_kritis_ml ?? existing.stok_kritis_ml ?? 40000),
        }
      : {}),
    pack_defs: input.pack_defs?.length ? input.pack_defs : existing.pack_defs,
  };
  await setProduk(p);
  revalidate();
  return { ok: true, message: `Produk sampingan "${p.nama}" diperbarui.` };
}

export async function deleteSampinganAction(id: string): Promise<ActionResult> {
  const denied = await ensureOwner();
  if (denied) return denied;
  const existing = await getProduk(id);
  if (!existing) return { ok: false, error: "Produk tidak ditemukan." };
  await delProduk(id);
  revalidate();
  return { ok: true, message: `Produk sampingan "${existing.nama}" dihapus.` };
}
```

- [ ] **Step 4.2: Typecheck**

```bash
cd kios-dashboard && npm run typecheck 2>&1 | head -30
```
Expected: no errors.

- [ ] **Step 4.3: Commit**

```bash
git add kios-dashboard/src/app/(app)/produk-sampingan/actions.ts
git commit -m "feat(produk-sampingan): Server Actions CRUD — create/update/delete"
```

---

## Task 5: sampingan-form.tsx

**Files:**
- Create: `kios-dashboard/src/components/produk-sampingan/sampingan-form.tsx`

- [ ] **Step 5.1: Buat file**

Buat `kios-dashboard/src/components/produk-sampingan/sampingan-form.tsx`:

```tsx
"use client";

import { useRef, useState, useTransition } from "react";
import { Loader2, Upload } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { fileToCompressedDataUrl } from "@/lib/image-upload";
import {
  createSampinganAction,
  updateSampinganAction,
  type ActionResult,
  type JenisSampingan,
  type SampinganInput,
} from "@/app/(app)/produk-sampingan/actions";
import type { Produk } from "@/lib/types";

const JENIS_OPTIONS: { value: JenisSampingan; label: string }[] = [
  { value: "pulsa", label: "Pulsa" },
  { value: "bensin", label: "Bensin" },
  { value: "solar", label: "Solar" },
  { value: "minyak_tanah", label: "Minyak Tanah" },
];

function emptyForm(jenis: JenisSampingan): SampinganInput {
  return {
    jenis,
    nama: "",
    barcode: "",
    kategori: "",
    satuan: "pcs",
    stok: 0,
    harga_beli: 0,
    harga_jual: 0,
    stok_minimum: 5,
    stok_kritis: 2,
    supplier: "",
    exp_date: "",
    image_url: "",
    saldo_modal: 0,
    stok_ml: 0,
    stok_kritis_ml: 40000,
  };
}

function fromProduk(p: Produk): SampinganInput {
  return {
    id: p.id,
    jenis: (p.jenis as JenisSampingan) ?? "pulsa",
    nama: p.nama,
    barcode: p.barcode,
    kategori: p.kategori,
    satuan: p.satuan,
    stok: p.stok,
    harga_beli: p.harga_beli,
    harga_jual: p.harga_jual,
    stok_minimum: p.stok_minimum,
    stok_kritis: p.stok_kritis,
    supplier: p.supplier,
    exp_date: p.exp_date,
    image_url: p.image_url ?? "",
    pack_defs: p.pack_defs ?? [],
    saldo_modal: p.saldo_modal ?? 0,
    stok_ml: p.stok_ml ?? 0,
    stok_kritis_ml: p.stok_kritis_ml ?? 40000,
  };
}

export function SampinganForm({
  produk,
  onResult,
  onCancel,
}: {
  produk?: Produk;
  onResult: (r: ActionResult) => void;
  onCancel: () => void;
}) {
  const [form, setForm] = useState<SampinganInput>(
    produk ? fromProduk(produk) : emptyForm("pulsa"),
  );
  const [error, setError] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);
  const [pending, startTransition] = useTransition();
  const fileRef = useRef<HTMLInputElement>(null);
  const isEdit = Boolean(produk);

  function set<K extends keyof SampinganInput>(key: K, value: SampinganInput[K]) {
    setForm((f) => ({ ...f, [key]: value }));
  }

  async function onPickImage(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    setError(null);
    setUploading(true);
    try {
      set("image_url", await fileToCompressedDataUrl(file, { maxDim: 600, quality: 0.72 }));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal memproses gambar.");
    } finally {
      setUploading(false);
    }
  }

  function numField(key: keyof SampinganInput, label: string, hint?: string) {
    return (
      <div className="space-y-1.5">
        <Label htmlFor={key}>{label}</Label>
        <Input
          id={key}
          type="number"
          inputMode="numeric"
          min={0}
          value={String(form[key] ?? 0)}
          onChange={(e) => set(key, Number(e.target.value) as never)}
          className="font-mono tabular-nums"
        />
        {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
      </div>
    );
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    if (!form.nama.trim()) {
      setError("Nama produk wajib diisi.");
      return;
    }
    startTransition(async () => {
      const result = isEdit
        ? await updateSampinganAction(form)
        : await createSampinganAction(form);
      if (!result.ok) {
        setError(result.error);
        return;
      }
      onResult(result);
    });
  }

  return (
    <form onSubmit={submit} className="space-y-4">
      {/* Jenis — locked on edit */}
      <div className="space-y-1.5">
        <Label htmlFor="jenis">
          Jenis Produk <span className="text-destructive">*</span>
        </Label>
        <Select
          id="jenis"
          value={form.jenis}
          onChange={(e) => !isEdit && set("jenis", e.target.value as JenisSampingan)}
          disabled={isEdit}
        >
          {JENIS_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </Select>
        {isEdit && (
          <p className="text-xs text-muted-foreground">Jenis tidak bisa diubah setelah disimpan.</p>
        )}
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="nama">
          Nama Produk <span className="text-destructive">*</span>
        </Label>
        <Input
          id="nama"
          value={form.nama}
          onChange={(e) => set("nama", e.target.value)}
          placeholder={
            form.jenis === "pulsa"
              ? "mis. Pulsa Telkomsel 25rb"
              : form.jenis === "bensin"
                ? "mis. Bensin Pertalite"
                : form.jenis === "solar"
                  ? "mis. Solar Bio"
                  : "mis. Minyak Tanah"
          }
          required
          autoFocus
        />
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-1.5">
          <Label htmlFor="kategori">Kategori</Label>
          <Input
            id="kategori"
            value={form.kategori}
            onChange={(e) => set("kategori", e.target.value)}
            placeholder={form.jenis}
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="satuan">Satuan</Label>
          <Input
            id="satuan"
            value={form.satuan}
            onChange={(e) => set("satuan", e.target.value)}
            placeholder={form.jenis === "bensin" || form.jenis === "solar" || form.jenis === "minyak_tanah" ? "liter" : "paket"}
          />
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4">
        {numField("harga_beli", "Harga Beli / Modal (Rp)")}
        {numField("harga_jual", "Harga Jual (Rp)")}
      </div>

      {/* Pulsa-specific: saldo_modal */}
      {form.jenis === "pulsa" && (
        <div className="rounded-lg border border-accent/20 bg-accent/5 p-3 space-y-3">
          <p className="text-xs font-medium text-accent">Khusus Pulsa</p>
          {numField("saldo_modal", "Saldo Modal Deposit (Rp)", "Saldo rupiah yang dibeli dari agen pulsa.")}
        </div>
      )}

      {/* Bensin-specific: stok_ml */}
      {form.jenis === "bensin" && (
        <div className="rounded-lg border border-warning/20 bg-warning/5 p-3 space-y-3">
          <p className="text-xs font-medium text-warning">Khusus Bensin</p>
          <div className="grid grid-cols-2 gap-4">
            {numField("stok_ml", "Stok (ml)", "1 liter = 1000 ml")}
            {numField("stok_kritis_ml", "Stok Kritis (ml)", "Default 40000 ml = 40 liter")}
          </div>
        </div>
      )}

      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
        {numField("stok", "Stok")}
        {numField("stok_minimum", "Stok Min")}
        {numField("stok_kritis", "Stok Kritis")}
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-1.5">
          <Label htmlFor="supplier">Supplier</Label>
          <Input
            id="supplier"
            value={form.supplier}
            onChange={(e) => set("supplier", e.target.value)}
            placeholder="opsional"
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="barcode">Barcode</Label>
          <Input
            id="barcode"
            value={form.barcode}
            onChange={(e) => set("barcode", e.target.value)}
            placeholder="opsional"
            className="font-mono"
          />
        </div>
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="image_url">Gambar Produk</Label>
        <div className="flex items-start gap-3">
          {form.image_url.trim() ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={form.image_url} alt="Pratinjau" className="size-16 shrink-0 rounded-lg border object-cover" />
          ) : (
            <div className="flex size-16 shrink-0 items-center justify-center rounded-lg border bg-muted/40 text-muted-foreground">
              <Upload className="size-5" />
            </div>
          )}
          <div className="flex-1 space-y-2">
            <input ref={fileRef} type="file" accept="image/*" className="hidden" onChange={onPickImage} />
            <div className="flex gap-2">
              <Button type="button" variant="outline" size="sm" onClick={() => fileRef.current?.click()} disabled={uploading}>
                {uploading ? <Loader2 className="size-4 animate-spin" /> : <Upload className="size-4" />}
                {form.image_url.trim() ? "Ganti gambar" : "Unggah gambar"}
              </Button>
              {form.image_url.trim() && (
                <Button type="button" variant="ghost" size="sm" onClick={() => set("image_url", "")}>Hapus</Button>
              )}
            </div>
            <Input
              id="image_url"
              value={form.image_url.startsWith("data:") ? "" : form.image_url}
              onChange={(e) => set("image_url", e.target.value)}
              placeholder="atau tempel URL gambar…"
              inputMode="url"
              className="font-mono"
              disabled={form.image_url.startsWith("data:")}
            />
          </div>
        </div>
      </div>

      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}

      <div className="flex justify-end gap-2 pt-2">
        <Button type="button" variant="outline" size="sm" onClick={onCancel} disabled={pending}>Batal</Button>
        <Button type="submit" variant="accent" size="sm" disabled={pending}>
          {pending && <Loader2 className="size-4 animate-spin" />}
          {isEdit ? "Simpan Perubahan" : "Tambah Produk"}
        </Button>
      </div>
    </form>
  );
}
```

- [ ] **Step 5.2: Typecheck**

```bash
cd kios-dashboard && npm run typecheck 2>&1 | head -30
```
Expected: no errors.

- [ ] **Step 5.3: Commit**

```bash
git add kios-dashboard/src/components/produk-sampingan/sampingan-form.tsx
git commit -m "feat(produk-sampingan): SampinganForm — form kondisional per jenis (pulsa/bensin/solar/minyak tanah)"
```

---

## Task 6: sampingan-table.tsx

**Files:**
- Create: `kios-dashboard/src/components/produk-sampingan/sampingan-table.tsx`

- [ ] **Step 6.1: Buat file**

Buat `kios-dashboard/src/components/produk-sampingan/sampingan-table.tsx`:

```tsx
"use client";

import { useMemo, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { CheckCircle2, Loader2, Package, Pencil, Plus, Search, Trash2, TriangleAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input, Select } from "@/components/ui/input";
import { Modal } from "@/components/ui/modal";
import { EmptyState } from "@/components/ui/empty-state";
import { SampinganForm } from "./sampingan-form";
import { deleteSampinganAction, type ActionResult } from "@/app/(app)/produk-sampingan/actions";
import { formatRupiah, formatTanggal } from "@/lib/format";
import { produkImage } from "@/lib/produk-image";
import { STATUS_META, stokStatus } from "@/lib/produk-status";
import { cn, matchesQuery } from "@/lib/utils";
import type { Produk } from "@/lib/types";

type JenisFilter = "" | "pulsa" | "bensin" | "solar" | "minyak_tanah";

const JENIS_META: Record<string, { label: string; variant: "accent" | "warning" | "secondary" | "success" }> = {
  pulsa: { label: "Pulsa", variant: "accent" },
  bensin: { label: "Bensin", variant: "warning" },
  solar: { label: "Solar", variant: "secondary" },
  minyak_tanah: { label: "Minyak Tanah", variant: "success" },
};

export function SampinganTable({
  produk,
  canManage,
}: {
  produk: Produk[];
  canManage: boolean;
}) {
  const router = useRouter();
  const [query, setQuery] = useState("");
  const [jenisFilter, setJenisFilter] = useState<JenisFilter>("");
  const [dialog, setDialog] = useState<{ mode: "add" | "edit"; produk?: Produk } | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Produk | null>(null);
  const [toast, setToast] = useState<ActionResult | null>(null);
  const [pendingDelete, startDelete] = useTransition();

  const rows = useMemo(() => {
    const q = query.trim().toLowerCase();
    return produk.filter((p) => {
      if (jenisFilter && p.jenis !== jenisFilter) return false;
      if (!q) return true;
      return matchesQuery(q, p.nama, p.id, p.barcode);
    });
  }, [produk, query, jenisFilter]);

  function showToast(r: ActionResult) {
    setToast(r);
    window.setTimeout(() => setToast(null), 4000);
  }

  function onMutated(r: ActionResult) {
    setDialog(null);
    showToast(r);
    router.refresh();
  }

  function runDelete() {
    if (!deleteTarget) return;
    const target = deleteTarget;
    startDelete(async () => {
      const r = await deleteSampinganAction(target.id);
      setDeleteTarget(null);
      showToast(r);
      if (r.ok) router.refresh();
    });
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Cari nama, ID, atau barcode…"
            className="pl-9"
            aria-label="Cari produk"
          />
        </div>
        <Select
          value={jenisFilter}
          onChange={(e) => setJenisFilter(e.target.value as JenisFilter)}
          aria-label="Filter jenis"
          className="sm:w-44"
        >
          <option value="">Semua jenis</option>
          <option value="pulsa">Pulsa</option>
          <option value="bensin">Bensin</option>
          <option value="solar">Solar</option>
          <option value="minyak_tanah">Minyak Tanah</option>
        </Select>
        {canManage && (
          <Button variant="accent" size="md" onClick={() => setDialog({ mode: "add" })}>
            <Plus className="size-4" /> Tambah
          </Button>
        )}
      </div>

      <p className="text-xs text-muted-foreground">
        Menampilkan {rows.length} dari {produk.length} produk
      </p>

      {rows.length === 0 ? (
        <EmptyState
          icon={Package}
          title={produk.length === 0 ? "Belum ada produk sampingan" : "Tidak ada hasil"}
          description={
            produk.length === 0
              ? "Tambahkan pulsa, bensin, solar, atau minyak tanah."
              : "Coba ubah kata kunci atau filter."
          }
          action={
            canManage && produk.length === 0 ? (
              <Button variant="accent" size="sm" onClick={() => setDialog({ mode: "add" })}>
                <Plus className="size-4" /> Tambah Produk Sampingan
              </Button>
            ) : undefined
          }
        />
      ) : (
        <div className="overflow-x-auto rounded-xl border bg-card">
          <table className="w-full min-w-[760px] text-sm">
            <thead>
              <tr className="border-b text-left text-xs text-muted-foreground">
                <th className="p-3 font-medium">Produk</th>
                <th className="p-3 font-medium">Jenis</th>
                <th className="p-3 font-medium">Stok</th>
                <th className="p-3 font-medium">Status</th>
                <th className="p-3 text-right font-medium">Harga Beli</th>
                <th className="p-3 text-right font-medium">Harga Jual</th>
                <th className="p-3 font-medium">Update</th>
                {canManage && <th className="p-3 text-right font-medium">Aksi</th>}
              </tr>
            </thead>
            <tbody>
              {rows.map((p) => {
                const st = STATUS_META[stokStatus(p)];
                const jenisMeta = JENIS_META[p.jenis ?? ""] ?? null;
                return (
                  <tr key={p.id} className="border-b transition-colors last:border-0 hover:bg-muted/40">
                    <td className="p-3">
                      <div className="flex items-center gap-3">
                        {/* eslint-disable-next-line @next/next/no-img-element */}
                        <img
                          src={produkImage(p)}
                          alt={p.nama}
                          loading="lazy"
                          className="size-10 shrink-0 rounded-md border bg-muted/40 object-cover"
                        />
                        <div className="min-w-0">
                          <p className="font-medium">{p.nama}</p>
                          <p className="font-mono text-xs text-muted-foreground">
                            {p.id}{p.kategori ? ` · ${p.kategori}` : ""}
                          </p>
                        </div>
                      </div>
                    </td>
                    <td className="p-3">
                      {jenisMeta && <Badge variant={jenisMeta.variant}>{jenisMeta.label}</Badge>}
                    </td>
                    <td className="p-3 font-mono tabular-nums">
                      <div>
                        <span>{p.stok} <span className="text-xs text-muted-foreground">{p.satuan}</span></span>
                        {p.jenis === "pulsa" && p.saldo_modal !== undefined && (
                          <p className="text-xs text-muted-foreground">Saldo: {formatRupiah(p.saldo_modal)}</p>
                        )}
                        {p.jenis === "bensin" && p.stok_ml !== undefined && (
                          <p className="text-xs text-muted-foreground">{(p.stok_ml / 1000).toFixed(1)} L</p>
                        )}
                      </div>
                    </td>
                    <td className="p-3">
                      <Badge variant={st.variant}>{st.label}</Badge>
                    </td>
                    <td className="p-3 text-right font-mono tabular-nums text-muted-foreground">
                      {formatRupiah(p.harga_beli)}
                    </td>
                    <td className="p-3 text-right font-mono tabular-nums">
                      {formatRupiah(p.harga_jual)}
                    </td>
                    <td className="p-3 text-xs text-muted-foreground">
                      {formatTanggal(p.last_update)}
                    </td>
                    {canManage && (
                      <td className="p-3">
                        <div className="flex justify-end gap-1">
                          <button
                            type="button"
                            onClick={() => setDialog({ mode: "edit", produk: p })}
                            aria-label={`Edit ${p.nama}`}
                            className="flex size-8 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
                          >
                            <Pencil className="size-4" />
                          </button>
                          <button
                            type="button"
                            onClick={() => setDeleteTarget(p)}
                            aria-label={`Hapus ${p.nama}`}
                            className="flex size-8 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                          >
                            <Trash2 className="size-4" />
                          </button>
                        </div>
                      </td>
                    )}
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <Modal
        open={dialog !== null}
        onClose={() => setDialog(null)}
        title={dialog?.mode === "edit" ? "Edit Produk Sampingan" : "Tambah Produk Sampingan"}
        description={dialog?.mode === "edit" ? dialog.produk?.nama : "Pilih jenis dan lengkapi detail."}
      >
        {dialog && (
          <SampinganForm produk={dialog.produk} onResult={onMutated} onCancel={() => setDialog(null)} />
        )}
      </Modal>

      <Modal
        open={deleteTarget !== null}
        onClose={() => !pendingDelete && setDeleteTarget(null)}
        title="Hapus produk sampingan?"
        description="Tindakan ini tidak bisa dibatalkan."
        className="max-w-md"
      >
        <div className="space-y-4">
          <p className="text-sm">
            Yakin mau menghapus <span className="font-medium">{deleteTarget?.nama}</span> ({deleteTarget?.id})?
          </p>
          <div className="flex justify-end gap-2">
            <Button variant="outline" size="sm" onClick={() => setDeleteTarget(null)} disabled={pendingDelete}>Batal</Button>
            <Button variant="destructive" size="sm" onClick={runDelete} disabled={pendingDelete}>
              {pendingDelete && <Loader2 className="size-4 animate-spin" />}
              Hapus
            </Button>
          </div>
        </div>
      </Modal>

      {toast && (
        <div
          role="status"
          aria-live="polite"
          className={cn(
            "fixed bottom-4 left-1/2 z-[60] flex -translate-x-1/2 items-center gap-2 rounded-lg border px-4 py-2.5 text-sm shadow-lg",
            toast.ok
              ? "border-success/30 bg-card text-foreground"
              : "border-destructive/30 bg-card text-destructive",
          )}
        >
          {toast.ok ? <CheckCircle2 className="size-4 text-success" /> : <TriangleAlert className="size-4" />}
          {toast.ok ? toast.message : toast.error}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 6.2: Typecheck**

```bash
cd kios-dashboard && npm run typecheck 2>&1 | head -30
```
Expected: no errors.

- [ ] **Step 6.3: Commit**

```bash
git add kios-dashboard/src/components/produk-sampingan/sampingan-table.tsx
git commit -m "feat(produk-sampingan): SampinganTable — tabel dengan badge jenis + filter"
```

---

## Task 7: produk-sampingan/page.tsx

**Files:**
- Create: `kios-dashboard/src/app/(app)/produk-sampingan/page.tsx`

- [ ] **Step 7.1: Buat halaman**

Buat `kios-dashboard/src/app/(app)/produk-sampingan/page.tsx`:

```tsx
import type { Metadata } from "next";
import { getAllProduk } from "@/lib/kios";
import { getSession } from "@/lib/auth";
import { ConnectionError } from "@/components/connection-error";
import { SampinganTable } from "@/components/produk-sampingan/sampingan-table";

export const metadata: Metadata = { title: "Produk Sampingan" };
export const dynamic = "force-dynamic";

const JENIS_SAMPINGAN = new Set(["pulsa", "bensin", "solar", "minyak_tanah"]);

export default async function ProdukSampinganPage() {
  const session = await getSession();
  let produk;
  try {
    const all = await getAllProduk();
    produk = all.filter((p) => JENIS_SAMPINGAN.has(p.jenis ?? ""));
  } catch (e) {
    return <ConnectionError message={e instanceof Error ? e.message : String(e)} />;
  }

  const canManage = session?.role === "owner";

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold">Produk Sampingan</h2>
        <p className="text-sm text-muted-foreground">
          {canManage
            ? "Kelola pulsa, bensin, solar, dan minyak tanah."
            : "Lihat stok produk sampingan. Pengelolaan khusus pemilik."}
        </p>
      </div>
      <SampinganTable produk={produk} canManage={canManage} />
    </div>
  );
}
```

- [ ] **Step 7.2: Typecheck**

```bash
cd kios-dashboard && npm run typecheck 2>&1 | head -30
```
Expected: no errors.

- [ ] **Step 7.3: Commit**

```bash
git add kios-dashboard/src/app/(app)/produk-sampingan/page.tsx
git commit -m "feat(produk-sampingan): halaman /produk-sampingan — Server Component"
```

---

## Task 8: kasir-form.tsx — badge jenis + catatan pulsa

**Files:**
- Modify: `kios-dashboard/src/components/kasir/kasir-form.tsx`

- [ ] **Step 8.1: Tambah JENIS_META dan update product picker**

Di `kios-dashboard/src/components/kasir/kasir-form.tsx`, tambah konstanta `JENIS_META` setelah baris import:

```ts
const JENIS_META: Record<string, { label: string; variant: "accent" | "warning" | "secondary" | "success" }> = {
  pulsa: { label: "Pulsa", variant: "accent" },
  bensin: { label: "Bensin", variant: "warning" },
  solar: { label: "Solar", variant: "secondary" },
  minyak_tanah: { label: "Minyak Tanah", variant: "success" },
};
```

Lalu di dalam `matches.map((p) => { ... })`, ubah bagian nama produk di picker dari:
```tsx
<div className="min-w-0">
  <p className="truncate text-sm font-medium">{p.nama}</p>
  <p className="font-mono text-xs text-muted-foreground">
    {formatRupiah(p.harga_jual)}
  </p>
</div>
```
Menjadi:
```tsx
<div className="min-w-0">
  <div className="flex items-center gap-1.5 flex-wrap">
    <p className="truncate text-sm font-medium">{p.nama}</p>
    {p.jenis && JENIS_META[p.jenis] && (
      <Badge variant={JENIS_META[p.jenis].variant} className="text-[10px] px-1.5 py-0">
        {JENIS_META[p.jenis].label}
      </Badge>
    )}
  </div>
  <p className="font-mono text-xs text-muted-foreground">
    {formatRupiah(p.harga_jual)}
  </p>
</div>
```

- [ ] **Step 8.2: Tambah badge jenis di cart items**

Di dalam `cart.map((l) => { ... })`, ubah nama di cart dari:
```tsx
<p className="truncate text-sm font-medium">{l.produk.nama}</p>
```
Menjadi:
```tsx
<div className="flex items-center gap-1.5 flex-wrap">
  <p className="truncate text-sm font-medium">{l.produk.nama}</p>
  {l.produk.jenis && JENIS_META[l.produk.jenis] && (
    <Badge variant={JENIS_META[l.produk.jenis].variant} className="text-[10px] px-1.5 py-0">
      {JENIS_META[l.produk.jenis].label}
    </Badge>
  )}
</div>
```

- [ ] **Step 8.3: Tampilkan catatan_sampingan di struk**

Di dalam `result.ok` section, setelah `</ul>`, tambahkan sebelum blok kembalian:
```tsx
{result.lines.some((l) => l.catatan_sampingan) && (
  <div className="space-y-0.5 border-t pt-1 mt-1">
    {result.lines
      .filter((l) => l.catatan_sampingan)
      .map((l) => (
        <p key={l.id} className="text-xs text-muted-foreground">
          {l.nama}: {l.catatan_sampingan}
        </p>
      ))}
  </div>
)}
```

Sehingga blok `result.ok` menjadi:
```tsx
<div className="space-y-1">
  <p className="flex items-center gap-1.5 font-medium text-success">
    <CheckCircle2 className="size-4" /> Tercatat — {formatRupiah(result.total)}
  </p>
  <ul className="font-mono text-xs text-foreground">
    {result.lines.map((l) => (
      <li key={l.id}>
        {l.nama} x{l.qty} = {formatRupiah(l.subtotal)} ({l.id})
      </li>
    ))}
  </ul>
  {result.lines.some((l) => l.catatan_sampingan) && (
    <div className="space-y-0.5 border-t pt-1 mt-1">
      {result.lines
        .filter((l) => l.catatan_sampingan)
        .map((l) => (
          <p key={l.id} className="text-xs text-muted-foreground">
            {l.nama}: {l.catatan_sampingan}
          </p>
        ))}
    </div>
  )}
  {result.kembalian !== null && (
    <p className="text-xs text-muted-foreground">
      Kembalian: {formatRupiah(result.kembalian)}
    </p>
  )}
</div>
```

- [ ] **Step 8.4: Typecheck**

```bash
cd kios-dashboard && npm run typecheck 2>&1 | head -30
```
Expected: no errors.

- [ ] **Step 8.5: Commit**

```bash
git add kios-dashboard/src/components/kasir/kasir-form.tsx
git commit -m "feat(kasir): badge jenis produk sampingan + catatan saldo modal di struk"
```

---

## Task 9: laporan-view.tsx — breakdown modal per jenis

**Files:**
- Modify: `kios-dashboard/src/components/laporan/laporan-view.tsx`

- [ ] **Step 9.1: Import modalPerJenis**

Di `kios-dashboard/src/components/laporan/laporan-view.tsx`, tambah `modalPerJenis` ke import dari analytics:
```ts
import {
  hitungLaba,
  metodeBayarShare,
  modalPerJenis,
  omzetHarian,
  produkTerlaris,
} from "@/lib/analytics";
```

- [ ] **Step 9.2: Hitung modalPerJenis di useMemo**

Setelah baris `const metode = useMemo(...)`, tambahkan:
```ts
const modalJenis = useMemo(() => modalPerJenis(txPeriode, produk), [txPeriode, produk]);
```

- [ ] **Step 9.3: Tambah tabel breakdown modal setelah KPI cards**

Setelah `</section>` yang berisi 4 KPI card, tambahkan blok Card berikut:

```tsx
{/* Modal per jenis breakdown */}
<Card>
  <CardHeader>
    <CardTitle>Breakdown Modal</CardTitle>
    <p className="text-xs text-muted-foreground">Modal pokok per kategori produk · {periodeLabel(periode)}</p>
  </CardHeader>
  <CardContent>
    <table className="w-full text-sm">
      <tbody className="divide-y">
        {[
          { label: "Produk Biasa", value: modalJenis.biasa },
          { label: "Pulsa", value: modalJenis.pulsa },
          { label: "Bensin", value: modalJenis.bensin },
          { label: "Solar", value: modalJenis.solar },
          { label: "Minyak Tanah", value: modalJenis.minyak_tanah },
        ]
          .filter((row) => row.value > 0)
          .map((row) => (
            <tr key={row.label}>
              <td className="py-2 text-muted-foreground">{row.label}</td>
              <td className="py-2 text-right font-mono tabular-nums">{formatRupiah(row.value)}</td>
            </tr>
          ))}
        <tr className="font-semibold">
          <td className="py-2 pt-3">Total Modal</td>
          <td className="py-2 pt-3 text-right font-mono tabular-nums">{formatRupiah(modalJenis.total)}</td>
        </tr>
      </tbody>
    </table>
  </CardContent>
</Card>
```

Tambahkan import `Card, CardContent, CardHeader, CardTitle` jika belum ada (sudah ada di file).

- [ ] **Step 9.4: Typecheck**

```bash
cd kios-dashboard && npm run typecheck 2>&1 | head -30
```
Expected: no errors.

- [ ] **Step 9.5: Commit**

```bash
git add kios-dashboard/src/components/laporan/laporan-view.tsx
git commit -m "feat(laporan): breakdown modal per jenis — biasa, pulsa, bensin, solar, minyak tanah"
```

---

## Task 10: dashboard/page.tsx — KPI modal sampingan

**Files:**
- Modify: `kios-dashboard/src/app/(app)/dashboard/page.tsx`

- [ ] **Step 10.1: Import modalPerJenis**

Di `kios-dashboard/src/app/(app)/dashboard/page.tsx`, tambah `modalPerJenis` ke import dari analytics:
```ts
import {
  hitungLaba,
  metodeBayarShare,
  modalPerJenis,
  omzetHarian,
  produkTerlaris,
  stokKritis,
  stokMenipis,
} from "@/lib/analytics";
```

- [ ] **Step 10.2: Hitung modalSampingan hari ini**

Setelah baris `const labaHariIni = hitungLaba(txHariIni, produk);`, tambahkan:
```ts
const modalSampinganHariIni = modalPerJenis(txHariIni, produk);

const LABEL_JENIS: Record<string, string> = {
  pulsa: "Pulsa",
  bensin: "Bensin",
  solar: "Solar",
  minyak_tanah: "Minyak Tanah",
};
const modalSampinganRows = (
  ["pulsa", "bensin", "solar", "minyak_tanah"] as const
).filter((j) => modalSampinganHariIni[j] > 0);
```

- [ ] **Step 10.3: Tambah section KPI modal sampingan**

Setelah `</section>` pada bagian KPI row (section pertama), tambahkan section baru:

```tsx
{/* Modal sampingan — hanya tampil jika ada transaksi hari ini */}
{modalSampinganRows.length > 0 && (
  <section className="rounded-xl border bg-card p-4">
    <p className="mb-3 text-sm font-semibold">Modal Produk Sampingan Hari Ini</p>
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
      {modalSampinganRows.map((jenis) => (
        <div key={jenis} className="space-y-0.5">
          <p className="text-xs text-muted-foreground">{LABEL_JENIS[jenis]}</p>
          <p className="font-mono text-sm font-semibold tabular-nums">
            {formatRupiah(modalSampinganHariIni[jenis])}
          </p>
        </div>
      ))}
    </div>
  </section>
)}
```

- [ ] **Step 10.4: Typecheck**

```bash
cd kios-dashboard && npm run typecheck 2>&1 | head -30
```
Expected: no errors.

- [ ] **Step 10.5: Commit**

```bash
git add kios-dashboard/src/app/(app)/dashboard/page.tsx
git commit -m "feat(dashboard): KPI modal sampingan per jenis — tampil bila ada transaksi hari ini"
```

---

## Task 11: Build final + verifikasi

- [ ] **Step 11.1: Jalankan semua test**

```bash
cd kios-dashboard && node --test "src/**/*.test.ts"
```
Expected: semua test pass, termasuk `analytics.test.ts` yang baru.

- [ ] **Step 11.2: Build production**

```bash
cd kios-dashboard && npm run build 2>&1 | tail -20
```
Expected: build sukses tanpa error. Warning unused vars boleh ada.

- [ ] **Step 11.3: Typecheck final**

```bash
cd kios-dashboard && npm run typecheck 2>&1
```
Expected: no errors.

- [ ] **Step 11.4: Commit final jika ada perubahan**

```bash
git status
```
Jika ada file belum di-commit, commit dengan pesan yang sesuai.

---

## Catatan Implementasi

- **Backward compatibility:** Transaksi lama (dari bot Telegram) tidak punya `[jenis]` di catatan → dianggap `"biasa"` oleh `jenisFromCatatan`. Laporan tidak rusak.
- **Bensin yang dibuat via bot:** Produk dengan `jenis="bensin"` yang sudah ada di Redis (dibuat via bot) akan muncul di `/produk-sampingan` dan bisa diedit via `SampinganForm`. Field `stok_ml` yang sudah ada terbawa.
- **kasir-form.tsx** tidak perlu diubah untuk menerima produk sampingan — halaman kasir sudah memanggil `getAllProduk()` tanpa filter.
- **Urutan commit:** Tiap task menghasilkan commit sendiri. Kalau ada task yang gagal typecheck, jangan commit dulu — fix dulu.
