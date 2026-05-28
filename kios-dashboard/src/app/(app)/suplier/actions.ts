"use server";

import { revalidatePath } from "next/cache";
import { getSession } from "@/lib/auth";
import { setSuplier, delSuplier, nextSuplierId } from "@/lib/kios";
import type { Supplier } from "@/lib/types";

async function ensureStaff() {
  const session = await getSession();
  if (!session || (session.role !== "owner" && session.role !== "kasir")) {
    return { ok: false as const, error: "Akses ditolak. Khusus owner/kasir." };
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

function sanitize(input: Partial<Supplier>): Supplier | { error: string } {
  const nama = (input.nama ?? "").trim();
  if (!nama) return { error: "Nama supplier wajib diisi." };
  return {
    id: input.id ?? "",
    nama,
    kontak: (input.kontak ?? "").trim(),
    alamat: (input.alamat ?? "").trim(),
    produk_utama: (input.produk_utama ?? "").trim(),
    pic: (input.pic ?? "").trim(),
    catatan: (input.catatan ?? "").trim(),
  };
}

export async function createSuplierAction(input: Partial<Supplier>) {
  const gate = await ensureStaff();
  if (!gate.ok) return gate;
  const s = sanitize(input);
  if ("error" in s) return { ok: false as const, error: s.error };
  s.id = await nextSuplierId();
  await setSuplier(s);
  revalidatePath("/suplier");
  return { ok: true as const };
}

export async function updateSuplierAction(input: Partial<Supplier>) {
  const gate = await ensureStaff();
  if (!gate.ok) return gate;
  if (!input.id) return { ok: false as const, error: "ID supplier tidak ada." };
  const s = sanitize(input);
  if ("error" in s) return { ok: false as const, error: s.error };
  s.id = input.id;
  await setSuplier(s);
  revalidatePath("/suplier");
  return { ok: true as const };
}

export async function deleteSuplierAction(id: string) {
  const gate = await ensureOwner();
  if (!gate.ok) return gate;
  await delSuplier(id);
  revalidatePath("/suplier");
  return { ok: true as const };
}

export async function setHargaOverrideAction(
  produkId: string,
  supplier: string,
  harga: number,
) {
  const gate = await ensureStaff();
  if (!gate.ok) return gate;
  if (!produkId || !supplier || !(harga >= 0)) {
    return { ok: false as const, error: "Data harga override tidak valid." };
  }
  const { setHargaSupplier } = await import("@/lib/kios");
  await setHargaSupplier(produkId, supplier, Math.floor(harga));
  revalidatePath("/suplier");
  return { ok: true as const };
}
