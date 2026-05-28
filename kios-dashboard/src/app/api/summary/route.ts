import { NextResponse } from "next/server";
import { getAllProduk, getAllTransaksi, getAllPesanan } from "@/lib/kios";
import { verifyServiceToken } from "@/lib/service-auth";
import { filterPeriode, nowWITA } from "@/lib/format";
import {
  hitungLaba,
  produkTerlaris,
  stokKritis,
  stokMenipis,
} from "@/lib/analytics";
import type { Transaksi } from "@/lib/types";

// Service-only summary the bot pulls via the authenticated /web command. Shape
// MUST stay in sync with the Go struct DashboardSummary (dashboard_summary.go).
export const runtime = "nodejs";
export const dynamic = "force-dynamic";

// "YYYY-MM-DD HH:mm" in WITA, matching the format the Go test parses.
function waktuLabel(): string {
  const d = nowWITA();
  const p = (n: number) => (n < 10 ? `0${n}` : String(n));
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
}

// Busiest hours across all recorded transactions: count per "HH" bucket,
// sorted by transaction count, top 5.
function jamRamai(txs: Transaksi[]): { jam: string; transaksi: number }[] {
  const byHour = new Map<string, number>();
  for (const tx of txs) {
    const hour = (tx.jam || "").slice(0, 2);
    if (!/^\d\d$/.test(hour)) continue;
    byHour.set(hour, (byHour.get(hour) ?? 0) + 1);
  }
  return [...byHour.entries()]
    .map(([jam, transaksi]) => ({ jam, transaksi }))
    .sort((a, b) => b.transaksi - a.transaksi)
    .slice(0, 5);
}

export async function GET(request: Request) {
  if (!verifyServiceToken(request.headers.get("x-kios-service"))) {
    return NextResponse.json({ ok: false, error: "unauthorized" }, { status: 401 });
  }

  let redis = false;
  let produk = [] as Awaited<ReturnType<typeof getAllProduk>>;
  let txs = [] as Transaksi[];
  let pesananPending = 0;
  try {
    [produk, txs] = await Promise.all([getAllProduk(), getAllTransaksi()]);
    const orders = await getAllPesanan();
    pesananPending = orders.filter((o) => o.status === "pending").length;
    redis = true;
  } catch {
    redis = false;
  }

  const today = filterPeriode(txs, "hari_ini");
  const laba = hitungLaba(today, produk);
  const top = produkTerlaris(today, 5);
  const kritis = stokKritis(produk);
  const habis = produk.filter((p) => p.stok <= 0).length;

  return NextResponse.json({
    ok: true,
    waktu: waktuLabel(),
    penjualan_hari_ini: {
      omzet: laba.omzet,
      laba: laba.laba,
      transaksi: laba.jumlahTransaksi,
    },
    terlaris: top.map((t) => ({ nama: t.nama, qty: t.qty, omzet: t.omzet })),
    jam_ramai: jamRamai(txs),
    pesanan_pending: pesananPending,
    stok: {
      menipis: stokMenipis(produk).length,
      kritis: kritis.length,
      habis,
      daftar_kritis: kritis.slice(0, 10).map((p) => p.nama),
    },
    sistem: { redis },
  });
}
