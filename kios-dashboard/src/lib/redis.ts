import { Redis } from "@upstash/redis";

// Single shared client. @upstash/redis talks to the Upstash REST API, which is
// the serverless-friendly transport for Vercel. It points at the SAME database
// the Go bot uses over rediss:// — just a different protocol.
//
// automaticDeserialization (default true) JSON-parses values on read and
// JSON-stringifies objects on write, matching exactly how the Go bot stores
// each record as a JSON string via go-redis.

let client: Redis | null = null;

export function redis(): Redis {
  if (client) return client;
  const url = process.env.UPSTASH_REDIS_REST_URL;
  const token = process.env.UPSTASH_REDIS_REST_TOKEN;
  if (!url || !token) {
    throw new Error(
      "UPSTASH_REDIS_REST_URL / UPSTASH_REDIS_REST_TOKEN belum di-set. Lihat .env.example.",
    );
  }
  client = new Redis({ url, token });
  return client;
}

// Redis key names — must match pkg/tools/kios/store.go exactly.
export const KEY = {
  produk: "kios:produk",
  transaksi: "kios:transaksi",
  pembelian: "kios:pembelian",
  priceHistory: "kios:price_history",
  shift: "kios:shift",
  users: "kios:users",
  config: "kios:config",
  seqTrx: "kios:seq:trx",
  pesanan: "kios:pesanan",
  seqPesanan: "kios:seq:psn",
  suplier: "kios:supplier",
  seqSup: "kios:seq:sup",
  hargaSupplier: "kios:harga_supplier",
  hargaSupplierLast: "kios:harga_supplier_last",
  pelanggan: "kios:pelanggan",
  piutang: "kios:piutang",
  hutang: "kios:hutang",
  pembayaran: "kios:pembayaran",
  seqPiu: "kios:seq:piu",
  seqHut: "kios:seq:hut",
  seqPay: "kios:seq:pay",
  pulsaDenom: "kios:pulsa:denom",
  pulsaTopup: "kios:pulsa:topup",
  seqPtu: "kios:seq:ptu",
} as const;
