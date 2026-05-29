# Dashboard: Shift, Promo, Harga Pasar — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Tambah UI dashboard untuk mengelola shift kasir (tab di /kasir), promo diskon (halaman /promo), dan melihat harga pasar supplier (kolom di /produk).

**Architecture:** Semua fitur baca/tulis langsung ke Redis yang sama dengan bot Go — tidak ada perubahan backend. Foundation Task 1 menyiapkan shared types/keys/functions; Task 2–4 shift; Task 5–7 promo; Task 8 harga pasar.

**Tech Stack:** Next.js 15 App Router, TypeScript, @upstash/redis, Tailwind CSS, lucide-react. Semua komponen mengikuti pola yang sudah ada di `kios-dashboard/`.

---

### Task 1: Foundation — Tambah Promo type, Redis keys, dan fungsi kios.ts

**Files:**
- Modify: `kios-dashboard/src/lib/types.ts`
- Modify: `kios-dashboard/src/lib/redis.ts`
- Modify: `kios-dashboard/src/lib/kios.ts`

- [ ] **Step 1: Tambah interface `Promo` ke `types.ts`**

Tambahkan di bawah interface `Shift` (setelah baris `status: string; // buka | tutup`):

```typescript
export interface Promo {
  id: string;        // PROMO-NNNN
  produk: string;    // nama produk
  produk_id: string;
  tipe: "persen" | "fixed";
  nilai: number;     // persen (mis. 10) atau nominal rupiah
  min_qty: number;   // min qty agar promo berlaku (default 1)
  aktif: boolean;
  mulai: string;     // YYYY-MM-DD
  selesai: string;   // YYYY-MM-DD
  catatan: string;
}
```

- [ ] **Step 2: Tambah Redis keys ke `redis.ts`**

Tambahkan 3 keys baru ke objek `KEY` (setelah `hargaSupplier`):

```typescript
  promo: "kios:promo",
  seqPromo: "kios:seq:promo",
  shiftHistory: "kios:shift:history",
```

- [ ] **Step 3: Tambah fungsi shift ke `kios.ts`**

Tambahkan setelah fungsi `getShift()`:

```typescript
export async function setShift(s: Shift): Promise<void> {
  await redis().set(KEY.shift, s);
}

export async function clearShift(): Promise<void> {
  await redis().del(KEY.shift);
}

/** Ambil riwayat shift terakhir (newest first). */
export async function getShiftHistory(n = 10): Promise<Shift[]> {
  const vals = await redis().lrange<unknown>(KEY.shiftHistory, -n, -1);
  return normalizeList<Shift>(vals).reverse();
}

export async function pushShiftHistory(s: Shift): Promise<void> {
  await redis().rpush(KEY.shiftHistory, s);
}
```

- [ ] **Step 4: Tambah import `Promo` ke header `kios.ts`**

Ubah baris import types di `kios.ts` dari:

```typescript
import type {
  Produk,
  Transaksi,
  Pembelian,
  PriceHistory,
  Shift,
  UserKios,
  KiosConfig,
  Pesanan,
  Supplier,
} from "./types";
```

menjadi:

```typescript
import type {
  Produk,
  Transaksi,
  Pembelian,
  PriceHistory,
  Promo,
  Shift,
  UserKios,
  KiosConfig,
  Pesanan,
  Supplier,
} from "./types";
```

- [ ] **Step 5: Tambah fungsi promo ke `kios.ts`**

Tambahkan di bagian bawah file, setelah fungsi `bumpLoginAttempts`:

```typescript
// ── Promo ────────────────────────────────────────────────────────────────────

export async function getAllPromo(): Promise<Promo[]> {
  const map = await redis().hgetall<Record<string, unknown>>(KEY.promo);
  if (!map) return [];
  const list = normalizeList<Promo>(Object.values(map));
  list.sort((a, b) => a.id.localeCompare(b.id));
  return list;
}

export async function setPromo(p: Promo): Promise<void> {
  await redis().hset(KEY.promo, { [p.id]: p });
}

export async function deletePromo(id: string): Promise<void> {
  await redis().hdel(KEY.promo, id);
}

/** Next promo ID, mirroring Go: INCR kios:seq:promo -> "PROMO-NNNN". */
export async function nextPromoId(): Promise<string> {
  const n = await redis().incr(KEY.seqPromo);
  return `PROMO-${String(n).padStart(4, "0")}`;
}
```

- [ ] **Step 6: Commit foundation**

```bash
git add kios-dashboard/src/lib/types.ts \
        kios-dashboard/src/lib/redis.ts \
        kios-dashboard/src/lib/kios.ts
git commit -m "feat(dashboard): tambah tipe Promo, Redis keys, dan fungsi shift/promo ke kios.ts"
```

---

### Task 2: Shift Server Actions

**Files:**
- Modify: `kios-dashboard/src/app/(app)/kasir/actions.ts`

- [ ] **Step 1: Tambah shift actions ke `kasir/actions.ts`**

Tambahkan import baru dan dua action di bawah `checkoutAction`:

```typescript
import {
  getShift,
  setShift,
  clearShift,
  pushShiftHistory,
} from "@/lib/kios";
import type { Shift } from "@/lib/types";

export type ShiftResult = { ok: true } | { ok: false; error: string };

export async function bukaShiftAction(
  kasir: string,
  saldoAwal: number,
): Promise<ShiftResult> {
  const session = await getSession();
  if (!session) return { ok: false, error: "Sesi berakhir. Silakan masuk lagi." };

  const existing = await getShift();
  if (existing?.status === "buka") {
    return { ok: false, error: "Ada shift yang sedang buka. Tutup dulu sebelum buka shift baru." };
  }

  const now = new Date().toLocaleString("id-ID", { timeZone: "Asia/Makassar" });
  const shift: Shift = {
    kasir: kasir.trim() || session.nama,
    saldo_awal: Math.floor(saldoAwal),
    saldo_akhir: 0,
    waktu_buka: now,
    waktu_tutup: "",
    status: "buka",
  };
  await setShift(shift);
  revalidatePath("/kasir");
  return { ok: true };
}

export async function tutupShiftAction(saldoAkhir: number): Promise<ShiftResult> {
  const session = await getSession();
  if (!session) return { ok: false, error: "Sesi berakhir. Silakan masuk lagi." };

  const shift = await getShift();
  if (!shift || shift.status !== "buka") {
    return { ok: false, error: "Tidak ada shift yang sedang buka." };
  }

  const now = new Date().toLocaleString("id-ID", { timeZone: "Asia/Makassar" });
  const closed: Shift = {
    ...shift,
    saldo_akhir: Math.floor(saldoAkhir),
    waktu_tutup: now,
    status: "tutup",
  };
  await pushShiftHistory(closed);
  await clearShift();
  revalidatePath("/kasir");
  return { ok: true };
}
```

- [ ] **Step 2: Commit**

```bash
git add kios-dashboard/src/app/\(app\)/kasir/actions.ts
git commit -m "feat(kasir): tambah server actions buka/tutup shift"
```

---

### Task 3: Komponen ShiftTab

**Files:**
- Create: `kios-dashboard/src/components/kasir/shift-tab.tsx`

- [ ] **Step 1: Buat `shift-tab.tsx`**

```typescript
"use client";

import { useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { CheckCircle2, Clock, Loader2, TriangleAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { formatRupiah } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { Shift } from "@/lib/types";
import {
  bukaShiftAction,
  tutupShiftAction,
  type ShiftResult,
} from "@/app/(app)/kasir/actions";

interface Props {
  shift: Shift | null;
  history: Shift[];
}

export function ShiftTab({ shift, history }: Props) {
  const router = useRouter();
  const [pending, start] = useTransition();
  const [toast, setToast] = useState<ShiftResult | null>(null);

  // form state buka shift
  const [kasir, setKasir] = useState("");
  const [saldoAwal, setSaldoAwal] = useState("");
  // form state tutup shift
  const [saldoAkhir, setSaldoAkhir] = useState("");

  function showToast(r: ShiftResult) {
    setToast(r);
    window.setTimeout(() => setToast(null), 4000);
  }

  function handleBuka() {
    const nominal = Number(saldoAwal.replace(/\D/g, "")) || 0;
    start(async () => {
      const r = await bukaShiftAction(kasir, nominal);
      showToast(r);
      if (r.ok) {
        setKasir("");
        setSaldoAwal("");
        router.refresh();
      }
    });
  }

  function handleTutup() {
    const nominal = Number(saldoAkhir.replace(/\D/g, "")) || 0;
    start(async () => {
      const r = await tutupShiftAction(nominal);
      showToast(r);
      if (r.ok) {
        setSaldoAkhir("");
        router.refresh();
      }
    });
  }

  const shiftAktif = shift?.status === "buka";

  return (
    <div className="space-y-6">
      {/* Status shift berjalan */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Clock className="size-4" />
            {shiftAktif ? "Shift Sedang Buka" : "Tidak Ada Shift Aktif"}
          </CardTitle>
        </CardHeader>
        <CardContent>
          {shiftAktif && shift ? (
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-3 text-sm sm:grid-cols-3">
                <div>
                  <p className="text-xs text-muted-foreground">Kasir</p>
                  <p className="font-medium">{shift.kasir}</p>
                </div>
                <div>
                  <p className="text-xs text-muted-foreground">Waktu Buka</p>
                  <p className="font-medium">{shift.waktu_buka}</p>
                </div>
                <div>
                  <p className="text-xs text-muted-foreground">Saldo Awal</p>
                  <p className="font-medium font-mono">{formatRupiah(shift.saldo_awal)}</p>
                </div>
              </div>
              <div className="space-y-1.5 border-t pt-4">
                <Label htmlFor="saldo-akhir">Saldo Akhir (saat tutup)</Label>
                <div className="flex gap-2">
                  <Input
                    id="saldo-akhir"
                    value={saldoAkhir}
                    onChange={(e) => setSaldoAkhir(e.target.value)}
                    placeholder="mis. 500000"
                    inputMode="numeric"
                    className="max-w-xs font-mono"
                  />
                  <Button
                    variant="outline"
                    size="md"
                    onClick={handleTutup}
                    disabled={pending}
                  >
                    {pending ? <Loader2 className="size-4 animate-spin" /> : null}
                    Tutup Shift
                  </Button>
                </div>
              </div>
            </div>
          ) : (
            <div className="space-y-3">
              <div className="grid gap-3 sm:grid-cols-2">
                <div className="space-y-1.5">
                  <Label htmlFor="kasir-nama">Nama Kasir</Label>
                  <Input
                    id="kasir-nama"
                    value={kasir}
                    onChange={(e) => setKasir(e.target.value)}
                    placeholder="Kosong = nama akun login"
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="saldo-awal">Saldo Awal</Label>
                  <Input
                    id="saldo-awal"
                    value={saldoAwal}
                    onChange={(e) => setSaldoAwal(e.target.value)}
                    placeholder="mis. 200000"
                    inputMode="numeric"
                    className="font-mono"
                  />
                </div>
              </div>
              <Button variant="accent" size="md" onClick={handleBuka} disabled={pending}>
                {pending ? <Loader2 className="size-4 animate-spin" /> : null}
                Buka Shift
              </Button>
            </div>
          )}

          {/* Toast */}
          {toast && (
            <p
              className={cn(
                "mt-3 flex items-center gap-1.5 text-sm",
                toast.ok ? "text-success" : "text-destructive",
              )}
            >
              {toast.ok ? (
                <CheckCircle2 className="size-4" />
              ) : (
                <TriangleAlert className="size-4" />
              )}
              {toast.ok ? "Berhasil." : toast.error}
            </p>
          )}
        </CardContent>
      </Card>

      {/* Riwayat shift */}
      {history.length > 0 && (
        <div>
          <h3 className="mb-2 text-sm font-semibold text-muted-foreground">
            Riwayat Shift Terakhir
          </h3>
          <div className="overflow-x-auto rounded-xl border bg-card">
            <table className="w-full min-w-[600px] text-sm">
              <thead>
                <tr className="border-b text-left text-xs text-muted-foreground">
                  <th className="p-3 font-medium">Kasir</th>
                  <th className="p-3 font-medium">Waktu Buka</th>
                  <th className="p-3 font-medium">Waktu Tutup</th>
                  <th className="p-3 text-right font-medium">Saldo Awal</th>
                  <th className="p-3 text-right font-medium">Saldo Akhir</th>
                </tr>
              </thead>
              <tbody>
                {history.map((s, i) => (
                  <tr
                    key={i}
                    className="border-b text-sm last:border-0 hover:bg-muted/40"
                  >
                    <td className="p-3 font-medium">{s.kasir}</td>
                    <td className="p-3 text-muted-foreground">{s.waktu_buka}</td>
                    <td className="p-3 text-muted-foreground">{s.waktu_tutup || "–"}</td>
                    <td className="p-3 text-right font-mono tabular-nums">
                      {formatRupiah(s.saldo_awal)}
                    </td>
                    <td className="p-3 text-right font-mono tabular-nums">
                      {s.saldo_akhir > 0 ? formatRupiah(s.saldo_akhir) : "–"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add kios-dashboard/src/components/kasir/shift-tab.tsx
git commit -m "feat(kasir): komponen ShiftTab dengan buka/tutup shift dan riwayat"
```

---

### Task 4: Update Halaman Kasir — Tambah Tab Shift

**Files:**
- Create: `kios-dashboard/src/components/kasir/kasir-tabs.tsx`
- Modify: `kios-dashboard/src/app/(app)/kasir/page.tsx`

- [ ] **Step 1: Buat wrapper client `kasir-tabs.tsx`**

```typescript
"use client";

import { useState } from "react";
import { cn } from "@/lib/utils";
import type { Produk, Shift } from "@/lib/types";
import { KasirForm } from "./kasir-form";
import { ShiftTab } from "./shift-tab";

type Tab = "transaksi" | "shift";

interface Props {
  produk: Produk[];
  shift: Shift | null;
  shiftHistory: Shift[];
}

export function KasirTabs({ produk, shift, shiftHistory }: Props) {
  const [tab, setTab] = useState<Tab>("transaksi");

  return (
    <div className="space-y-4">
      {/* Tab bar */}
      <div className="flex border-b">
        {(["transaksi", "shift"] as Tab[]).map((t) => (
          <button
            key={t}
            type="button"
            onClick={() => setTab(t)}
            className={cn(
              "-mb-px border-b-2 px-4 py-2 text-sm font-medium capitalize transition-colors",
              tab === t
                ? "border-accent text-accent"
                : "border-transparent text-muted-foreground hover:text-foreground",
            )}
          >
            {t === "transaksi" ? "Transaksi" : "Shift"}
          </button>
        ))}
      </div>

      {tab === "transaksi" && <KasirForm produk={produk} />}
      {tab === "shift" && <ShiftTab shift={shift} history={shiftHistory} />}
    </div>
  );
}
```

- [ ] **Step 2: Update `kasir/page.tsx` untuk fetch shift data dan render KasirTabs**

Ganti seluruh isi file dengan:

```typescript
import type { Metadata } from "next";
import { getAllProduk, getShift, getShiftHistory } from "@/lib/kios";
import { ConnectionError } from "@/components/connection-error";
import { KasirTabs } from "@/components/kasir/kasir-tabs";

export const metadata: Metadata = { title: "Kasir" };
export const dynamic = "force-dynamic";

export default async function KasirPage() {
  let produk, shift, shiftHistory;
  try {
    [produk, shift, shiftHistory] = await Promise.all([
      getAllProduk(),
      getShift(),
      getShiftHistory(10),
    ]);
  } catch (e) {
    return <ConnectionError message={e instanceof Error ? e.message : String(e)} />;
  }

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold">Kasir</h2>
        <p className="text-sm text-muted-foreground">
          Catat penjualan cepat. Stok &amp; laporan otomatis ter-update.
        </p>
      </div>
      <KasirTabs produk={produk} shift={shift} shiftHistory={shiftHistory} />
    </div>
  );
}
```

- [ ] **Step 3: Verifikasi manual**

Jalankan dev server:
```bash
cd kios-dashboard && pnpm dev
```

Buka http://localhost:3000/kasir — pastikan:
- Tab "Transaksi" menampilkan KasirForm seperti sebelumnya
- Tab "Shift" menampilkan form buka shift (jika tidak ada shift aktif)
- Buka shift dengan saldo awal → muncul info shift + form tutup
- Tutup shift → muncul di tabel riwayat

- [ ] **Step 4: Commit**

```bash
git add kios-dashboard/src/components/kasir/kasir-tabs.tsx \
        kios-dashboard/src/app/\(app\)/kasir/page.tsx
git commit -m "feat(kasir): tambah tab Shift dengan buka/tutup shift dan riwayat"
```

---

### Task 5: Promo Server Actions

**Files:**
- Create: `kios-dashboard/src/app/(app)/promo/actions.ts`

- [ ] **Step 1: Buat `promo/actions.ts`**

```typescript
"use server";

import { revalidatePath } from "next/cache";
import { getSession } from "@/lib/auth";
import { getAllPromo, setPromo, deletePromo, nextPromoId } from "@/lib/kios";
import type { Promo } from "@/lib/types";

export type PromoResult = { ok: true } | { ok: false; error: string };

async function ensureStaff() {
  const session = await getSession();
  if (!session || (session.role !== "owner" && session.role !== "kasir")) {
    return { ok: false as const, error: "Akses ditolak." };
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

export interface CreatePromoInput {
  produk_id: string;
  produk: string;
  tipe: "persen" | "fixed";
  nilai: number;
  min_qty: number;
  mulai: string;
  selesai: string;
  catatan: string;
}

export async function createPromoAction(input: CreatePromoInput): Promise<PromoResult> {
  const gate = await ensureStaff();
  if (!gate.ok) return gate;

  if (!input.produk_id || !input.produk) return { ok: false, error: "Produk wajib dipilih." };
  if (input.nilai <= 0) return { ok: false, error: "Nilai diskon harus lebih dari 0." };
  if (input.tipe === "persen" && input.nilai > 100)
    return { ok: false, error: "Diskon persen tidak boleh lebih dari 100." };
  if (!input.mulai || !input.selesai) return { ok: false, error: "Tanggal wajib diisi." };
  if (input.mulai > input.selesai) return { ok: false, error: "Tanggal mulai harus sebelum selesai." };

  const id = await nextPromoId();
  const promo: Promo = {
    id,
    produk: input.produk.trim(),
    produk_id: input.produk_id,
    tipe: input.tipe,
    nilai: Math.abs(input.nilai),
    min_qty: Math.max(1, Math.floor(input.min_qty)),
    aktif: gate.session.role === "owner", // owner → langsung aktif; kasir → menunggu
    mulai: input.mulai,
    selesai: input.selesai,
    catatan: (input.catatan ?? "").trim(),
  };
  await setPromo(promo);
  revalidatePath("/promo");
  return { ok: true };
}

export async function togglePromoAction(id: string): Promise<PromoResult> {
  const gate = await ensureOwner();
  if (!gate.ok) return gate;

  const all = await getAllPromo();
  const promo = all.find((p) => p.id === id);
  if (!promo) return { ok: false, error: "Promo tidak ditemukan." };

  await setPromo({ ...promo, aktif: !promo.aktif });
  revalidatePath("/promo");
  return { ok: true };
}

export async function deletePromoAction(id: string): Promise<PromoResult> {
  const gate = await ensureOwner();
  if (!gate.ok) return gate;

  await deletePromo(id);
  revalidatePath("/promo");
  return { ok: true };
}
```

- [ ] **Step 2: Commit**

```bash
git add kios-dashboard/src/app/\(app\)/promo/actions.ts
git commit -m "feat(promo): server actions create/toggle/delete promo"
```

---

### Task 6: Komponen Promo (Table + Form)

**Files:**
- Create: `kios-dashboard/src/components/promo/promo-form.tsx`
- Create: `kios-dashboard/src/components/promo/promo-table.tsx`

- [ ] **Step 1: Buat `promo-form.tsx`**

```typescript
"use client";

import { useState, useTransition } from "react";
import { CheckCircle2, Loader2, TriangleAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import type { Produk } from "@/lib/types";
import { createPromoAction, type PromoResult } from "@/app/(app)/promo/actions";

interface Props {
  produk: Produk[];
  onResult: (r: PromoResult) => void;
  onCancel: () => void;
}

export function PromoForm({ produk, onResult, onCancel }: Props) {
  const [pending, start] = useTransition();
  const [toast, setToast] = useState<PromoResult | null>(null);

  const today = new Date().toISOString().slice(0, 10);

  const [produkId, setProdukId] = useState("");
  const [tipe, setTipe] = useState<"persen" | "fixed">("persen");
  const [nilai, setNilai] = useState("");
  const [minQty, setMinQty] = useState("1");
  const [mulai, setMulai] = useState(today);
  const [selesai, setSelesai] = useState(today);
  const [catatan, setCatatan] = useState("");

  const selectedProduk = produk.find((p) => p.id === produkId);

  function handleSubmit() {
    start(async () => {
      const r = await createPromoAction({
        produk_id: produkId,
        produk: selectedProduk?.nama ?? "",
        tipe,
        nilai: parseFloat(nilai) || 0,
        min_qty: parseInt(minQty) || 1,
        mulai,
        selesai,
        catatan,
      });
      if (!r.ok) {
        setToast(r);
        window.setTimeout(() => setToast(null), 4000);
      } else {
        onResult(r);
      }
    });
  }

  return (
    <div className="space-y-4">
      <div className="space-y-1.5">
        <Label htmlFor="promo-produk">Produk</Label>
        <Select
          id="promo-produk"
          value={produkId}
          onChange={(e) => setProdukId(e.target.value)}
        >
          <option value="">— Pilih produk —</option>
          {produk.map((p) => (
            <option key={p.id} value={p.id}>
              {p.nama}
            </option>
          ))}
        </Select>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label htmlFor="promo-tipe">Tipe Diskon</Label>
          <Select
            id="promo-tipe"
            value={tipe}
            onChange={(e) => setTipe(e.target.value as "persen" | "fixed")}
          >
            <option value="persen">Persen (%)</option>
            <option value="fixed">Nominal (Rp)</option>
          </Select>
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="promo-nilai">
            Nilai {tipe === "persen" ? "(%)" : "(Rp)"}
          </Label>
          <Input
            id="promo-nilai"
            value={nilai}
            onChange={(e) => setNilai(e.target.value)}
            placeholder={tipe === "persen" ? "mis. 10" : "mis. 5000"}
            inputMode="decimal"
            className="font-mono"
          />
        </div>
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="promo-minqty">Min Qty (agar promo berlaku)</Label>
        <Input
          id="promo-minqty"
          value={minQty}
          onChange={(e) => setMinQty(e.target.value)}
          placeholder="1"
          inputMode="numeric"
          className="max-w-[100px] font-mono"
        />
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label htmlFor="promo-mulai">Mulai</Label>
          <Input
            id="promo-mulai"
            type="date"
            value={mulai}
            onChange={(e) => setMulai(e.target.value)}
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="promo-selesai">Selesai</Label>
          <Input
            id="promo-selesai"
            type="date"
            value={selesai}
            onChange={(e) => setSelesai(e.target.value)}
          />
        </div>
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="promo-catatan">Catatan (opsional)</Label>
        <Input
          id="promo-catatan"
          value={catatan}
          onChange={(e) => setCatatan(e.target.value)}
          placeholder="mis. Promo hari raya"
        />
      </div>

      {toast && !toast.ok && (
        <p className="flex items-center gap-1.5 text-sm text-destructive">
          <TriangleAlert className="size-4" />
          {toast.error}
        </p>
      )}

      <div className="flex justify-end gap-2 pt-2">
        <Button type="button" variant="outline" size="md" onClick={onCancel} disabled={pending}>
          Batal
        </Button>
        <Button variant="accent" size="md" onClick={handleSubmit} disabled={pending || !produkId}>
          {pending ? <Loader2 className="size-4 animate-spin" /> : <CheckCircle2 className="size-4" />}
          Simpan Promo
        </Button>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Buat `promo-table.tsx`**

```typescript
"use client";

import { useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { CheckCircle2, Loader2, Plus, TriangleAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Modal } from "@/components/ui/modal";
import { EmptyState } from "@/components/ui/empty-state";
import { formatRupiah } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { Produk, Promo } from "@/lib/types";
import {
  togglePromoAction,
  deletePromoAction,
  type PromoResult,
} from "@/app/(app)/promo/actions";
import { PromoForm } from "./promo-form";

type PromoStatus = "aktif" | "menunggu" | "nonaktif" | "kedaluwarsa";

function getPromoStatus(p: Promo): PromoStatus {
  const today = new Date().toISOString().slice(0, 10);
  if (p.aktif && p.selesai < today) return "kedaluwarsa";
  if (p.aktif) return "aktif";
  if (p.selesai >= today) return "menunggu";
  return "nonaktif";
}

const STATUS_PROMO: Record<PromoStatus, { label: string; variant: "default" | "destructive" | "outline" | "secondary" }> = {
  aktif: { label: "Aktif", variant: "default" },
  menunggu: { label: "Menunggu", variant: "secondary" },
  nonaktif: { label: "Nonaktif", variant: "outline" },
  kedaluwarsa: { label: "Kedaluwarsa", variant: "destructive" },
};

function formatNilai(p: Promo): string {
  return p.tipe === "persen" ? `${p.nilai}%` : formatRupiah(p.nilai);
}

interface Props {
  promo: Promo[];
  produk: Produk[];
  isOwner: boolean;
}

export function PromoTable({ promo, produk, isOwner }: Props) {
  const router = useRouter();
  const [pending, start] = useTransition();
  const [showForm, setShowForm] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Promo | null>(null);
  const [toast, setToast] = useState<PromoResult | null>(null);

  function showToast(r: PromoResult) {
    setToast(r);
    window.setTimeout(() => setToast(null), 4000);
  }

  function onCreated(r: PromoResult) {
    setShowForm(false);
    showToast(r);
    router.refresh();
  }

  function handleToggle(id: string) {
    start(async () => {
      const r = await togglePromoAction(id);
      showToast(r);
      if (r.ok) router.refresh();
    });
  }

  function handleDelete() {
    if (!deleteTarget) return;
    const id = deleteTarget.id;
    start(async () => {
      const r = await deletePromoAction(id);
      setDeleteTarget(null);
      showToast(r);
      if (r.ok) router.refresh();
    });
  }

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex items-center justify-between">
        <p className="text-xs text-muted-foreground">{promo.length} promo terdaftar</p>
        <Button variant="accent" size="md" onClick={() => setShowForm(true)}>
          <Plus className="size-4" /> Buat Promo
        </Button>
      </div>

      {/* Toast */}
      {toast && (
        <p
          className={cn(
            "flex items-center gap-1.5 text-sm",
            toast.ok ? "text-success" : "text-destructive",
          )}
        >
          {toast.ok ? <CheckCircle2 className="size-4" /> : <TriangleAlert className="size-4" />}
          {toast.ok ? "Berhasil." : toast.error}
        </p>
      )}

      {promo.length === 0 ? (
        <EmptyState
          icon={CheckCircle2}
          title="Belum ada promo"
          description="Buat promo diskon untuk produk kios."
          action={
            <Button variant="accent" size="sm" onClick={() => setShowForm(true)}>
              <Plus className="size-4" /> Buat Promo
            </Button>
          }
        />
      ) : (
        <div className="overflow-x-auto rounded-xl border bg-card">
          <table className="w-full min-w-[700px] text-sm">
            <thead>
              <tr className="border-b text-left text-xs text-muted-foreground">
                <th className="p-3 font-medium">Produk</th>
                <th className="p-3 font-medium">Diskon</th>
                <th className="p-3 font-medium">Min Qty</th>
                <th className="p-3 font-medium">Periode</th>
                <th className="p-3 font-medium">Status</th>
                <th className="p-3 font-medium">Catatan</th>
                {isOwner && <th className="p-3 text-right font-medium">Aksi</th>}
              </tr>
            </thead>
            <tbody>
              {promo.map((p) => {
                const st = getPromoStatus(p);
                const meta = STATUS_PROMO[st];
                return (
                  <tr key={p.id} className="border-b last:border-0 hover:bg-muted/40">
                    <td className="p-3">
                      <p className="font-medium">{p.produk}</p>
                      <p className="font-mono text-xs text-muted-foreground">{p.id}</p>
                    </td>
                    <td className="p-3 font-mono font-medium tabular-nums">
                      {formatNilai(p)}
                    </td>
                    <td className="p-3 text-muted-foreground">{p.min_qty}x</td>
                    <td className="p-3 text-xs text-muted-foreground">
                      {p.mulai} – {p.selesai}
                    </td>
                    <td className="p-3">
                      <Badge variant={meta.variant}>{meta.label}</Badge>
                    </td>
                    <td className="p-3 text-xs text-muted-foreground">
                      {p.catatan || "–"}
                    </td>
                    {isOwner && (
                      <td className="p-3">
                        <div className="flex justify-end gap-1">
                          <button
                            type="button"
                            onClick={() => handleToggle(p.id)}
                            disabled={pending}
                            className="rounded-md px-2 py-1 text-xs font-medium text-muted-foreground hover:bg-muted hover:text-foreground disabled:opacity-50"
                          >
                            {p.aktif ? "Nonaktifkan" : "Aktifkan"}
                          </button>
                          <button
                            type="button"
                            onClick={() => setDeleteTarget(p)}
                            disabled={pending}
                            className="rounded-md px-2 py-1 text-xs font-medium text-muted-foreground hover:bg-destructive/10 hover:text-destructive disabled:opacity-50"
                          >
                            Hapus
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

      {/* Form buat promo */}
      <Modal
        open={showForm}
        onClose={() => setShowForm(false)}
        title="Buat Promo Baru"
        description="Kasir: tersimpan nonaktif, menunggu aktivasi owner. Owner: langsung aktif."
      >
        <PromoForm produk={produk} onResult={onCreated} onCancel={() => setShowForm(false)} />
      </Modal>

      {/* Konfirmasi hapus */}
      <Modal
        open={deleteTarget !== null}
        onClose={() => !pending && setDeleteTarget(null)}
        title="Hapus promo?"
        description="Tindakan ini tidak bisa dibatalkan."
        className="max-w-md"
      >
        <div className="space-y-4">
          <p className="text-sm">
            Yakin mau menghapus promo{" "}
            <span className="font-medium">{deleteTarget?.id}</span> untuk{" "}
            <span className="font-medium">{deleteTarget?.produk}</span>?
          </p>
          <div className="flex justify-end gap-2">
            <Button variant="outline" size="md" onClick={() => setDeleteTarget(null)} disabled={pending}>
              Batal
            </Button>
            <Button variant="destructive" size="md" onClick={handleDelete} disabled={pending}>
              {pending ? <Loader2 className="size-4 animate-spin" /> : null}
              Hapus
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
```

- [ ] **Step 3: Commit**

```bash
git add kios-dashboard/src/components/promo/
git commit -m "feat(promo): komponen PromoForm dan PromoTable"
```

---

### Task 7: Halaman /promo dan Nav Item

**Files:**
- Create: `kios-dashboard/src/app/(app)/promo/page.tsx`
- Modify: `kios-dashboard/src/components/nav-items.tsx`

- [ ] **Step 1: Buat `promo/page.tsx`**

```typescript
import type { Metadata } from "next";
import { getAllPromo, getAllProduk } from "@/lib/kios";
import { getSession } from "@/lib/auth";
import { ConnectionError } from "@/components/connection-error";
import { PromoTable } from "@/components/promo/promo-table";

export const metadata: Metadata = { title: "Promo" };
export const dynamic = "force-dynamic";

export default async function PromoPage() {
  const session = await getSession();
  let promo, produk;
  try {
    [promo, produk] = await Promise.all([getAllPromo(), getAllProduk()]);
  } catch (e) {
    return <ConnectionError message={e instanceof Error ? e.message : String(e)} />;
  }

  const isOwner = session?.role === "owner";

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold">Promo &amp; Diskon</h2>
        <p className="text-sm text-muted-foreground">
          {isOwner
            ? "Kelola promo diskon. Promo dari kasir perlu diaktifkan dulu."
            : "Ajukan promo diskon. Promo aktif setelah disetujui owner."}
        </p>
      </div>
      <PromoTable promo={promo} produk={produk} isOwner={isOwner} />
    </div>
  );
}
```

- [ ] **Step 2: Tambah `/promo` ke `nav-items.tsx`**

Tambahkan import `Tag` dari lucide-react:

```typescript
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
  Tag,
  Truck,
} from "lucide-react";
```

Tambahkan entry promo setelah `/suplier` di `NAV_ITEMS`:

```typescript
  { href: "/promo", label: "Promo", icon: Tag },
```

Sehingga urutan menjadi:
```
/dashboard, /kasir, /pesanan, /produk, /suplier, /promo, /impor, /penjualan, /laporan, /pengguna, /pengaturan
```

- [ ] **Step 3: Verifikasi manual**

Buka http://localhost:3000/promo (login sebagai kasir dan owner):
- Kasir: bisa lihat daftar dan buat promo (tombol "Buat Promo" muncul)
- Kasir: promo yang dibuat muncul dengan badge "Menunggu"
- Owner: tombol "Aktifkan" / "Nonaktifkan" / "Hapus" muncul di tiap baris
- Owner: mengaktifkan promo → badge berubah ke "Aktif"

- [ ] **Step 4: Commit**

```bash
git add kios-dashboard/src/app/\(app\)/promo/ \
        kios-dashboard/src/components/nav-items.tsx
git commit -m "feat(promo): halaman /promo dan tambah ke sidebar nav"
```

---

### Task 8: Kolom Harga Pasar di Tabel Produk

**Files:**
- Modify: `kios-dashboard/src/components/produk/produk-table.tsx`
- Modify: `kios-dashboard/src/app/(app)/produk/page.tsx`

- [ ] **Step 1: Tambah helper `getHargaRange` dan update props `ProdukTable`**

Di `produk-table.tsx`, tambahkan helper function sebelum komponen `ProdukTable`:

```typescript
/** Hitung rentang harga supplier untuk satu produk dari semua entri hargaSupplier. */
function getHargaRange(
  produkId: string,
  hargaSupplier: Record<string, number>,
): { min: number; max: number; count: number } | null {
  const prefix = `${produkId}|`;
  const vals = Object.entries(hargaSupplier)
    .filter(([k]) => k.startsWith(prefix))
    .map(([, v]) => v)
    .filter((v) => v > 0);
  if (vals.length === 0) return null;
  return { min: Math.min(...vals), max: Math.max(...vals), count: vals.length };
}
```

Ubah signature komponen dari:

```typescript
export function ProdukTable({
  produk,
  canManage,
}: {
  produk: Produk[];
  canManage: boolean;
}) {
```

menjadi:

```typescript
export function ProdukTable({
  produk,
  canManage,
  hargaSupplier = {},
}: {
  produk: Produk[];
  canManage: boolean;
  hargaSupplier?: Record<string, number>;
}) {
```

- [ ] **Step 2: Tambah 2 kolom header di tabel**

Di bagian `<thead>`, setelah `<th>` untuk "Harga Jual" dan sebelum `<th>` untuk "Margin":

```tsx
<th className="p-3 text-right font-medium">Pasar Min</th>
<th className="p-3 text-right font-medium">Pasar Max</th>
```

Juga update atribut `min-w` pada table dari `min-w-[760px]` ke `min-w-[900px]`.

- [ ] **Step 3: Tambah 2 sel data di setiap baris**

Di dalam `rows.map((p) => {...})`, setelah `<td>` untuk `{formatRupiah(p.harga_jual)}` dan sebelum `<td>` untuk margin:

```tsx
{(() => {
  const range = getHargaRange(p.id, hargaSupplier);
  return (
    <>
      <td className="p-3 text-right font-mono tabular-nums text-muted-foreground">
        {range ? formatRupiah(range.min) : "–"}
      </td>
      <td className="p-3 text-right font-mono tabular-nums text-muted-foreground">
        {range
          ? range.count > 1
            ? `${formatRupiah(range.max)}`
            : formatRupiah(range.max)
          : "–"}
      </td>
    </>
  );
})()}
```

- [ ] **Step 4: Update `produk/page.tsx` untuk fetch hargaSupplier**

Ganti seluruh isi file dengan:

```typescript
import type { Metadata } from "next";
import { getAllProduk, getAllHargaSupplier } from "@/lib/kios";
import { getSession } from "@/lib/auth";
import { ConnectionError } from "@/components/connection-error";
import { ProdukTable } from "@/components/produk/produk-table";

export const metadata: Metadata = { title: "Produk & Stok" };
export const dynamic = "force-dynamic";

export default async function ProdukPage() {
  const session = await getSession();
  let produk, hargaSupplier;
  try {
    [produk, hargaSupplier] = await Promise.all([
      getAllProduk(),
      getAllHargaSupplier(),
    ]);
  } catch (e) {
    return <ConnectionError message={e instanceof Error ? e.message : String(e)} />;
  }

  const canManage = session?.role === "owner";

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold">Produk &amp; Stok</h2>
        <p className="text-sm text-muted-foreground">
          {canManage
            ? "Kelola produk, harga, dan tingkat stok kios."
            : "Lihat daftar produk dan stok. Pengelolaan khusus pemilik."}
        </p>
      </div>
      <ProdukTable produk={produk} canManage={canManage} hargaSupplier={hargaSupplier} />
    </div>
  );
}
```

- [ ] **Step 5: Verifikasi manual**

Buka http://localhost:3000/produk:
- Tabel punya 2 kolom baru: "Pasar Min" dan "Pasar Max"
- Produk tanpa data supplier menampilkan "–"
- Produk dengan harga supplier menampilkan rentang harga dengan format Rupiah

- [ ] **Step 6: Commit dan push**

```bash
git add kios-dashboard/src/components/produk/produk-table.tsx \
        kios-dashboard/src/app/\(app\)/produk/page.tsx
git commit -m "feat(produk): tambah kolom Pasar Min/Max dari harga supplier"
git push origin main
```

---

## Checklist Selesai

- [ ] Task 1: Foundation (types, redis keys, kios.ts functions)
- [ ] Task 2: Shift server actions
- [ ] Task 3: ShiftTab component
- [ ] Task 4: KasirTabs wrapper + update kasir page
- [ ] Task 5: Promo server actions
- [ ] Task 6: PromoForm + PromoTable components
- [ ] Task 7: Promo page + nav item
- [ ] Task 8: Produk table harga pasar columns
