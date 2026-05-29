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

  const qrisImageUrl = (input.qris_image_url ?? "").trim();
  if (qrisImageUrl && !/^(https?:\/\/|data:image\/)/i.test(qrisImageUrl)) {
    return { ok: false, error: "Gambar QRIS harus berupa URL (http/https) atau hasil unggahan." };
  }
  if (qrisImageUrl.length > 600_000) {
    return { ok: false, error: "Gambar QRIS terlalu besar. Coba gambar yang lebih kecil ya." };
  }

  const bannerImageUrl = (input.banner_image_url ?? "").trim();
  if (bannerImageUrl && !/^(https?:\/\/|data:image\/)/i.test(bannerImageUrl)) {
    return { ok: false, error: "Gambar banner harus berupa URL (http/https) atau hasil unggahan." };
  }
  if (bannerImageUrl.length > 600_000) {
    return { ok: false, error: "Gambar banner terlalu besar. Coba gambar yang lebih kecil ya." };
  }

  const jamBuka = (input.jam_buka ?? "").trim();
  const jamTutup = (input.jam_tutup ?? "").trim();
  if (jamBuka && !/^\d{2}:\d{2}$/.test(jamBuka)) {
    return { ok: false, error: "Format jam buka salah. Gunakan format HH:MM (mis. 08:00)." };
  }
  if (jamTutup && !/^\d{2}:\d{2}$/.test(jamTutup)) {
    return { ok: false, error: "Format jam tutup salah. Gunakan format HH:MM (mis. 21:00)." };
  }

  const waNumber = (input.wa_number ?? "").replace(/\D/g, "").slice(0, 20);

  const current = await getConfig();
  const cfg: KiosConfig = {
    ...current,
    auto_learn_enabled: Boolean(input.auto_learn_enabled),
    notif_enabled: Boolean(input.notif_enabled),
    notif_jam: jam,
    learn_model: (input.learn_model ?? "").trim(),
    model_utama: (input.model_utama ?? "").trim(),
    qris_enabled: Boolean(input.qris_enabled),
    qris_nama: (input.qris_nama ?? "").trim().slice(0, 60),
    qris_image_url: qrisImageUrl,
    wa_number: waNumber,
    nama_toko: (input.nama_toko ?? "").trim().slice(0, 60),
    deskripsi_toko: (input.deskripsi_toko ?? "").trim().slice(0, 200),
    lokasi_toko: (input.lokasi_toko ?? "").trim().slice(0, 80),
    banner_image_url: bannerImageUrl,
    jam_buka: jamBuka,
    jam_tutup: jamTutup,
  };
  await saveConfig(cfg);
  revalidatePath("/pengaturan");
  revalidatePath("/toko");
  return { ok: true, message: "Pengaturan disimpan." };
}
