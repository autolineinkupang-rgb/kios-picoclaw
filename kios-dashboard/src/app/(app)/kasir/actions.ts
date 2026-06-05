"use server";

import { revalidatePath } from "next/cache";
import { getSession } from "@/lib/auth";
import { recordSale, type SaleLine } from "@/lib/sales";

export interface CheckoutInput {
  items: { produkId: string; qty: number }[];
  metode: string; // tunai | transfer | qris | bon
  bayar?: number;
  pelangganPhone?: string;
}

export type CheckoutResult =
  | { ok: true; total: number; kembalian: number | null; lines: SaleLine[]; piutang_id?: string; piutang_warning?: string }
  | { ok: false; error: string };

export async function checkoutAction(input: CheckoutInput): Promise<CheckoutResult> {
  const session = await getSession();
  if (!session) return { ok: false, error: "Sesi berakhir. Silakan masuk lagi." };

  const sale = await recordSale(
    input.items,
    input.metode,
    session.nama,
    "via dashboard",
    input.pelangganPhone,
  );
  if (!sale.ok) return sale;

  revalidatePath("/kasir");
  revalidatePath("/dashboard");
  revalidatePath("/penjualan");
  revalidatePath("/produk");
  revalidatePath("/laporan");

  const bayar =
    typeof input.bayar === "number" && input.bayar > 0 ? Math.trunc(input.bayar) : null;
  const kembalian = bayar !== null && bayar >= sale.total ? bayar - sale.total : null;

  return {
    ok: true,
    total: sale.total,
    kembalian,
    lines: sale.lines,
    ...(sale.piutang_id ? { piutang_id: sale.piutang_id } : {}),
    ...(sale.piutang_warning ? { piutang_warning: sale.piutang_warning } : {}),
  };
}
