import { KEY, redis } from "./redis";
import { formatSuplierId } from "./format";
import type {
  Produk,
  Transaksi,
  Pembelian,
  PriceHistory,
  Shift,
  UserKios,
  KiosConfig,
  Pesanan,
  Supplier,
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

export async function setUser(u: UserKios): Promise<void> {
  await redis().hset(KEY.users, { [u.phone]: u });
}

export async function delUser(id: string): Promise<void> {
  await redis().hdel(KEY.users, id);
}

// ── Config (kios:config) ────────────────────────────────────────────────────

const DEFAULT_CONFIG: KiosConfig = {
  auto_learn_enabled: true,
  learn_model: "",
  model_utama: "",
  notif_enabled: true,
  notif_jam: "07",
  qris_enabled: false,
  qris_nama: "",
  qris_image_url: "",
  wa_number: "",
  nama_toko: "Kios Cerdas",
  deskripsi_toko: "Pesan online, ambil di kios. Tanpa perlu daftar.",
  lokasi_toko: "",
  banner_image_url: "",
  jam_buka: "08:00",
  jam_tutup: "21:00",
};

export async function getConfig(): Promise<KiosConfig> {
  const raw = normalize<Partial<KiosConfig>>(await redis().get<unknown>(KEY.config));
  const cfg = { ...DEFAULT_CONFIG, ...(raw ?? {}) };
  if (!cfg.notif_jam) cfg.notif_jam = "07";
  return cfg;
}

export async function saveConfig(cfg: KiosConfig): Promise<void> {
  await redis().set(KEY.config, cfg);
}

// ── Transaksi (penjualan via dashboard) ─────────────────────────────────────

/** Next transaction ID, mirroring Go: INCR kios:seq:trx -> "TRX-%04d". */
export async function nextTrxId(): Promise<string> {
  const n = await redis().incr(KEY.seqTrx);
  return `TRX-${String(n).padStart(4, "0")}`;
}

export async function pushTransaksi(tx: Transaksi): Promise<void> {
  await redis().rpush(KEY.transaksi, tx);
}

// ── Pesanan (orders from the public storefront) ─────────────────────────────

export async function nextPesananId(): Promise<string> {
  const n = await redis().incr(KEY.seqPesanan);
  return `PSN-${String(n).padStart(4, "0")}`;
}

export async function setPesanan(p: Pesanan): Promise<void> {
  await redis().hset(KEY.pesanan, { [p.id]: p });
}

export async function getPesanan(id: string): Promise<Pesanan | null> {
  return normalize<Pesanan>(await redis().hget<unknown>(KEY.pesanan, id));
}

export async function getAllPesanan(): Promise<Pesanan[]> {
  const map = await redis().hgetall<Record<string, unknown>>(KEY.pesanan);
  if (!map) return [];
  const list = normalizeList<Pesanan>(Object.values(map));
  list.sort((a, b) => b.created_at - a.created_at); // newest first
  return list;
}

/** Generic per-IP rate limiter (sliding window via INCR+EXPIRE). */
export async function bumpRate(scope: string, ip: string, windowSec: number): Promise<number> {
  const key = `kios:rate:${scope}:${ip}`;
  const n = await redis().incr(key);
  if (n === 1) await redis().expire(key, windowSec);
  return n;
}

// ── Login dashboard (kode dari /login bot) ──────────────────────────────────

// --- Supplier ---

export { formatSuplierId };

export async function getAllSuplier(): Promise<Supplier[]> {
  const m = await redis().hgetall(KEY.suplier);
  return normalizeList<Supplier>(Object.values(m ?? {}));
}

export async function getSuplier(id: string): Promise<Supplier | null> {
  const v = await redis().hget<unknown>(KEY.suplier, id);
  return normalize<Supplier>(v);
}

export async function setSuplier(s: Supplier): Promise<void> {
  await redis().hset(KEY.suplier, { [s.id]: s });
}

export async function delSuplier(id: string): Promise<void> {
  await redis().hdel(KEY.suplier, id);
}

export async function nextSuplierId(): Promise<string> {
  const n = await redis().incr(KEY.seqSup);
  return formatSuplierId(n as number);
}

export async function getAllHargaSupplier(): Promise<Record<string, number>> {
  const m = (await redis().hgetall(KEY.hargaSupplier)) ?? {};
  const out: Record<string, number> = {};
  for (const [k, v] of Object.entries(m)) {
    const n = typeof v === "number" ? v : parseInt(String(v), 10);
    if (!Number.isNaN(n)) out[k] = n;
  }
  return out;
}

export async function setHargaSupplier(
  produkId: string,
  supplier: string,
  harga: number,
): Promise<void> {
  await redis().hset(KEY.hargaSupplier, {
    [`${produkId}|${supplier}`]: harga,
  });
}

/**
 * Look up and consume a one-time login code written by the bot's /login
 * command (key kios:login:<code>). Deletes it on read so it can't be reused.
 */
export async function consumeLoginCode(
  code: string,
): Promise<{ id: string; nama: string } | null> {
  const key = `kios:login:${code}`;
  const rec = normalize<{ id: string; nama: string }>(await redis().get<unknown>(key));
  if (!rec || !rec.id) return null;
  await redis().del(key);
  return rec;
}

/** Increment a per-IP attempt counter (5-min window). Returns the new count. */
export async function bumpLoginAttempts(ip: string): Promise<number> {
  const key = `kios:login:attempts:${ip}`;
  const n = await redis().incr(key);
  if (n === 1) await redis().expire(key, 300);
  return n;
}
