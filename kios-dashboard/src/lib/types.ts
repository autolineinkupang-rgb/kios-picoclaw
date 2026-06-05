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
  jenis?: string;        // "" | "biasa" | "pulsa" | "bensin" | "solar" | "minyak_tanah"
  supplier_id?: string;
  saldo_modal?: number;
  saldo_minimum?: number; // alert threshold Rp untuk pulsa
  stok_ml?: number;
  stok_kritis_ml?: number;
  pack_defs?: Kemasan[];
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
  piutang_id?: string;  // diisi saat jual bon
  modal?: number;
  liter?: number;
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
  kemasan?: string;
  isi?: number;
  qty_pack?: number;
  harga_pack?: number;
  supplier_id?: string;
}

export interface Kemasan {
  nama: string;
  isi: number;
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

export type PesananStatus = "pending" | "diproses" | "ditolak";

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
  pelanggan_id?: string; // FK ke Pelanggan.ID; undefined = pembeli anonim (data lama)
}

export interface Pelanggan {
  id: string;            // "PLG-<phone>"
  phone: string;         // no. WA ternormalisasi "62..." (= HASH field key)
  nama: string;
  total_utang: number;   // cache dari piutang terbuka; ditulis oleh bon ledger
  total_pesanan: number;
  total_belanja: number;
  catatan: string;
  created_at: number;    // unix seconds
  last_order: string;    // "YYYY-MM-DD"
}

export type StatusBon = "terbuka" | "lunas" | "dihapus";

export interface Piutang {
  id: string;
  pelanggan_id: string;
  phone: string;
  transaksi_id?: string;
  pokok: number;
  dibayar: number;
  sisa: number;
  status: StatusBon;
  tanggal: string;
  jam: string;
  kasir: string;
  catatan: string;
}

export interface Hutang {
  id: string;
  supplier_id: string;
  pembelian_id?: string;
  pokok: number;
  dibayar: number;
  sisa: number;
  status: StatusBon;
  jatuh_tempo?: string;
  tanggal: string;
  catatan: string;
}

export interface Pembayaran {
  id: string;
  ledger_id: string;
  jenis: "piutang" | "hutang";
  jumlah: number;
  metode: string;
  tanggal: string;
  jam: string;
  kasir: string;
  catatan: string;
}

export interface KiosConfig {
  auto_learn_enabled: boolean;
  learn_model: string;
  notif_enabled: boolean;
  notif_jam: string; // "HH" (WITA)
  notif_pesanan_enabled?: boolean; // default true
  notif_piutang_enabled?: boolean; // default true
  qris_enabled: boolean;
  qris_nama: string;
  qris_image_url: string;
  wa_number: string; // kios WhatsApp number (buyer contact / order confirmation)
}

export interface Penarikan {
  id: string;        // "PRK-0001"
  tanggal: string;   // YYYY-MM-DD WITA
  jam: string;       // HH:mm:ss WITA
  jumlah: number;    // Rp ditarik
  produk_id: string; // produk yang di-topup
  produk_nama: string;
  kasir: string;
  catatan: string;
}

export interface PulsaDenom {
  nominal: number;
  harga_modal: number;
  harga_jual: number;
  aktif: boolean;
}

export interface PulsaTopup {
  id: string;
  tanggal: string;
  jam: string;
  jumlah: number;
  saldo_sesudah: number;
  kasir: string;
  catatan: string;
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
