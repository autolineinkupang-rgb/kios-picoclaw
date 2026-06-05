import { filterPeriode, nowWITA, formatDateISO } from "./format.ts";
import type { Periode, Produk, Transaksi } from "./types.ts";

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
