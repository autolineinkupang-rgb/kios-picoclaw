"use client";

import { useMemo, useState } from "react";
import {
  CheckCircle2,
  Loader2,
  Minus,
  Plus,
  Search,
  ShoppingBag,
  Trash2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input, Label, Select } from "@/components/ui/input";
import { Modal } from "@/components/ui/modal";
import { EmptyState } from "@/components/ui/empty-state";
import { formatRupiah } from "@/lib/format";
import { cn } from "@/lib/utils";

export interface PublicProduk {
  id: string;
  nama: string;
  kategori: string;
  satuan: string;
  harga_jual: number;
  stok: number;
}

interface CartLine {
  id: string;
  nama: string;
  harga: number;
  satuan: string;
  stok: number;
  qty: number;
}

export function TokoView({ produk }: { produk: PublicProduk[] }) {
  const [query, setQuery] = useState("");
  const [kategori, setKategori] = useState("");
  const [cart, setCart] = useState<CartLine[]>([]);
  const [open, setOpen] = useState(false);
  const [nama, setNama] = useState("");
  const [kontak, setKontak] = useState("");
  const [catatan, setCatatan] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState<{ id: string; total: number } | null>(null);

  const categories = useMemo(
    () => [...new Set(produk.map((p) => p.kategori).filter(Boolean))].sort(),
    [produk],
  );

  const rows = useMemo(() => {
    const q = query.trim().toLowerCase();
    return produk.filter((p) => {
      if (kategori && p.kategori !== kategori) return false;
      if (!q) return true;
      return p.nama.toLowerCase().includes(q);
    });
  }, [produk, query, kategori]);

  const count = cart.reduce((s, l) => s + l.qty, 0);
  const total = cart.reduce((s, l) => s + l.qty * l.harga, 0);

  function qtyOf(id: string) {
    return cart.find((l) => l.id === id)?.qty ?? 0;
  }

  function add(p: PublicProduk) {
    setCart((prev) => {
      const idx = prev.findIndex((l) => l.id === p.id);
      if (idx === -1)
        return [
          ...prev,
          { id: p.id, nama: p.nama, harga: p.harga_jual, satuan: p.satuan, stok: p.stok, qty: 1 },
        ];
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
    setError(null);
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
        setError(data.error || "Gagal mengirim pesanan.");
        return;
      }
      setDone({ id: data.id, total: data.total });
      setCart([]);
      setNama("");
      setKontak("");
      setCatatan("");
    } catch {
      setError("Gagal terhubung. Cek koneksi internet ya.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="space-y-4 pb-24">
      {/* Filters */}
      <div className="flex flex-col gap-3 sm:flex-row">
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Cari produk…"
            className="pl-9"
            aria-label="Cari produk"
          />
        </div>
        <Select
          value={kategori}
          onChange={(e) => setKategori(e.target.value)}
          aria-label="Filter kategori"
          className="sm:w-44"
        >
          <option value="">Semua kategori</option>
          {categories.map((c) => (
            <option key={c} value={c}>
              {c}
            </option>
          ))}
        </Select>
      </div>

      {/* Product grid */}
      {rows.length === 0 ? (
        <EmptyState icon={ShoppingBag} title="Produk tidak ditemukan" />
      ) : (
        <ul className="grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-4">
          {rows.map((p) => {
            const inCart = qtyOf(p.id);
            const habis = p.stok <= 0;
            return (
              <li
                key={p.id}
                className="flex flex-col rounded-xl border bg-card p-3 shadow-sm"
              >
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

      {/* Sticky cart bar */}
      {count > 0 && (
        <div className="fixed inset-x-0 bottom-0 z-40 border-t bg-background/95 p-3 backdrop-blur">
          <div className="mx-auto flex max-w-5xl items-center gap-3">
            <div className="flex-1">
              <p className="text-xs text-muted-foreground">{count} item</p>
              <p className="font-mono text-base font-semibold tabular-nums">{formatRupiah(total)}</p>
            </div>
            <Button variant="accent" size="md" onClick={() => setOpen(true)}>
              <ShoppingBag className="size-4" /> Lihat Keranjang
            </Button>
          </div>
        </div>
      )}

      {/* Cart + checkout sheet */}
      <Modal open={open} onClose={() => setOpen(false)} title="Keranjang" description="Periksa pesanan & kirim ke kios.">
        <div className="space-y-4">
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
              <Input
                id="kontak"
                value={kontak}
                onChange={(e) => setKontak(e.target.value)}
                placeholder="08xx"
                inputMode="tel"
                autoComplete="tel"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="catatan">Catatan (opsional)</Label>
              <Input id="catatan" value={catatan} onChange={(e) => setCatatan(e.target.value)} placeholder="mis. ambil jam 5 sore" />
            </div>
          </div>

          {error && (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          )}

          <Button
            variant="accent"
            size="md"
            className="w-full"
            onClick={submit}
            disabled={submitting || cart.length === 0}
          >
            {submitting && <Loader2 className="size-4 animate-spin" />}
            Kirim Pesanan
          </Button>
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
              Total {done ? formatRupiah(done.total) : ""}. Kasir kios akan segera memproses pesananmu.
            </p>
          </div>
          <Button
            variant="outline"
            size="md"
            className="w-full"
            onClick={() => {
              setDone(null);
              setOpen(false);
            }}
          >
            Selesai
          </Button>
        </div>
      </Modal>
    </div>
  );
}
