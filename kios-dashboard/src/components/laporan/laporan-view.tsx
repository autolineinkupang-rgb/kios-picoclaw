"use client";

import { useMemo, useState } from "react";
import { Coins, Package2, TrendingUp, Wallet } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { KpiCard } from "@/components/kpi-card";
import { EmptyState } from "@/components/ui/empty-state";
import { PeriodeTabs } from "@/components/periode-tabs";
import { MetodeBayarChart, SalesTrendChart } from "@/components/charts";
import { LaporanPulsaBensin } from "@/components/laporan/laporan-pulsa-bensin";
import {
  hitungLaba,
  metodeBayarShare,
  modalPerJenis,
  omzetHarian,
  produkTerlaris,
} from "@/lib/analytics";
import { filterPeriode, formatNumber, formatRupiah, periodeLabel } from "@/lib/format";
import type { Periode, Produk, SampinganSaldo, Transaksi } from "@/lib/types";

export function LaporanView({
  transaksi,
  produk,
  saldoKategori,
}: {
  transaksi: Transaksi[];
  produk: Produk[];
  saldoKategori?: SampinganSaldo;
}) {
  const [periode, setPeriode] = useState<Periode>("minggu");

  const txPeriode = useMemo(() => filterPeriode(transaksi, periode), [transaksi, periode]);
  const laba = useMemo(() => hitungLaba(txPeriode, produk), [txPeriode, produk]);
  const top = useMemo(() => produkTerlaris(txPeriode, 10), [txPeriode]);
  const metode = useMemo(() => metodeBayarShare(txPeriode), [txPeriode]);
  const modalJenis = useMemo(() => modalPerJenis(txPeriode, produk), [txPeriode, produk]);
  const trend = useMemo(
    () => omzetHarian(transaksi, produk, periode === "bulan" ? 30 : periode === "minggu" ? 14 : 7),
    [transaksi, produk, periode],
  );

  const marginPct = laba.omzet > 0 ? Math.round((laba.laba / laba.omzet) * 100) : 0;

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 className="text-lg font-semibold">Laporan {periodeLabel(periode)}</h2>
          <p className="text-sm text-muted-foreground">
            Ringkasan penjualan dan laba kios.
          </p>
        </div>
        <PeriodeTabs value={periode} onChange={setPeriode} />
      </div>

      <section className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <KpiCard title="Omzet" value={formatRupiah(laba.omzet)} icon={Wallet} tone="primary" />
        <KpiCard
          title="Modal (HPP)"
          value={formatRupiah(laba.modal)}
          icon={Package2}
          tone="warning"
        />
        <KpiCard
          title="Laba Kotor"
          value={formatRupiah(laba.laba)}
          hint={`Margin ${marginPct}%`}
          icon={TrendingUp}
          tone="success"
        />
        <KpiCard
          title="Transaksi"
          value={formatNumber(laba.jumlahTransaksi)}
          icon={Coins}
          tone="primary"
        />
      </section>

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
                { label: "Pertalite", value: modalJenis.pertalite },
                { label: "Pertamax", value: modalJenis.pertamax },
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

      <Card>
        <CardHeader className="flex-row items-center justify-between">
          <div>
            <CardTitle>Tren Omzet &amp; Laba</CardTitle>
            <p className="text-xs text-muted-foreground">
              {periode === "bulan" ? "30" : periode === "minggu" ? "14" : "7"} hari terakhir
            </p>
          </div>
          <div className="flex items-center gap-3 text-xs text-muted-foreground">
            <span className="flex items-center gap-1.5">
              <span className="size-2.5 rounded-full bg-chart-1" /> Omzet
            </span>
            <span className="flex items-center gap-1.5">
              <span className="size-2.5 rounded-full bg-chart-3" /> Laba
            </span>
          </div>
        </CardHeader>
        <CardContent>
          {transaksi.length === 0 ? (
            <EmptyState title="Belum ada data penjualan" />
          ) : (
            <SalesTrendChart data={trend} />
          )}
        </CardContent>
      </Card>

      <section className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <CardHeader>
            <CardTitle>Produk Terlaris</CardTitle>
            <p className="text-xs text-muted-foreground">Top 10 · {periodeLabel(periode)}</p>
          </CardHeader>
          <CardContent>
            {top.length === 0 ? (
              <EmptyState title="Belum ada penjualan pada periode ini" />
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b text-left text-xs text-muted-foreground">
                      <th className="py-2 pr-2 font-medium">#</th>
                      <th className="py-2 pr-2 font-medium">Produk</th>
                      <th className="py-2 pr-2 text-right font-medium">Terjual</th>
                      <th className="py-2 text-right font-medium">Omzet</th>
                    </tr>
                  </thead>
                  <tbody>
                    {top.map((t, i) => (
                      <tr key={t.nama} className="border-b last:border-0">
                        <td className="py-2.5 pr-2 font-mono text-muted-foreground tabular-nums">
                          {i + 1}
                        </td>
                        <td className="py-2.5 pr-2 font-medium">{t.nama}</td>
                        <td className="py-2.5 pr-2 text-right font-mono tabular-nums">{t.qty}</td>
                        <td className="py-2.5 text-right font-mono tabular-nums">
                          {formatRupiah(t.omzet)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Metode Pembayaran</CardTitle>
            <p className="text-xs text-muted-foreground">{periodeLabel(periode)}</p>
          </CardHeader>
          <CardContent>
            {metode.length === 0 ? (
              <EmptyState title="Belum ada data" />
            ) : (
              <MetodeBayarChart data={metode} />
            )}
          </CardContent>
        </Card>
      </section>

      <LaporanPulsaBensin transaksi={transaksi} produk={produk} saldoKategori={saldoKategori} />
    </div>
  );
}
