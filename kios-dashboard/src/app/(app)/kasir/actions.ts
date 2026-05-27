"use server";

import { revalidatePath } from "next/cache";
import { getSession } from "@/lib/auth";
import { recordSale, type SaleLine } from "@/lib/sales";

export interface CheckoutInput {
  items: { produkId: string; qty: number }[];
  metode: string; // tunai | transfer | qris
  bayar?: number;
}

export type CheckoutResult =
  | { ok: true; total: number; kembalian: number | null; lines: SaleLine[] }
  | { ok: false; error: string };

export async function checkoutAction(input: CheckoutInput): Promise<CheckoutResult> {
  const session = await getSession();
  if (!session) return { ok: false, error: "Sesi berakhir. Silakan masuk lagi." };

  const sale = await recordSale(input.items, input.metode, session.nama);
  if (!sale.ok) return sale;

  revalidatePath("/kasir");
  revalidatePath("/dashboard");
  revalidatePath("/penjualan");
  revalidatePath("/produk");
  revalidatePath("/laporan");

  const bayar =
    typeof input.bayar === "number" && input.bayar > 0 ? Math.trunc(input.bayar) : null;
  const kembalian = bayar !== null && bayar >= sale.total ? bayar - sale.total : null;

  return { ok: true, total: sale.total, kembalian, lines: sale.lines };
}
