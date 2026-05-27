"use server";

import { revalidatePath } from "next/cache";
import { getSession } from "@/lib/auth";
import { delProduk, getProduk, nextProdukId, setProduk } from "@/lib/kios";
import { todayWITA } from "@/lib/format";
import type { Produk } from "@/lib/types";

export interface ProdukInput {
  id?: string;
  nama: string;
  barcode: string;
  kategori: string;
  satuan: string;
  stok: number;
  harga_beli: number;
  harga_jual: number;
  stok_minimum: number;
  stok_kritis: number;
  supplier: string;
  exp_date: string;
}

export type ActionResult = { ok: true; message: string } | { ok: false; error: string };

async function ensureOwner(): Promise<ActionResult | null> {
  const session = await getSession();
  if (!session) return { ok: false, error: "Sesi berakhir. Silakan masuk lagi." };
  if (session.role !== "owner") {
    return { ok: false, error: "Aksi ini khusus pemilik (owner)." };
  }
  return null;
}

function sanitize(input: ProdukInput): ActionResult | null {
  if (!input.nama?.trim()) return { ok: false, error: "Nama produk wajib diisi." };
  if (input.harga_jual < 0 || input.harga_beli < 0)
    return { ok: false, error: "Harga tidak boleh minus." };
  if (input.stok < 0) return { ok: false, error: "Stok tidak boleh minus." };
  return null;
}

function num(v: number): number {
  return Number.isFinite(v) ? Math.trunc(v) : 0;
}

export async function createProdukAction(input: ProdukInput): Promise<ActionResult> {
  const denied = await ensureOwner();
  if (denied) return denied;
  const invalid = sanitize(input);
  if (invalid) return invalid;

  const id = await nextProdukId();
  const exp = input.exp_date?.trim() ?? "";
  const p: Produk = {
    id,
    barcode: input.barcode?.trim() ?? "",
    nama: input.nama.trim(),
    kategori: input.kategori?.trim() || "umum",
    satuan: input.satuan?.trim() || "pcs",
    stok: num(input.stok),
    harga_beli: num(input.harga_beli),
    harga_jual: num(input.harga_jual),
    stok_minimum: num(input.stok_minimum) || 5,
    stok_kritis: num(input.stok_kritis) || 2,
    supplier: input.supplier?.trim() ?? "",
    last_update: todayWITA(),
    has_exp: exp !== "",
    exp_date: exp,
  };
  await setProduk(p);
  revalidatePath("/produk");
  revalidatePath("/dashboard");
  return { ok: true, message: `Produk "${p.nama}" ditambahkan (${p.id}).` };
}

export async function updateProdukAction(input: ProdukInput): Promise<ActionResult> {
  const denied = await ensureOwner();
  if (denied) return denied;
  if (!input.id) return { ok: false, error: "ID produk tidak valid." };
  const invalid = sanitize(input);
  if (invalid) return invalid;

  const existing = await getProduk(input.id);
  if (!existing) return { ok: false, error: "Produk tidak ditemukan." };

  const exp = input.exp_date?.trim() ?? "";
  const p: Produk = {
    ...existing,
    barcode: input.barcode?.trim() ?? "",
    nama: input.nama.trim(),
    kategori: input.kategori?.trim() || "umum",
    satuan: input.satuan?.trim() || "pcs",
    stok: num(input.stok),
    harga_beli: num(input.harga_beli),
    harga_jual: num(input.harga_jual),
    stok_minimum: num(input.stok_minimum),
    stok_kritis: num(input.stok_kritis),
    supplier: input.supplier?.trim() ?? "",
    last_update: todayWITA(),
    has_exp: exp !== "",
    exp_date: exp,
  };
  await setProduk(p);
  revalidatePath("/produk");
  revalidatePath("/dashboard");
  return { ok: true, message: `Produk "${p.nama}" diperbarui.` };
}

export async function deleteProdukAction(id: string): Promise<ActionResult> {
  const denied = await ensureOwner();
  if (denied) return denied;
  const existing = await getProduk(id);
  if (!existing) return { ok: false, error: "Produk tidak ditemukan." };
  await delProduk(id);
  revalidatePath("/produk");
  revalidatePath("/dashboard");
  return { ok: true, message: `Produk "${existing.nama}" dihapus.` };
}
