"use server";

import { revalidatePath } from "next/cache";
import { getSession } from "@/lib/auth";
import { recordSale, type SaleLine } from "@/lib/sales";
import {
  getShift,
  setShift,
  clearShift,
  pushShiftHistory,
} from "@/lib/kios";
import type { Shift } from "@/lib/types";

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

export type ShiftResult = { ok: true } | { ok: false; error: string };

export async function bukaShiftAction(
  kasir: string,
  saldoAwal: number,
): Promise<ShiftResult> {
  const session = await getSession();
  if (!session) return { ok: false, error: "Sesi berakhir. Silakan masuk lagi." };

  const existing = await getShift();
  if (existing?.status === "buka") {
    return { ok: false, error: "Ada shift yang sedang buka. Tutup dulu sebelum buka shift baru." };
  }

  const now = new Date().toLocaleString("id-ID", { timeZone: "Asia/Makassar" });
  const shift: Shift = {
    kasir: kasir.trim() || session.nama,
    saldo_awal: Math.floor(saldoAwal),
    saldo_akhir: 0,
    waktu_buka: now,
    waktu_tutup: "",
    status: "buka",
  };
  await setShift(shift);
  revalidatePath("/kasir");
  return { ok: true };
}

export async function tutupShiftAction(saldoAkhir: number): Promise<ShiftResult> {
  const session = await getSession();
  if (!session) return { ok: false, error: "Sesi berakhir. Silakan masuk lagi." };

  const shift = await getShift();
  if (!shift || shift.status !== "buka") {
    return { ok: false, error: "Tidak ada shift yang sedang buka." };
  }

  const now = new Date().toLocaleString("id-ID", { timeZone: "Asia/Makassar" });
  const closed: Shift = {
    ...shift,
    saldo_akhir: Math.floor(saldoAkhir),
    waktu_tutup: now,
    status: "tutup",
  };
  await pushShiftHistory(closed);
  await clearShift();
  revalidatePath("/kasir");
  return { ok: true };
}
