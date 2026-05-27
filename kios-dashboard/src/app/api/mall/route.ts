import { NextResponse } from "next/server";
import { getAllProduk, getConfig } from "@/lib/kios";
import type { PublicProduk } from "@/lib/types";

// Public products API for the buyer storefront (/mall). No auth.
// Returns only buyer-safe fields — never cost price or supplier.
export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function GET() {
  try {
    const [all, cfg] = await Promise.all([getAllProduk(), getConfig()]);
    const produk: PublicProduk[] = all
      .map((p) => ({
        id: p.id,
        nama: p.nama,
        kategori: p.kategori,
        satuan: p.satuan,
        harga_jual: p.harga_jual,
        stok: p.stok,
        image_url: p.image_url || "",
      }))
      .sort((a, b) => a.nama.localeCompare(b.nama));
    const kategori = [...new Set(produk.map((p) => p.kategori).filter(Boolean))].sort();
    const qris =
      cfg.qris_enabled && cfg.qris_image_url
        ? { enabled: true, nama: cfg.qris_nama || "Kios Cerdas", image_url: cfg.qris_image_url }
        : { enabled: false };
    return NextResponse.json(
      { ok: true, produk, kategori, qris, wa_number: cfg.wa_number || "" },
      { headers: { "Cache-Control": "no-store" } },
    );
  } catch {
    return NextResponse.json(
      { ok: false, error: "Daftar produk belum bisa dimuat. Coba lagi nanti." },
      { status: 503 },
    );
  }
}
