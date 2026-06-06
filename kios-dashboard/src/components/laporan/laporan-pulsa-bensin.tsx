"use client";

import { useMemo } from "react";
import { Bar, BarChart, Cell, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { laporanBensin, laporanPulsa } from "@/lib/analytics";
import { formatNumber, formatRupiah, formatRupiahCompact } from "@/lib/format";
import type { ModuleLaporan } from "@/lib/analytics";
import type { Produk, SampinganSaldo, Transaksi } from "@/lib/types";

function TooltipBox({ children }: { children: React.ReactNode }) {
  return (
    <div className="rounded-lg border bg-popover px-3 py-2 text-xs text-popover-foreground shadow-md">
      {children}
    </div>
  );
}

function ModuleBarChart({
  data,
  unitLabel,
  isRevenue,
}: {
  data: ModuleLaporan["daily_chart"];
  unitLabel: string;
  isRevenue: boolean;
}) {
  const todayLabel = data[data.length - 1]?.label;
  return (
    <ResponsiveContainer width="100%" height={160}>
      <BarChart data={data} margin={{ top: 4, right: 4, left: 4, bottom: 0 }}>
        <XAxis
          dataKey="label"
          tickLine={false}
          axisLine={false}
          tick={{ fontSize: 10, fill: "var(--muted-foreground)" }}
        />
        <YAxis
          tickLine={false}
          axisLine={false}
          width={48}
          tick={{ fontSize: 10, fill: "var(--muted-foreground)" }}
          tickFormatter={(v) =>
            isRevenue ? formatRupiahCompact(Number(v)) : `${v}L`
          }
        />
        <Tooltip
          content={({ active, payload, label }) => {
            if (!active || !payload?.length) return null;
            const v = payload[0].value as number;
            return (
              <TooltipBox>
                <p className="font-medium">{label}</p>
                <p>{isRevenue ? formatRupiah(v) : `${formatNumber(v)} ${unitLabel}`}</p>
              </TooltipBox>
            );
          }}
        />
        <Bar dataKey={isRevenue ? "revenue" : "volume"} radius={[3, 3, 0, 0]}>
          {data.map((d) => (
            <Cell
              key={d.label}
              fill={d.label === todayLabel ? "var(--chart-1)" : "var(--chart-1)"}
              opacity={d.label === todayLabel ? 1 : 0.5}
            />
          ))}
        </Bar>
      </BarChart>
    </ResponsiveContainer>
  );
}

function StockProgress({ used, total }: { used: number; total: number }) {
  const pct = total > 0 ? Math.min(100, (used / total) * 100) : 0;
  const color = pct >= 90 ? "bg-destructive" : pct >= 70 ? "bg-amber-500" : "bg-emerald-500";
  return (
    <div className="space-y-1">
      <div className="flex justify-between text-xs text-muted-foreground">
        <span>Terpakai {Math.round(pct)}%</span>
        <span>Sisa {Math.round(100 - pct)}%</span>
      </div>
      <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
        <div className={`h-full rounded-full transition-all ${color}`} style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}

function ModuleCard({
  title,
  icon,
  data,
  unitLabel,
  isRevenue,
}: {
  title: string;
  icon: string;
  data: ModuleLaporan;
  unitLabel: string;
  isRevenue: boolean;
}) {
  const stockConsistencyOk =
    Math.abs(data.current_stock + data.stock_used - data.total_stock_in) < 1;

  const formatStock = (v: number) =>
    isRevenue ? formatRupiah(v) : `${formatNumber(v)} L`;

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="text-xl">{icon}</span>
            <CardTitle className="text-base">{title}</CardTitle>
          </div>
          {!stockConsistencyOk && (
            <span className="rounded-full bg-destructive/15 px-2 py-0.5 text-xs font-medium text-destructive">
              Data tidak konsisten
            </span>
          )}
        </div>
      </CardHeader>
      <CardContent className="space-y-5">
        {/* Stok/Saldo cards */}
        <div className="grid grid-cols-3 gap-3">
          <div className="rounded-lg bg-emerald-500/10 p-3 text-center">
            <p className="text-xs text-muted-foreground">Sisa</p>
            <p className="mt-0.5 text-sm font-semibold tabular-nums">
              {formatStock(data.current_stock)}
            </p>
          </div>
          <div className="rounded-lg bg-blue-500/10 p-3 text-center">
            <p className="text-xs text-muted-foreground">Total Masuk</p>
            <p className="mt-0.5 text-sm font-semibold tabular-nums">
              {formatStock(data.total_stock_in)}
            </p>
          </div>
          <div className="rounded-lg bg-amber-500/10 p-3 text-center">
            <p className="text-xs text-muted-foreground">Terpakai</p>
            <p className="mt-0.5 text-sm font-semibold tabular-nums">
              {formatStock(data.stock_used)}
            </p>
          </div>
        </div>

        <StockProgress used={data.stock_used} total={data.total_stock_in} />

        {/* Revenue cards */}
        <div className="grid grid-cols-3 gap-3">
          <div className="rounded-lg border p-3">
            <p className="text-xs text-muted-foreground">Hari Ini</p>
            <p className="mt-0.5 text-sm font-semibold tabular-nums">
              {formatRupiah(data.revenue_today)}
            </p>
            <p className="text-xs text-muted-foreground tabular-nums">
              {isRevenue
                ? `${data.volume_today} trx`
                : `${formatNumber(data.volume_today)} L`}
            </p>
          </div>
          <div className="rounded-lg border p-3">
            <p className="text-xs text-muted-foreground">7 Hari</p>
            <p className="mt-0.5 text-sm font-semibold tabular-nums">
              {formatRupiah(data.revenue_7d)}
            </p>
            <p className="text-xs text-muted-foreground tabular-nums">
              avg {formatRupiahCompact(data.avg_daily_7d)}/hari
            </p>
          </div>
          <div className="rounded-lg border p-3">
            <p className="text-xs text-muted-foreground">Bulan Ini</p>
            <p className="mt-0.5 text-sm font-semibold tabular-nums">
              {formatRupiah(data.revenue_month)}
            </p>
          </div>
        </div>

        {/* Profit box */}
        {(data.profit_today > 0 || data.profit_7d > 0) && (
          <div className="rounded-lg border border-violet-500/30 bg-violet-500/5 p-3 space-y-2">
            <p className="text-xs font-semibold text-violet-600 dark:text-violet-400">
              Keuntungan Bersih · Margin {Math.round(data.margin_pct * 100)}%
              {data.sell_price && data.cost_price && (
                <span className="ml-1 font-normal text-muted-foreground">
                  ({formatRupiah(data.sell_price)}/L − {formatRupiah(data.cost_price)}/L)
                </span>
              )}
            </p>
            <div className="grid grid-cols-3 gap-2 text-sm">
              <div>
                <p className="text-xs text-muted-foreground">Hari Ini</p>
                <p className="font-semibold tabular-nums">{formatRupiah(data.profit_today)}</p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground">7 Hari</p>
                <p className="font-semibold tabular-nums">{formatRupiah(data.profit_7d)}</p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground">Bulan Ini</p>
                <p className="font-semibold tabular-nums">{formatRupiah(data.profit_month)}</p>
              </div>
            </div>
          </div>
        )}

        {/* 7-day bar chart */}
        <div>
          <p className="mb-2 text-xs text-muted-foreground">
            {isRevenue ? "Omzet 7 hari (Rp)" : "Volume 7 hari (Liter)"}
          </p>
          <ModuleBarChart
            data={data.daily_chart}
            unitLabel={unitLabel}
            isRevenue={isRevenue}
          />
        </div>
      </CardContent>
    </Card>
  );
}

export function LaporanPulsaBensin({
  transaksi,
  produk,
  saldoKategori,
}: {
  transaksi: Transaksi[];
  produk: Produk[];
  saldoKategori?: SampinganSaldo;
}) {
  const pulsaSaldo = saldoKategori?.pulsa;
  const bensinSaldo = saldoKategori
    ? (saldoKategori.pertalite ?? 0) + (saldoKategori.pertamax ?? 0) + saldoKategori.solar + saldoKategori.minyak_tanah
    : undefined;
  const pulsa = useMemo(
    () => laporanPulsa(transaksi, produk, pulsaSaldo),
    [transaksi, produk, pulsaSaldo],
  );
  const bensin = useMemo(
    () => laporanBensin(transaksi, produk, bensinSaldo),
    [transaksi, produk, bensinSaldo],
  );

  const hasPulsa = pulsa.total_stock_in > 0 || pulsa.revenue_month > 0;
  const hasBensin = bensin.total_stock_in > 0 || bensin.revenue_month > 0;

  if (!hasPulsa && !hasBensin) return null;

  return (
    <section className="space-y-4">
      <div>
        <h3 className="text-base font-semibold">Laporan Pulsa &amp; Bensin</h3>
        <p className="text-sm text-muted-foreground">Saldo, omzet, dan keuntungan per modul.</p>
      </div>
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        {hasPulsa && (
          <ModuleCard
            title="Pulsa"
            icon="📶"
            data={pulsa}
            unitLabel="Trx"
            isRevenue={true}
          />
        )}
        {hasBensin && (
          <ModuleCard
            title="Bensin / BBM"
            icon="⛽"
            data={bensin}
            unitLabel="Liter"
            isRevenue={false}
          />
        )}
      </div>
    </section>
  );
}
