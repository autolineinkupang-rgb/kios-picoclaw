"use client";

import { useEffect, useMemo, useState } from "react";
import {
  CheckCircle2,
  Loader2,
  Minus,
  Plus,
  RefreshCw,
  Search,
  ShoppingBag,
  Store,
  Trash2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input, Label } from "@/components/ui/input";
import { Modal } from "@/components/ui/modal";
import { EmptyState } from "@/components/ui/empty-state";
import { formatRupiah } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { PublicProduk } from "@/lib/types";

interface CartLine {
  id: string;
  nama: string;
  harga: number;
  satuan: string;
  stok: number;
  qty: number;
}

export function StorefrontView() {
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  const [produk, setProduk] = useState<PublicProduk[]>([]);
  const [kategori, setKategori] = useState<string[]>([]);

  const [query, setQuery] = useState("");
  const [cat, setCat] = useState("");
  const [cart, setCart] = useState<CartLine[]>([]);
  const [open, setOpen] = useState(false);
  const [nama, setNama] = useState("");
  const [kontak, setKontak] = useState("");
  const [catatan, setCatatan] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [done, setDone] = useState<{ id: string; total: number } | null>(null);

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
      return p.nama.toLowerCase().includes(q);
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
    setSubmitError(null);
    setSubmitting(true);
    try {
      const res = await fetch("/api/pesanan", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          items: cart.map((l) => ({ produkId: l.id, qty: l.qty })),
          nama,
          kontak,
          catatan,
        }),
      });
      const data = await res.json();
      if (!res.ok || !data.ok) {
        setSubmitError(data.error || "Gagal mengirim pesanan.");
        return;
      }
      setDone({ id: data.id, total: data.total });
      setCart([]);
      setNama("");
      setKontak("");
      setCatatan("");
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
                <li key={p.id} className="flex flex-col rounded-xl border bg-card p-3 shadow-sm">
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
      <Modal open={open} onClose={() => setOpen(false)} title="Keranjang" description="Periksa pesanan & kirim ke kios.">
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

              <div className="space-y-3">
                <div className="space-y-1.5">
                  <Label htmlFor="nama">Nama (opsional)</Label>
                  <Input id="nama" value={nama} onChange={(e) => setNama(e.target.value)} placeholder="Nama kamu" autoComplete="name" />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="kontak">No. HP / WA (opsional)</Label>
                  <Input id="kontak" value={kontak} onChange={(e) => setKontak(e.target.value)} placeholder="08xx" inputMode="tel" autoComplete="tel" />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="catatan">Catatan (opsional)</Label>
                  <Input id="catatan" value={catatan} onChange={(e) => setCatatan(e.target.value)} placeholder="mis. ambil jam 5 sore" />
                </div>
              </div>

              {submitError && (
                <p role="alert" className="text-sm text-destructive">
                  {submitError}
                </p>
              )}

              <Button variant="accent" size="md" className="w-full" onClick={submit} disabled={submitting}>
                {submitting && <Loader2 className="size-4 animate-spin" />}
                Kirim Pesanan
              </Button>
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
              Total {done ? formatRupiah(done.total) : ""}. Kasir kios akan segera memprosesnya.
            </p>
          </div>
          <Button variant="outline" size="md" className="w-full" onClick={() => setDone(null)}>
            Selesai
          </Button>
        </div>
      </Modal>
    </div>
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
