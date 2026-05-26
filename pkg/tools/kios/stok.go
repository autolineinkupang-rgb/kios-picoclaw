package kios

import (
	"context"
	"fmt"
	"math"
	"strings"

	tools "github.com/sipeed/picoclaw/pkg/tools"
)

// StokTool manages products, sales, and restocking.
type StokTool struct{ store *Store }

func (t *StokTool) Name() string { return "kios_stok" }

func (t *StokTool) Description() string {
	return "Kelola stok kios: cek daftar stok, cari produk, jual barang, tambah/restock, " +
		"daftar produk baru, hapus, set stok, atur tanggal kedaluwarsa, batalkan transaksi, " +
		"dan lihat produk yang menipis. Gunakan action sesuai kebutuhan."
}

func (t *StokTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type": "string",
				"enum": []string{"cek", "cari", "jual", "tambah", "tambah_produk",
					"hapus", "set_stok", "update_exp", "batalkan_tx", "stok_menipis"},
				"description": "Aksi yang dijalankan.",
			},
			"produk":      map[string]any{"type": "string", "description": "id atau nama produk"},
			"qty":         map[string]any{"type": "integer", "description": "jumlah (jual/restock)"},
			"metode":      map[string]any{"type": "string", "enum": []string{"tunai", "transfer", "qris"}, "description": "metode bayar saat jual"},
			"harga":       map[string]any{"type": "integer", "description": "harga beli (restock)"},
			"supplier":    map[string]any{"type": "string"},
			"auto_create": map[string]any{"type": "boolean", "description": "buat produk otomatis saat restock bila belum ada"},
			"nama":        map[string]any{"type": "string", "description": "nama produk baru (tambah_produk)"},
			"kategori":    map[string]any{"type": "string"},
			"satuan":      map[string]any{"type": "string"},
			"harga_beli":  map[string]any{"type": "integer"},
			"harga_jual":  map[string]any{"type": "integer"},
			"stok_awal":   map[string]any{"type": "integer"},
			"stok_baru":   map[string]any{"type": "integer", "description": "nilai stok absolut (set_stok)"},
			"exp_date":    map[string]any{"type": "string", "description": "tanggal kedaluwarsa YYYY-MM-DD"},
			"id":          map[string]any{"type": "string", "description": "id transaksi (batalkan_tx)"},
		},
		"required": []string{"action"},
	}
}

func (t *StokTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	role, kasir, refusal := resolveRole(ctx, t.store)
	if refusal != nil {
		return refusal
	}
	switch argStr(args, "action") {
	case "cek":
		return t.cek(ctx)
	case "cari":
		return t.cari(ctx, args)
	case "jual":
		return t.jual(ctx, args, kasir)
	case "tambah":
		return t.tambah(ctx, args, kasir)
	case "tambah_produk":
		if r := requireOwner(role); r != nil {
			return r
		}
		return t.tambahProduk(ctx, args)
	case "hapus":
		if r := requireOwner(role); r != nil {
			return r
		}
		return t.hapus(ctx, args)
	case "set_stok":
		if r := requireOwner(role); r != nil {
			return r
		}
		return t.setStok(ctx, args)
	case "update_exp":
		if r := requireOwner(role); r != nil {
			return r
		}
		return t.updateExp(ctx, args)
	case "batalkan_tx":
		if r := requireOwner(role); r != nil {
			return r
		}
		return t.batalkanTx(ctx, args)
	case "stok_menipis":
		return t.stokMenipis(ctx)
	default:
		return tools.ErrorResult("Aksi stok tidak dikenal.")
	}
}

func (t *StokTool) cek(ctx context.Context) *tools.ToolResult {
	all, err := t.store.GetAllProduk(ctx)
	if err != nil {
		return tools.ErrorResult("Gagal baca stok.").WithError(err)
	}
	if len(all) == 0 {
		return tools.NewToolResult("Belum ada produk di stok.")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Daftar stok (%d produk):\n", len(all))
	for _, p := range all {
		fmt.Fprintf(&b, "- [%s] %s: %d %s @ %s\n", p.ID, p.Nama, p.Stok, p.Satuan, FormatRupiah(p.HargaJual))
	}
	return tools.NewToolResult(b.String())
}

func (t *StokTool) cari(ctx context.Context, args map[string]any) *tools.ToolResult {
	item, err := findOne(ctx, t.store, argStr(args, "produk"))
	if err != nil {
		return tools.ErrorResult("Gagal cari produk.").WithError(err)
	}
	if item == nil {
		return tools.NewToolResult("Produk tidak ditemukan.")
	}
	return tools.NewToolResult(fmt.Sprintf("[%s] %s — stok %d %s, jual %s, beli %s, supplier %s",
		item.ID, item.Nama, item.Stok, item.Satuan,
		FormatRupiah(item.HargaJual), FormatRupiah(item.HargaBeli), item.Supplier))
}

func (t *StokTool) jual(ctx context.Context, args map[string]any, kasir string) *tools.ToolResult {
	tx, item, sisa, err := performJual(ctx, t.store, argStr(args, "produk"), argInt(args, "qty"), argStr(args, "metode"), kasir)
	if err != nil {
		return tools.ErrorResult(err.Error())
	}
	msg := fmt.Sprintf("Terjual %d %s %s = %s (%s). Sisa stok: %d. %s",
		tx.Qty, item.Satuan, item.Nama, FormatRupiah(tx.Total), tx.MetodeBayar, sisa, tx.ID)
	if sisa <= 0 {
		msg += fmt.Sprintf("\n⚠️ %s HABIS!", item.Nama)
	} else if sisa <= item.StokKritis {
		msg += fmt.Sprintf("\n⚠️ Stok %s menipis (sisa %d, kritis %d).", item.Nama, sisa, item.StokKritis)
	}
	return tools.NewToolResult(msg)
}

func (t *StokTool) tambah(ctx context.Context, args map[string]any, kasir string) *tools.ToolResult {
	nama := argStr(args, "produk")
	qty := argInt(args, "qty")
	hargaBeli := argInt(args, "harga")
	supplier := argStr(args, "supplier")
	if qty <= 0 {
		return tools.ErrorResult("Jumlah restock harus lebih dari 0.")
	}
	if hargaBeli < 0 {
		return tools.ErrorResult("Harga beli tidak boleh negatif.")
	}
	item, err := findOne(ctx, t.store, nama)
	if err != nil {
		return tools.ErrorResult("Gagal cari produk.").WithError(err)
	}
	now := NowWITA().Format("2006-01-02")

	if item == nil {
		if !argBool(args, "auto_create") {
			return tools.ErrorResult(fmt.Sprintf("Produk %q belum terdaftar. Daftarkan dulu atau set auto_create=true.", nama))
		}
		id, err := t.store.NextProdukID(ctx)
		if err != nil {
			return tools.ErrorResult("Gagal buat id produk.").WithError(err)
		}
		hargaJual := 0
		if hargaBeli > 0 {
			hargaJual = int(math.Round(float64(hargaBeli) * 1.15))
		}
		item = &Produk{
			ID: id, Nama: strings.TrimSpace(nama), Kategori: "umum", Satuan: "pcs",
			Stok: qty, HargaBeli: hargaBeli, HargaJual: hargaJual,
			StokMinimum: 5, StokKritis: 2, Supplier: supplier, LastUpdate: now,
		}
		if err := t.store.SetProduk(ctx, item); err != nil {
			return tools.ErrorResult("Gagal simpan produk baru.").WithError(err)
		}
		t.recordPembelian(ctx, item, qty, hargaBeli, supplier, kasir, "auto-create")
		return tools.NewToolResult(fmt.Sprintf("Produk baru dibuat: [%s] %s, stok %d, harga jual %s (margin 15%%).",
			item.ID, item.Nama, item.Stok, FormatRupiah(hargaJual)))
	}

	hargaLama := item.HargaBeli
	priceChanged := hargaBeli > 0 && hargaBeli != hargaLama
	if priceChanged {
		t.store.AppendPriceHistory(ctx, &PriceHistory{
			Tanggal: now, Jam: NowWITA().Format("15:04:05"),
			ProdukID: item.ID, NamaProduk: item.Nama,
			HargaLama: hargaLama, HargaBaru: hargaBeli, Selisih: hargaBeli - hargaLama,
			Supplier: supplier, Kasir: kasir,
		})
	}
	item.Stok += qty
	item.LastUpdate = now
	if hargaBeli > 0 {
		item.HargaBeli = hargaBeli
	}
	if supplier != "" {
		item.Supplier = supplier
	}
	if err := t.store.SetProduk(ctx, item); err != nil {
		return tools.ErrorResult("Gagal update stok.").WithError(err)
	}
	t.recordPembelian(ctx, item, qty, hargaBeli, item.Supplier, kasir, "")
	msg := fmt.Sprintf("Restock %s +%d, stok jadi %d.", item.Nama, qty, item.Stok)
	if priceChanged {
		msg += fmt.Sprintf(" Harga beli berubah %s → %s.", FormatRupiah(hargaLama), FormatRupiah(hargaBeli))
	}
	return tools.NewToolResult(msg)
}

func (t *StokTool) recordPembelian(ctx context.Context, item *Produk, qty, hargaBeli int, supplier, kasir, catatan string) {
	now := NowWITA()
	t.store.AppendPembelian(ctx, &Pembelian{
		Tanggal: now.Format("2006-01-02"), Jam: now.Format("15:04:05"),
		ProdukID: item.ID, NamaProduk: item.Nama, Qty: qty, HargaBeli: hargaBeli,
		Subtotal: qty * hargaBeli, Supplier: supplier, Kasir: kasir, Catatan: catatan,
	})
}

func (t *StokTool) tambahProduk(ctx context.Context, args map[string]any) *tools.ToolResult {
	nama := argStr(args, "nama")
	if nama == "" {
		nama = argStr(args, "produk")
	}
	if nama == "" {
		return tools.ErrorResult("Nama produk wajib diisi.")
	}
	hargaJual := argInt(args, "harga_jual")
	if hargaJual <= 0 {
		return tools.ErrorResult("Harga jual wajib diisi.")
	}
	stokAwal := argInt(args, "stok_awal")
	if stokAwal < 0 {
		return tools.ErrorResult("Stok awal tidak boleh negatif.")
	}
	id, err := t.store.NextProdukID(ctx)
	if err != nil {
		return tools.ErrorResult("Gagal buat id produk.").WithError(err)
	}
	kategori := argStr(args, "kategori")
	if kategori == "" {
		kategori = "umum"
	}
	satuan := argStr(args, "satuan")
	if satuan == "" {
		satuan = "pcs"
	}
	exp := argStr(args, "exp_date")
	p := &Produk{
		ID: id, Nama: nama, Kategori: kategori, Satuan: satuan,
		Stok: stokAwal, HargaBeli: argInt(args, "harga_beli"), HargaJual: hargaJual,
		StokMinimum: 5, StokKritis: 2, Supplier: argStr(args, "supplier"),
		LastUpdate: NowWITA().Format("2006-01-02"), HasExp: exp != "", ExpDate: exp,
	}
	if err := t.store.SetProduk(ctx, p); err != nil {
		return tools.ErrorResult("Gagal simpan produk.").WithError(err)
	}
	return tools.NewToolResult(fmt.Sprintf("Produk terdaftar: [%s] %s, stok %d %s, jual %s.",
		p.ID, p.Nama, p.Stok, p.Satuan, FormatRupiah(p.HargaJual)))
}

func (t *StokTool) hapus(ctx context.Context, args map[string]any) *tools.ToolResult {
	item, err := findOne(ctx, t.store, argStr(args, "produk"))
	if err != nil {
		return tools.ErrorResult("Gagal cari produk.").WithError(err)
	}
	if item == nil {
		return tools.NewToolResult("Produk tidak ditemukan.")
	}
	if err := t.store.DelProduk(ctx, item.ID); err != nil {
		return tools.ErrorResult("Gagal hapus produk.").WithError(err)
	}
	return tools.NewToolResult(fmt.Sprintf("Produk %s ([%s]) dihapus.", item.Nama, item.ID))
}

func (t *StokTool) setStok(ctx context.Context, args map[string]any) *tools.ToolResult {
	baru := argInt(args, "stok_baru")
	if baru < 0 {
		return tools.ErrorResult("Stok baru tidak boleh negatif.")
	}
	item, err := findOne(ctx, t.store, argStr(args, "produk"))
	if err != nil {
		return tools.ErrorResult("Gagal cari produk.").WithError(err)
	}
	if item == nil {
		return tools.NewToolResult("Produk tidak ditemukan.")
	}
	lama := item.Stok
	item.Stok = baru
	item.LastUpdate = NowWITA().Format("2006-01-02")
	if err := t.store.SetProduk(ctx, item); err != nil {
		return tools.ErrorResult("Gagal set stok.").WithError(err)
	}
	return tools.NewToolResult(fmt.Sprintf("Stok %s: %d → %d.", item.Nama, lama, baru))
}

func (t *StokTool) updateExp(ctx context.Context, args map[string]any) *tools.ToolResult {
	exp := argStr(args, "exp_date")
	if exp == "" {
		return tools.ErrorResult("exp_date wajib diisi.")
	}
	item, err := findOne(ctx, t.store, argStr(args, "produk"))
	if err != nil {
		return tools.ErrorResult("Gagal cari produk.").WithError(err)
	}
	if item == nil {
		return tools.NewToolResult("Produk tidak ditemukan.")
	}
	item.HasExp = true
	item.ExpDate = exp
	item.LastUpdate = NowWITA().Format("2006-01-02")
	if err := t.store.SetProduk(ctx, item); err != nil {
		return tools.ErrorResult("Gagal update exp.").WithError(err)
	}
	return tools.NewToolResult(fmt.Sprintf("Tanggal kedaluwarsa %s = %s.", item.Nama, exp))
}

func (t *StokTool) batalkanTx(ctx context.Context, args map[string]any) *tools.ToolResult {
	id := strings.ToUpper(argStr(args, "id"))
	if id == "" {
		return tools.ErrorResult("ID transaksi wajib diisi.")
	}
	tx, err := t.store.RemoveTransaksi(ctx, id)
	if err != nil {
		return tools.ErrorResult("Gagal batalkan transaksi.").WithError(err)
	}
	if tx == nil {
		return tools.NewToolResult(fmt.Sprintf("Transaksi %s tidak ditemukan.", id))
	}
	if item, _ := t.store.GetProduk(ctx, tx.ProdukID); item != nil {
		item.Stok += tx.Qty
		item.LastUpdate = NowWITA().Format("2006-01-02")
		t.store.SetProduk(ctx, item)
	}
	return tools.NewToolResult(fmt.Sprintf("Transaksi %s dibatalkan, stok %s dikembalikan (+%d).", tx.ID, tx.NamaProduk, tx.Qty))
}

func (t *StokTool) stokMenipis(ctx context.Context) *tools.ToolResult {
	all, err := t.store.GetAllProduk(ctx)
	if err != nil {
		return tools.ErrorResult("Gagal baca stok.").WithError(err)
	}
	var b strings.Builder
	count := 0
	for _, p := range all {
		if p.Stok <= p.StokMinimum {
			butuh := p.StokMinimum*3 - p.Stok
			if butuh < 0 {
				butuh = 0
			}
			fmt.Fprintf(&b, "- %s: sisa %d (min %d), perlu restock ±%d\n", p.Nama, p.Stok, p.StokMinimum, butuh)
			count++
		}
	}
	if count == 0 {
		return tools.NewToolResult("Semua stok aman, tidak ada yang menipis.")
	}
	return tools.NewToolResult(fmt.Sprintf("%d produk menipis:\n%s", count, b.String()))
}
