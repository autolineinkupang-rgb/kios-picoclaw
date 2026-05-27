"use server";

import { revalidatePath } from "next/cache";
import { getSession } from "@/lib/auth";
import { getProduk, nextTrxId, pushTransaksi, setProduk } from "@/lib/kios";
import { formatRupiah, timeWITA, todayWITA } from "@/lib/format";
import type { Transaksi } from "@/lib/types";

export interface JualInput {
  produkId: string;
  qty: number;
  metode: string; // tunai | transfer | qris
  bayar?: number;
}

export type JualResult =
  | { ok: true; struk: string; kembalian: number | null; sisa: number }
  | { ok: false; error: string };

const METODE = new Set(["tunai", "transfer", "qris"]);

export async function jualAction(input: JualInput): Promise<JualResult> {
  const session = await getSession();
  if (!session) return { ok: false, error: "Sesi berakhir. Silakan masuk lagi." };

  const qty = Math.trunc(input.qty);
  if (!Number.isFinite(qty) || qty <= 0) return { ok: false, error: "Jumlah harus lebih dari 0." };
  const metode = METODE.has(input.metode) ? input.metode : "tunai";

  const produk = await getProduk(input.produkId);
  if (!produk) return { ok: false, error: "Produk tidak ditemukan." };
  if (produk.stok < qty) {
    return { ok: false, error: `Stok ${produk.nama} tidak cukup (sisa ${produk.stok}).` };
  }

  const total = qty * produk.harga_jual;
  const id = await nextTrxId();
  const tx: Transaksi = {
    id,
    tanggal: todayWITA(),
    jam: timeWITA(),
    produk_id: produk.id,
    nama_produk: produk.nama,
    kategori: produk.kategori,
    qty,
    harga_satuan: produk.harga_jual,
    total,
    metode_bayar: metode,
    kasir: session.nama,
    catatan: "via dashboard",
    session_id: "",
  };

  await pushTransaksi(tx);
  produk.stok -= qty;
  produk.last_update = todayWITA();
  await setProduk(produk);

  revalidatePath("/kasir");
  revalidatePath("/dashboard");
  revalidatePath("/penjualan");
  revalidatePath("/produk");
  revalidatePath("/laporan");

  const bayar = typeof input.bayar === "number" && input.bayar > 0 ? Math.trunc(input.bayar) : null;
  const kembalian = bayar !== null && bayar >= total ? bayar - total : null;

  const lines = [
    `${produk.nama} x${qty}`,
    `Total: ${formatRupiah(total)} (${metode})`,
    bayar !== null ? `Bayar: ${formatRupiah(bayar)}` : "",
    kembalian !== null ? `Kembalian: ${formatRupiah(kembalian)}` : "",
    `Sisa stok: ${produk.stok}`,
    `#${id}`,
  ].filter(Boolean);

  return { ok: true, struk: lines.join("\n"), kembalian, sisa: produk.stok };
}
