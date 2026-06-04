The Write tool is not available — this is read-only planning mode. Let me provide the complete plan as my final response, as required by the READ-ONLY planning constraint.

---

# Plan D — Storefront/Pelanggan UX + Barcode + QRIS button

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Menyelesaikan UX storefront pembeli (nama+WA wajib, error inline, QRIS button fix), menyambungkan checkout ke registry Pelanggan, menambah halaman Pelanggan di dashboard, dan mengintegrasikan BarcodeDetector ke kasir — tanpa dependensi baru.

**Architecture:** Perubahan bersifat additive di atas Plan 1 (registry Pelanggan sudah landing). `storefront-view.tsx` mendapat state `touched` + derived `formOk`; `/api/pesanan` jadi authoritative validator + memanggil `upsertPelanggan`; halaman `/pelanggan` + `/pelanggan/[id]` mengikuti pola server component yang sama dengan `/pesanan` dan `/suplier`. `BarcodeDetector` dibungkus komponen `barcode-scanner.tsx` yang deklarasikan type-nya secara manual karena lib.dom belum mencakup semua env.

**Tech Stack:** Go (`pkg/tools/kios/`), Next.js 15 App Router, TypeScript strict, Tailwind CSS, Upstash Redis, miniredis (test), BarcodeDetector Web API (client-side only, zero new npm deps).

**Prasyarat:**
- Plan 0 landing (branch `feat/spec-bon-pulsa-bensin-pelanggan`)
- Plan 1 landing: `store_pelanggan.go`, `NormalizePhone`, `UpsertPelanggan`, `GetPelanggan`, `GetAllPelanggan`, `DelPelanggan` sudah ada di Go; `interface Pelanggan`, `KEY.pelanggan`, `upsertPelanggan`/`getPelanggan`/`getAllPelanggan` sudah ada di TS.

**Prasyarat toolchain:**
```bash
export GOROOT=$HOME/sdk/go PATH=$HOME/sdk/go/bin:$PATH GOPATH=$HOME/go
```

---

## File Structure

| File | Tanggung jawab | Aksi |
|---|---|---|
| `pkg/tools/kios/store.go` | Tambah `PelangganID` ke struct `Pesanan` | Modify |
| `pkg/tools/kios/pelanggan_test.go` | Tambah `TestPesananPelangganIDRoundtrip` | Modify |
| `kios-dashboard/src/lib/types.ts` | Tambah `pelanggan_id?: string` ke `interface Pesanan` | Modify |
| `kios-dashboard/src/lib/wa.ts` | Tambah `isValidWaNumber(raw)` | Modify |
| `kios-dashboard/src/components/toko/storefront-view.tsx` | Tambah validasi wajib + QRIS button fix | Modify |
| `kios-dashboard/src/app/api/pesanan/route.ts` | Validasi server + upsert Pelanggan + set `pelanggan_id` | Modify |
| `kios-dashboard/src/app/(app)/pelanggan/page.tsx` | **BARU** — server component halaman daftar Pelanggan | Create |
| `kios-dashboard/src/app/(app)/pelanggan/[id]/page.tsx` | **BARU** — server component detail Pelanggan + riwayat pesanan | Create |
| `kios-dashboard/src/app/(app)/pelanggan/actions.ts` | **BARU** — `deleteCustomerAction` (owner-only) | Create |
| `kios-dashboard/src/components/pelanggan/pelanggan-list.tsx` | **BARU** — client component tabel daftar Pelanggan | Create |
| `kios-dashboard/src/components/pelanggan/pelanggan-detail.tsx` | **BARU** — client component detail + riwayat + WA link | Create |
| `kios-dashboard/src/components/nav-items.tsx` | Tambah entri `/pelanggan` (bukan ownerOnly) | Modify |
| `kios-dashboard/src/components/kasir/barcode-scanner.tsx` | **BARU** — BarcodeDetector wrapper, zero deps | Create |
| `kios-dashboard/src/components/kasir/kasir-form.tsx` | Integrasikan `<BarcodeScanner>` di samping input search | Modify |

---

## Task 1: Tambah PelangganID ke struct Pesanan (Go + TS)

**Files:**
- Modify: `pkg/tools/kios/store.go` (struct Pesanan)
- Modify: `pkg/tools/kios/pelanggan_test.go` (tambah test roundtrip)
- Modify: `kios-dashboard/src/lib/types.ts` (interface Pesanan)

- [ ] **Step 1: Tulis test yang gagal**

Di `pkg/tools/kios/pelanggan_test.go`, tambahkan fungsi baru di akhir file:

```go
func TestPesananPelangganIDRoundtrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	original := &Pesanan{
		ID:          "PSN-0001",
		Tanggal:     "2026-06-03",
		Jam:         "08:00:00",
		NamaPembeli: "Budi",
		Kontak:      "628123456789",
		Items:       []PesananItem{{ProdukID: "P001", NamaProduk: "Beras", Qty: 1, HargaSatuan: 10000, Subtotal: 10000}},
		Total:       10000,
		MetodeBayar: "tunai",
		Status:      "pending",
		CreatedAt:   1748916000,
		PelangganID: "PLG-628123456789",
	}

	b, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Pesanan
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.PelangganID != "PLG-628123456789" {
		t.Errorf("PelangganID=%q want PLG-628123456789", decoded.PelangganID)
	}

	// Pesanan lama tanpa PelangganID tetap decode valid (omitempty)
	oldJSON := `{"id":"PSN-0000","tanggal":"2026-01-01","jam":"07:00","nama_pembeli":"Anon","kontak":"","items":[],"total":0,"metode_bayar":"tunai","status":"pending","created_at":0}`
	var old Pesanan
	if err := json.Unmarshal([]byte(oldJSON), &old); err != nil {
		t.Fatalf("decode old format: %v", err)
	}
	if old.PelangganID != "" {
		t.Errorf("old format PelangganID=%q want empty", old.PelangganID)
	}
}
```

Perhatian: file ini perlu `import "encoding/json"` di blok import yang ada (tambahkan bila belum ada di `pelanggan_test.go`).

- [ ] **Step 2: Jalankan, pastikan GAGAL (kompilasi)**

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run TestPesananPelangganIDRoundtrip
```

Expected: FAIL — `Pesanan.PelangganID undefined`.

- [ ] **Step 3: Tambah field ke struct `Pesanan` di `store.go`**

Di `pkg/tools/kios/store.go`, di struct `Pesanan` (sekitar baris 118), tambahkan satu field baru setelah `CreatedAt`:

```go
// Pesanan adalah pesanan dari halaman toko pembeli (dibuat oleh dashboard web).
type Pesanan struct {
    ID          string        `json:"id"`
    Tanggal     string        `json:"tanggal"`
    Jam         string        `json:"jam"`
    NamaPembeli string        `json:"nama_pembeli"`
    Kontak      string        `json:"kontak"`
    Items       []PesananItem `json:"items"`
    Total       int           `json:"total"`
    Catatan     string        `json:"catatan"`
    MetodeBayar string        `json:"metode_bayar"`
    Status      string        `json:"status"` // pending | diproses | ditolak
    CreatedAt   int64         `json:"created_at"`
    PelangganID string        `json:"pelanggan_id,omitempty"` // FK ke Pelanggan.ID; kosong = pembeli anonim
}
```

- [ ] **Step 4: Tambah `pelanggan_id` ke `interface Pesanan` di `types.ts`**

Di `kios-dashboard/src/lib/types.ts`, dalam `interface Pesanan` (sekitar baris 118), tambahkan satu field baru setelah `created_at`:

```ts
export interface Pesanan {
  id: string;
  tanggal: string;
  jam: string;
  nama_pembeli: string;
  kontak: string;
  items: PesananItem[];
  total: number;
  catatan: string;
  metode_bayar: string; // tunai | qris
  status: PesananStatus;
  created_at: number; // unix seconds
  pelanggan_id?: string; // FK ke Pelanggan.ID; undefined = pembeli anonim (data lama)
}
```

- [ ] **Step 5: Jalankan test Go, pastikan LULUS**

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... -run TestPesananPelangganIDRoundtrip
```

Expected: PASS.

- [ ] **Step 6: Jalankan seluruh paket Go + typecheck TS**

```bash
go test -tags goolm,stdjson ./pkg/tools/kios/...
cd kios-dashboard && npm run typecheck 2>&1 | tail -5; cd ..
```

Expected: semua PASS.

- [ ] **Step 7: Commit**

```bash
git add pkg/tools/kios/store.go pkg/tools/kios/pelanggan_test.go kios-dashboard/src/lib/types.ts
git commit -m "feat(kios): tambah PelangganID omitempty ke struct Pesanan (Go + TS)"
```

---

## Task 2: `isValidWaNumber` + storefront validasi + QRIS button fix

**Files:**
- Modify: `kios-dashboard/src/lib/wa.ts`
- Modify: `kios-dashboard/src/components/toko/storefront-view.tsx`

### Sub-task 2a: `isValidWaNumber` di `wa.ts`

- [ ] **Step 1: Tambah `isValidWaNumber` ke `wa.ts`**

Di `kios-dashboard/src/lib/wa.ts`, setelah fungsi `normalizeWaNumber` (setelah baris 14), tambahkan:

```ts
/**
 * Returns true when the raw input resolves to a valid Indonesian mobile number
 * in the canonical "62…" format (2-digit country code + 8–13 local digits).
 */
export function isValidWaNumber(raw: string): boolean {
  const n = normalizeWaNumber(raw);
  return /^62\d{8,13}$/.test(n);
}
```

- [ ] **Step 2: Typecheck**

```bash
cd kios-dashboard && npm run typecheck 2>&1 | tail -5; cd ..
```

Expected: bersih.

### Sub-task 2b: Storefront validasi + QRIS button fix

- [ ] **Step 3: Ganti seluruh `storefront-view.tsx` dengan versi berikut**

```tsx
"use client";

import { useEffect, useMemo, useState } from "react";
import {
  CheckCircle2,
  Loader2,
  MessageCircle,
  Minus,
  Plus,
  QrCode,
  RefreshCw,
  Search,
  ShoppingBag,
  Store,
  Trash2,
  Wallet,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input, Label } from "@/components/ui/input";
import { Modal } from "@/components/ui/modal";
import { EmptyState } from "@/components/ui/empty-state";
import { formatRupiah } from "@/lib/format";
import { cn, matchesQuery } from "@/lib/utils";
import { buildOrderWa, buildQrisDinamisWa, isValidWaNumber, normalizeWaNumber, waLink } from "@/lib/wa";
import { categoryImage } from "@/lib/produk-image";
import type { PublicProduk } from "@/lib/types";

interface CartLine {
  id: string;
  nama: string;
  harga: number;
  satuan: string;
  stok: number;
  qty: number;
}

type QrisInfo = { enabled: true; nama: string; image_url: string } | { enabled: false };

type Metode = "tunai" | "qris";

export function StorefrontView() {
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  const [produk, setProduk] = useState<PublicProduk[]>([]);
  const [kategori, setKategori] = useState<string[]>([]);
  const [qris, setQris] = useState<QrisInfo>({ enabled: false });
  const [waNumber, setWaNumber] = useState("");

  const [query, setQuery] = useState("");
  const [cat, setCat] = useState("");
  const [cart, setCart] = useState<CartLine[]>([]);
  const [open, setOpen] = useState(false);
  const [nama, setNama] = useState("");
  const [kontak, setKontak] = useState("");
  const [catatan, setCatatan] = useState("");
  const [metode, setMetode] = useState<Metode>("tunai");
  const [qrisChatOpened, setQrisChatOpened] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [touched, setTouched] = useState(false);
  const [done, setDone] = useState<{
    id: string;
    total: number;
    metode: Metode;
    lines: { nama: string; qty: number; subtotal: number }[];
  } | null>(null);

  // Derived validation
  const namaOk = nama.trim().length >= 2;
  const waOk = isValidWaNumber(kontak);
  const formOk = namaOk && waOk;

  // QRIS chat flow: hide generic submit button when this flow is active
  const qrisChatFlow = qris.enabled && metode === "qris" && !!waNumber;

  async function load() {
    setLoading(true);
    setLoadError(false);
    try {
      const res = await fetch("/api/mall", { cache: "no-store" });
      const data = await res.json();
      if (!res.ok || !data.ok) {
        setLoadError(true);
        return;
      }
      setProduk(data.produk);
      setKategori(data.kategori);
      const q: QrisInfo = data.qris?.enabled
        ? { enabled: true, nama: data.qris.nama, image_url: data.qris.image_url }
        : { enabled: false };
      setQris(q);
      if (!q.enabled) setMetode("tunai");
      setWaNumber(typeof data.wa_number === "string" ? data.wa_number : "");
    } catch {
      setLoadError(true);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  const rows = useMemo(() => {
    const q = query.trim().toLowerCase();
    return produk.filter((p) => {
      if (cat && p.kategori !== cat) return false;
      if (!q) return true;
      return matchesQuery(q, p.nama);
    });
  }, [produk, query, cat]);

  const count = cart.reduce((s, l) => s + l.qty, 0);
  const total = cart.reduce((s, l) => s + l.qty * l.harga, 0);
  const qtyOf = (id: string) => cart.find((l) => l.id === id)?.qty ?? 0;

  function add(p: PublicProduk) {
    setCart((prev) => {
      const idx = prev.findIndex((l) => l.id === p.id);
      if (idx === -1)
        return [...prev, { id: p.id, nama: p.nama, harga: p.harga_jual, satuan: p.satuan, stok: p.stok, qty: 1 }];
      const next = [...prev];
      next[idx] = { ...next[idx], qty: Math.min(next[idx].qty + 1, p.stok) };
      return next;
    });
  }

  function setQty(id: string, qty: number) {
    setCart((prev) =>
      prev
        .map((l) => (l.id === id ? { ...l, qty: Math.max(0, Math.min(qty || 0, l.stok)) } : l))
        .filter((l) => l.qty > 0),
    );
  }

  async function submit() {
    setTouched(true);
    if (!formOk) return;

    setSubmitError(null);
    setSubmitting(true);
    try {
      const res = await fetch("/api/pesanan", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          items: cart.map((l) => ({ produkId: l.id, qty: l.qty })),
          nama,
          kontak_wa: normalizeWaNumber(kontak),
          catatan,
          metode,
        }),
      });
      const data = await res.json();
      if (!res.ok || !data.ok) {
        setSubmitError(data.error || "Gagal mengirim pesanan.");
        return;
      }
      setDone({
        id: data.id,
        total: data.total,
        metode,
        lines: cart.map((l) => ({ nama: l.nama, qty: l.qty, subtotal: l.qty * l.harga })),
      });
      setCart([]);
      setNama("");
      setKontak("");
      setCatatan("");
      setMetode("tunai");
      setQrisChatOpened(false);
      setTouched(false);
      setOpen(false);
      void load(); // refresh stock
    } catch {
      setSubmitError("Gagal terhubung. Cek koneksi internet ya.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="min-h-dvh bg-background pb-24">
      {/* Header */}
      <header className="sticky top-0 z-30 border-b bg-background/95 backdrop-blur">
        <div className="mx-auto flex h-16 max-w-5xl items-center justify-between gap-3 px-4">
          <div className="flex items-center gap-2.5">
            <div className="flex size-9 items-center justify-center rounded-lg bg-accent text-accent-foreground">
              <Store className="size-5" aria-hidden />
            </div>
            <div className="leading-tight">
              <p className="text-sm font-semibold">Kios Cerdas</p>
              <p className="text-xs text-muted-foreground">Belanja Online · Rote Ndao</p>
            </div>
          </div>
          <Button variant="outline" size="sm" onClick={() => setOpen(true)} aria-label="Buka keranjang">
            <ShoppingBag className="size-4" />
            Keranjang
            {count > 0 && (
              <span className="ml-1 inline-flex min-w-5 items-center justify-center rounded-full bg-accent px-1.5 text-xs font-semibold text-accent-foreground tabular-nums">
                {count}
              </span>
            )}
          </Button>
        </div>
      </header>

      {/* Hero search */}
      <section className="border-b bg-gradient-to-b from-accent/10 to-background">
        <div className="mx-auto max-w-5xl px-4 py-8 sm:py-10">
          <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">
            Belanja kebutuhan harian, langsung dari kios.
          </h1>
          <p className="mt-1.5 text-sm text-muted-foreground">
            Pesan online, ambil di kios. Tanpa perlu daftar.
          </p>
          <div className="relative mt-5 max-w-xl">
            <Search className="pointer-events-none absolute top-1/2 left-4 size-5 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Cari produk… (mis. beras, minyak, gula)"
              className="h-12 pl-11 text-base"
              aria-label="Cari produk"
            />
          </div>
        </div>
      </section>

      <main className="mx-auto max-w-5xl px-4 py-6">
        {/* Category chips */}
        {!loading && !loadError && kategori.length > 0 && (
          <div className="-mx-4 mb-5 flex gap-2 overflow-x-auto px-4 pb-1">
            <CategoryChip label="Semua" active={cat === ""} onClick={() => setCat("")} />
            {kategori.map((c) => (
              <CategoryChip key={c} label={c} active={cat === c} onClick={() => setCat(c)} />
            ))}
          </div>
        )}

        {loading ? (
          <ul className="grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-4">
            {Array.from({ length: 8 }).map((_, i) => (
              <li key={i} className="h-44 animate-pulse rounded-xl border bg-muted/40" />
            ))}
          </ul>
        ) : loadError ? (
          <EmptyState
            icon={RefreshCw}
            title="Gagal memuat produk"
            description="Coba muat ulang sebentar ya."
            action={
              <Button variant="outline" size="sm" onClick={load}>
                <RefreshCw className="size-4" /> Muat ulang
              </Button>
            }
          />
        ) : rows.length === 0 ? (
          <EmptyState icon={ShoppingBag} title="Produk tidak ditemukan" />
        ) : (
          <ul className="grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-4">
            {rows.map((p) => {
              const inCart = qtyOf(p.id);
              const habis = p.stok <= 0;
              return (
                <li key={p.id} className="flex flex-col overflow-hidden rounded-xl border bg-card shadow-sm">
                  <ProductImage
                    src={p.image_url}
                    fallback={categoryImage(p.kategori)}
                    alt={p.nama}
                    dim={habis}
                  />
                  <div className="flex flex-1 flex-col p-3">
                    <div className="flex-1">
                      <p className="line-clamp-2 text-sm font-medium">{p.nama}</p>
                      <p className="mt-1 font-mono text-base font-semibold text-accent">
                        {formatRupiah(p.harga_jual)}
                      </p>
                      <p className="text-xs text-muted-foreground">per {p.satuan}</p>
                    </div>
                    <div className="mt-3">
                    {habis ? (
                      <Badge variant="destructive" className="w-full justify-center py-1">
                        Habis
                      </Badge>
                    ) : inCart > 0 ? (
                      <div className="flex items-center justify-between gap-1">
                        <button
                          type="button"
                          onClick={() => setQty(p.id, inCart - 1)}
                          aria-label={`Kurangi ${p.nama}`}
                          className="flex size-9 cursor-pointer items-center justify-center rounded-lg border border-input hover:bg-muted"
                        >
                          <Minus className="size-4" />
                        </button>
                        <span className="font-mono text-sm font-medium tabular-nums">{inCart}</span>
                        <button
                          type="button"
                          onClick={() => add(p)}
                          disabled={inCart >= p.stok}
                          aria-label={`Tambah ${p.nama}`}
                          className="flex size-9 cursor-pointer items-center justify-center rounded-lg border border-input hover:bg-muted disabled:opacity-40"
                        >
                          <Plus className="size-4" />
                        </button>
                      </div>
                    ) : (
                      <Button variant="outline" size="sm" className="w-full" onClick={() => add(p)}>
                        <Plus className="size-4" /> Tambah
                      </Button>
                    )}
                    </div>
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </main>

      {/* Sticky cart bar */}
      {count > 0 && (
        <div className="fixed inset-x-0 bottom-0 z-40 border-t bg-background/95 p-3 backdrop-blur">
          <div className="mx-auto flex max-w-5xl items-center gap-3">
            <div className="flex-1">
              <p className="text-xs text-muted-foreground">{count} item</p>
              <p className="font-mono text-base font-semibold tabular-nums">{formatRupiah(total)}</p>
            </div>
            <Button variant="accent" size="md" onClick={() => setOpen(true)}>
              <ShoppingBag className="size-4" /> Pesan Sekarang
            </Button>
          </div>
        </div>
      )}

      {/* Cart + checkout */}
      <Modal
        open={open}
        onClose={() => {
          setOpen(false);
          setQrisChatOpened(false);
        }}
        title="Keranjang"
        description="Periksa pesanan & kirim ke kios."
      >
        <div className="space-y-4">
          {cart.length === 0 ? (
            <EmptyState icon={ShoppingBag} title="Keranjang kosong" description="Tambahkan produk dulu ya." />
          ) : (
            <>
              <ul className="space-y-2">
                {cart.map((l) => (
                  <li key={l.id} className="flex items-center justify-between gap-2 rounded-lg border p-2.5">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">{l.nama}</p>
                      <p className="font-mono text-xs text-muted-foreground">{formatRupiah(l.harga)}</p>
                    </div>
                    <div className="flex items-center gap-1">
                      <button
                        type="button"
                        onClick={() => setQty(l.id, l.qty - 1)}
                        aria-label="Kurangi"
                        className="flex size-8 cursor-pointer items-center justify-center rounded-md border border-input hover:bg-muted"
                      >
                        <Minus className="size-3.5" />
                      </button>
                      <span className="w-8 text-center font-mono text-sm tabular-nums">{l.qty}</span>
                      <button
                        type="button"
                        onClick={() => setQty(l.id, l.qty + 1)}
                        disabled={l.qty >= l.stok}
                        aria-label="Tambah"
                        className="flex size-8 cursor-pointer items-center justify-center rounded-md border border-input hover:bg-muted disabled:opacity-40"
                      >
                        <Plus className="size-3.5" />
                      </button>
                      <button
                        type="button"
                        onClick={() => setQty(l.id, 0)}
                        aria-label="Hapus"
                        className="ml-1 flex size-8 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                      >
                        <Trash2 className="size-4" />
                      </button>
                    </div>
                  </li>
                ))}
              </ul>

              <div className="flex justify-between border-t pt-3 text-sm">
                <span className="text-muted-foreground">Total</span>
                <span className="text-base font-semibold tabular-nums">{formatRupiah(total)}</span>
              </div>

              {/* ── Form data pembeli ── */}
              <div className="space-y-3">
                <div className="space-y-1.5">
                  <Label htmlFor="nama">
                    Nama <span aria-hidden className="text-destructive">*</span>
                  </Label>
                  <Input
                    id="nama"
                    value={nama}
                    onChange={(e) => setNama(e.target.value)}
                    onBlur={() => setTouched(true)}
                    placeholder="Nama kamu (min. 2 huruf)"
                    autoComplete="name"
                    aria-invalid={touched && !namaOk}
                    aria-describedby={touched && !namaOk ? "nama-error" : undefined}
                  />
                  {touched && !namaOk && (
                    <p id="nama-error" role="alert" className="text-xs text-destructive">
                      Nama wajib diisi ya kak (minimal 2 huruf).
                    </p>
                  )}
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="kontak">
                    No. WhatsApp <span aria-hidden className="text-destructive">*</span>
                  </Label>
                  <Input
                    id="kontak"
                    value={kontak}
                    onChange={(e) => setKontak(e.target.value)}
                    onBlur={() => setTouched(true)}
                    placeholder="08123456789"
                    inputMode="tel"
                    autoComplete="tel"
                    aria-invalid={touched && !waOk}
                    aria-describedby={touched && !waOk ? "kontak-error" : undefined}
                  />
                  {touched && !waOk && (
                    <p id="kontak-error" role="alert" className="text-xs text-destructive">
                      Masukkan nomor WA yang valid (contoh: 08123456789).
                    </p>
                  )}
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="catatan">Catatan (opsional)</Label>
                  <Input
                    id="catatan"
                    value={catatan}
                    onChange={(e) => setCatatan(e.target.value)}
                    placeholder="mis. ambil jam 5 sore"
                  />
                </div>
              </div>

              {qris.enabled && (
                <div className="space-y-2">
                  <Label>Metode pembayaran</Label>
                  <div className="grid grid-cols-2 gap-2">
                    <MetodeOption
                      icon={Wallet}
                      label="Tunai di kios"
                      active={metode === "tunai"}
                      onClick={() => {
                        setMetode("tunai");
                        setQrisChatOpened(false);
                      }}
                    />
                    <MetodeOption
                      icon={QrCode}
                      label="QRIS"
                      active={metode === "qris"}
                      onClick={() => setMetode("qris")}
                    />
                  </div>
                  {metode === "qris" && (
                    <div className="rounded-lg border bg-muted/30 p-3 text-center">
                      <p className="text-sm font-medium">{qris.nama}</p>
                      {/* eslint-disable-next-line @next/next/no-img-element */}
                      <img
                        src={qris.image_url}
                        alt={`QRIS ${qris.nama}`}
                        className="mx-auto mt-2 size-48 rounded-md border bg-white object-contain p-1"
                      />
                      <p className="mt-2 font-mono text-sm font-semibold tabular-nums">
                        {formatRupiah(total)}
                      </p>
                      <p className="mt-1 text-xs text-muted-foreground">
                        Scan QR di atas untuk bayar, lalu kirim pesanan & tunjukkan bukti ke kasir.
                      </p>
                      {waNumber && (
                        <>
                          <div className="my-3 flex items-center gap-2 text-xs text-muted-foreground">
                            <span className="h-px flex-1 bg-border" />
                            atau
                            <span className="h-px flex-1 bg-border" />
                          </div>
                          <a
                            href={
                              formOk
                                ? waLink(
                                    waNumber,
                                    buildQrisDinamisWa(
                                      cart.map((l) => ({ nama: l.nama, qty: l.qty, subtotal: l.qty * l.harga })),
                                      total,
                                    ),
                                  )
                                : "#"
                            }
                            target="_blank"
                            rel="noopener noreferrer"
                            onClick={(e) => {
                              if (!formOk) {
                                e.preventDefault();
                                setTouched(true);
                                return;
                              }
                              setQrisChatOpened(true);
                            }}
                            aria-disabled={!formOk}
                            className={cn(
                              "inline-flex h-10 w-full items-center justify-center gap-2 rounded-lg px-4 text-sm font-medium text-white transition-colors",
                              formOk
                                ? "bg-[#25D366] hover:bg-[#1ebe5b]"
                                : "cursor-not-allowed bg-muted text-muted-foreground",
                            )}
                          >
                            <MessageCircle className="size-4" />
                            Chat kasir untuk QR dinamis
                          </a>
                          {!qrisChatOpened ? (
                            <p className="mt-1 text-xs text-muted-foreground">
                              Kasir online akan mengirim QR sesuai total belanjamu.
                            </p>
                          ) : (
                            <div className="mt-3 space-y-2 rounded-lg border border-accent/40 bg-accent/5 p-2.5 text-left">
                              <p className="text-xs text-muted-foreground">
                                Sudah mengirim pesan ke kasir lewat WhatsApp? Kalau sudah, kami masukkan
                                pesananmu ke antrian supaya kasir bisa proses &amp; kirim QR-nya.
                              </p>
                              <Button
                                variant="accent"
                                size="sm"
                                className="w-full"
                                onClick={submit}
                                disabled={submitting || !formOk}
                              >
                                {submitting && <Loader2 className="size-4 animate-spin" />}
                                Sudah kirim — Buat Pesanan
                              </Button>
                            </div>
                          )}
                        </>
                      )}
                    </div>
                  )}
                </div>
              )}

              {submitError && (
                <p role="alert" className="text-sm text-destructive">
                  {submitError}
                </p>
              )}

              {/* Tombol generik hanya tampil saat bukan alur QRIS-chat */}
              {!qrisChatFlow && (
                <Button
                  variant="accent"
                  size="md"
                  className="w-full"
                  onClick={submit}
                  disabled={submitting}
                >
                  {submitting && <Loader2 className="size-4 animate-spin" />}
                  Kirim Pesanan
                </Button>
              )}
            </>
          )}
        </div>
      </Modal>

      {/* Success */}
      <Modal open={done !== null} onClose={() => setDone(null)} title="Pesanan terkirim!" className="max-w-sm">
        <div className="space-y-4 text-center">
          <div className="mx-auto flex size-14 items-center justify-center rounded-full bg-success/15 text-success">
            <CheckCircle2 className="size-7" />
          </div>
          <div>
            <p className="text-sm">
              Pesanan <span className="font-mono font-semibold">{done?.id}</span> diterima.
            </p>
            <p className="mt-1 text-sm text-muted-foreground">
              Total {done ? formatRupiah(done.total) : ""}.{" "}
              {done?.metode === "qris"
                ? "Pastikan sudah bayar via QRIS, lalu tunjukkan bukti ke kasir ya."
                : "Kasir kios akan segera memprosesnya."}
            </p>
          </div>
          {waNumber && done && (
            <a
              href={waLink(
                waNumber,
                buildOrderWa({ id: done.id, total: done.total, metode: done.metode, lines: done.lines }),
              )}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex h-11 w-full items-center justify-center gap-2 rounded-lg bg-[#25D366] px-5 text-sm font-medium text-white transition-colors hover:bg-[#1ebe5b]"
            >
              <MessageCircle className="size-4" />
              Konfirmasi via WhatsApp
            </a>
          )}
          <Button variant="outline" size="md" className="w-full" onClick={() => setDone(null)}>
            Selesai
          </Button>
        </div>
      </Modal>
    </div>
  );
}

function ProductImage({
  src,
  fallback,
  alt,
  dim,
}: {
  src: string;
  fallback: string;
  alt: string;
  dim?: boolean;
}) {
  const [broken, setBroken] = useState(false);
  const url = src && !broken ? src : fallback;
  return (
    <div className={cn("aspect-square w-full bg-muted/40", dim && "opacity-60")}>
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src={url}
        alt={alt}
        loading="lazy"
        onError={() => setBroken(true)}
        className="size-full object-cover"
      />
    </div>
  );
}

function MetodeOption({
  icon: Icon,
  label,
  active,
  onClick,
}: {
  icon: typeof Wallet;
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={cn(
        "flex cursor-pointer items-center justify-center gap-2 rounded-lg border px-3 py-2.5 text-sm font-medium transition-colors",
        active ? "border-accent bg-accent/10 text-accent" : "border-border hover:bg-muted",
      )}
    >
      <Icon className="size-4" />
      {label}
    </button>
  );
}

function CategoryChip({ label, active, onClick }: { label: string; active: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={cn(
        "shrink-0 cursor-pointer rounded-full border px-4 py-1.5 text-sm font-medium whitespace-nowrap transition-colors",
        active ? "border-accent bg-accent text-accent-foreground" : "border-border hover:bg-muted",
      )}
    >
      {label}
    </button>
  );
}
```

- [ ] **Step 4: Typecheck**

```bash
cd kios-dashboard && npm run typecheck 2>&1 | tail -5; cd ..
```

Expected: bersih.

- [ ] **Step 5: Commit**

```bash
git add kios-dashboard/src/lib/wa.ts kios-dashboard/src/components/toko/storefront-view.tsx
git commit -m "feat(storefront): validasi nama+WA wajib, QRIS button fix, error inline"
```

---

## Task 3: `/api/pesanan` — validasi server + upsert Pelanggan + `pelanggan_id`

**Files:**
- Modify: `kios-dashboard/src/app/api/pesanan/route.ts`

- [ ] **Step 1: Ganti seluruh `route.ts` dengan versi berikut**

Perubahan kunci dari versi lama:
1. Terima `kontak_wa` (canonical dari client), fallback ke `kontak` untuk kompatibilitas
2. Validasi server-side: nama wajib ≥2 char, WA wajib valid — return 400 bila gagal
3. Drop fallback `"Pembeli"` — nama kosong = reject
4. Upsert Pelanggan sebelum `setPesanan`
5. Set `pesanan.pelanggan_id`

```ts
import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { bumpRate, getAllProduk, nextPesananId, setPesanan, upsertPelanggan } from "@/lib/kios";
import { isValidWaNumber, normalizeWaNumber } from "@/lib/wa";
import { timeWITA, todayWITA } from "@/lib/format";
import type { Pesanan, PesananItem } from "@/lib/types";

export const runtime = "nodejs";

const MAX_PER_MINUTE = 6;

function clip(s: unknown, max: number): string {
  return String(s ?? "").trim().slice(0, max);
}

export async function POST(req: NextRequest) {
  let body: {
    items?: { produkId?: string; qty?: number }[];
    nama?: string;
    kontak_wa?: string;
    kontak?: string;   // fallback dari klien lama (sebelum Task 2)
    catatan?: string;
    metode?: string;
  };
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ ok: false, error: "Format tidak valid." }, { status: 400 });
  }

  const ip =
    req.headers.get("x-forwarded-for")?.split(",")[0]?.trim() ||
    req.headers.get("x-real-ip") ||
    "unknown";

  try {
    const hits = await bumpRate("pesanan", ip, 60);
    if (hits > MAX_PER_MINUTE) {
      return NextResponse.json(
        { ok: false, error: "Terlalu banyak pesanan. Coba lagi sebentar ya." },
        { status: 429 },
      );
    }

    // ── Validasi identitas pembeli (server-authoritative) ──────────────────────
    const namaTrimmed = clip(body.nama, 60);
    if (namaTrimmed.length < 2) {
      return NextResponse.json(
        { ok: false, error: "Nama wajib diisi (minimal 2 huruf)." },
        { status: 400 },
      );
    }

    // Terima kontak_wa (dari storefront baru) atau kontak (fallback klien lama)
    const waRaw = String(body.kontak_wa || body.kontak || "");
    if (!isValidWaNumber(waRaw)) {
      return NextResponse.json(
        { ok: false, error: "Nomor WhatsApp tidak valid (contoh: 08123456789)." },
        { status: 400 },
      );
    }
    const waCanon = normalizeWaNumber(waRaw);

    // ── Validasi items ─────────────────────────────────────────────────────────
    const wanted = new Map<string, number>();
    for (const it of body.items ?? []) {
      const q = Math.trunc(Number(it?.qty));
      if (it?.produkId && Number.isFinite(q) && q > 0) {
        wanted.set(it.produkId, (wanted.get(it.produkId) ?? 0) + q);
      }
    }
    if (wanted.size === 0) {
      return NextResponse.json({ ok: false, error: "Keranjang kosong." }, { status: 400 });
    }

    const byId = new Map((await getAllProduk()).map((p) => [p.id, p]));
    const items: PesananItem[] = [];
    let total = 0;
    for (const [id, qty] of wanted) {
      const p = byId.get(id);
      if (!p) return NextResponse.json({ ok: false, error: "Ada produk yang tidak tersedia." }, { status: 400 });
      if (p.stok < qty) {
        return NextResponse.json(
          { ok: false, error: `Stok ${p.nama} tinggal ${p.stok}.` },
          { status: 409 },
        );
      }
      const subtotal = qty * p.harga_jual;
      items.push({ produk_id: p.id, nama_produk: p.nama, qty, harga_satuan: p.harga_jual, subtotal });
      total += subtotal;
    }

    // ── Upsert Pelanggan ───────────────────────────────────────────────────────
    const pelanggan = await upsertPelanggan(namaTrimmed, waRaw);

    // ── Simpan pesanan ─────────────────────────────────────────────────────────
    const metode = body.metode === "qris" ? "qris" : "tunai";
    const id = await nextPesananId();
    const pesanan: Pesanan = {
      id,
      tanggal: todayWITA(),
      jam: timeWITA(),
      nama_pembeli: namaTrimmed,
      kontak: waCanon,
      items,
      total,
      catatan: clip(body.catatan, 200),
      metode_bayar: metode,
      status: "pending",
      created_at: Math.floor(Date.now() / 1000),
      pelanggan_id: pelanggan.id,
    };
    await setPesanan(pesanan);

    return NextResponse.json({ ok: true, id, total });
  } catch {
    return NextResponse.json(
      { ok: false, error: "Server belum siap menerima pesanan. Coba lagi nanti." },
      { status: 500 },
    );
  }
}
```

- [ ] **Step 2: Typecheck**

```bash
cd kios-dashboard && npm run typecheck 2>&1 | tail -5; cd ..
```

Expected: bersih.

- [ ] **Step 3: Commit**

```bash
git add kios-dashboard/src/app/api/pesanan/route.ts
git commit -m "feat(api/pesanan): validasi server nama+WA wajib, upsert Pelanggan, set pelanggan_id"
```

---

## Task 4: Halaman Pelanggan di dashboard

**Files:**
- Create: `kios-dashboard/src/app/(app)/pelanggan/page.tsx`
- Create: `kios-dashboard/src/app/(app)/pelanggan/[id]/page.tsx`
- Create: `kios-dashboard/src/app/(app)/pelanggan/actions.ts`
- Create: `kios-dashboard/src/components/pelanggan/pelanggan-list.tsx`
- Create: `kios-dashboard/src/components/pelanggan/pelanggan-detail.tsx`
- Modify: `kios-dashboard/src/components/nav-items.tsx`

### Sub-task 4a: Server component `page.tsx`

- [ ] **Step 1: Buat `kios-dashboard/src/app/(app)/pelanggan/page.tsx`**

```tsx
import type { Metadata } from "next";
import { getAllPelanggan } from "@/lib/kios";
import { ConnectionError } from "@/components/connection-error";
import { PelangganList } from "@/components/pelanggan/pelanggan-list";

export const metadata: Metadata = { title: "Pelanggan" };
export const dynamic = "force-dynamic";

export default async function PelangganPage() {
  let pelanggan;
  try {
    pelanggan = await getAllPelanggan();
  } catch (e) {
    return <ConnectionError message={e instanceof Error ? e.message : String(e)} />;
  }

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold">Pelanggan</h2>
        <p className="text-sm text-muted-foreground">
          Daftar pembeli yang pernah memesan. Klik baris untuk melihat riwayat pesanan.
        </p>
      </div>
      <PelangganList pelanggan={pelanggan} />
    </div>
  );
}
```

### Sub-task 4b: Client component `pelanggan-list.tsx`

- [ ] **Step 2: Buat `kios-dashboard/src/components/pelanggan/pelanggan-list.tsx`**

```tsx
"use client";

import { useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { Search, UserRound } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { formatRupiah } from "@/lib/format";
import { matchesQuery } from "@/lib/utils";
import type { Pelanggan } from "@/lib/types";

export function PelangganList({ pelanggan }: { pelanggan: Pelanggan[] }) {
  const router = useRouter();
  const [query, setQuery] = useState("");

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return pelanggan;
    return pelanggan.filter((p) => matchesQuery(q, p.nama, p.phone));
  }, [pelanggan, query]);

  return (
    <div className="space-y-3">
      <div className="relative max-w-sm">
        <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Cari nama atau nomor WA…"
          className="pl-9"
          aria-label="Cari pelanggan"
        />
      </div>

      {filtered.length === 0 ? (
        <EmptyState icon={UserRound} title="Belum ada pelanggan" description="Pelanggan akan muncul saat ada pesanan masuk dari storefront." />
      ) : (
        <div className="overflow-x-auto rounded-lg border">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/40 text-left">
                <th className="px-4 py-3 font-medium text-muted-foreground">Nama</th>
                <th className="px-4 py-3 font-medium text-muted-foreground">No. WA</th>
                <th className="px-4 py-3 text-right font-medium text-muted-foreground">Pesanan</th>
                <th className="px-4 py-3 text-right font-medium text-muted-foreground">Total Belanja</th>
                <th className="px-4 py-3 text-right font-medium text-muted-foreground">Piutang</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((p) => (
                <tr
                  key={p.id}
                  onClick={() => router.push(`/pelanggan/${encodeURIComponent(p.phone)}`)}
                  className="cursor-pointer border-b last:border-0 hover:bg-muted/30 transition-colors"
                >
                  <td className="px-4 py-3 font-medium">{p.nama}</td>
                  <td className="px-4 py-3 font-mono text-muted-foreground">{p.phone}</td>
                  <td className="px-4 py-3 text-right tabular-nums">{p.total_pesanan}</td>
                  <td className="px-4 py-3 text-right font-mono tabular-nums">{formatRupiah(p.total_belanja)}</td>
                  <td className="px-4 py-3 text-right">
                    {p.total_utang > 0 ? (
                      <Badge variant="destructive">{formatRupiah(p.total_utang)}</Badge>
                    ) : (
                      <span className="text-muted-foreground">—</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
```

### Sub-task 4c: Detail page + client component

- [ ] **Step 3: Buat `kios-dashboard/src/app/(app)/pelanggan/[id]/page.tsx`**

ID di URL = phone ternormalisasi (URL-encoded). Decode lalu lookup.

```tsx
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { getAllPesanan, getPelanggan } from "@/lib/kios";
import { ConnectionError } from "@/components/connection-error";
import { PelangganDetail } from "@/components/pelanggan/pelanggan-detail";

export const metadata: Metadata = { title: "Detail Pelanggan" };
export const dynamic = "force-dynamic";

export default async function PelangganDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const phone = decodeURIComponent(id);

  let pelanggan;
  let pesanan;
  try {
    [pelanggan, pesanan] = await Promise.all([
      getPelanggan(phone),
      getAllPesanan(),
    ]);
  } catch (e) {
    return <ConnectionError message={e instanceof Error ? e.message : String(e)} />;
  }

  if (!pelanggan) notFound();

  const riwayat = pesanan.filter((p) => p.pelanggan_id === pelanggan.id);

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold">{pelanggan.nama}</h2>
        <p className="text-sm text-muted-foreground font-mono">{pelanggan.phone}</p>
      </div>
      <PelangganDetail pelanggan={pelanggan} riwayat={riwayat} />
    </div>
  );
}
```

- [ ] **Step 4: Buat `kios-dashboard/src/components/pelanggan/pelanggan-detail.tsx`**

```tsx
"use client";

import { MessageCircle } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { formatRupiah, formatTanggal } from "@/lib/format";
import { waLink, buildOrderWa } from "@/lib/wa";
import type { Pelanggan, Pesanan } from "@/lib/types";

const STATUS_LABEL: Record<string, string> = {
  pending: "Menunggu",
  diproses: "Diproses",
  ditolak: "Ditolak",
};

const STATUS_VARIANT: Record<string, "warning" | "success" | "secondary"> = {
  pending: "warning",
  diproses: "success",
  ditolak: "secondary",
};

export function PelangganDetail({
  pelanggan,
  riwayat,
}: {
  pelanggan: Pelanggan;
  riwayat: Pesanan[];
}) {
  return (
    <div className="space-y-6">
      {/* Ringkasan */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <Card className="p-4 text-center">
          <p className="text-2xl font-bold tabular-nums">{pelanggan.total_pesanan}</p>
          <p className="text-xs text-muted-foreground">Total Pesanan</p>
        </Card>
        <Card className="p-4 text-center">
          <p className="font-mono text-xl font-bold tabular-nums">{formatRupiah(pelanggan.total_belanja)}</p>
          <p className="text-xs text-muted-foreground">Total Belanja</p>
        </Card>
        <Card className="p-4 text-center">
          <p className={`font-mono text-xl font-bold tabular-nums ${pelanggan.total_utang > 0 ? "text-destructive" : ""}`}>
            {formatRupiah(pelanggan.total_utang)}
          </p>
          <p className="text-xs text-muted-foreground">Piutang</p>
        </Card>
        <Card className="p-4 text-center">
          <p className="text-sm font-medium">{pelanggan.last_order || "—"}</p>
          <p className="text-xs text-muted-foreground">Terakhir Order</p>
        </Card>
      </div>

      {/* WA link */}
      {pelanggan.phone && (
        <a
          href={`https://wa.me/${pelanggan.phone}`}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex h-10 items-center gap-2 rounded-lg bg-[#25D366] px-4 text-sm font-medium text-white transition-colors hover:bg-[#1ebe5b]"
        >
          <MessageCircle className="size-4" />
          Hubungi via WhatsApp
        </a>
      )}

      {/* Riwayat pesanan */}
      <div>
        <h3 className="mb-3 text-base font-semibold">Riwayat Pesanan</h3>
        {riwayat.length === 0 ? (
          <EmptyState icon={MessageCircle} title="Belum ada pesanan" />
        ) : (
          <div className="space-y-2">
            {riwayat.map((p) => (
              <Card key={p.id} className="flex items-center justify-between gap-3 p-3">
                <div className="min-w-0">
                  <p className="font-mono text-sm font-semibold">{p.id}</p>
                  <p className="text-xs text-muted-foreground">
                    {formatTanggal(p.tanggal)} · {p.metode_bayar}
                  </p>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {p.items.map((it) => `${it.nama_produk} ×${it.qty}`).join(", ")}
                  </p>
                </div>
                <div className="flex shrink-0 flex-col items-end gap-1.5">
                  <span className="font-mono text-sm font-semibold tabular-nums">{formatRupiah(p.total)}</span>
                  <Badge variant={STATUS_VARIANT[p.status] ?? "secondary"}>
                    {STATUS_LABEL[p.status] ?? p.status}
                  </Badge>
                </div>
              </Card>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
```

### Sub-task 4d: `deleteCustomerAction`

- [ ] **Step 5: Buat `kios-dashboard/src/app/(app)/pelanggan/actions.ts`**

```ts
"use server";

import { revalidatePath } from "next/cache";
import { getSession } from "@/lib/auth";
import { delPelanggan } from "@/lib/kios";

export type ActionResult = { ok: true; message: string } | { ok: false; error: string };

async function ensureOwner(): Promise<{ id: string } | ActionResult> {
  const session = await getSession();
  if (!session) return { ok: false, error: "Sesi berakhir. Silakan masuk lagi." };
  if (session.role !== "owner") return { ok: false, error: "Aksi ini khusus pemilik (owner)." };
  return { id: session.id };
}

export async function deleteCustomerAction(phone: string): Promise<ActionResult> {
  const gate = await ensureOwner();
  if ("ok" in gate) return gate;

  if (!phone.trim()) return { ok: false, error: "ID pelanggan tidak valid." };

  await delPelanggan(phone);
  revalidatePath("/pelanggan");
  return { ok: true, message: "Pelanggan dihapus." };
}
```

Catatan: fungsi `delPelanggan` perlu ditambahkan ke `kios-dashboard/src/lib/kios.ts` (mirip `delUser`):

```ts
export async function delPelanggan(phone: string): Promise<void> {
  await redis().hdel(KEY.pelanggan, phone);
}
```

### Sub-task 4e: Nav item

- [ ] **Step 6: Tambah entri Pelanggan ke `nav-items.tsx`**

Ganti seluruh file `kios-dashboard/src/components/nav-items.tsx`:

```tsx
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
import type { Role } from "@/lib/types";

export interface NavItem {
  href: string;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
  ownerOnly?: boolean;
}

export const NAV_ITEMS: NavItem[] = [
  { href: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { href: "/kasir", label: "Kasir", icon: ShoppingCart },
  { href: "/pesanan", label: "Pesanan", icon: ClipboardList },
  { href: "/pelanggan", label: "Pelanggan", icon: Contact },
  { href: "/produk", label: "Produk & Stok", icon: Package },
  { href: "/suplier", label: "Supplier", icon: Truck },
  { href: "/impor", label: "Impor Data", icon: FileUp },
  { href: "/penjualan", label: "Penjualan", icon: Receipt },
  { href: "/laporan", label: "Laporan", icon: BarChart3 },
  { href: "/pengguna", label: "Pengguna", icon: Users, ownerOnly: true },
  { href: "/pengaturan", label: "Pengaturan", icon: Settings, ownerOnly: true },
];

/** Nav items visible to the given role (owner sees all; kasir sees non-owner items). */
export function navItemsForRole(role: Role | undefined): NavItem[] {
  if (role === "owner") return NAV_ITEMS;
  return NAV_ITEMS.filter((i) => !i.ownerOnly);
}
```

- [ ] **Step 7: Typecheck + build**

```bash
cd kios-dashboard && npm run typecheck 2>&1 | tail -5; cd ..
```

Expected: bersih.

- [ ] **Step 8: Commit**

```bash
git add \
  kios-dashboard/src/app/\(app\)/pelanggan/page.tsx \
  kios-dashboard/src/app/\(app\)/pelanggan/\[id\]/page.tsx \
  kios-dashboard/src/app/\(app\)/pelanggan/actions.ts \
  kios-dashboard/src/components/pelanggan/pelanggan-list.tsx \
  kios-dashboard/src/components/pelanggan/pelanggan-detail.tsx \
  kios-dashboard/src/components/nav-items.tsx \
  kios-dashboard/src/lib/kios.ts
git commit -m "feat(dashboard): halaman Pelanggan + detail + deleteCustomerAction (owner-only)"
```

---

## Task 5: Barcode scan kasir (BarcodeDetector)

**Files:**
- Create: `kios-dashboard/src/components/kasir/barcode-scanner.tsx`
- Modify: `kios-dashboard/src/components/kasir/kasir-form.tsx`

### Sub-task 5a: Komponen `barcode-scanner.tsx`

- [ ] **Step 1: Buat `kios-dashboard/src/components/kasir/barcode-scanner.tsx`**

BarcodeDetector belum ada di TypeScript `lib.dom` semua env — deklarasikan manual di dalam file.

```tsx
"use client";

import { useEffect, useRef, useState } from "react";
import { ScanLine, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Modal } from "@/components/ui/modal";

// ── Manual type declaration (BarcodeDetector belum di semua lib.dom versi) ──
interface BarcodeDetectorResult {
  rawValue: string;
  format: string;
}

interface BarcodeDetectorConstructor {
  new (options?: { formats?: string[] }): {
    detect(source: HTMLVideoElement | ImageBitmap): Promise<BarcodeDetectorResult[]>;
  };
  getSupportedFormats(): Promise<string[]>;
}

declare global {
  interface Window {
    BarcodeDetector?: BarcodeDetectorConstructor;
  }
}

interface Props {
  onDetected: (code: string) => void;
}

export function BarcodeScanner({ onDetected }: Props) {
  const [supported, setSupported] = useState<boolean | null>(null);
  const [scanning, setScanning] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const rafRef = useRef<number | null>(null);

  useEffect(() => {
    setSupported(typeof window !== "undefined" && "BarcodeDetector" in window);
  }, []);

  async function startScan() {
    setError(null);
    setScanning(true);
  }

  async function stopScan() {
    if (rafRef.current !== null) {
      cancelAnimationFrame(rafRef.current);
      rafRef.current = null;
    }
    if (streamRef.current) {
      streamRef.current.getTracks().forEach((t) => t.stop());
      streamRef.current = null;
    }
    setScanning(false);
  }

  useEffect(() => {
    if (!scanning) return;
    if (!window.BarcodeDetector) return;

    let cancelled = false;
    const detector = new window.BarcodeDetector({
      formats: ["ean_13", "ean_8", "code_128", "qr_code"],
    });

    async function init() {
      try {
        const stream = await navigator.mediaDevices.getUserMedia({
          video: { facingMode: "environment" },
        });
        if (cancelled) {
          stream.getTracks().forEach((t) => t.stop());
          return;
        }
        streamRef.current = stream;
        if (videoRef.current) {
          videoRef.current.srcObject = stream;
          await videoRef.current.play();
        }
        tick();
      } catch (err) {
        if (!cancelled) {
          setError("Tidak bisa mengakses kamera. Pastikan izin kamera diperbolehkan.");
          setScanning(false);
        }
      }
    }

    function tick() {
      if (cancelled || !videoRef.current) return;
      const video = videoRef.current;
      if (video.readyState < video.HAVE_ENOUGH_DATA) {
        rafRef.current = requestAnimationFrame(tick);
        return;
      }
      detector
        .detect(video)
        .then((results) => {
          if (cancelled) return;
          if (results.length > 0) {
            const raw = results[0].rawValue;
            stopScan();
            onDetected(raw);
          } else {
            rafRef.current = requestAnimationFrame(tick);
          }
        })
        .catch(() => {
          if (!cancelled) rafRef.current = requestAnimationFrame(tick);
        });
    }

    void init();

    return () => {
      cancelled = true;
      if (rafRef.current !== null) cancelAnimationFrame(rafRef.current);
      if (streamRef.current) streamRef.current.getTracks().forEach((t) => t.stop());
    };
  }, [scanning]); // eslint-disable-line react-hooks/exhaustive-deps

  // Not yet determined (SSR / mount)
  if (supported === null) return null;

  // Browser tidak support: teks fallback saja
  if (!supported) {
    return (
      <span className="text-xs text-muted-foreground" title="Scanner kamera tidak didukung — ketik barcode manual atau gunakan scanner USB">
        <ScanLine className="inline size-4 opacity-40" />
      </span>
    );
  }

  return (
    <>
      <Button
        type="button"
        variant="outline"
        size="sm"
        onClick={startScan}
        aria-label="Scan barcode produk"
        title="Scan barcode (kamera)"
      >
        <ScanLine className="size-4" />
      </Button>

      <Modal
        open={scanning}
        onClose={stopScan}
        title="Scan Barcode"
        description="Arahkan kamera ke barcode produk."
      >
        <div className="space-y-3">
          {error ? (
            <p role="alert" className="text-sm text-destructive">{error}</p>
          ) : (
            <video
              ref={videoRef}
              muted
              playsInline
              className="w-full rounded-lg border bg-black aspect-video object-cover"
              aria-label="Kamera barcode scanner"
            />
          )}
          <Button variant="outline" size="sm" className="w-full" onClick={stopScan}>
            <X className="size-4" /> Batal
          </Button>
        </div>
      </Modal>
    </>
  );
}
```

### Sub-task 5b: Integrasi ke `kasir-form.tsx`

- [ ] **Step 2: Tambah `<BarcodeScanner>` ke `kasir-form.tsx`**

Di `kasir-form.tsx`, tambahkan import `BarcodeScanner`:

```tsx
import { BarcodeScanner } from "@/components/kasir/barcode-scanner";
```

Lalu ganti blok input search (sekitar baris 104–113) dari:

```tsx
<div className="relative">
  <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
  <Input
    value={query}
    onChange={(e) => setQuery(e.target.value)}
    placeholder="Cari nama / ID / barcode…"
    className="pl-9"
    aria-label="Cari produk"
  />
</div>
```

Menjadi:

```tsx
<div className="flex gap-2">
  <div className="relative flex-1">
    <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
    <Input
      value={query}
      onChange={(e) => setQuery(e.target.value)}
      placeholder="Cari nama / ID / barcode…"
      className="pl-9"
      aria-label="Cari produk"
    />
  </div>
  <BarcodeScanner onDetected={(code) => setQuery(code)} />
</div>
```

- [ ] **Step 3: Typecheck**

```bash
cd kios-dashboard && npm run typecheck 2>&1 | tail -5; cd ..
```

Expected: bersih.

- [ ] **Step 4: Commit**

```bash
git add \
  kios-dashboard/src/components/kasir/barcode-scanner.tsx \
  kios-dashboard/src/components/kasir/kasir-form.tsx
git commit -m "feat(kasir): BarcodeScanner via BarcodeDetector Web API, tanpa dep baru"
```

---

## Self-Review

**Spec coverage check:**

| Spec requirement | Task |
|---|---|
| PelangganID di struct Pesanan (Go + TS) | Task 1 |
| isValidWaNumber di wa.ts | Task 2a |
| Nama + WA wajib, error inline, aria-invalid | Task 2b |
| QRIS button: sembunyikan generik saat qrisChatFlow | Task 2b |
| submit() gate !formOk + setTouched | Task 2b |
| kontak_wa canonical di body | Task 2b |
| Server-side validasi nama + WA | Task 3 |
| Drop fallback "Pembeli" | Task 3 |
| upsertPelanggan sebelum setPesanan | Task 3 |
| pelanggan_id di pesanan object | Task 3 |
| Halaman `/pelanggan` list + search | Task 4b |
| Tabel: nama, WA, total_pesanan, total_belanja, piutang badge | Task 4b |
| Halaman `/pelanggan/[id]` detail + riwayat | Task 4c |
| WA link di detail | Task 4d (pelanggan-detail.tsx) |
| deleteCustomerAction + ensureOwner | Task 4d |
| delPelanggan di kios.ts | Task 4d |
| Nav item Contact bukan ownerOnly | Task 4e |
| BarcodeScanner tanpa dep baru | Task 5a |
| TypeScript type manual BarcodeDetector | Task 5a |
| getUserMedia facingMode environment | Task 5a |
| formats: ean_13, ean_8, code_128, qr_code | Task 5a |
| onDetected → setQuery di kasir-form | Task 5b |
| Fallback text bila tidak supported | Task 5a |
| npm run typecheck clean | Setiap task |
| go test -tags goolm,stdjson | Task 1 |

**Placeholder scan:** tidak ada TBD, TODO, atau "similar to" dalam plan ini.

**Type consistency:**
- `isValidWaNumber` dideklarasikan di `wa.ts` (Task 2a) dan dipakai di `storefront-view.tsx` (Task 2b) serta `route.ts` (Task 3). ✓
- `normalizeWaNumber` sudah ada di `wa.ts` sebelum plan ini. ✓
- `upsertPelanggan` didefinisikan di Plan 1 (`kios.ts`), diimport di `route.ts` Task 3. ✓
- `getPelanggan` dipakai di Task 4c — didefinisikan di Plan 1. ✓
- `getAllPelanggan` dipakai di Task 4a — didefinisikan di Plan 1. ✓
- `delPelanggan` perlu ditambah ke `kios.ts` — tercover di Step 5 Task 4d. ✓
- `Pelanggan` interface dari Plan 1 dipakai di semua komponen pelanggan. ✓
- `pelanggan_id?: string` di `interface Pesanan` (Task 1 Step 4) dipakai di filter `riwayat` (Task 4c). ✓
- `BarcodeScanner` props `onDetected: (code: string) => void` konsisten antara Task 5a (definisi) dan Task 5b (penggunaan). ✓

---

### Critical Files for Implementation

- `/home/kevinman/Publik/project/kios-picoclaw/kios-dashboard/src/components/toko/storefront-view.tsx`
- `/home/kevinman/Publik/project/kios-picoclaw/kios-dashboard/src/app/api/pesanan/route.ts`
- `/home/kevinman/Publik/project/kios-picoclaw/kios-dashboard/src/components/kasir/barcode-scanner.tsx`
- `/home/kevinman/Publik/project/kios-picoclaw/pkg/tools/kios/store.go`
- `/home/kevinman/Publik/project/kios-picoclaw/kios-dashboard/src/lib/wa.ts`

---

**Catatan:** Plan ini adalah READ-ONLY output — implementasi harus disimpan ke `/home/kevinman/Publik/project/kios-picoclaw/docs/specs/2026-06-03-kios-plan-D-storefront-barcode.md` oleh executor. Plan mencakup **5 tasks, 25+ steps**, semua kode JSX/TSX/Go lengkap verbatim sesuai spec.