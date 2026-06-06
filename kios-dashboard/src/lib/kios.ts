import { KEY, redis } from "./redis";
import { formatSuplierId, todayWITA } from "./format";
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
  Pelanggan,
  Piutang,
  Hutang,
  Pembayaran,
  PulsaDenom,
  PulsaTopup,
  Penarikan,
  SampinganSaldo,
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
  notif_enabled: true,
  notif_jam: "07",
  qris_enabled: false,
  qris_nama: "",
  qris_image_url: "",
  wa_number: "",
};

export async function getConfig(): Promise<KiosConfig> {
  const raw = normalize<Partial<KiosConfig>>(await redis().get<unknown>(KEY.config));
  const cfg = { ...DEFAULT_CONFIG, ...(raw ?? {}) };
  if (!cfg.notif_jam) cfg.notif_jam = "07";
  if (cfg.notif_piutang_enabled === undefined) cfg.notif_piutang_enabled = true;
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

// ── Pelanggan (customer registry) ────────────────────────────────────────────

// normalizeWaTs converts a raw phone input to canonical "62..." with the same
// guards as Go's NormalizePhone: Indonesian numbers only (must start "62"),
// length 10–15 chars. Uses normalizeWaNumber from wa.ts for the base conversion.
function normalizeWaTs(raw: string): string {
  const d = (raw || "").replace(/\D/g, "");
  if (!d) return "";
  let n = d;
  if (n.startsWith("0")) n = "62" + n.slice(1);
  else if (n.startsWith("8")) n = "62" + n;
  if (n.length < 10 || n.length > 15) return "";
  if (!n.startsWith("62")) return "";
  return n;
}

export async function getPelanggan(phone: string): Promise<Pelanggan | null> {
  return normalize<Pelanggan>(await redis().hget<unknown>(KEY.pelanggan, phone));
}

export async function getAllPelanggan(): Promise<Pelanggan[]> {
  const map = await redis().hgetall<Record<string, unknown>>(KEY.pelanggan);
  if (!map) return [];
  return normalizeList<Pelanggan>(Object.values(map));
}

export async function setPelanggan(p: Pelanggan): Promise<void> {
  await redis().hset(KEY.pelanggan, { [p.phone]: p });
}

export async function delPelanggan(phone: string): Promise<void> {
  await redis().hdel(KEY.pelanggan, phone);
}

export async function upsertPelanggan(
  nama: string,
  rawPhone: string,
): Promise<Pelanggan> {
  const phone = normalizeWaTs(rawPhone);
  if (!phone) throw new Error("Nomor WhatsApp tidak valid");

  const existing = await getPelanggan(phone);
  const now = Math.floor(Date.now() / 1000);
  const today = todayWITA(); // WITA date, consistent with Go's NowWITA()

  const updated: Pelanggan = existing
    ? {
        ...existing,
        nama: nama.trim(),
        total_pesanan: existing.total_pesanan + 1,
        last_order: today,
      }
    : {
        id: `PLG-${phone}`,
        phone,
        nama: nama.trim(),
        total_utang: 0,
        total_pesanan: 1,
        total_belanja: 0,
        catatan: "",
        created_at: now,
        last_order: today,
      };

  await setPelanggan(updated);
  return updated;
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

export interface HargaSupplierLast {
  harga: number;
  kemasan: string;
  isi: number;
  harga_pack: number;
  tanggal: string;
}

export async function getAllHargaSupplierLast(): Promise<Record<string, HargaSupplierLast>> {
  const m = (await redis().hgetall(KEY.hargaSupplierLast)) ?? {};
  const out: Record<string, HargaSupplierLast> = {};
  for (const [k, v] of Object.entries(m)) {
    const parsed = normalize<HargaSupplierLast>(v);
    if (parsed) out[k] = parsed;
  }
  return out;
}

export async function setHargaSupplierLast(
  produkId: string,
  supplierId: string,
  v: HargaSupplierLast,
): Promise<void> {
  await redis().hset(KEY.hargaSupplierLast, { [`${produkId}|${supplierId}`]: v });
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

// ── Bon / Hutang ──────────────────────────────────────────────────────────────

export async function getAllPiutang(): Promise<Piutang[]> {
  const map = await redis().hgetall<Record<string, unknown>>(KEY.piutang);
  if (!map) return [];
  return normalizeList<Piutang>(Object.values(map));
}
export async function getPiutang(id: string): Promise<Piutang | null> {
  return normalize<Piutang>(await redis().hget<unknown>(KEY.piutang, id));
}
export async function setPiutang(p: Piutang): Promise<void> {
  await redis().hset(KEY.piutang, { [p.id]: p });
}
export async function getAllHutang(): Promise<Hutang[]> {
  const map = await redis().hgetall<Record<string, unknown>>(KEY.hutang);
  if (!map) return [];
  return normalizeList<Hutang>(Object.values(map));
}
export async function getHutang(id: string): Promise<Hutang | null> {
  return normalize<Hutang>(await redis().hget<unknown>(KEY.hutang, id));
}
export async function setHutang(h: Hutang): Promise<void> {
  await redis().hset(KEY.hutang, { [h.id]: h });
}
export async function getAllPembayaran(): Promise<Pembayaran[]> {
  const list = await redis().lrange<unknown>(KEY.pembayaran, 0, -1);
  if (!list) return [];
  return normalizeList<Pembayaran>(list);
}
export async function appendPembayaran(p: Pembayaran): Promise<void> {
  await redis().rpush(KEY.pembayaran, p);
}
export async function nextPayId(): Promise<string> {
  const n = await redis().incr(KEY.seqPay);
  return `PAY-${String(n).padStart(4, "0")}`;
}

export async function nextPiutangId(): Promise<string> {
  const n = await redis().incr(KEY.seqPiu);
  return `PIU-${String(n).padStart(4, "0")}`;
}

export async function nextHutangId(): Promise<string> {
  const n = await redis().incr(KEY.seqHut);
  return `HUT-${String(n).padStart(4, "0")}`;
}

// ── Pulsa Denom ───────────────────────────────────────────────────────────────

export async function getAllPulsaDenom(): Promise<PulsaDenom[]> {
  const map = await redis().hgetall<Record<string, unknown>>(KEY.pulsaDenom);
  if (!map) return [];
  return normalizeList<PulsaDenom>(Object.values(map));
}

export async function setPulsaDenom(d: PulsaDenom): Promise<void> {
  await redis().hset(KEY.pulsaDenom, { [String(d.nominal)]: d });
}

export async function getAllPulsaTopup(): Promise<PulsaTopup[]> {
  const vals = await redis().lrange<unknown>(KEY.pulsaTopup, 0, -1);
  return normalizeList<PulsaTopup>(vals);
}

export async function nextPulsaTopupId(): Promise<string> {
  const n = await redis().incr(KEY.seqPtu);
  return `PTU-${String(n).padStart(4, "0")}`;
}

export async function pushPulsaTopup(t: PulsaTopup): Promise<void> {
  await redis().rpush(KEY.pulsaTopup, t);
}

// ── Penarikan Modal ───────────────────────────────────────────────────────────

export async function getAllPenarikan(): Promise<Penarikan[]> {
  const vals = await redis().lrange<unknown>(KEY.penarikan, 0, -1);
  return normalizeList<Penarikan>(vals);
}

export async function nextPenarikandId(): Promise<string> {
  const n = await redis().incr(KEY.seqPrk);
  return `PRK-${String(n).padStart(4, "0")}`;
}

export async function pushPenarikan(p: Penarikan): Promise<void> {
  await redis().rpush(KEY.penarikan, p);
}

export async function getPulsaAnchor(): Promise<Produk | null> {
  const all = await getAllProduk();
  return all.find((p) => p.jenis === "pulsa") ?? null;
}

// ── Sampingan Saldo (universal per-category balance) ─────────────────────────

const SAMPINGAN_SALDO_DEFAULT: SampinganSaldo = {
  pulsa: 0, pertalite: 0, pertamax: 0, solar: 0, minyak_tanah: 0,
};

export async function getSampinganSaldo(): Promise<SampinganSaldo | null> {
  return normalize<SampinganSaldo>(await redis().get<unknown>(KEY.sampinganSaldo));
}

export async function setSampinganSaldo(s: SampinganSaldo): Promise<void> {
  await redis().set(KEY.sampinganSaldo, s);
}

export async function getOrInitSampinganSaldo(): Promise<SampinganSaldo> {
  const saved = await getSampinganSaldo();
  return saved ? { ...SAMPINGAN_SALDO_DEFAULT, ...saved } : { ...SAMPINGAN_SALDO_DEFAULT };
}
