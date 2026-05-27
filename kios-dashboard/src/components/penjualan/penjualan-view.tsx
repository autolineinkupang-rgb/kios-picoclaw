"use client";

import { useMemo, useState } from "react";
import { Receipt, Search } from "lucide-react";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Input, Select } from "@/components/ui/input";
import { EmptyState } from "@/components/ui/empty-state";
import { PeriodeTabs } from "@/components/periode-tabs";
import { filterPeriode, formatNumber, formatRupiah, periodeLabel } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { Periode, Transaksi } from "@/lib/types";

const METODE_VARIANT: Record<string, "secondary" | "default" | "success"> = {
  tunai: "secondary",
  transfer: "default",
  qris: "success",
};

const MAX_ROWS = 300;

export function PenjualanView({ transaksi }: { transaksi: Transaksi[] }) {
  const [periode, setPeriode] = useState<Periode>("hari_ini");
  const [query, setQuery] = useState("");
  const [metode, setMetode] = useState("");

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return filterPeriode(transaksi, periode)
      .filter((t) => (metode ? t.metode_bayar === metode : true))
      .filter((t) =>
        q
          ? t.nama_produk.toLowerCase().includes(q) ||
            t.id.toLowerCase().includes(q) ||
            t.kasir.toLowerCase().includes(q)
          : true,
      )
      .slice()
      .reverse(); // newest first
  }, [transaksi, periode, query, metode]);

  const summary = useMemo(() => {
    const omzet = filtered.reduce((s, t) => s + t.total, 0);
    const qty = filtered.reduce((s, t) => s + t.qty, 0);
    const avg = filtered.length ? Math.round(omzet / filtered.length) : 0;
    return { omzet, qty, jumlah: filtered.length, avg };
  }, [filtered]);

  const shown = filtered.slice(0, MAX_ROWS);

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <PeriodeTabs value={periode} onChange={setPeriode} />
        <div className="flex flex-col gap-3 sm:flex-row">
          <div className="relative flex-1">
            <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Cari produk / ID / kasir…"
              className="pl-9 sm:w-64"
              aria-label="Cari transaksi"
            />
          </div>
          <Select
            value={metode}
            onChange={(e) => setMetode(e.target.value)}
            aria-label="Filter metode bayar"
            className="sm:w-36"
          >
            <option value="">Semua metode</option>
            <option value="tunai">Tunai</option>
            <option value="transfer">Transfer</option>
            <option value="qris">QRIS</option>
          </Select>
        </div>
      </div>

      {/* Summary */}
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <SummaryStat label={`Omzet · ${periodeLabel(periode)}`} value={formatRupiah(summary.omzet)} />
        <SummaryStat label="Jumlah Transaksi" value={formatNumber(summary.jumlah)} />
        <SummaryStat label="Item Terjual" value={formatNumber(summary.qty)} />
        <SummaryStat label="Rata-rata / Transaksi" value={formatRupiah(summary.avg)} />
      </div>

      {shown.length === 0 ? (
        <EmptyState
          icon={Receipt}
          title="Belum ada transaksi"
          description={`Tidak ada penjualan untuk ${periodeLabel(periode).toLowerCase()}.`}
        />
      ) : (
        <>
          <div className="overflow-x-auto rounded-xl border bg-card">
            <table className="w-full min-w-[720px] text-sm">
              <thead>
                <tr className="border-b text-left text-xs text-muted-foreground">
                  <th className="p-3 font-medium">ID</th>
                  <th className="p-3 font-medium">Waktu</th>
                  <th className="p-3 font-medium">Produk</th>
                  <th className="p-3 text-right font-medium">Qty</th>
                  <th className="p-3 text-right font-medium">Harga</th>
                  <th className="p-3 text-right font-medium">Total</th>
                  <th className="p-3 font-medium">Metode</th>
                  <th className="p-3 font-medium">Kasir</th>
                </tr>
              </thead>
              <tbody>
                {shown.map((t) => (
                  <tr key={t.id} className="border-b transition-colors last:border-0 hover:bg-muted/40">
                    <td className="p-3 font-mono text-xs text-muted-foreground">{t.id}</td>
                    <td className="p-3 whitespace-nowrap text-xs text-muted-foreground">
                      {t.tanggal} · {t.jam?.slice(0, 5)}
                    </td>
                    <td className="p-3 font-medium">{t.nama_produk}</td>
                    <td className="p-3 text-right font-mono tabular-nums">{t.qty}</td>
                    <td className="p-3 text-right font-mono tabular-nums text-muted-foreground">
                      {formatRupiah(t.harga_satuan)}
                    </td>
                    <td className="p-3 text-right font-mono tabular-nums font-medium">
                      {formatRupiah(t.total)}
                    </td>
                    <td className="p-3">
                      <Badge variant={METODE_VARIANT[t.metode_bayar] ?? "secondary"} className="capitalize">
                        {t.metode_bayar || "tunai"}
                      </Badge>
                    </td>
                    <td className="p-3 text-xs text-muted-foreground">{t.kasir || "-"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {filtered.length > MAX_ROWS && (
            <p className="text-center text-xs text-muted-foreground">
              Menampilkan {MAX_ROWS} transaksi terbaru dari {formatNumber(filtered.length)}.
            </p>
          )}
        </>
      )}
    </div>
  );
}

function SummaryStat({ label, value }: { label: string; value: string }) {
  return (
    <Card className="p-4">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className={cn("mt-1 truncate text-lg font-semibold tabular-nums")}>{value}</p>
    </Card>
  );
}
