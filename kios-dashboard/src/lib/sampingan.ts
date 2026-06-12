import { jenisOfTx } from "./analytics";
import type { Penarikan, Produk, SampinganSaldo, Transaksi } from "./types";

export type KategoriBBM = "pertalite" | "pertamax" | "solar" | "minyak_tanah";

const BBM_SET = new Set<string>(["pertalite", "pertamax", "solar", "minyak_tanah"]);

/**
 * Maps a product to its BBM saldo category, or null when it is not a
 * liter-based fuel product. Handles both the new model (jenis is the
 * category itself) and legacy products (jenis "bensin" + kategori).
 */
export function kategoriBBM(p: Produk): KategoriBBM | null {
  const j = p.jenis ?? "";
  if (BBM_SET.has(j)) return j as KategoriBBM;
  if (j === "bensin" && BBM_SET.has(p.kategori)) return p.kategori as KategoriBBM;
  return null;
}

/**
 * Effective sellable stock shown to the cashier and used to validate sales.
 * Pulsa: how many units the universal modal pool can cover at this product's
 * cost price. BBM: liters from the per-category saldo. Others: unit stock.
 */
export function efektifStok(p: Produk, saldo?: SampinganSaldo | null): number {
  if (p.jenis === "pulsa") {
    return p.harga_beli > 0 ? Math.floor((saldo?.pulsa ?? 0) / p.harga_beli) : 0;
  }
  const kat = kategoriBBM(p);
  if (kat) return saldo ? (saldo[kat] ?? p.stok) : p.stok;
  return p.stok;
}

/**
 * Total omzet (revenue) of one sampingan category. Legacy "bensin"
 * transactions are attributed to pertalite/pertamax via the product's
 * kategori field.
 */
export function omzetJenis(txs: Transaksi[], produk: Produk[], jenis: string): number {
  const byId = new Map(produk.map((p) => [p.id, p]));
  let omzet = 0;
  for (const tx of txs) {
    const txJenis = jenisOfTx(tx, byId);
    const cocok =
      txJenis === jenis ||
      ((jenis === "pertalite" || jenis === "pertamax") &&
        txJenis === "bensin" &&
        byId.get(tx.produk_id)?.kategori === jenis);
    if (cocok) omzet += tx.total;
  }
  return omzet;
}

/**
 * Total withdrawn from one sampingan category. Legacy withdrawals recorded
 * with produk_id "bensin" (before the pertalite/pertamax split) are allocated
 * to pertalite.
 */
export function tarikJenis(penarikan: Penarikan[], jenis: string): number {
  return penarikan
    .filter((p) => p.produk_id === jenis || (jenis === "pertalite" && p.produk_id === "bensin"))
    .reduce((s, p) => s + p.jumlah, 0);
}

/** Withdrawable income of one category: omzet minus withdrawals, floored at 0. */
export function penghasilanTersediaJenis(
  txs: Transaksi[],
  produk: Produk[],
  penarikan: Penarikan[],
  jenis: string,
): number {
  return Math.max(0, omzetJenis(txs, produk, jenis) - tarikJenis(penarikan, jenis));
}
