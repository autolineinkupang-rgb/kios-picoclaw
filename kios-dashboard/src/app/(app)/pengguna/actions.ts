"use server";

import { revalidatePath } from "next/cache";
import { getSession } from "@/lib/auth";
import { delUser, getUser, setUser } from "@/lib/kios";
import { todayWITA } from "@/lib/format";
import type { Role, UserKios } from "@/lib/types";

export type ActionResult = { ok: true; message: string } | { ok: false; error: string };

async function ensureOwner(): Promise<{ id: string } | ActionResult> {
  const session = await getSession();
  if (!session) return { ok: false, error: "Sesi berakhir. Silakan masuk lagi." };
  if (session.role !== "owner") return { ok: false, error: "Aksi ini khusus pemilik (owner)." };
  return { id: session.id };
}

export interface UserInput {
  phone: string; // Telegram user ID
  nama: string;
  role: Role;
}

export async function saveUserAction(input: UserInput): Promise<ActionResult> {
  const gate = await ensureOwner();
  if ("ok" in gate) return gate;

  const phone = input.phone.trim();
  if (!/^\d+$/.test(phone)) return { ok: false, error: "Telegram ID harus berupa angka." };
  if (!input.nama.trim()) return { ok: false, error: "Nama wajib diisi." };
  if (input.role !== "owner" && input.role !== "kasir")
    return { ok: false, error: "Role tidak valid." };

  const existing = await getUser(phone);
  const u: UserKios = {
    phone,
    nama: input.nama.trim(),
    role: input.role,
    aktif: existing?.aktif ?? true,
    ditambahkan: existing?.ditambahkan || todayWITA(),
  };
  await setUser(u);
  revalidatePath("/pengguna");
  return { ok: true, message: existing ? `Pengguna "${u.nama}" diperbarui.` : `Pengguna "${u.nama}" ditambahkan.` };
}

export async function toggleUserAktifAction(phone: string): Promise<ActionResult> {
  const gate = await ensureOwner();
  if ("ok" in gate) return gate;
  if (gate.id === phone) return { ok: false, error: "Tidak bisa menonaktifkan akun sendiri." };

  const u = await getUser(phone);
  if (!u) return { ok: false, error: "Pengguna tidak ditemukan." };
  u.aktif = !u.aktif;
  await setUser(u);
  revalidatePath("/pengguna");
  return { ok: true, message: `Pengguna "${u.nama}" ${u.aktif ? "diaktifkan" : "dinonaktifkan"}.` };
}

export async function deleteUserAction(phone: string): Promise<ActionResult> {
  const gate = await ensureOwner();
  if ("ok" in gate) return gate;
  if (gate.id === phone) return { ok: false, error: "Tidak bisa menghapus akun sendiri." };

  const u = await getUser(phone);
  if (!u) return { ok: false, error: "Pengguna tidak ditemukan." };
  await delUser(phone);
  revalidatePath("/pengguna");
  return { ok: true, message: `Pengguna "${u.nama}" dihapus.` };
}
