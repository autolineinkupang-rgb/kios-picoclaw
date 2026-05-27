"use server";

import { revalidatePath } from "next/cache";
import { getSession } from "@/lib/auth";
import { getPesanan, setPesanan } from "@/lib/kios";
import { recordSale } from "@/lib/sales";

export type ActionResult = { ok: true; message: string } | { ok: false; error: string };

async function requireStaff(): Promise<{ nama: string } | ActionResult> {
  const session = await getSession();
  if (!session) return { ok: false, error: "Sesi berakhir. Silakan masuk lagi." };
  return { nama: session.nama };
}

export async function prosesPesananAction(id: string, metode = "tunai"): Promise<ActionResult> {
  const gate = await requireStaff();
  if ("ok" in gate) return gate;

  const pesanan = await getPesanan(id);
  if (!pesanan) return { ok: false, error: "Pesanan tidak ditemukan." };
  if (pesanan.status !== "pending") {
    return { ok: false, error: "Pesanan ini sudah diproses atau ditolak." };
  }

  const sale = await recordSale(
    pesanan.items.map((it) => ({ produkId: it.produk_id, qty: it.qty })),
    metode,
    gate.nama,
    `pesanan ${pesanan.id}`,
  );
  if (!sale.ok) return sale; // stock insufficient — keep order pending

  pesanan.status = "diproses";
  await setPesanan(pesanan);

  revalidatePath("/pesanan");
  revalidatePath("/dashboard");
  revalidatePath("/penjualan");
  revalidatePath("/produk");
  revalidatePath("/laporan");
  return { ok: true, message: `Pesanan ${pesanan.id} diproses jadi penjualan.` };
}

export async function tolakPesananAction(id: string): Promise<ActionResult> {
  const gate = await requireStaff();
  if ("ok" in gate) return gate;

  const pesanan = await getPesanan(id);
  if (!pesanan) return { ok: false, error: "Pesanan tidak ditemukan." };
  if (pesanan.status !== "pending") {
    return { ok: false, error: "Pesanan ini sudah diproses atau ditolak." };
  }
  pesanan.status = "ditolak";
  await setPesanan(pesanan);
  revalidatePath("/pesanan");
  return { ok: true, message: `Pesanan ${pesanan.id} ditolak.` };
}
