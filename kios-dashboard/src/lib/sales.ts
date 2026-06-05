import { getAllProduk, nextTrxId, pushTransaksi, setProduk } from "./kios";
import { timeWITA, todayWITA, formatRupiah } from "./format";
import type { Transaksi } from "./types";

export interface SaleItemInput {
  produkId: string;
  qty: number;
}

export interface SaleLine {
  id: string;
  nama: string;
  qty: number;
  harga: number;
  subtotal: number;
  sisa: number;
  jenis?: string;
  catatan_sampingan?: string;
}

export type SaleResult =
  | { ok: true; total: number; lines: SaleLine[] }
  | { ok: false; error: string };

const METODE = new Set(["tunai", "transfer", "qris"]);

/**
 * Records a multi-item sale: validates all stock first, then commits one
 * Transaksi per product line (matching the bot's per-item model) and decrements
 * stock. Prices are always taken from the server-side product record.
 * Shared by the cashier cart checkout and buyer-order processing.
 */
export async function recordSale(
  rawItems: SaleItemInput[],
  metode: string,
  kasirNama: string,
  catatan = "via dashboard",
): Promise<SaleResult> {
  const wanted = new Map<string, number>();
  for (const it of rawItems) {
    const q = Math.trunc(it.qty);
    if (!it.produkId || !Number.isFinite(q) || q <= 0) continue;
    wanted.set(it.produkId, (wanted.get(it.produkId) ?? 0) + q);
  }
  if (wanted.size === 0) return { ok: false, error: "Keranjang kosong." };

  const all = await getAllProduk();
  const byId = new Map(all.map((p) => [p.id, p]));

  const insufficient: string[] = [];
  for (const [id, q] of wanted) {
    const p = byId.get(id);
    if (!p) return { ok: false, error: `Produk ${id} tidak ditemukan.` };
    if (p.stok < q) insufficient.push(`${p.nama} (minta ${q}, sisa ${p.stok})`);
  }
  if (insufficient.length) {
    return { ok: false, error: `Stok tidak cukup: ${insufficient.join("; ")}.` };
  }

  const m = METODE.has(metode) ? metode : "tunai";
  const today = todayWITA();
  const jam = timeWITA();
  const lines: SaleLine[] = [];
  let total = 0;

  for (const [id, q] of wanted) {
    const p = byId.get(id)!;
    const sub = q * p.harga_jual;
    const modal = p.harga_beli * q;
    const jenis = p.jenis && p.jenis !== "biasa" ? p.jenis : undefined;
    const catatanTx = jenis ? `${catatan} [${jenis}]` : catatan;

    const txId = await nextTrxId();
    const tx: Transaksi = {
      id: txId,
      tanggal: today,
      jam,
      produk_id: p.id,
      nama_produk: p.nama,
      kategori: p.kategori,
      qty: q,
      harga_satuan: p.harga_jual,
      total: sub,
      metode_bayar: m,
      kasir: kasirNama,
      catatan: catatanTx,
      session_id: "",
      modal,
      ...(jenis === "bensin" ? { liter: q } : {}),
    };
    await pushTransaksi(tx);

    p.stok -= q;
    p.last_update = today;

    let catatan_sampingan: string | undefined;
    if (jenis === "pulsa") {
      const debit = p.harga_beli * q;
      p.saldo_modal = Math.max(0, (p.saldo_modal ?? 0) - debit);
      catatan_sampingan = `saldo modal -${formatRupiah(debit)}`;
    } else if (jenis === "bensin") {
      p.stok_ml = Math.max(0, (p.stok_ml ?? 0) - q * 1000);
    }

    await setProduk(p);
    total += sub;
    lines.push({
      id: txId,
      nama: p.nama,
      qty: q,
      harga: p.harga_jual,
      subtotal: sub,
      sisa: p.stok,
      ...(jenis ? { jenis } : {}),
      ...(catatan_sampingan ? { catatan_sampingan } : {}),
    });
  }

  return { ok: true, total, lines };
}
