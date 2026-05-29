"use server";

import { revalidatePath } from "next/cache";
import { getSession } from "@/lib/auth";
import { getAllPromo, setPromo, deletePromo, nextPromoId } from "@/lib/kios";
import type { Promo } from "@/lib/types";

export type PromoResult = { ok: true } | { ok: false; error: string };

async function ensureStaff() {
  const session = await getSession();
  if (!session || (session.role !== "owner" && session.role !== "kasir")) {
    return { ok: false as const, error: "Akses ditolak." };
  }
  return { ok: true as const, session };
}

async function ensureOwner() {
  const session = await getSession();
  if (!session || session.role !== "owner") {
    return { ok: false as const, error: "Akses ditolak. Khusus owner." };
  }
  return { ok: true as const, session };
}

export interface CreatePromoInput {
  produk_id: string;
  produk: string;
  tipe: "persen" | "fixed";
  nilai: number;
  min_qty: number;
  mulai: string;
  selesai: string;
  catatan: string;
}

export async function createPromoAction(input: CreatePromoInput): Promise<PromoResult> {
  const gate = await ensureStaff();
  if (!gate.ok) return gate;

  if (!input.produk_id || !input.produk) return { ok: false, error: "Produk wajib dipilih." };
  if (input.nilai <= 0) return { ok: false, error: "Nilai diskon harus lebih dari 0." };
  if (input.tipe === "persen" && input.nilai > 100)
    return { ok: false, error: "Diskon persen tidak boleh lebih dari 100." };
  if (!input.mulai || !input.selesai) return { ok: false, error: "Tanggal wajib diisi." };
  if (input.mulai > input.selesai) return { ok: false, error: "Tanggal mulai harus sebelum selesai." };

  const id = await nextPromoId();
  const promo: Promo = {
    id,
    produk: input.produk.trim(),
    produk_id: input.produk_id,
    tipe: input.tipe,
    nilai: Math.abs(input.nilai),
    min_qty: Math.max(1, Math.floor(input.min_qty)),
    aktif: gate.session.role === "owner", // owner → langsung aktif; kasir → menunggu
    mulai: input.mulai,
    selesai: input.selesai,
    catatan: (input.catatan ?? "").trim(),
  };
  await setPromo(promo);
  revalidatePath("/promo");
  return { ok: true };
}

export async function togglePromoAction(id: string): Promise<PromoResult> {
  const gate = await ensureOwner();
  if (!gate.ok) return gate;

  const all = await getAllPromo();
  const promo = all.find((p) => p.id === id);
  if (!promo) return { ok: false, error: "Promo tidak ditemukan." };

  await setPromo({ ...promo, aktif: !promo.aktif });
  revalidatePath("/promo");
  return { ok: true };
}

export async function deletePromoAction(id: string): Promise<PromoResult> {
  const gate = await ensureOwner();
  if (!gate.ok) return gate;

  await deletePromo(id);
  revalidatePath("/promo");
  return { ok: true };
}
