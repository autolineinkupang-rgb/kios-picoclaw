"use client";

import { useEffect, useState } from "react";
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  Cell,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { formatRupiah, formatRupiahCompact } from "@/lib/format";
import type { DailyPoint, MetodeShare, TopProduk } from "@/lib/analytics";

function useReducedMotion() {
  const [reduce, setReduce] = useState(false);
  useEffect(() => {
    setReduce(window.matchMedia("(prefers-reduced-motion: reduce)").matches);
  }, []);
  return reduce;
}

const PIE_COLORS = [
  "var(--chart-1)",
  "var(--chart-3)",
  "var(--chart-4)",
  "var(--chart-5)",
  "var(--chart-2)",
];

const METODE_LABEL: Record<string, string> = {
  tunai: "Tunai",
  transfer: "Transfer",
  qris: "QRIS",
};

function TooltipBox({ children }: { children: React.ReactNode }) {
  return (
    <div className="rounded-lg border bg-popover px-3 py-2 text-xs text-popover-foreground shadow-md">
      {children}
    </div>
  );
}

// ── Sales trend (omzet + laba) ──────────────────────────────────────────────

export function SalesTrendChart({ data }: { data: DailyPoint[] }) {
  const reduce = useReducedMotion();
  return (
    <ResponsiveContainer width="100%" height={280}>
      <AreaChart data={data} margin={{ top: 8, right: 8, left: 8, bottom: 0 }}>
        <defs>
          <linearGradient id="gOmzet" x1="0" y1="0" x2="0" y2="1">
            <stop offset="5%" stopColor="var(--chart-1)" stopOpacity={0.35} />
            <stop offset="95%" stopColor="var(--chart-1)" stopOpacity={0} />
          </linearGradient>
          <linearGradient id="gLaba" x1="0" y1="0" x2="0" y2="1">
            <stop offset="5%" stopColor="var(--chart-3)" stopOpacity={0.3} />
            <stop offset="95%" stopColor="var(--chart-3)" stopOpacity={0} />
          </linearGradient>
        </defs>
        <XAxis
          dataKey="label"
          tickLine={false}
          axisLine={false}
          tick={{ fontSize: 11, fill: "var(--muted-foreground)" }}
          minTickGap={16}
        />
        <YAxis
          tickLine={false}
          axisLine={false}
          width={56}
          tick={{ fontSize: 11, fill: "var(--muted-foreground)" }}
          tickFormatter={(v) => formatRupiahCompact(Number(v))}
        />
        <Tooltip
          cursor={{ stroke: "var(--border)" }}
          content={({ active, payload, label }) => {
            if (!active || !payload?.length) return null;
            const p = payload[0]?.payload as DailyPoint;
            return (
              <TooltipBox>
                <p className="mb-1 font-medium text-foreground">{label}</p>
                <p className="text-muted-foreground">
                  Omzet:{" "}
                  <span className="font-medium text-foreground">
                    {formatRupiah(p.omzet)}
                  </span>
                </p>
                <p className="text-muted-foreground">
                  Laba:{" "}
                  <span className="font-medium text-foreground">{formatRupiah(p.laba)}</span>
                </p>
                <p className="text-muted-foreground">
                  Transaksi:{" "}
                  <span className="font-medium text-foreground tabular-nums">
                    {p.transaksi}
                  </span>
                </p>
              </TooltipBox>
            );
          }}
        />
        <Area
          type="monotone"
          dataKey="omzet"
          stroke="var(--chart-1)"
          strokeWidth={2}
          fill="url(#gOmzet)"
          isAnimationActive={!reduce}
        />
        <Area
          type="monotone"
          dataKey="laba"
          stroke="var(--chart-3)"
          strokeWidth={2}
          fill="url(#gLaba)"
          isAnimationActive={!reduce}
        />
      </AreaChart>
    </ResponsiveContainer>
  );
}

// ── Top products (horizontal bars) ──────────────────────────────────────────

export function TopProdukChart({ data }: { data: TopProduk[] }) {
  const reduce = useReducedMotion();
  return (
    <ResponsiveContainer width="100%" height={Math.max(160, data.length * 44)}>
      <BarChart
        data={data}
        layout="vertical"
        margin={{ top: 0, right: 16, left: 8, bottom: 0 }}
      >
        <XAxis type="number" hide />
        <YAxis
          type="category"
          dataKey="nama"
          width={110}
          tickLine={false}
          axisLine={false}
          tick={{ fontSize: 12, fill: "var(--muted-foreground)" }}
        />
        <Tooltip
          cursor={{ fill: "var(--muted)" }}
          content={({ active, payload }) => {
            if (!active || !payload?.length) return null;
            const p = payload[0]?.payload as TopProduk;
            return (
              <TooltipBox>
                <p className="mb-1 font-medium text-foreground">{p.nama}</p>
                <p className="text-muted-foreground tabular-nums">{p.qty} terjual</p>
                <p className="text-muted-foreground">{formatRupiah(p.omzet)}</p>
              </TooltipBox>
            );
          }}
        />
        <Bar
          dataKey="qty"
          fill="var(--chart-1)"
          radius={[0, 6, 6, 0]}
          isAnimationActive={!reduce}
          barSize={20}
        />
      </BarChart>
    </ResponsiveContainer>
  );
}

// ── Payment method share (donut) ────────────────────────────────────────────

export function MetodeBayarChart({ data }: { data: MetodeShare[] }) {
  const reduce = useReducedMotion();
  const total = data.reduce((s, d) => s + d.total, 0);
  return (
    <div className="flex flex-col items-center gap-4 sm:flex-row">
      <ResponsiveContainer width={160} height={160}>
        <PieChart>
          <Pie
            data={data}
            dataKey="total"
            nameKey="metode"
            innerRadius={48}
            outerRadius={72}
            paddingAngle={2}
            isAnimationActive={!reduce}
            stroke="var(--card)"
            strokeWidth={2}
          >
            {data.map((d, i) => (
              <Cell key={d.metode} fill={PIE_COLORS[i % PIE_COLORS.length]} />
            ))}
          </Pie>
          <Tooltip
            content={({ active, payload }) => {
              if (!active || !payload?.length) return null;
              const p = payload[0]?.payload as MetodeShare;
              const pct = total > 0 ? Math.round((p.total / total) * 100) : 0;
              return (
                <TooltipBox>
                  <p className="font-medium text-foreground capitalize">
                    {METODE_LABEL[p.metode] ?? p.metode}
                  </p>
                  <p className="text-muted-foreground">
                    {formatRupiah(p.total)} · {pct}%
                  </p>
                  <p className="text-muted-foreground tabular-nums">{p.jumlah} transaksi</p>
                </TooltipBox>
              );
            }}
          />
        </PieChart>
      </ResponsiveContainer>
      <ul className="flex-1 space-y-2 self-stretch">
        {data.map((d, i) => {
          const pct = total > 0 ? Math.round((d.total / total) * 100) : 0;
          return (
            <li key={d.metode} className="flex items-center gap-2 text-sm">
              <span
                className="size-3 shrink-0 rounded-sm"
                style={{ backgroundColor: PIE_COLORS[i % PIE_COLORS.length] }}
                aria-hidden
              />
              <span className="capitalize">{METODE_LABEL[d.metode] ?? d.metode}</span>
              <span className="ml-auto font-medium tabular-nums">{pct}%</span>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
