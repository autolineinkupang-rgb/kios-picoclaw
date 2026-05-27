import type { Produk } from "./types";

export type StokStatus = "habis" | "kritis" | "menipis" | "aman";

export function stokStatus(p: Produk): StokStatus {
  if (p.stok <= 0) return "habis";
  if (p.stok <= p.stok_kritis) return "kritis";
  if (p.stok <= p.stok_minimum) return "menipis";
  return "aman";
}

type BadgeVariant = "success" | "warning" | "destructive" | "secondary";

export const STATUS_META: Record<
  StokStatus,
  { label: string; variant: BadgeVariant }
> = {
  habis: { label: "Habis", variant: "destructive" },
  kritis: { label: "Kritis", variant: "destructive" },
  menipis: { label: "Menipis", variant: "warning" },
  aman: { label: "Aman", variant: "success" },
};

/** Profit margin percent of selling price; null when not computable. */
export function marginPct(p: Produk): number | null {
  if (p.harga_jual <= 0 || p.harga_beli <= 0) return null;
  return Math.round(((p.harga_jual - p.harga_beli) / p.harga_jual) * 100);
}
