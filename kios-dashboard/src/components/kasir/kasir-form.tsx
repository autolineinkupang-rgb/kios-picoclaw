"use client";

import { useMemo, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { CheckCircle2, Loader2, Search, ShoppingCart, TriangleAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input, Label, Select } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { formatRupiah } from "@/lib/format";
import { stokStatus, STATUS_META } from "@/lib/produk-status";
import { cn } from "@/lib/utils";
import type { Produk } from "@/lib/types";
import { jualAction, type JualResult } from "@/app/(app)/kasir/actions";

export function KasirForm({ produk }: { produk: Produk[] }) {
  const router = useRouter();
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<Produk | null>(null);
  const [qty, setQty] = useState(1);
  const [metode, setMetode] = useState("tunai");
  const [bayar, setBayar] = useState("");
  const [result, setResult] = useState<JualResult | null>(null);
  const [pending, start] = useTransition();

  const matches = useMemo(() => {
    const q = query.trim().toLowerCase();
    const list = q
      ? produk.filter(
          (p) =>
            p.nama.toLowerCase().includes(q) ||
            p.id.toLowerCase().includes(q) ||
            p.barcode.toLowerCase().includes(q),
        )
      : produk;
    return list.slice(0, 8);
  }, [produk, query]);

  const total = selected ? qty * selected.harga_jual : 0;
  const bayarNum = Number(bayar.replace(/\D/g, "")) || 0;
  const kembalian = bayarNum >= total && bayarNum > 0 ? bayarNum - total : null;
  const canSubmit =
    selected !== null && qty > 0 && qty <= selected.stok && !pending;

  function submit() {
    if (!selected) return;
    start(async () => {
      const r = await jualAction({
        produkId: selected.id,
        qty,
        metode,
        bayar: bayarNum > 0 ? bayarNum : undefined,
      });
      setResult(r);
      if (r.ok) {
        setSelected(null);
        setQty(1);
        setBayar("");
        setQuery("");
        router.refresh();
        window.setTimeout(() => setResult(null), 6000);
      }
    });
  }

  return (
    <div className="grid grid-cols-1 gap-4 lg:grid-cols-5">
      {/* Product picker */}
      <Card className="lg:col-span-3">
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
          <ul className="max-h-80 space-y-1 overflow-y-auto">
            {matches.length === 0 && (
              <li className="py-6 text-center text-sm text-muted-foreground">
                Produk tidak ditemukan.
              </li>
            )}
            {matches.map((p) => {
              const st = STATUS_META[stokStatus(p)];
              const active = selected?.id === p.id;
              const habis = p.stok <= 0;
              return (
                <li key={p.id}>
                  <button
                    type="button"
                    disabled={habis}
                    onClick={() => {
                      setSelected(p);
                      setQty(1);
                    }}
                    className={cn(
                      "flex w-full items-center justify-between gap-3 rounded-lg border p-3 text-left transition-colors",
                      active ? "border-accent bg-accent/10" : "border-transparent hover:bg-muted",
                      habis && "cursor-not-allowed opacity-50",
                    )}
                  >
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">{p.nama}</p>
                      <p className="font-mono text-xs text-muted-foreground">
                        {p.id} · {formatRupiah(p.harga_jual)}
                      </p>
                    </div>
                    <Badge variant={st.variant}>
                      {p.stok} {p.satuan}
                    </Badge>
                  </button>
                </li>
              );
            })}
          </ul>
        </CardContent>
      </Card>

      {/* Cart / checkout */}
      <Card className="lg:col-span-2">
        <CardHeader>
          <CardTitle>Transaksi</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {!selected ? (
            <p className="py-8 text-center text-sm text-muted-foreground">
              Pilih produk dulu di sebelah kiri.
            </p>
          ) : (
            <>
              <div className="rounded-lg border bg-muted/30 p-3">
                <p className="text-sm font-medium">{selected.nama}</p>
                <p className="text-xs text-muted-foreground">
                  {formatRupiah(selected.harga_jual)} / {selected.satuan} · stok {selected.stok}
                </p>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="qty">Jumlah</Label>
                <Input
                  id="qty"
                  type="number"
                  inputMode="numeric"
                  min={1}
                  max={selected.stok}
                  value={String(qty)}
                  onChange={(e) => setQty(Math.max(1, Number(e.target.value) || 1))}
                  className="font-mono tabular-nums"
                />
                {qty > selected.stok && (
                  <p className="text-xs text-destructive">Melebihi stok ({selected.stok}).</p>
                )}
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="metode">Metode Bayar</Label>
                <Select id="metode" value={metode} onChange={(e) => setMetode(e.target.value)}>
                  <option value="tunai">Tunai</option>
                  <option value="transfer">Transfer</option>
                  <option value="qris">QRIS</option>
                </Select>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="bayar">Uang Dibayar (opsional)</Label>
                <Input
                  id="bayar"
                  inputMode="numeric"
                  value={bayar}
                  onChange={(e) => setBayar(e.target.value.replace(/\D/g, ""))}
                  placeholder="untuk hitung kembalian"
                  className="font-mono tabular-nums"
                />
              </div>

              <div className="space-y-1 border-t pt-3 text-sm">
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Total</span>
                  <span className="font-semibold tabular-nums">{formatRupiah(total)}</span>
                </div>
                {kembalian !== null && (
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Kembalian</span>
                    <span className="font-medium tabular-nums">{formatRupiah(kembalian)}</span>
                  </div>
                )}
                {bayarNum > 0 && bayarNum < total && (
                  <p className="text-xs text-destructive">
                    Uang kurang {formatRupiah(total - bayarNum)}.
                  </p>
                )}
              </div>

              <Button
                variant="accent"
                size="md"
                className="w-full"
                onClick={submit}
                disabled={!canSubmit}
              >
                {pending ? <Loader2 className="size-4 animate-spin" /> : <ShoppingCart className="size-4" />}
                Catat Penjualan
              </Button>
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
                    <CheckCircle2 className="size-4" /> Tercatat
                  </p>
                  <pre className="font-mono text-xs whitespace-pre-wrap text-foreground">
                    {result.struk}
                  </pre>
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
