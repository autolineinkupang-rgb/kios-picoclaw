import type { Produk, SampinganSaldo } from "./types";

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
