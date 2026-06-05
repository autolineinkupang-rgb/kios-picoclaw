"use client";

import { useMemo, useState, useTransition } from "react";

function isValidHp(raw: string): boolean {
  const d = raw.replace(/\D/g, "");
  return (
    d.length >= 10 &&
    d.length <= 15 &&
    (d.startsWith("62") || d.startsWith("08") || d.startsWith("8"))
  );
}
import { useRouter } from "next/navigation";
import {
  CheckCircle2,
  Loader2,
  Minus,
  Plus,
  Search,
  ShoppingCart,
  Trash2,
  TriangleAlert,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input, Label, Select } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { formatRupiah } from "@/lib/format";
import { stokStatus, STATUS_META } from "@/lib/produk-status";
import { cn, matchesQuery } from "@/lib/utils";
import type { Produk } from "@/lib/types";
import { checkoutAction, type CheckoutResult } from "@/app/(app)/kasir/actions";

const JENIS_META: Record<string, { label: string; variant: "accent" | "warning" | "secondary" | "success" }> = {
  pulsa: { label: "Pulsa", variant: "accent" },
  bensin: { label: "Bensin", variant: "warning" },
  solar: { label: "Solar", variant: "secondary" },
  minyak_tanah: { label: "Minyak Tanah", variant: "success" },
};

interface CartLine {
  produk: Produk;
  qty: number;
}

export function KasirForm({ produk }: { produk: Produk[] }) {
  const router = useRouter();
  const [query, setQuery] = useState("");
  const [cart, setCart] = useState<CartLine[]>([]);
  const [metode, setMetode] = useState("tunai");
  const [bayar, setBayar] = useState("");
  const [bonPhone, setBonPhone] = useState("");
  const [bonError, setBonError] = useState("");
  const [result, setResult] = useState<CheckoutResult | null>(null);
  const [pending, start] = useTransition();

  const matches = useMemo(() => {
    const q = query.trim().toLowerCase();
    const list = q
      ? produk.filter((p) => matchesQuery(q, p.nama, p.id, p.barcode))
      : produk;
    return list.slice(0, 12);
  }, [produk, query]);

  const total = cart.reduce((s, l) => s + l.qty * l.produk.harga_jual, 0);
  const bayarNum = Number(bayar.replace(/\D/g, "")) || 0;
  const kembalian = bayarNum >= total && bayarNum > 0 ? bayarNum - total : null;

  function addToCart(p: Produk) {
    setResult(null);
    setCart((prev) => {
      const idx = prev.findIndex((l) => l.produk.id === p.id);
      if (idx === -1) return [...prev, { produk: p, qty: 1 }];
      const next = [...prev];
      next[idx] = { ...next[idx], qty: Math.min(next[idx].qty + 1, p.stok) };
      return next;
    });
  }

  function setQty(id: string, qty: number) {
    setCart((prev) =>
      prev.map((l) =>
        l.produk.id === id
          ? { ...l, qty: Math.max(1, Math.min(qty || 1, l.produk.stok)) }
          : l,
      ),
    );
  }

  function removeLine(id: string) {
    setCart((prev) => prev.filter((l) => l.produk.id !== id));
  }

  function checkout() {
    if (cart.length === 0) return;
    if (metode === "bon" && !isValidHp(bonPhone)) {
      setBonError("Nomor HP pelanggan tidak valid untuk metode bon.");
      return;
    }
    setBonError("");
    start(async () => {
      const r = await checkoutAction({
        items: cart.map((l) => ({ produkId: l.produk.id, qty: l.qty })),
        metode,
        bayar: bayarNum > 0 ? bayarNum : undefined,
        pelangganPhone: metode === "bon" ? bonPhone : undefined,
      });
      setResult(r);
      if (r.ok) {
        setCart([]);
        setBayar("");
        setBonPhone("");
        setBonError("");
        setQuery("");
        router.refresh();
        window.setTimeout(() => setResult(null), 8000);
      }
    });
  }

  return (
    <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-5">
      {/* Product picker */}
      <Card className="md:col-span-1 lg:col-span-3">
        <CardHeader>
          <CardTitle>Pilih Produk</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
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
          <ul className="grid max-h-[28rem] grid-cols-1 gap-1.5 overflow-y-auto sm:grid-cols-2">
            {matches.length === 0 && (
              <li className="col-span-full py-6 text-center text-sm text-muted-foreground">
                Produk tidak ditemukan.
              </li>
            )}
            {matches.map((p) => {
              const st = STATUS_META[stokStatus(p)];
              const inCart = cart.find((l) => l.produk.id === p.id)?.qty ?? 0;
              const habis = p.stok <= 0;
              const maxed = inCart >= p.stok;
              return (
                <li key={p.id}>
                  <button
                    type="button"
                    disabled={habis || maxed}
                    onClick={() => addToCart(p)}
                    className={cn(
                      "flex w-full items-center justify-between gap-2 rounded-lg border p-3 text-left transition-colors",
                      inCart > 0 ? "border-accent/40 bg-accent/5" : "border-border hover:bg-muted",
                      (habis || maxed) && "cursor-not-allowed opacity-50",
                    )}
                  >
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
                    <div className="flex shrink-0 flex-col items-end gap-1">
                      <Badge variant={st.variant}>
                        {p.stok} {p.satuan}
                      </Badge>
                      {inCart > 0 && (
                        <span className="text-xs font-medium text-accent">{inCart} di keranjang</span>
                      )}
                    </div>
                  </button>
                </li>
              );
            })}
          </ul>
        </CardContent>
      </Card>

      {/* Cart */}
      <Card className="md:col-span-1 lg:col-span-2">
        <CardHeader className="flex-row items-center justify-between">
          <CardTitle>Keranjang</CardTitle>
          {cart.length > 0 && (
            <button
              type="button"
              onClick={() => setCart([])}
              className="text-xs font-medium text-muted-foreground hover:text-destructive"
            >
              Kosongkan
            </button>
          )}
        </CardHeader>
        <CardContent className="space-y-4">
          {cart.length === 0 ? (
            <EmptyState
              icon={ShoppingCart}
              title="Keranjang kosong"
              description="Klik produk di kiri untuk menambah ke keranjang."
            />
          ) : (
            <>
              <ul className="space-y-2">
                {cart.map((l) => {
                  const sub = l.qty * l.produk.harga_jual;
                  return (
                    <li key={l.produk.id} className="rounded-lg border p-2.5">
                      <div className="flex items-start justify-between gap-2">
                        <div className="min-w-0">
                          <div className="flex items-center gap-1.5 flex-wrap">
                            <p className="truncate text-sm font-medium">{l.produk.nama}</p>
                            {l.produk.jenis && JENIS_META[l.produk.jenis] && (
                              <Badge variant={JENIS_META[l.produk.jenis].variant} className="text-[10px] px-1.5 py-0">
                                {JENIS_META[l.produk.jenis].label}
                              </Badge>
                            )}
                          </div>
                          <p className="font-mono text-xs text-muted-foreground">
                            {formatRupiah(l.produk.harga_jual)} · stok {l.produk.stok}
                          </p>
                        </div>
                        <button
                          type="button"
                          onClick={() => removeLine(l.produk.id)}
                          aria-label={`Hapus ${l.produk.nama}`}
                          className="flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                        >
                          <Trash2 className="size-4" />
                        </button>
                      </div>
                      <div className="mt-2 flex items-center justify-between">
                        <div className="flex items-center gap-1">
                          <button
                            type="button"
                            onClick={() => setQty(l.produk.id, l.qty - 1)}
                            disabled={l.qty <= 1}
                            aria-label="Kurangi"
                            className="flex size-8 cursor-pointer items-center justify-center rounded-md border border-input hover:bg-muted disabled:opacity-40"
                          >
                            <Minus className="size-3.5" />
                          </button>
                          <input
                            value={String(l.qty)}
                            onChange={(e) => setQty(l.produk.id, Number(e.target.value.replace(/\D/g, "")))}
                            inputMode="numeric"
                            aria-label={`Jumlah ${l.produk.nama}`}
                            className="h-8 w-12 rounded-md border border-input bg-background text-center font-mono text-sm tabular-nums outline-none focus-visible:ring-2 focus-visible:ring-ring/30"
                          />
                          <button
                            type="button"
                            onClick={() => setQty(l.produk.id, l.qty + 1)}
                            disabled={l.qty >= l.produk.stok}
                            aria-label="Tambah"
                            className="flex size-8 cursor-pointer items-center justify-center rounded-md border border-input hover:bg-muted disabled:opacity-40"
                          >
                            <Plus className="size-3.5" />
                          </button>
                        </div>
                        <span className="font-mono text-sm font-medium tabular-nums">
                          {formatRupiah(sub)}
                        </span>
                      </div>
                    </li>
                  );
                })}
              </ul>

              <div className="space-y-3 border-t pt-3">
                <div className="grid grid-cols-2 gap-3">
                  <div className="space-y-1.5">
                    <Label htmlFor="metode">Metode</Label>
                    <Select id="metode" value={metode} onChange={(e) => setMetode(e.target.value)}>
                      <option value="tunai">Tunai</option>
                      <option value="transfer">Transfer</option>
                      <option value="qris">QRIS</option>
                      <option value="bon">Bon (Kredit)</option>
                    </Select>
                  </div>
                  {metode !== "bon" && (
                    <div className="space-y-1.5">
                      <Label htmlFor="bayar">Bayar</Label>
                      <Input
                        id="bayar"
                        inputMode="numeric"
                        value={bayar}
                        onChange={(e) => setBayar(e.target.value.replace(/\D/g, ""))}
                        placeholder="opsional"
                        className="font-mono tabular-nums"
                      />
                    </div>
                  )}
                </div>

                {metode === "bon" && (
                  <div className="space-y-1.5">
                    <label htmlFor="bon-phone" className="text-sm font-medium">
                      No HP Pelanggan <span className="text-destructive">*</span>
                    </label>
                    <Input
                      id="bon-phone"
                      value={bonPhone}
                      onChange={(e) => { setBonPhone(e.target.value); setBonError(""); }}
                      placeholder="08123456789 atau 628123456789"
                      inputMode="tel"
                    />
                    {bonPhone && !isValidHp(bonPhone) && (
                      <p className="text-xs text-destructive">Format: 08xxx atau 628xxx (10–15 digit)</p>
                    )}
                    {bonError && (
                      <p className="text-xs text-destructive">{bonError}</p>
                    )}
                  </div>
                )}

                <div className="space-y-1 text-sm">
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Total ({cart.length} item)</span>
                    <span className="text-base font-semibold tabular-nums">{formatRupiah(total)}</span>
                  </div>
                  {metode !== "bon" && kembalian !== null && (
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Kembalian</span>
                      <span className="font-medium tabular-nums">{formatRupiah(kembalian)}</span>
                    </div>
                  )}
                  {metode !== "bon" && bayarNum > 0 && bayarNum < total && (
                    <p className="text-xs text-destructive">Uang kurang {formatRupiah(total - bayarNum)}.</p>
                  )}
                </div>

                <Button
                  variant="accent"
                  size="md"
                  className="w-full"
                  onClick={checkout}
                  disabled={pending || cart.length === 0}
                >
                  {pending ? <Loader2 className="size-4 animate-spin" /> : <ShoppingCart className="size-4" />}
                  {metode === "bon" ? "Catat Bon" : "Bayar"}
                </Button>
              </div>
            </>
          )}

          {result && (
            <div
              role="status"
              aria-live="polite"
              className={cn(
                "rounded-lg border p-3 text-sm",
                result.ok
                  ? "border-success/30 bg-success/10"
                  : "border-destructive/30 bg-destructive/10 text-destructive",
              )}
            >
              {result.ok ? (
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
                  {result.piutang_id && (
                    <p className="text-sm text-muted-foreground">
                      Piutang: <span className="font-mono font-medium">{result.piutang_id}</span>
                      {" · "}Rp {formatRupiah(result.total)} (belum dibayar)
                    </p>
                  )}
                </div>
              ) : (
                <p className="flex items-center gap-1.5">
                  <TriangleAlert className="size-4" /> {result.error}
                </p>
              )}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
