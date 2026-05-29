// TypeScript mirrors of the Go structs in pkg/tools/kios/store.go.
// Field names match the JSON tags so we read the exact same Redis payloads
// the Telegram bot writes. Do not rename without updating the Go side.

export interface Produk {
  id: string;
  barcode: string;
  nama: string;
  kategori: string;
  satuan: string;
  stok: number;
  harga_beli: number;
  harga_jual: number;
  stok_minimum: number;
  stok_kritis: number;
  supplier: string;
  last_update: string;
  has_exp: boolean;
  exp_date: string;
  image_url: string;
}

export interface Transaksi {
  id: string;
  tanggal: string; // YYYY-MM-DD (WITA)
  jam: string; // HH:mm:ss
  produk_id: string;
  nama_produk: string;
  kategori: string;
  qty: number;
  harga_satuan: number;
  total: number;
  metode_bayar: string; // tunai | transfer | qris
  kasir: string;
  catatan: string;
  session_id: string;
}

export interface Pembelian {
  id: string;
  session_id: string;
  tanggal: string;
  jam: string;
  produk_id: string;
  nama_produk: string;
  qty: number;
  harga_beli: number;
  subtotal: number;
  supplier: string;
  kasir: string;
  catatan: string;
}

export interface Supplier {
  id: string;
  nama: string;
  kontak: string;
  alamat: string;
  produk_utama: string;
  pic: string;
  catatan: string;
}

export interface PriceHistory {
  id: string;
  tanggal: string;
  jam: string;
  produk_id: string;
  nama_produk: string;
  harga_lama: number;
  harga_baru: number;
  selisih: number;
  supplier: string;
  kasir: string;
}

export interface Shift {
  kasir: string;
  saldo_awal: number;
  saldo_akhir: number;
  waktu_buka: string;
  waktu_tutup: string;
  status: string; // buka | tutup
}

export interface Promo {
  id: string;        // PROMO-NNNN
  produk: string;    // nama produk
  produk_id: string;
  tipe: "persen" | "fixed";
  nilai: number;     // persen (mis. 10) atau nominal rupiah
  min_qty: number;   // min qty agar promo berlaku (default 1)
  aktif: boolean;
  mulai: string;     // YYYY-MM-DD
  selesai: string;   // YYYY-MM-DD
  catatan: string;
}

export interface UserKios {
  phone: string; // holds the Telegram user ID
  nama: string;
  role: string; // owner | kasir
  aktif: boolean;
  ditambahkan: string;
}

// Public-safe product projection for the buyer storefront (never includes
// cost price / supplier).
export interface PublicProduk {
  id: string;
  nama: string;
  kategori: string;
  satuan: string;
  harga_jual: number;
  stok: number;
  image_url: string;
}

export interface PesananItem {
  produk_id: string;
  nama_produk: string;
  qty: number;
  harga_satuan: number;
  subtotal: number;
}

export type PesananStatus = "pending" | "diproses" | "selesai" | "ditolak";

export interface Pesanan {
  id: string;
  tanggal: string;
  jam: string;
  nama_pembeli: string;
  kontak: string;
  items: PesananItem[];
  total: number;
  catatan: string;
  metode_bayar: string; // tunai | qris
  status: PesananStatus;
  created_at: number; // unix seconds
}

export interface KiosConfig {
  auto_learn_enabled: boolean;
  learn_model: string;
  notif_enabled: boolean;
  notif_jam: string; // "HH" (WITA)
  qris_enabled: boolean;
  qris_nama: string;
  qris_image_url: string;
  wa_number: string; // kios WhatsApp number (buyer contact / order confirmation)
  // Model AI utama untuk semua respons bot (format: "provider/model-id").
  // Kosong = ikuti routing default dari config.json.
  model_utama: string;
  // Pengaturan tampilan toko publik (/toko)
  nama_toko: string;
  deskripsi_toko: string;
  lokasi_toko: string;
  banner_image_url: string;
  jam_buka: string; // "HH:mm" WITA
  jam_tutup: string; // "HH:mm" WITA
}

export type Role = "owner" | "kasir";

export type Periode = "hari_ini" | "minggu" | "bulan";

export interface SessionUser {
  id: string; // Telegram user ID
  nama: string;
  username: string;
  photo_url: string;
  role: Role;
}
