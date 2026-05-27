package commands

import "context"

const startMessage = `Halo kak! Saya Kios Cerdas 🛒

Perintah cepat (tanpa AI):
/stok — cek daftar & cari produk
/harga <produk> — cek harga satuan
/jual <produk> <jml> — jual + struk
/jualmassal — jual banyak barang sekaligus
/laporan — ringkasan penjualan hari ini
/menipis — stok hampir habis
/shift — status shift kasir
/promo — daftar promo aktif
/pasar [produk] — harga kita vs pasar
/produk — daftar semua produk
/suplier — daftar supplier
/template — download template Excel
/backup — export data (khusus owner)
/bantuan — panduan lengkap

Atau ketik pesan biasa untuk tanya ke AI! 💬`

func startCommand() Definition {
	return Definition{
		Name:        "start",
		Description: "Mulai bot & lihat daftar perintah",
		Usage:       "/start",
		Handler: func(_ context.Context, req Request, _ *Runtime) error {
			return req.Reply(startMessage)
		},
	}
}
