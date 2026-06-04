// Package kios provides village-shop AI tools for picoclaw.
// Data persists in Upstash Redis (rediss:// URL from $UPSTASH_REDIS_URL).
package kios

import (
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Produk represents a product in the shop.
type Produk struct {
	ID          string `json:"id"`
	Barcode     string `json:"barcode"`
	Nama        string `json:"nama"`
	Kategori    string `json:"kategori"`
	Satuan      string `json:"satuan"`
	Stok        int    `json:"stok"`
	HargaBeli   int    `json:"harga_beli"`
	HargaJual   int    `json:"harga_jual"`
	StokMinimum int    `json:"stok_minimum"`
	StokKritis  int    `json:"stok_kritis"`
	Supplier    string `json:"supplier"`
	LastUpdate  string `json:"last_update"`
	HasExp      bool   `json:"has_exp"`
	ExpDate     string `json:"exp_date"`
	ImageURL    string `json:"image_url"`
	Jenis        string `json:"jenis,omitempty"`        // "" | "biasa" | "pulsa" | "bensin"
	SupplierID   string `json:"supplier_id,omitempty"`  // FK stabil ke Supplier.ID
	SaldoModal   int    `json:"saldo_modal,omitempty"`  // pulsa: saldo modal rupiah
	StokMl       int    `json:"stok_ml,omitempty"`      // bensin: stok mili-liter
	StokKritisMl int    `json:"stok_kritis_ml,omitempty"` // bensin: ambang kritis (default 40000 = 40L)
	PackDefs     []Kemasan `json:"pack_defs,omitempty"` // kemasan-kemasan restock yang dikenal
}

// JenisOrDefault returns the product kind, defaulting to "biasa" when unset so
// pre-existing products (which have no jenis field) behave as ordinary stock items.
func (p *Produk) JenisOrDefault() string {
	if p.Jenis == "" {
		return "biasa"
	}
	return p.Jenis
}

// Transaksi represents a sales transaction.
type Transaksi struct {
	ID          string `json:"id"`
	Tanggal     string `json:"tanggal"`
	Jam         string `json:"jam"`
	ProdukID    string `json:"produk_id"`
	NamaProduk  string `json:"nama_produk"`
	Kategori    string `json:"kategori"`
	Qty         int    `json:"qty"`
	HargaSatuan int    `json:"harga_satuan"`
	Total       int    `json:"total"`
	MetodeBayar string `json:"metode_bayar"`
	Kasir       string `json:"kasir"`
	Catatan     string `json:"catatan"`
	SessionID   string  `json:"session_id"`
	PiutangID   string  `json:"piutang_id,omitempty"` // diisi saat jual bon
	Modal       int     `json:"modal,omitempty"`      // modal dikunci saat jual → laba akurat & historis
	Liter       float64 `json:"liter,omitempty"`      // volume bensin yang terjual (display)
}

// Pembelian represents a restocking purchase.
type Pembelian struct {
	ID         string `json:"id"`
	SessionID  string `json:"session_id"`
	Tanggal    string `json:"tanggal"`
	Jam        string `json:"jam"`
	ProdukID   string `json:"produk_id"`
	NamaProduk string `json:"nama_produk"`
	Qty        int    `json:"qty"`
	HargaBeli  int    `json:"harga_beli"`
	Subtotal   int    `json:"subtotal"`
	Supplier   string `json:"supplier"`
	Kasir      string `json:"kasir"`
	Catatan    string `json:"catatan"`
	// Pack fields — diisi saat restock menggunakan satuan kemasan.
	Kemasan    string `json:"kemasan,omitempty"`    // mis. "dos", "lusin"
	Isi        int    `json:"isi,omitempty"`         // pcs per kemasan
	QtyPack    int    `json:"qty_pack,omitempty"`    // jumlah kemasan dibeli
	HargaPack  int    `json:"harga_pack,omitempty"`  // harga total satu kemasan (rupiah)
	SupplierID string `json:"supplier_id,omitempty"` // FK ke Supplier.ID
}

// PriceHistory represents a price change event.
type PriceHistory struct {
	ID         string `json:"id"`
	Tanggal    string `json:"tanggal"`
	Jam        string `json:"jam"`
	ProdukID   string `json:"produk_id"`
	NamaProduk string `json:"nama_produk"`
	HargaLama  int    `json:"harga_lama"`
	HargaBaru  int    `json:"harga_baru"`
	Selisih    int    `json:"selisih"`
	Supplier   string `json:"supplier"`
	Kasir      string `json:"kasir"`
}

// Shift represents the current cashier shift.
type Shift struct {
	Kasir      string `json:"kasir"`
	SaldoAwal  int    `json:"saldo_awal"`
	SaldoAkhir int    `json:"saldo_akhir"`
	WaktuBuka  string `json:"waktu_buka"`
	WaktuTutup string `json:"waktu_tutup"`
	Status     string `json:"status"` // "buka" | "tutup"
}

// UserKios represents an authorized shop user.
type UserKios struct {
	Phone       string `json:"phone"`
	Nama        string `json:"nama"`
	Role        string `json:"role"` // "kasir" | "owner"
	Aktif       bool   `json:"aktif"`
	Ditambahkan string `json:"ditambahkan"`
}

// PesananItem adalah satu baris item dalam pesanan pembeli.
type PesananItem struct {
	ProdukID    string `json:"produk_id"`
	NamaProduk  string `json:"nama_produk"`
	Qty         int    `json:"qty"`
	HargaSatuan int    `json:"harga_satuan"`
	Subtotal    int    `json:"subtotal"`
}

// Pesanan adalah pesanan dari halaman toko pembeli (dibuat oleh dashboard web).
type Pesanan struct {
	ID          string        `json:"id"`
	Tanggal     string        `json:"tanggal"`
	Jam         string        `json:"jam"`
	NamaPembeli string        `json:"nama_pembeli"`
	Kontak      string        `json:"kontak"`
	Items       []PesananItem `json:"items"`
	Total       int           `json:"total"`
	Catatan     string        `json:"catatan"`
	Status      string        `json:"status"` // pending | diproses | ditolak
	CreatedAt   int64         `json:"created_at"`
}

// Pelanggan adalah pembeli yang terdaftar dari storefront. Diidentifikasi oleh
// nomor WA yang dinormalisasi (format "62..."). Dipakai oleh piutang (bon) dan
// storefront checkout.
type Pelanggan struct {
	ID           string `json:"id"`    // "PLG-<phone>"
	Phone        string `json:"phone"` // no. WA ternormalisasi "62..." (= HASH field key)
	Nama         string `json:"nama"`
	TotalUtang   int    `json:"total_utang"`   // cache dari piutang terbuka; ditulis oleh bon ledger
	TotalPesanan int    `json:"total_pesanan"` // jumlah order seumur hidup
	TotalBelanja int    `json:"total_belanja"` // total rupiah seumur hidup
	Catatan      string `json:"catatan"`
	CreatedAt    int64  `json:"created_at"` // unix seconds
	LastOrder    string `json:"last_order"` // tanggal YYYY-MM-DD terakhir order
}

// Piutang adalah catatan kredit pembeli (pembeli ngutang ke kios).
type Piutang struct {
	ID          string `json:"id"` // "PIU-0001"
	PelangganID string `json:"pelanggan_id"`
	Phone       string `json:"phone"`                  // WA ternormalisasi
	TransaksiID string `json:"transaksi_id,omitempty"` // TRX-xxxx dari jual bon
	Pokok       int    `json:"pokok"`
	Dibayar     int    `json:"dibayar"`
	Sisa        int    `json:"sisa"`
	Status      string `json:"status"` // "terbuka" | "lunas" | "dihapus"
	Tanggal     string `json:"tanggal"`
	Jam         string `json:"jam"`
	Kasir       string `json:"kasir"`
	Catatan     string `json:"catatan"`
}

// Hutang adalah catatan kios berutang ke supplier.
type Hutang struct {
	ID          string `json:"id"` // "HUT-0001"
	SupplierID  string `json:"supplier_id"`
	PembelianID string `json:"pembelian_id,omitempty"` // PEM-xxxx dari restock
	Pokok       int    `json:"pokok"`
	Dibayar     int    `json:"dibayar"`
	Sisa        int    `json:"sisa"`
	Status      string `json:"status"` // "terbuka" | "lunas" | "dihapus"
	JatuhTempo  string `json:"jatuh_tempo,omitempty"`
	Tanggal     string `json:"tanggal"`
	Catatan     string `json:"catatan"`
}

// Pembayaran adalah satu event cicilan/lunas terhadap Piutang atau Hutang.
type Pembayaran struct {
	ID       string `json:"id"`        // "PAY-0001"
	LedgerID string `json:"ledger_id"` // PIU-xxxx atau HUT-xxxx
	Jenis    string `json:"jenis"`     // "piutang" | "hutang"
	Jumlah   int    `json:"jumlah"`
	Metode   string `json:"metode"` // tunai | transfer | qris
	Tanggal  string `json:"tanggal"`
	Jam      string `json:"jam"`
	Kasir    string `json:"kasir"`
	Catatan  string `json:"catatan"`
}

// Redis key constants.
const (
	keyProduk            = "kios:produk"
	keyTransaksi         = "kios:transaksi"
	keySeqTrx            = "kios:seq:trx"
	keyPembelian         = "kios:pembelian"
	keySeqPem            = "kios:seq:pem"
	keyPriceHist         = "kios:price_history"
	keySeqPhg            = "kios:seq:phg"
	keyShift             = "kios:shift"
	keyUsers             = "kios:users"
	keySeedDone          = "kios:seed:done"
	keySupplier          = "kios:supplier"
	keySeqSup            = "kios:seq:sup"
	keyHargaSupplier     = "kios:harga_supplier"
	keyPromo             = "kios:promo"
	keySeqPromo          = "kios:seq:promo"
	keyPustaka           = "kios:pustaka"
	keySeqPus            = "kios:seq:pus"
	keyPasarRef          = "kios:pasar:ref"
	keyLearnPat          = "kios:learn:patterns"
	keyLearnAls          = "kios:learn:aliases"
	keyLearnShc          = "kios:learn:shortcuts"
	keyLearnHab          = "kios:learn:habits"
	keyLearnUnk          = "kios:learn:unknowns"
	keyKiosConfig        = "kios:config"
	keyNotifLastDate     = "kios:notif:last_date"
	keyLoginPrefix       = "kios:login:" // one-time dashboard login codes
	keyPesanan           = "kios:pesanan"
	keyNotifPesananLast  = "kios:notif:pesanan_last"  // highest PSN seq notified to owners
	keyNotifPendingState = "kios:notif:pending_state" // "alerted" | "clear"
	keyPelanggan         = "kios:pelanggan"           // HASH: field = no. WA ternormalisasi; value = Pelanggan JSON
	keyPiutang           = "kios:piutang"
	keyHutang            = "kios:hutang"
	keyPembayaran        = "kios:pembayaran"
	keySeqPiu            = "kios:seq:piu"
	keySeqHut            = "kios:seq:hut"
	keySeqPay            = "kios:seq:pay"
	keyPulsaDenom        = "kios:pulsa:denom"         // HASH field=nominal string, value=PulsaDenom JSON
	keyPulsaTopup        = "kios:pulsa:topup"         // LIST append-only, value=PulsaTopup JSON
	keySeqPtu            = "kios:seq:ptu"             // INCR counter untuk PTU-NNNN
	keyHargaSupplierLast = "kios:harga_supplier_last" // HASH: field=produkID|supplierID, value=HargaSupplierLast JSON
)

// loginCodeTTL is how long a /login code stays valid.
const loginCodeTTL = 5 * time.Minute

// KiosConfig menyimpan preferensi konfigurasi kios yang dapat diubah oleh owner.
type KiosConfig struct {
	// AutoLearnEnabled mengaktifkan/menonaktifkan fitur belajar otomatis.
	// Default true — alias, shortcut, pola, dan kebiasaan dicatat otomatis dari interaksi.
	AutoLearnEnabled bool `json:"auto_learn_enabled"`
	// LearnModel adalah nama model AI yang dipilih owner untuk tugas pelatihan dan pembelajaran.
	// Kosong berarti ikuti routing default.
	LearnModel string `json:"learn_model"`
	// NotifEnabled mengaktifkan notifikasi stok menipis otomatis ke owner. Default true.
	NotifEnabled bool `json:"notif_enabled"`
	// NotifJam adalah jam WITA (format "HH") saat notif dikirim setiap hari. Default "07".
	NotifJam string `json:"notif_jam"`
	// QrisEnabled menampilkan opsi pembayaran QRIS di toko & perintah /qris. Default false.
	QrisEnabled bool `json:"qris_enabled"`
	// QrisNama adalah nama merchant yang ditampilkan saat pembayaran QRIS (mis. "Kios Cerdas").
	QrisNama string `json:"qris_nama"`
	// QrisImageURL adalah URL gambar QR statis QRIS kios untuk dipindai pembeli.
	QrisImageURL string `json:"qris_image_url"`
	// WaNumber adalah nomor WhatsApp kios (untuk konfirmasi pesanan & kontak pembeli).
	WaNumber string `json:"wa_number"`
}

// PulsaDenom menyimpan konfigurasi harga satu nominal pulsa.
type PulsaDenom struct {
	Nominal    int  `json:"nominal"`
	HargaModal int  `json:"harga_modal"`
	HargaJual  int  `json:"harga_jual"`
	Aktif      bool `json:"aktif"`
}

// Margin mengembalikan selisih harga jual − modal per denom.
func (d *PulsaDenom) Margin() int { return d.HargaJual - d.HargaModal }

// PulsaTopup mencatat satu event top-up saldo modal pulsa (append-only).
type PulsaTopup struct {
	ID           string `json:"id"`
	Tanggal      string `json:"tanggal"`
	Jam          string `json:"jam"`
	Jumlah       int    `json:"jumlah"`
	SaldoSesudah int    `json:"saldo_sesudah"`
	Kasir        string `json:"kasir"`
	Catatan      string `json:"catatan"`
}

// Store holds the Redis client used by all kios data-access methods
// (defined in store_access.go and the other store_*.go files).
type Store struct {
	rdb *redis.Client
	mu  sync.Mutex
}
