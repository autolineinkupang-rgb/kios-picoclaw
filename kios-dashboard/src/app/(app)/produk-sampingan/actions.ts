"use server";

import { revalidatePath } from "next/cache";
import { getSession } from "@/lib/auth";
import {
  delProduk,
  getAllPenarikan,
  getAllProduk,
  getAllTransaksi,
  getProduk,
  nextPenarikandId,
  nextProdukId,
  nextPulsaTopupId,
  pushPenarikan,
  pushPulsaTopup,
  setProduk,
} from "@/lib/kios";
import { timeWITA, todayWITA } from "@/lib/format";
import { hitungLaba } from "@/lib/analytics";
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

async function computeLabaTersedia(): Promise<number> {
  const [txs, produk, penarikan] = await Promise.all([
    getAllTransaksi(),
    getAllProduk(),
    getAllPenarikan(),
  ]);
  const totalLaba = hitungLaba(txs, produk).laba;
  const totalTarik = penarikan.reduce((s, p) => s + p.jumlah, 0);
  return Math.max(0, totalLaba - totalTarik);
}

export async function tarikHasilAction(
  produkId: string,
  produkNama: string,
  jumlah: number,
  catatan = "",
): Promise<ActionResult> {
  const denied = await ensureOwner();
  if (denied) return denied;
  if (!Number.isFinite(jumlah) || jumlah <= 0)
    return { ok: false, error: "Jumlah harus lebih dari 0." };

  const tersedia = await computeLabaTersedia();
  if (jumlah > tersedia)
    return { ok: false, error: `Laba tersedia hanya Rp ${tersedia.toLocaleString("id-ID")}.` };

  const session = await getSession();
  const prkId = await nextPenarikandId();
  await pushPenarikan({
    id: prkId,
    tanggal: todayWITA(),
    jam: timeWITA(),
    jumlah: Math.trunc(jumlah),
    produk_id: produkId,
    produk_nama: produkNama,
    kasir: session?.nama ?? "owner",
    catatan: catatan.trim() || `tarik hasil ${produkNama}`,
  });

  revalidate();
  revalidatePath("/laporan");
  revalidatePath("/produk-sampingan");
  return { ok: true, message: `Berhasil tarik Rp ${jumlah.toLocaleString("id-ID")} dari laba "${produkNama}".` };
}

export async function setAmbangBatasAction(
  produkId: string,
  nilai: number,
): Promise<ActionResult> {
  const denied = await ensureOwner();
  if (denied) return denied;
  if (!Number.isFinite(nilai) || nilai < 0)
    return { ok: false, error: "Nilai ambang batas tidak valid." };

  const p = await getProduk(produkId);
  if (!p) return { ok: false, error: "Produk tidak ditemukan." };

  if (p.jenis === "pulsa") {
    p.saldo_minimum = Math.trunc(nilai);
  } else {
    p.stok_minimum = Math.trunc(nilai);
  }
  await setProduk(p);
  revalidate();
  return { ok: true, message: `Ambang batas "${p.nama}" diatur ke ${p.jenis === "pulsa" ? `Rp ${nilai.toLocaleString("id-ID")}` : `${nilai} liter`}.` };
}

export async function topupSaldoAction(
  produkId: string,
  jumlah: number,
  catatan = "",
  fromLaba = false,
): Promise<ActionResult> {
  const denied = await ensureOwner();
  if (denied) return denied;
  if (!Number.isFinite(jumlah) || jumlah <= 0)
    return { ok: false, error: "Jumlah harus lebih dari 0." };

  if (fromLaba) {
    const tersedia = await computeLabaTersedia();
    if (jumlah > tersedia)
      return { ok: false, error: `Laba tersedia hanya Rp ${tersedia.toLocaleString("id-ID")}.` };
  }

  const session = await getSession();
  const p = await getProduk(produkId);
  if (!p) return { ok: false, error: "Produk tidak ditemukan." };
  if (p.jenis !== "pulsa") return { ok: false, error: "Bukan produk pulsa." };

  const sebelum = p.saldo_modal ?? 0;
  p.saldo_modal = sebelum + Math.trunc(jumlah);
  p.last_update = todayWITA();
  await setProduk(p);

  const topupId = await nextPulsaTopupId();
  await pushPulsaTopup({
    id: topupId,
    tanggal: todayWITA(),
    jam: timeWITA(),
    jumlah: Math.trunc(jumlah),
    saldo_sesudah: p.saldo_modal,
    kasir: session?.nama ?? "owner",
    catatan: catatan.trim() || `topup saldo ${p.nama}${fromLaba ? " (dari laba)" : ""}`,
  });

  if (fromLaba) {
    const prkId = await nextPenarikandId();
    await pushPenarikan({
      id: prkId,
      tanggal: todayWITA(),
      jam: timeWITA(),
      jumlah: Math.trunc(jumlah),
      produk_id: p.id,
      produk_nama: p.nama,
      kasir: session?.nama ?? "owner",
      catatan: catatan.trim() || `topup saldo pulsa ${p.nama}`,
    });
  }

  revalidate();
  revalidatePath("/laporan");
  revalidatePath("/produk-sampingan");
  return {
    ok: true,
    message: `Saldo "${p.nama}" +Rp ${jumlah.toLocaleString("id-ID")} → total Rp ${p.saldo_modal.toLocaleString("id-ID")}${fromLaba ? " (dari laba)" : ""}.`,
  };
}

export async function topupStokAction(
  produkId: string,
  jumlah: number,
  catatan = "",
  fromLaba = false,
): Promise<ActionResult> {
  const denied = await ensureOwner();
  if (denied) return denied;
  if (!Number.isFinite(jumlah) || jumlah <= 0)
    return { ok: false, error: "Jumlah harus lebih dari 0." };

  const session = await getSession();
  const p = await getProduk(produkId);
  if (!p) return { ok: false, error: "Produk tidak ditemukan." };
  const isBBM = p.jenis === "bensin" || p.jenis === "solar" || p.jenis === "minyak_tanah";
  if (!isBBM) return { ok: false, error: "Bukan produk BBM." };

  if (fromLaba) {
    const hargaBeli = p.harga_beli;
    const biayaModal = Math.trunc(jumlah) * hargaBeli;
    if (biayaModal > 0) {
      const tersedia = await computeLabaTersedia();
      if (biayaModal > tersedia)
        return {
          ok: false,
          error: `Biaya modal ${jumlah} liter = Rp ${biayaModal.toLocaleString("id-ID")}. Laba tersedia hanya Rp ${tersedia.toLocaleString("id-ID")}.`,
        };
    }
  }

  const tambah = Math.trunc(jumlah);
  p.stok += tambah;
  if (p.stok_ml !== undefined) p.stok_ml += tambah * 1000;
  p.last_update = todayWITA();
  await setProduk(p);

  if (fromLaba && p.harga_beli > 0) {
    const biaya = tambah * p.harga_beli;
    const prkId = await nextPenarikandId();
    await pushPenarikan({
      id: prkId,
      tanggal: todayWITA(),
      jam: timeWITA(),
      jumlah: biaya,
      produk_id: p.id,
      produk_nama: p.nama,
      kasir: session?.nama ?? "owner",
      catatan: catatan.trim() || `topup stok ${p.nama} ${tambah}L`,
    });
  }

  revalidate();
  revalidatePath("/laporan");
  revalidatePath("/produk-sampingan");
  return {
    ok: true,
    message: `Stok "${p.nama}" +${tambah} liter → total ${p.stok} liter${fromLaba ? " (dari laba)" : ""}. ${catatan.trim()}`.trim(),
  };
}
