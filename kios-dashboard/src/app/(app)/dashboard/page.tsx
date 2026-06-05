import type { Metadata } from "next";
import Link from "next/link";
import {
  AlertTriangle,
  ArrowRight,
  PackageX,
  Receipt,
  TrendingUp,
  Wallet,
} from "lucide-react";
import { getAllProduk, getAllTransaksi } from "@/lib/kios";
import {
  hitungLaba,
  metodeBayarShare,
  modalPerJenis,
  omzetHarian,
  produkTerlaris,
  stokKritis,
  stokMenipis,
} from "@/lib/analytics";
import { filterPeriode, formatRupiah, nowWITA, formatDateISO } from "@/lib/format";
import { KpiCard } from "@/components/kpi-card";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { ConnectionError } from "@/components/connection-error";
import {
  MetodeBayarChart,
  SalesTrendChart,
  TopProdukChart,
} from "@/components/charts";

export const metadata: Metadata = { title: "Dashboard" };
export const dynamic = "force-dynamic";

export default async function DashboardPage() {
  let produk, transaksi;
  try {
    [produk, transaksi] = await Promise.all([getAllProduk(), getAllTransaksi()]);
  } catch (e) {
    return <ConnectionError message={e instanceof Error ? e.message : String(e)} />;
  }

  const txHariIni = filterPeriode(transaksi, "hari_ini");
  const txMinggu = filterPeriode(transaksi, "minggu");
  const labaHariIni = hitungLaba(txHariIni, produk);

  const modalSampinganHariIni = modalPerJenis(txHariIni, produk);

  const LABEL_JENIS: Record<string, string> = {
    pulsa: "Pulsa",
    bensin: "Bensin",
    solar: "Solar",
    minyak_tanah: "Minyak Tanah",
  };
  const modalSampinganRows = (
    ["pulsa", "bensin", "solar", "minyak_tanah"] as const
  ).filter((j) => modalSampinganHariIni[j] > 0);

  // Yesterday's omzet for the trend hint.
  const y = nowWITA();
  y.setDate(y.getDate() - 1);
  const kemarinISO = formatDateISO(y);
  const omzetKemarin = transaksi
    .filter((t) => t.tanggal === kemarinISO)
    .reduce((s, t) => s + t.total, 0);
  const deltaPct =
    omzetKemarin > 0
      ? Math.round(((labaHariIni.omzet - omzetKemarin) / omzetKemarin) * 100)
      : null;

  const trend = omzetHarian(transaksi, produk, 14);
  const top = produkTerlaris(txMinggu, 5);
  const metode = metodeBayarShare(txMinggu);
  const kritis = stokKritis(produk);
  const menipis = stokMenipis(produk);

  const omzetHint =
    deltaPct === null
      ? "Belum ada pembanding kemarin"
      : `${deltaPct >= 0 ? "▲" : "▼"} ${Math.abs(deltaPct)}% vs kemarin`;

  return (
    <div className="space-y-6">
      {/* KPI row */}
      <section className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <KpiCard
          title="Omzet Hari Ini"
          value={formatRupiah(labaHariIni.omzet)}
          hint={omzetHint}
          icon={Wallet}
          tone="primary"
        />
        <KpiCard
          title="Laba Hari Ini"
          value={formatRupiah(labaHariIni.laba)}
          hint={`Modal ${formatRupiah(labaHariIni.modal)}`}
          icon={TrendingUp}
          tone="success"
        />
        <KpiCard
          title="Transaksi Hari Ini"
          value={String(labaHariIni.jumlahTransaksi)}
          hint={`${txMinggu.length} transaksi minggu ini`}
          icon={Receipt}
          tone="primary"
        />
        <KpiCard
          title="Stok Kritis"
          value={String(kritis.length)}
          hint={`${menipis.length} produk perlu restock`}
          icon={AlertTriangle}
          tone={kritis.length > 0 ? "destructive" : "success"}
        />
      </section>

      {/* Modal sampingan — hanya tampil jika ada transaksi hari ini */}
      {modalSampinganRows.length > 0 && (
        <section className="rounded-xl border bg-card p-4">
          <p className="mb-3 text-sm font-semibold">Modal Produk Sampingan Hari Ini</p>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            {modalSampinganRows.map((jenis) => (
              <div key={jenis} className="space-y-0.5">
                <p className="text-xs text-muted-foreground">{LABEL_JENIS[jenis]}</p>
                <p className="font-mono text-sm font-semibold tabular-nums">
                  {formatRupiah(modalSampinganHariIni[jenis])}
                </p>
              </div>
            ))}
          </div>
        </section>
      )}

      {/* Trend + payment method */}
      <section className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <CardHeader className="flex-row items-center justify-between">
            <div>
              <CardTitle>Tren Omzet &amp; Laba</CardTitle>
              <p className="text-xs text-muted-foreground">14 hari terakhir (WITA)</p>
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
              <EmptyState
                icon={Receipt}
                title="Belum ada transaksi"
                description="Penjualan dari bot Telegram akan otomatis tampil di sini."
              />
            ) : (
              <SalesTrendChart data={trend} />
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Metode Pembayaran</CardTitle>
            <p className="text-xs text-muted-foreground">7 hari terakhir</p>
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

      {/* Top products + restock list */}
      <section className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Produk Terlaris</CardTitle>
            <p className="text-xs text-muted-foreground">Berdasarkan kuantitas, 7 hari terakhir</p>
          </CardHeader>
          <CardContent>
            {top.length === 0 ? (
              <EmptyState title="Belum ada penjualan minggu ini" />
            ) : (
              <TopProdukChart data={top} />
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex-row items-center justify-between">
            <CardTitle>Perlu Restock</CardTitle>
            <Link
              href="/produk"
              className="flex items-center gap-1 text-xs font-medium text-accent hover:underline"
            >
              Lihat semua <ArrowRight className="size-3.5" />
            </Link>
          </CardHeader>
          <CardContent>
            {menipis.length === 0 ? (
              <EmptyState
                icon={PackageX}
                title="Semua stok aman"
                description="Tidak ada produk yang berada di bawah stok minimum."
              />
            ) : (
              <ul className="divide-y">
                {menipis.slice(0, 6).map((p) => (
                  <li key={p.id} className="flex items-center justify-between gap-3 py-2.5">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">{p.nama}</p>
                      <p className="text-xs text-muted-foreground">
                        Min {p.stok_minimum} {p.satuan} · Kritis {p.stok_kritis}
                      </p>
                    </div>
                    <Badge variant={p.stok <= p.stok_kritis ? "destructive" : "warning"}>
                      Sisa {p.stok} {p.satuan}
                    </Badge>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
