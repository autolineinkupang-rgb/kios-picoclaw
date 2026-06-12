"use server";

import { revalidatePath } from "next/cache";
import { getSession } from "@/lib/auth";
import {
  delProduk,
  getAllPenarikan,
  getAllProduk,
  getAllTransaksi,
  getOrInitSampinganSaldo,
  getProduk,
  getPulsaAnchor,
  nextPenarikandId,
  nextProdukId,
  nextPulsaTopupId,
  pushPenarikan,
  pushPulsaTopup,
  setSampinganSaldo,
  setProduk,
} from "@/lib/kios";
import { timeWITA, todayWITA } from "@/lib/format";
import { hitungLaba } from "@/lib/analytics";
import { kategoriBBM, penghasilanTersediaJenis } from "@/lib/sampingan";
import type { Produk } from "@/lib/types";

export type JenisSampingan = "pulsa" | "pertalite" | "pertamax" | "solar" | "minyak_tanah";

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

const VALID_JENIS: JenisSampingan[] = ["pulsa", "pertalite", "pertamax", "solar", "minyak_tanah"];

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

async function computePenghasilanTersediaJenis(jenis: JenisSampingan): Promise<number> {
  const [txs, produk, penarikan] = await Promise.all([
    getAllTransaksi(),
    getAllProduk(),
    getAllPenarikan(),
  ]);
  return penghasilanTersediaJenis(txs, produk, penarikan, jenis);
}

const JENIS_LABEL: Record<JenisSampingan, string> = {
  pulsa: "Pulsa",
  pertalite: "Pertalite",
  pertamax: "Pertamax",
  solar: "Solar",
  minyak_tanah: "Minyak Tanah",
};

export async function tarikHasilAction(
  jenis: JenisSampingan,
  jumlah: number,
  catatan = "",
): Promise<ActionResult> {
  const denied = await ensureOwner();
  if (denied) return denied;
  if (!Number.isFinite(jumlah) || jumlah <= 0)
    return { ok: false, error: "Jumlah harus lebih dari 0." };

  const tersedia = await computePenghasilanTersediaJenis(jenis);
  if (jumlah > tersedia)
    return { ok: false, error: `Penghasilan ${JENIS_LABEL[jenis]} tersedia hanya Rp ${tersedia.toLocaleString("id-ID")}.` };

  const session = await getSession();
  const label = JENIS_LABEL[jenis];
  const prkId = await nextPenarikandId();
  await pushPenarikan({
    id: prkId,
    tanggal: todayWITA(),
    jam: timeWITA(),
    jumlah: Math.trunc(jumlah),
    produk_id: jenis,
    produk_nama: label,
    kasir: session?.nama ?? "owner",
    catatan: catatan.trim() || `tarik hasil ${label}`,
  });

  revalidate();
  revalidatePath("/laporan");
  revalidatePath("/produk-sampingan");
  return { ok: true, message: `Berhasil tarik Rp ${Math.trunc(jumlah).toLocaleString("id-ID")} dari penghasilan ${label}.` };
}

export async function setAmbangBatasKategoriAction(
  jenis: JenisSampingan,
  nilai: number,
): Promise<ActionResult> {
  const denied = await ensureOwner();
  if (denied) return denied;
  if (!Number.isFinite(nilai) || nilai < 0)
    return { ok: false, error: "Nilai ambang batas tidak valid." };

  const saldo = await getOrInitSampinganSaldo();
  (saldo as unknown as Record<string, number>)[`min_${jenis}`] = Math.trunc(nilai);
  await setSampinganSaldo(saldo);
  revalidate();
  const label = JENIS_LABEL[jenis];
  const unit = jenis === "pulsa" ? `Rp ${nilai.toLocaleString("id-ID")}` : `${nilai} liter`;
  return { ok: true, message: `Ambang batas ${label} diatur ke ${unit}.` };
}

export async function topupSaldoAction(
  jenis: "pulsa",
  jumlah: number,
  catatan = "",
  fromLaba = false,
): Promise<ActionResult> {
  const denied = await ensureOwner();
  if (denied) return denied;
  if (!Number.isFinite(jumlah) || jumlah <= 0)
    return { ok: false, error: "Jumlah harus lebih dari 0." };

  if (fromLaba) {
    const tersedia = await computePenghasilanTersediaJenis("pulsa");
    if (jumlah > tersedia)
      return { ok: false, error: `Penghasilan Pulsa tersedia hanya Rp ${tersedia.toLocaleString("id-ID")}.` };
  }

  const session = await getSession();
  const saldo = await getOrInitSampinganSaldo();
  saldo.pulsa += Math.trunc(jumlah);
  await setSampinganSaldo(saldo);

  // Mirror to the pulsa product's own modal balance — the Telegram bot sells
  // against produk.saldo_modal, not the dashboard pool.
  const anchor = await getPulsaAnchor();
  if (anchor) {
    anchor.saldo_modal = (anchor.saldo_modal ?? 0) + Math.trunc(jumlah);
    anchor.last_update = todayWITA();
    await setProduk(anchor);
  }

  const topupId = await nextPulsaTopupId();
  await pushPulsaTopup({
    id: topupId,
    tanggal: todayWITA(),
    jam: timeWITA(),
    jumlah: Math.trunc(jumlah),
    saldo_sesudah: saldo.pulsa,
    kasir: session?.nama ?? "owner",
    catatan: catatan.trim() || `topup saldo pulsa${fromLaba ? " (dari laba)" : ""}`,
  });

  if (fromLaba) {
    const prkId = await nextPenarikandId();
    await pushPenarikan({
      id: prkId,
      tanggal: todayWITA(),
      jam: timeWITA(),
      jumlah: Math.trunc(jumlah),
      produk_id: "pulsa",
      produk_nama: "Pulsa",
      kasir: session?.nama ?? "owner",
      catatan: catatan.trim() || "topup saldo pulsa",
    });
  }

  revalidate();
  revalidatePath("/laporan");
  return {
    ok: true,
    message: `Saldo Pulsa +Rp ${Math.trunc(jumlah).toLocaleString("id-ID")} → total Rp ${saldo.pulsa.toLocaleString("id-ID")}${fromLaba ? " (dari laba)" : ""}.`,
  };
}

export async function topupStokAction(
  jenis: "pertalite" | "pertamax" | "solar" | "minyak_tanah",
  jumlah: number,
  catatan = "",
  fromLaba = false,
  hargaBeliPemasok = 0,
): Promise<ActionResult> {
  const denied = await ensureOwner();
  if (denied) return denied;
  if (!Number.isFinite(jumlah) || jumlah <= 0)
    return { ok: false, error: "Jumlah harus lebih dari 0." };

  const session = await getSession();
  const tambah = Math.trunc(jumlah);

  if (fromLaba) {
    if (hargaBeliPemasok <= 0)
      return { ok: false, error: "Harga beli dari pemasok wajib diisi untuk topup dari penghasilan." };
    const biayaModal = tambah * hargaBeliPemasok;
    const tersedia = await computePenghasilanTersediaJenis(jenis);
    if (biayaModal > tersedia)
      return {
        ok: false,
        error: `Biaya topup ${tambah} liter = Rp ${biayaModal.toLocaleString("id-ID")}. Penghasilan ${JENIS_LABEL[jenis]} tersedia hanya Rp ${tersedia.toLocaleString("id-ID")}.`,
      };
    const prkId = await nextPenarikandId();
    const label = JENIS_LABEL[jenis];
    await pushPenarikan({
      id: prkId,
      tanggal: todayWITA(),
      jam: timeWITA(),
      jumlah: biayaModal,
      produk_id: jenis,
      produk_nama: label,
      kasir: session?.nama ?? "owner",
      catatan: catatan.trim() || `topup stok ${label} ${tambah}L`,
    });
  }

  const saldo = await getOrInitSampinganSaldo();
  saldo[jenis] += tambah;
  await setSampinganSaldo(saldo);

  // Mirror to the fuel product's ml stock — the Telegram bot sells against
  // produk.stok_ml, not the dashboard pool.
  const all = await getAllProduk();
  const target = all.find((p) => kategoriBBM(p) === jenis);
  if (target) {
    target.stok_ml = (target.stok_ml ?? target.stok * 1000) + tambah * 1000;
    target.stok = Math.floor(target.stok_ml / 1000);
    if (hargaBeliPemasok > 0) target.harga_beli = Math.trunc(hargaBeliPemasok);
    target.last_update = todayWITA();
    await setProduk(target);
  }

  revalidate();
  revalidatePath("/laporan");
  const label = JENIS_LABEL[jenis];
  return {
    ok: true,
    message: `Stok ${label} +${tambah} liter → total ${saldo[jenis]} liter${fromLaba ? " (dari laba)" : ""}. ${catatan.trim()}`.trim(),
  };
}
