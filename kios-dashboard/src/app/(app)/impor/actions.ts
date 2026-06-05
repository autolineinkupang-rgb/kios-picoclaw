"use server";

import { revalidatePath } from "next/cache";
import { getSession } from "@/lib/auth";
import { getAllProduk, getAllPiutang, getAllHutang, getAllTransaksi, setProduk } from "@/lib/kios";
import { todayWITA } from "@/lib/format";
import type { Produk, Piutang, Hutang, Transaksi } from "@/lib/types";

export type TableRow = Record<string, string>;

export interface ImportSummary {
  ok: true;
  created: number;
  updated: number;
  skipped: number;
  errors: string[];
}
export type ImportResult = ImportSummary | { ok: false; error: string };

// Resolve a value from a row by trying several header aliases.
function pick(row: TableRow, aliases: string[]): string | undefined {
  for (const a of aliases) {
    const v = row[a];
    if (v !== undefined && v !== "") return v;
  }
  return undefined;
}

function toInt(v: string | undefined): number | undefined {
  if (v === undefined) return undefined;
  const n = parseInt(v.replace(/[^\d-]/g, ""), 10);
  return Number.isFinite(n) ? n : undefined;
}

const F = {
  id: ["id", "kode_produk"],
  nama: ["nama", "nama_produk", "produk", "name"],
  kategori: ["kategori", "category"],
  satuan: ["satuan", "unit"],
  harga_beli: ["harga_beli", "beli", "modal", "cost"],
  harga_jual: ["harga_jual", "jual", "harga", "price"],
  stok: ["stok", "stock", "qty", "jumlah", "stok_akhir"],
  stok_minimum: ["stok_minimum", "stok_min", "minimum", "min"],
  stok_kritis: ["stok_kritis", "kritis", "critical"],
  supplier: ["supplier", "pemasok"],
  barcode: ["barcode", "kode"],
  image_url: ["gambar", "image", "image_url", "foto", "url_gambar"],
};

interface Indexed {
  byId: Map<string, Produk>;
  byBarcode: Map<string, Produk>;
  byNama: Map<string, Produk>;
  maxId: number;
}

function indexProducts(all: Produk[]): Indexed {
  const byId = new Map<string, Produk>();
  const byBarcode = new Map<string, Produk>();
  const byNama = new Map<string, Produk>();
  let maxId = 0;
  for (const p of all) {
    byId.set(p.id, p);
    if (p.barcode) byBarcode.set(p.barcode, p);
    byNama.set(p.nama.trim().toLowerCase(), p);
    const n = parseInt(p.id, 10);
    if (Number.isFinite(n) && n > maxId) maxId = n;
  }
  return { byId, byBarcode, byNama, maxId };
}

function findExisting(idx: Indexed, row: TableRow): Produk | undefined {
  const id = pick(row, F.id);
  if (id && idx.byId.has(id)) return idx.byId.get(id);
  const barcode = pick(row, F.barcode);
  if (barcode && idx.byBarcode.has(barcode)) return idx.byBarcode.get(barcode);
  const nama = pick(row, F.nama);
  if (nama && idx.byNama.has(nama.trim().toLowerCase())) return idx.byNama.get(nama.trim().toLowerCase());
  return undefined;
}

/** Owner: full product upsert from a spreadsheet. */
export async function importProdukAction(rows: TableRow[]): Promise<ImportResult> {
  const session = await getSession();
  if (!session) return { ok: false, error: "Sesi berakhir. Silakan masuk lagi." };
  if (session.role !== "owner") return { ok: false, error: "Impor produk khusus pemilik (owner)." };
  if (!Array.isArray(rows) || rows.length === 0) return { ok: false, error: "File kosong / tidak terbaca." };

  let all: Produk[];
  try {
    all = await getAllProduk();
  } catch (e) {
    return { ok: false, error: e instanceof Error ? e.message : "Gagal baca data." };
  }
  const idx = indexProducts(all);

  let created = 0;
  let updated = 0;
  let skipped = 0;
  const errors: string[] = [];

  for (let i = 0; i < rows.length; i++) {
    const row = rows[i];
    const baris = i + 2; // +1 header, +1 to 1-based
    const nama = pick(row, F.nama);
    const existing = findExisting(idx, row);

    if (!existing && !nama) {
      skipped++;
      if (errors.length < 12) errors.push(`Baris ${baris}: tidak ada nama produk.`);
      continue;
    }

    if (existing) {
      const p = { ...existing };
      const namaV = pick(row, F.nama);
      if (namaV) p.nama = namaV;
      const kat = pick(row, F.kategori);
      if (kat) p.kategori = kat;
      const sat = pick(row, F.satuan);
      if (sat) p.satuan = sat;
      const sup = pick(row, F.supplier);
      if (sup) p.supplier = sup;
      const bc = pick(row, F.barcode);
      if (bc) p.barcode = bc;
      const img = pick(row, F.image_url);
      if (img) p.image_url = img.trim();
      const hb = toInt(pick(row, F.harga_beli));
      if (hb !== undefined && hb >= 0) p.harga_beli = hb;
      const hj = toInt(pick(row, F.harga_jual));
      if (hj !== undefined && hj >= 0) p.harga_jual = hj;
      const st = toInt(pick(row, F.stok));
      if (st !== undefined && st >= 0) p.stok = st;
      const smin = toInt(pick(row, F.stok_minimum));
      if (smin !== undefined && smin >= 0) p.stok_minimum = smin;
      const skrit = toInt(pick(row, F.stok_kritis));
      if (skrit !== undefined && skrit >= 0) p.stok_kritis = skrit;
      p.last_update = todayWITA();
      await setProduk(p);
      updated++;
    } else {
      const hj = toInt(pick(row, F.harga_jual));
      if (hj === undefined || hj <= 0) {
        skipped++;
        if (errors.length < 12) errors.push(`Baris ${baris} (${nama}): produk baru perlu harga jual.`);
        continue;
      }
      idx.maxId += 1;
      const id = String(idx.maxId).padStart(3, "0");
      const p: Produk = {
        id,
        barcode: pick(row, F.barcode) ?? "",
        nama: nama!.trim(),
        kategori: pick(row, F.kategori) ?? "umum",
        satuan: pick(row, F.satuan) ?? "pcs",
        stok: toInt(pick(row, F.stok)) ?? 0,
        harga_beli: toInt(pick(row, F.harga_beli)) ?? 0,
        harga_jual: hj,
        stok_minimum: toInt(pick(row, F.stok_minimum)) ?? 5,
        stok_kritis: toInt(pick(row, F.stok_kritis)) ?? 2,
        supplier: pick(row, F.supplier) ?? "",
        last_update: todayWITA(),
        has_exp: false,
        exp_date: "",
        image_url: pick(row, F.image_url)?.trim() ?? "",
      };
      await setProduk(p);
      idx.byId.set(id, p);
      idx.byNama.set((p.nama ?? "").toLowerCase(), p);
      created++;
    }
  }

  revalidatePath("/produk");
  revalidatePath("/dashboard");
  return { ok: true, created, updated, skipped, errors };
}

/** Kasir/owner: stock-count import — only updates stok of existing products. */
export async function importStokAction(rows: TableRow[]): Promise<ImportResult> {
  const session = await getSession();
  if (!session) return { ok: false, error: "Sesi berakhir. Silakan masuk lagi." };
  if (!Array.isArray(rows) || rows.length === 0) return { ok: false, error: "File kosong / tidak terbaca." };

  let all: Produk[];
  try {
    all = await getAllProduk();
  } catch (e) {
    return { ok: false, error: e instanceof Error ? e.message : "Gagal baca data." };
  }
  const idx = indexProducts(all);

  let updated = 0;
  let skipped = 0;
  const errors: string[] = [];

  for (let i = 0; i < rows.length; i++) {
    const row = rows[i];
    const baris = i + 2;
    const existing = findExisting(idx, row);
    const st = toInt(pick(row, F.stok));
    if (!existing) {
      skipped++;
      if (errors.length < 12) errors.push(`Baris ${baris}: produk tidak ditemukan.`);
      continue;
    }
    if (st === undefined || st < 0) {
      skipped++;
      if (errors.length < 12) errors.push(`Baris ${baris} (${existing.nama}): nilai stok tidak valid.`);
      continue;
    }
    const p = { ...existing, stok: st, last_update: todayWITA() };
    await setProduk(p);
    updated++;
  }

  revalidatePath("/produk");
  revalidatePath("/dashboard");
  return { ok: true, created: 0, updated, skipped, errors };
}

// ── Export actions ────────────────────────────────────────────────────────────

export type PeriodeExport = "hari_ini" | "7hari" | "30hari" | "semua";

export async function exportProdukAction(): Promise<Produk[]> {
  const session = await getSession();
  if (!session || session.role !== "owner") return [];
  return getAllProduk();
}

export async function exportPiutangAction(): Promise<Piutang[]> {
  const session = await getSession();
  if (!session || session.role !== "owner") return [];
  return getAllPiutang();
}

export async function exportHutangAction(): Promise<Hutang[]> {
  const session = await getSession();
  if (!session || session.role !== "owner") return [];
  return getAllHutang();
}

export async function exportTransaksiAction(periode: PeriodeExport): Promise<Transaksi[]> {
  const session = await getSession();
  if (!session || session.role !== "owner") return [];
  const all = await getAllTransaksi();
  if (periode === "semua") return all;
  const today = todayWITA();
  const days = periode === "hari_ini" ? 0 : periode === "7hari" ? 7 : 30;
  const cutoff = new Date(today);
  cutoff.setDate(cutoff.getDate() - days);
  const cutoffStr = cutoff.toISOString().slice(0, 10);
  return all.filter((t) => t.tanggal >= cutoffStr);
}
