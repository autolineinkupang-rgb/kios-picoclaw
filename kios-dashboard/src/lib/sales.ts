import {
  getAllProduk,
  nextTrxId,
  pushTransaksi,
  setProduk,
  nextPiutangId,
  setPiutang,
  getPelanggan,
  setPelanggan,
  upsertPelanggan,
} from "./kios";
import { timeWITA, todayWITA, formatRupiah } from "./format";
import type { Transaksi, Piutang } from "./types";

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
  | { ok: true; total: number; lines: SaleLine[]; piutang_id?: string }
  | { ok: false; error: string };

const METODE = new Set(["tunai", "transfer", "qris", "bon"]);

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
  pelangganPhone?: string,
): Promise<SaleResult> {
  const wanted = new Map<string, number>();
  for (const it of rawItems) {
    const q = Math.trunc(it.qty);
    if (!it.produkId || !Number.isFinite(q) || q <= 0) continue;
    wanted.set(it.produkId, (wanted.get(it.produkId) ?? 0) + q);
  }
  if (wanted.size === 0) return { ok: false, error: "Keranjang kosong." };

  if (metode === "bon") {
    if (!pelangganPhone?.trim()) {
      return { ok: false, error: "Nomor HP pelanggan wajib diisi untuk metode bon." };
    }
    const digits = pelangganPhone.replace(/\D/g, "");
    if (digits.length < 10 || digits.length > 15) {
      return { ok: false, error: "Nomor HP tidak valid (contoh: 08123456789)." };
    }
  }

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

  let piutang_id: string | undefined;
  if (m === "bon" && pelangganPhone) {
    try {
      const pelanggan = await upsertPelanggan(pelangganPhone, pelangganPhone);
      pelanggan.total_utang = (pelanggan.total_utang ?? 0) + total;
      pelanggan.total_belanja = (pelanggan.total_belanja ?? 0) + total;
      await setPelanggan(pelanggan);

      piutang_id = await nextPiutangId();
      const piu: Piutang = {
        id: piutang_id,
        pelanggan_id: pelanggan.id,
        phone: pelanggan.phone,
        pokok: total,
        dibayar: 0,
        sisa: total,
        status: "terbuka",
        tanggal: todayWITA(),
        jam: timeWITA(),
        kasir: kasirNama,
        catatan: "bon via dashboard",
      };
      await setPiutang(piu);
    } catch (e) {
      return {
        ok: false,
        error: e instanceof Error ? e.message : "Gagal catat piutang.",
      };
    }
  }

  return { ok: true, total, lines, ...(piutang_id ? { piutang_id } : {}) };
}
