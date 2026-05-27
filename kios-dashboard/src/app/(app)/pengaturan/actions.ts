"use server";

import { revalidatePath } from "next/cache";
import { getSession } from "@/lib/auth";
import { getConfig, saveConfig } from "@/lib/kios";
import type { KiosConfig } from "@/lib/types";

export type ActionResult = { ok: true; message: string } | { ok: false; error: string };

export async function saveConfigAction(input: KiosConfig): Promise<ActionResult> {
  const session = await getSession();
  if (!session) return { ok: false, error: "Sesi berakhir. Silakan masuk lagi." };
  if (session.role !== "owner") return { ok: false, error: "Aksi ini khusus pemilik (owner)." };

  const jam = String(input.notif_jam ?? "").padStart(2, "0");
  if (!/^([01]\d|2[0-3])$/.test(jam)) {
    return { ok: false, error: "Jam notif harus 00–23." };
  }

  const current = await getConfig();
  const cfg: KiosConfig = {
    ...current,
    auto_learn_enabled: Boolean(input.auto_learn_enabled),
    notif_enabled: Boolean(input.notif_enabled),
    notif_jam: jam,
    learn_model: (input.learn_model ?? "").trim(),
  };
  await saveConfig(cfg);
  revalidatePath("/pengaturan");
  return { ok: true, message: "Pengaturan disimpan." };
}
