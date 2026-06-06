import { filterPeriode, nowWITA, formatDateISO } from "./format.ts";
import type { Periode, Produk, Transaksi } from "./types.ts";

export interface ModuleDailyPoint {
  label: string;
  revenue: number;
  volume: number;
}

export interface ModuleLaporan {
  current_stock: number;
  total_stock_in: number;
  stock_used: number;
  usage_pct: number;
  revenue_today: number;
  volume_today: number;
  revenue_7d: number;
  avg_daily_7d: number;
  revenue_month: number;
  sell_price: number | null;
  cost_price: number | null;
  margin_pct: number;
  profit_today: number;
  profit_7d: number;
  profit_month: number;
  daily_chart: ModuleDailyPoint[];
}

export function jenisOfTx(tx: Transaksi, byId: Map<string, Produk>): string {
  const fromCatatan = jenisFromCatatan(tx.catatan);
  if (fromCatatan !== "biasa") return fromCatatan;
  return byId.get(tx.produk_id)?.jenis ?? "biasa";
}

export interface RingkasanJenis {
  omzet: number;
  laba: number;
}

export function ringkasanPerJenis(txs: Transaksi[], produk: Produk[]): Record<string, RingkasanJenis> {
  const byId = new Map(produk.map((p) => [p.id, p]));
  const result: Record<string, RingkasanJenis> = {};
  for (const tx of txs) {
    const jenis = jenisOfTx(tx, byId);
    if (!result[jenis]) result[jenis] = { omzet: 0, laba: 0 };
    const modal = tx.modal && tx.modal > 0 ? tx.modal : tx.qty * (byId.get(tx.produk_id)?.harga_beli ?? 0);
    result[jenis].omzet += tx.total;
    result[jenis].laba += tx.total - modal;
  }
  return result;
}

export function laporanPulsa(txs: Transaksi[], produk: Produk[], currentSaldo?: number): ModuleLaporan {
  const byId = new Map(produk.map((p) => [p.id, p]));
  const pulsaTxs = txs.filter((tx) => jenisOfTx(tx, byId) === "pulsa");

  const current_stock = currentSaldo !== undefined
    ? currentSaldo
    : produk.filter((p) => p.jenis === "pulsa").reduce((sum, p) => sum + (p.saldo_modal ?? 0), 0);
  const stock_used = pulsaTxs.reduce((sum, tx) => sum + (tx.modal ?? 0), 0);
  const total_stock_in = current_stock + stock_used;
  const usage_pct = total_stock_in > 0 ? (stock_used / total_stock_in) * 100 : 0;

  const now = nowWITA();
  const today = formatDateISO(now);
  const cutoff7 = new Date(now);
  cutoff7.setDate(cutoff7.getDate() - 6);
  const cut7 = formatDateISO(cutoff7);
  const prefix = today.slice(0, 7);

  const todayTxs = pulsaTxs.filter((tx) => tx.tanggal === today);
  const tx7d = pulsaTxs.filter((tx) => tx.tanggal >= cut7);
  const txMonth = pulsaTxs.filter((tx) => tx.tanggal.startsWith(prefix));

  const revenue_today = todayTxs.reduce((s, t) => s + t.total, 0);
  const volume_today = todayTxs.length;
  const revenue_7d = tx7d.reduce((s, t) => s + t.total, 0);
  const avg_daily_7d = Math.round(revenue_7d / 7);
  const revenue_month = txMonth.reduce((s, t) => s + t.total, 0);

  const modal_today = todayTxs.reduce((s, t) => s + (t.modal ?? 0), 0);
  const modal_7d = tx7d.reduce((s, t) => s + (t.modal ?? 0), 0);
  const modal_month = txMonth.reduce((s, t) => s + (t.modal ?? 0), 0);
  const margin_pct = revenue_7d > 0 ? (revenue_7d - modal_7d) / revenue_7d : 0;

  const daily_chart: ModuleDailyPoint[] = [];
  for (let i = 6; i >= 0; i--) {
    const d = new Date(now);
    d.setDate(d.getDate() - i);
    const iso = formatDateISO(d);
    const dayTxs = pulsaTxs.filter((tx) => tx.tanggal === iso);
    daily_chart.push({
      label: `${iso.slice(8, 10)}/${iso.slice(5, 7)}`,
      revenue: dayTxs.reduce((s, t) => s + t.total, 0),
      volume: dayTxs.length,
    });
  }

  return {
    current_stock, total_stock_in, stock_used, usage_pct,
    revenue_today, volume_today, revenue_7d, avg_daily_7d, revenue_month,
    sell_price: null, cost_price: null, margin_pct,
    profit_today: revenue_today - modal_today,
    profit_7d: revenue_7d - modal_7d,
    profit_month: revenue_month - modal_month,
    daily_chart,
  };
}

const BBM_JENIS = new Set(["bensin", "pertalite", "pertamax", "solar", "minyak_tanah"]);

export function laporanBensin(txs: Transaksi[], produk: Produk[], currentSaldo?: number): ModuleLaporan {
  const byId = new Map(produk.map((p) => [p.id, p]));
  const bensinTxs = txs.filter((tx) => BBM_JENIS.has(jenisOfTx(tx, byId)));

  const current_stock = currentSaldo !== undefined
    ? currentSaldo
    : produk.filter((p) => p.jenis && BBM_JENIS.has(p.jenis)).reduce((sum, p) => sum + p.stok, 0);
  const stock_used = bensinTxs.reduce((sum, tx) => sum + (tx.liter ?? tx.qty), 0);
  const total_stock_in = current_stock + stock_used;
  const usage_pct = total_stock_in > 0 ? (stock_used / total_stock_in) * 100 : 0;

  const now = nowWITA();
  const today = formatDateISO(now);
  const cutoff7 = new Date(now);
  cutoff7.setDate(cutoff7.getDate() - 6);
  const cut7 = formatDateISO(cutoff7);
  const prefix = today.slice(0, 7);

  const todayTxs = bensinTxs.filter((tx) => tx.tanggal === today);
  const tx7d = bensinTxs.filter((tx) => tx.tanggal >= cut7);
  const txMonth = bensinTxs.filter((tx) => tx.tanggal.startsWith(prefix));

  const revenue_today = todayTxs.reduce((s, t) => s + t.total, 0);
  const volume_today = todayTxs.reduce((s, t) => s + (t.liter ?? t.qty), 0);
  const revenue_7d = tx7d.reduce((s, t) => s + t.total, 0);
  const avg_daily_7d = Math.round(revenue_7d / 7);
  const revenue_month = txMonth.reduce((s, t) => s + t.total, 0);

  const bbmProduk = produk.filter((p) => p.jenis && BBM_JENIS.has(p.jenis));
  const sell_price = bbmProduk.length > 0
    ? Math.round(bbmProduk.reduce((s, p) => s + p.harga_jual, 0) / bbmProduk.length)
    : null;
  const cost_price = bbmProduk.length > 0
    ? Math.round(bbmProduk.reduce((s, p) => s + p.harga_beli, 0) / bbmProduk.length)
    : null;
  const margin_per_unit = sell_price !== null && cost_price !== null ? sell_price - cost_price : null;
  const margin_pct = sell_price && margin_per_unit ? margin_per_unit / sell_price : 0;

  const daily_chart: ModuleDailyPoint[] = [];
  for (let i = 6; i >= 0; i--) {
    const d = new Date(now);
    d.setDate(d.getDate() - i);
    const iso = formatDateISO(d);
    const dayTxs = bensinTxs.filter((tx) => tx.tanggal === iso);
    daily_chart.push({
      label: `${iso.slice(8, 10)}/${iso.slice(5, 7)}`,
      revenue: dayTxs.reduce((s, t) => s + t.total, 0),
      volume: dayTxs.reduce((s, t) => s + (t.liter ?? t.qty), 0),
    });
  }

  return {
    current_stock, total_stock_in, stock_used, usage_pct,
    revenue_today, volume_today, revenue_7d, avg_daily_7d, revenue_month,
    sell_price, cost_price, margin_pct,
    profit_today: volume_today * (margin_per_unit ?? 0),
    profit_7d: tx7d.reduce((s, t) => s + (t.liter ?? t.qty) * (margin_per_unit ?? 0), 0),
    profit_month: txMonth.reduce((s, t) => s + (t.liter ?? t.qty) * (margin_per_unit ?? 0), 0),
    daily_chart,
  };
}

export interface LabaResult {
  omzet: number;
  modal: number;
  laba: number;
  jumlahTransaksi: number;
}

/**
 * Profit for a transaction set, mirroring LaporanTool.hitungLaba in Go:
 *   omzet = Σ tx.total
 *   modal = Σ tx.qty * produk.harga_beli
 *   laba  = omzet - modal
 */
export function hitungLaba(txs: Transaksi[], produk: Produk[]): LabaResult {
  const beli = new Map<string, number>();
  for (const p of produk) beli.set(p.id, p.harga_beli);
  let omzet = 0;
  let modal = 0;
  for (const tx of txs) {
    omzet += tx.total;
    modal += tx.qty * (beli.get(tx.produk_id) ?? 0);
  }
  return { omzet, modal, laba: omzet - modal, jumlahTransaksi: txs.length };
}

export function labaPeriode(
  txs: Transaksi[],
  produk: Produk[],
  periode: Periode,
): LabaResult {
  return hitungLaba(filterPeriode(txs, periode), produk);
}

export interface TopProduk {
  nama: string;
  qty: number;
  omzet: number;
}

export function produkTerlaris(txs: Transaksi[], limit = 5): TopProduk[] {
  const agg = new Map<string, TopProduk>();
  for (const tx of txs) {
    const cur = agg.get(tx.nama_produk) ?? {
      nama: tx.nama_produk,
      qty: 0,
      omzet: 0,
    };
    cur.qty += tx.qty;
    cur.omzet += tx.total;
    agg.set(tx.nama_produk, cur);
  }
  return [...agg.values()].sort((a, b) => b.qty - a.qty).slice(0, limit);
}

/** Stock that is at or below its minimum (needs restock). */
export function stokMenipis(produk: Produk[]): Produk[] {
  return produk
    .filter((p) => p.stok <= p.stok_minimum)
    .sort((a, b) => a.stok - b.stok);
}

/** Stock at or below the critical threshold (more urgent than menipis). */
export function stokKritis(produk: Produk[]): Produk[] {
  return produk.filter((p) => p.stok <= p.stok_kritis).sort((a, b) => a.stok - b.stok);
}

export interface DailyPoint {
  tanggal: string; // YYYY-MM-DD
  label: string; // dd/mm
  omzet: number;
  laba: number;
  transaksi: number;
}

/** Daily omzet/laba/count series for the last `days` days (oldest → newest). */
export function omzetHarian(
  txs: Transaksi[],
  produk: Produk[],
  days: number,
): DailyPoint[] {
  const beli = new Map<string, number>();
  for (const p of produk) beli.set(p.id, p.harga_beli);

  const points: DailyPoint[] = [];
  const now = nowWITA();
  for (let i = days - 1; i >= 0; i--) {
    const d = new Date(now);
    d.setDate(d.getDate() - i);
    const iso = formatDateISO(d);
    points.push({
      tanggal: iso,
      label: `${iso.slice(8, 10)}/${iso.slice(5, 7)}`,
      omzet: 0,
      laba: 0,
      transaksi: 0,
    });
  }
  const index = new Map(points.map((p) => [p.tanggal, p]));
  for (const tx of txs) {
    const pt = index.get(tx.tanggal);
    if (!pt) continue;
    pt.omzet += tx.total;
    pt.laba += tx.total - tx.qty * (beli.get(tx.produk_id) ?? 0);
    pt.transaksi += 1;
  }
  return points;
}

export interface MetodeShare {
  metode: string;
  total: number;
  jumlah: number;
}

export function metodeBayarShare(txs: Transaksi[]): MetodeShare[] {
  const agg = new Map<string, MetodeShare>();
  for (const tx of txs) {
    const key = tx.metode_bayar || "tunai";
    const cur = agg.get(key) ?? { metode: key, total: 0, jumlah: 0 };
    cur.total += tx.total;
    cur.jumlah += 1;
    agg.set(key, cur);
  }
  return [...agg.values()].sort((a, b) => b.total - a.total);
}

/** Parses the product kind from the catatan field written by recordSale.
 * Format: "via dashboard [<jenis>]" → jenis; anything else → "biasa". */
export function jenisFromCatatan(catatan: string): string {
  const m = catatan.match(/\[(\w+)\]/);
  return m ? m[1] : "biasa";
}

export interface ModalPerJenis {
  biasa: number;
  pulsa: number;
  bensin: number;
  solar: number;
  minyak_tanah: number;
  total: number;
}

/** Modal breakdown per product kind using tx.modal when set (accurate), falling
 * back to qty * current harga_beli for old bot transactions. */
export function modalPerJenis(txs: Transaksi[], produk: Produk[]): ModalPerJenis {
  const beli = new Map<string, number>();
  for (const p of produk) beli.set(p.id, p.harga_beli);

  const result: ModalPerJenis = { biasa: 0, pulsa: 0, bensin: 0, solar: 0, minyak_tanah: 0, total: 0 };
  for (const tx of txs) {
    const m = tx.modal && tx.modal > 0 ? tx.modal : tx.qty * (beli.get(tx.produk_id) ?? 0);
    const jenis = jenisFromCatatan(tx.catatan);
    if (jenis === "pulsa" || jenis === "bensin" || jenis === "solar" || jenis === "minyak_tanah") {
      result[jenis] += m;
    } else {
      result.biasa += m;
    }
    result.total += m;
  }
  return result;
}
