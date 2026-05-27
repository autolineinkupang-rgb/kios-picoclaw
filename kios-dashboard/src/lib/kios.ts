import { KEY, redis } from "./redis";
import type {
  Produk,
  Transaksi,
  Pembelian,
  PriceHistory,
  Shift,
  UserKios,
} from "./types";

// Values may come back from @upstash/redis already parsed (objects) or, if the
// auto-deserialiser left them as strings, as raw JSON. normalize handles both.
function normalize<T>(v: unknown): T | null {
  if (v == null) return null;
  if (typeof v === "string") {
    try {
      return JSON.parse(v) as T;
    } catch {
      return null;
    }
  }
  return v as T;
}

function normalizeList<T>(vals: unknown[]): T[] {
  const out: T[] = [];
  for (const v of vals) {
    const parsed = normalize<T>(v);
    if (parsed) out.push(parsed);
  }
  return out;
}

// ── Produk ────────────────────────────────────────────────────────────────

export async function getAllProduk(): Promise<Produk[]> {
  const map = await redis().hgetall<Record<string, unknown>>(KEY.produk);
  if (!map) return [];
  const list = normalizeList<Produk>(Object.values(map));
  list.sort((a, b) => a.id.localeCompare(b.id));
  return list;
}

export async function getProduk(id: string): Promise<Produk | null> {
  const v = await redis().hget<unknown>(KEY.produk, id);
  return normalize<Produk>(v);
}

export async function setProduk(p: Produk): Promise<void> {
  await redis().hset(KEY.produk, { [p.id]: p });
}

export async function delProduk(id: string): Promise<void> {
  await redis().hdel(KEY.produk, id);
}

/** Next zero-padded 3-digit product ID, mirroring Store.NextProdukID in Go. */
export async function nextProdukId(): Promise<string> {
  const all = await getAllProduk();
  let max = 0;
  for (const p of all) {
    const n = parseInt(p.id, 10);
    if (!Number.isNaN(n) && n > max) max = n;
  }
  return String(max + 1).padStart(3, "0");
}

// ── Transaksi / Pembelian / Price history ──────────────────────────────────

export async function getAllTransaksi(): Promise<Transaksi[]> {
  const vals = await redis().lrange<unknown>(KEY.transaksi, 0, -1);
  return normalizeList<Transaksi>(vals);
}

export async function getAllPembelian(): Promise<Pembelian[]> {
  const vals = await redis().lrange<unknown>(KEY.pembelian, 0, -1);
  return normalizeList<Pembelian>(vals);
}

export async function getAllPriceHistory(): Promise<PriceHistory[]> {
  const vals = await redis().lrange<unknown>(KEY.priceHistory, 0, -1);
  return normalizeList<PriceHistory>(vals);
}

// ── Shift / Users ───────────────────────────────────────────────────────────

export async function getShift(): Promise<Shift | null> {
  const v = await redis().get<unknown>(KEY.shift);
  return normalize<Shift>(v);
}

export async function getUser(id: string): Promise<UserKios | null> {
  const v = await redis().hget<unknown>(KEY.users, id);
  return normalize<UserKios>(v);
}

export async function getAllUsers(): Promise<UserKios[]> {
  const map = await redis().hgetall<Record<string, unknown>>(KEY.users);
  if (!map) return [];
  return normalizeList<UserKios>(Object.values(map));
}
