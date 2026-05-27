import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { bumpRate, getAllProduk, nextPesananId, setPesanan } from "@/lib/kios";
import { timeWITA, todayWITA } from "@/lib/format";
import type { Pesanan, PesananItem } from "@/lib/types";

export const runtime = "nodejs";

const MAX_PER_MINUTE = 6;

function clip(s: unknown, max: number): string {
  return String(s ?? "").trim().slice(0, max);
}

export async function POST(req: NextRequest) {
  let body: {
    items?: { produkId?: string; qty?: number }[];
    nama?: string;
    kontak?: string;
    catatan?: string;
    metode?: string;
  };
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ ok: false, error: "Format tidak valid." }, { status: 400 });
  }

  const ip =
    req.headers.get("x-forwarded-for")?.split(",")[0]?.trim() ||
    req.headers.get("x-real-ip") ||
    "unknown";

  try {
    const hits = await bumpRate("pesanan", ip, 60);
    if (hits > MAX_PER_MINUTE) {
      return NextResponse.json(
        { ok: false, error: "Terlalu banyak pesanan. Coba lagi sebentar ya." },
        { status: 429 },
      );
    }

    const wanted = new Map<string, number>();
    for (const it of body.items ?? []) {
      const q = Math.trunc(Number(it?.qty));
      if (it?.produkId && Number.isFinite(q) && q > 0) {
        wanted.set(it.produkId, (wanted.get(it.produkId) ?? 0) + q);
      }
    }
    if (wanted.size === 0) {
      return NextResponse.json({ ok: false, error: "Keranjang kosong." }, { status: 400 });
    }

    const byId = new Map((await getAllProduk()).map((p) => [p.id, p]));
    const items: PesananItem[] = [];
    let total = 0;
    for (const [id, qty] of wanted) {
      const p = byId.get(id);
      if (!p) return NextResponse.json({ ok: false, error: "Ada produk yang tidak tersedia." }, { status: 400 });
      if (p.stok < qty) {
        return NextResponse.json(
          { ok: false, error: `Stok ${p.nama} tinggal ${p.stok}.` },
          { status: 409 },
        );
      }
      const subtotal = qty * p.harga_jual;
      items.push({ produk_id: p.id, nama_produk: p.nama, qty, harga_satuan: p.harga_jual, subtotal });
      total += subtotal;
    }

    const metode = body.metode === "qris" ? "qris" : "tunai";
    const id = await nextPesananId();
    const pesanan: Pesanan = {
      id,
      tanggal: todayWITA(),
      jam: timeWITA(),
      nama_pembeli: clip(body.nama, 60) || "Pembeli",
      kontak: clip(body.kontak, 40),
      items,
      total,
      catatan: clip(body.catatan, 200),
      metode_bayar: metode,
      status: "pending",
      created_at: Math.floor(Date.now() / 1000),
    };
    await setPesanan(pesanan);

    return NextResponse.json({ ok: true, id, total });
  } catch {
    return NextResponse.json(
      { ok: false, error: "Server belum siap menerima pesanan. Coba lagi nanti." },
      { status: 500 },
    );
  }
}
