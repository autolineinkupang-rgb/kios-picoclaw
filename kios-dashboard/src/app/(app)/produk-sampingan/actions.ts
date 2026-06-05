"use server";

import { revalidatePath } from "next/cache";
import { getSession } from "@/lib/auth";
import { delProduk, getProduk, nextProdukId, setProduk } from "@/lib/kios";
import { todayWITA } from "@/lib/format";
import type { Produk } from "@/lib/types";

export type JenisSampingan = "pulsa" | "bensin" | "solar" | "minyak_tanah";

export interface SampinganInput {
  id?: string;
  jenis: JenisSampingan;
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
  image_url: string;
  pack_defs?: Array<{ nama: string; isi: number }>;
  // pulsa only
  saldo_modal?: number;
  // bensin only
  stok_ml?: number;
  stok_kritis_ml?: number;
}

export type ActionResult = { ok: true; message: string } | { ok: false; error: string };

const VALID_JENIS: JenisSampingan[] = ["pulsa", "bensin", "solar", "minyak_tanah"];

async function ensureOwner(): Promise<ActionResult | null> {
  const session = await getSession();
  if (!session) return { ok: false, error: "Sesi berakhir. Silakan masuk lagi." };
  if (session.role !== "owner") return { ok: false, error: "Aksi ini khusus pemilik (owner)." };
  return null;
}

function num(v: number): number {
  return Number.isFinite(v) ? Math.trunc(v) : 0;
}

function sanitize(input: SampinganInput): ActionResult | null {
  if (!input.nama?.trim()) return { ok: false, error: "Nama produk wajib diisi." };
  if (!VALID_JENIS.includes(input.jenis)) return { ok: false, error: "Jenis produk tidak valid." };
  if (input.harga_jual < 0 || input.harga_beli < 0) return { ok: false, error: "Harga tidak boleh minus." };
  if (input.stok < 0) return { ok: false, error: "Stok tidak boleh minus." };
  const img = input.image_url?.trim() ?? "";
  if (img && !/^(https?:\/\/|data:image\/)/i.test(img)) {
    return { ok: false, error: "Gambar harus berupa URL (http/https) atau hasil unggahan." };
  }
  if (img.length > 600_000) return { ok: false, error: "Gambar terlalu besar." };
  return null;
}

function revalidate() {
  revalidatePath("/produk-sampingan");
  revalidatePath("/dashboard");
  revalidatePath("/kasir");
}

export async function createSampinganAction(input: SampinganInput): Promise<ActionResult> {
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
    kategori: input.kategori?.trim() || input.jenis,
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
    image_url: input.image_url?.trim() ?? "",
    jenis: input.jenis,
    ...(input.jenis === "pulsa" ? { saldo_modal: num(input.saldo_modal ?? 0) } : {}),
    ...(input.jenis === "bensin"
      ? { stok_ml: num(input.stok_ml ?? 0), stok_kritis_ml: num(input.stok_kritis_ml ?? 40000) }
      : {}),
    ...(input.pack_defs?.length ? { pack_defs: input.pack_defs } : {}),
  };
  await setProduk(p);
  revalidate();
  return { ok: true, message: `Produk sampingan "${p.nama}" ditambahkan (${p.id}).` };
}

export async function updateSampinganAction(input: SampinganInput): Promise<ActionResult> {
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
    kategori: input.kategori?.trim() || input.jenis,
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
    image_url: input.image_url?.trim() ?? "",
    jenis: input.jenis,
    ...(input.jenis === "pulsa" ? { saldo_modal: num(input.saldo_modal ?? existing.saldo_modal ?? 0) } : {}),
    ...(input.jenis === "bensin"
      ? {
          stok_ml: num(input.stok_ml ?? existing.stok_ml ?? 0),
          stok_kritis_ml: num(input.stok_kritis_ml ?? existing.stok_kritis_ml ?? 40000),
        }
      : {}),
    pack_defs: input.pack_defs?.length ? input.pack_defs : existing.pack_defs,
  };
  await setProduk(p);
  revalidate();
  return { ok: true, message: `Produk sampingan "${p.nama}" diperbarui.` };
}

export async function deleteSampinganAction(id: string): Promise<ActionResult> {
  const denied = await ensureOwner();
  if (denied) return denied;
  const existing = await getProduk(id);
  if (!existing) return { ok: false, error: "Produk tidak ditemukan." };
  await delProduk(id);
  revalidate();
  return { ok: true, message: `Produk sampingan "${existing.nama}" dihapus.` };
}
