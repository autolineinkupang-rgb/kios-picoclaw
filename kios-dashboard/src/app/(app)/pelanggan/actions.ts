"use server";

import { revalidatePath } from "next/cache";
import { getSession } from "@/lib/auth";
import { delPelanggan } from "@/lib/kios";

export type ActionResult = { ok: true; message: string } | { ok: false; error: string };

async function ensureOwner(): Promise<{ id: string } | ActionResult> {
  const session = await getSession();
  if (!session) return { ok: false, error: "Sesi berakhir. Silakan masuk lagi." };
  if (session.role !== "owner") return { ok: false, error: "Aksi ini khusus pemilik (owner)." };
  return { id: session.id };
}

export async function deleteCustomerAction(phone: string): Promise<ActionResult> {
  const gate = await ensureOwner();
  if ("ok" in gate) return gate;

  if (!phone.trim()) return { ok: false, error: "ID pelanggan tidak valid." };

  await delPelanggan(phone);
  revalidatePath("/pelanggan");
  return { ok: true, message: "Pelanggan dihapus." };
}
