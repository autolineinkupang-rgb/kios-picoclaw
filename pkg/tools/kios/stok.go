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
				"enum": []string{
					"cek", "cari", "jual", "tambah", "tambah_produk", "edit_produk",
					"tambah_massal", "edit_massal", "hapus", "set_stok", "update_exp", "batalkan_tx", "stok_menipis",
				},
				"description": "Aksi yang dijalankan.",
			},
			"items": map[string]any{
				"type":        "array",
				"description": "daftar item untuk tambah_massal/edit_massal (tiap item berisi field yang sama seperti tambah/edit_produk)",
				"items":       map[string]any{"type": "object"},
			},
			"produk": map[string]any{"type": "string", "description": "id atau nama produk"},
			"qty":    map[string]any{"type": "integer", "description": "jumlah (jual/restock)"},
			"metode": map[string]any{
				"type":        "string",
				"enum":        []string{"tunai", "transfer", "qris"},
				"description": "metode bayar saat jual",
			},
			"harga":    map[string]any{"type": "integer", "description": "harga beli (restock)"},
			"supplier": map[string]any{"type": "string"},
			"auto_create": map[string]any{
				"type":        "boolean",
				"description": "buat produk otomatis saat restock bila belum ada",
			},
			"nama": map[string]any{
				"type":        "string",
				"description": "nama produk (tambah_produk) / nama baru (edit_produk)",
			},
			"barcode": map[string]any{
				"type":        "string",
				"description": "kode barcode produk (opsional, untuk scan)",
			},
			"kategori":     map[string]any{"type": "string"},
			"satuan":       map[string]any{"type": "string"},
			"harga_beli":   map[string]any{"type": "integer"},
			"harga_jual":   map[string]any{"type": "integer"},
			"stok_awal":    map[string]any{"type": "integer"},
			"stok_baru":    map[string]any{"type": "integer", "description": "nilai stok absolut (set_stok)"},
			"stok_minimum": map[string]any{"type": "integer"},
			"stok_kritis":  map[string]any{"type": "integer"},
			"exp_date":     map[string]any{"type": "string", "description": "tanggal kedaluwarsa YYYY-MM-DD"},
			"id":           map[string]any{"type": "string", "description": "id transaksi (batalkan_tx)"},
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
	case "edit_produk":
		if r := requireOwner(role); r != nil {
			return r
		}
		return t.editProduk(ctx, args)
	case "tambah_massal":
		return t.tambahMassal(ctx, args, kasir)
	case "edit_massal":
		if r := requireOwner(role); r != nil {
			return r
		}
		return t.editMassal(ctx, args)
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
		return tools.ErrorResult("Hmm, aksi stok belum dikenal kak 🤔")
	}
}

func (t *StokTool) cek(ctx context.Context) *tools.ToolResult {
	all, err := t.store.GetAllProduk(ctx)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal baca data stok kak 😣 Coba lagi sebentar ya.").WithError(err)
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
		return tools.ErrorResult("Aduh, ada kendala pas cari produk kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	if item == nil {
		return tools.NewToolResult("Hmm, produknya nggak ketemu kak 🔍 Coba ketik /stok buat lihat daftar produk ya.")
	}
	msg := fmt.Sprintf("[%s] %s — stok %d %s, jual %s, beli %s, supplier %s",
		item.ID, item.Nama, item.Stok, item.Satuan,
		FormatRupiah(item.HargaJual), FormatRupiah(item.HargaBeli), item.Supplier)
	if item.Barcode != "" {
		msg += "\nBarcode: " + item.Barcode
	}
	return tools.NewToolResult(msg)
}

func (t *StokTool) jual(ctx context.Context, args map[string]any, kasir string) *tools.ToolResult {
	tx, item, sisa, err := performJual(
		ctx,
		t.store,
		argStr(args, "produk"),
		argInt(args, "qty"),
		argStr(args, "metode"),
		kasir,
		0,
		nil,
	)
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
		return tools.ErrorResult("Jumlah restock-nya harus lebih dari 0 ya kak 🙏")
	}
	if hargaBeli < 0 {
		return tools.ErrorResult("Harga beli nggak boleh minus ya kak 😊")
	}
	item, err := findOne(ctx, t.store, nama)
	if err != nil {
		return tools.ErrorResult("Aduh, ada kendala pas cari produk kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	now := NowWITA().Format("2006-01-02")

	if item == nil {
		if !argBool(args, "auto_create") {
			return tools.ErrorResult(
				fmt.Sprintf(
					"Produk \"%s\" belum terdaftar kak 😊 Daftarkan dulu ya, atau set auto_create kalau mau langsung dibuat.",
					nama,
				),
			)
		}
		id, err := t.store.NextProdukID(ctx)
		if err != nil {
			return tools.ErrorResult("Aduh, ada kendala bikin id produk kak 😣 Coba lagi ya.").WithError(err)
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
			return tools.ErrorResult("Aduh, gagal simpan produk baru kak 😣 Coba lagi sebentar ya.").WithError(err)
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
		return tools.ErrorResult("Aduh, gagal update stok kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	t.recordPembelian(ctx, item, qty, hargaBeli, item.Supplier, kasir, "")
	msg := fmt.Sprintf("Restock %s +%d, stok jadi %d.", item.Nama, qty, item.Stok)
	if priceChanged {
		msg += fmt.Sprintf(" Harga beli berubah %s → %s.", FormatRupiah(hargaLama), FormatRupiah(hargaBeli))
	}
	return tools.NewToolResult(msg)
}

func (t *StokTool) recordPembelian(
	ctx context.Context,
	item *Produk,
	qty, hargaBeli int,
	supplier, kasir, catatan string,
) {
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
		return tools.ErrorResult("Nama produknya diisi dulu ya kak 🙏")
	}
	hargaJual := argInt(args, "harga_jual")
	if hargaJual <= 0 {
		return tools.ErrorResult("Harga jualnya diisi dulu ya kak 😊")
	}
	stokAwal := argInt(args, "stok_awal")
	if stokAwal < 0 {
		return tools.ErrorResult("Stok awal nggak boleh minus ya kak.")
	}
	id, err := t.store.NextProdukID(ctx)
	if err != nil {
		return tools.ErrorResult("Aduh, ada kendala bikin id produk kak 😣 Coba lagi ya.").WithError(err)
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
		ID: id, Barcode: argStr(args, "barcode"), Nama: nama, Kategori: kategori, Satuan: satuan,
		Stok: stokAwal, HargaBeli: argInt(args, "harga_beli"), HargaJual: hargaJual,
		StokMinimum: 5, StokKritis: 2, Supplier: argStr(args, "supplier"),
		LastUpdate: NowWITA().Format("2006-01-02"), HasExp: exp != "", ExpDate: exp,
	}
	if err := t.store.SetProduk(ctx, p); err != nil {
		return tools.ErrorResult("Aduh, gagal simpan produk kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	return tools.NewToolResult(fmt.Sprintf("Produk terdaftar: [%s] %s, stok %d %s, jual %s.",
		p.ID, p.Nama, p.Stok, p.Satuan, FormatRupiah(p.HargaJual)))
}

func (t *StokTool) editProduk(ctx context.Context, args map[string]any) *tools.ToolResult {
	item, err := findOne(ctx, t.store, argStr(args, "produk"))
	if err != nil {
		return tools.ErrorResult("Aduh, ada kendala pas cari produk kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	if item == nil {
		return tools.NewToolResult("Hmm, produknya nggak ketemu kak 🔍 Coba ketik /stok buat lihat daftar produk ya.")
	}
	var changed []string
	if v := argStr(args, "nama"); v != "" {
		item.Nama = v
		changed = append(changed, "nama")
	}
	if v := argStr(args, "barcode"); v != "" {
		item.Barcode = v
		changed = append(changed, "barcode")
	}
	if v := argStr(args, "kategori"); v != "" {
		item.Kategori = v
		changed = append(changed, "kategori")
	}
	if v := argStr(args, "satuan"); v != "" {
		item.Satuan = v
		changed = append(changed, "satuan")
	}
	if v := argStr(args, "supplier"); v != "" {
		item.Supplier = v
		changed = append(changed, "supplier")
	}
	if p := argIntPtr(args, "harga_jual"); p != nil && *p > 0 {
		item.HargaJual = *p
		changed = append(changed, "harga_jual")
	}
	if p := argIntPtr(args, "harga_beli"); p != nil && *p >= 0 {
		item.HargaBeli = *p
		changed = append(changed, "harga_beli")
	}
	if p := argIntPtr(args, "stok_minimum"); p != nil && *p >= 0 {
		item.StokMinimum = *p
		changed = append(changed, "stok_minimum")
	}
	if p := argIntPtr(args, "stok_kritis"); p != nil && *p >= 0 {
		item.StokKritis = *p
		changed = append(changed, "stok_kritis")
	}
	if len(changed) == 0 {
		return tools.ErrorResult(
			"Tidak ada field yang diubah. Sebutkan nama/kategori/satuan/supplier/harga_jual/harga_beli/stok_minimum/stok_kritis.",
		)
	}
	item.LastUpdate = NowWITA().Format("2006-01-02")
	if err := t.store.SetProduk(ctx, item); err != nil {
		return tools.ErrorResult("Aduh, gagal simpan perubahan kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	return tools.NewToolResult(
		fmt.Sprintf("Produk %s ([%s]) diperbarui: %s.", item.Nama, item.ID, strings.Join(changed, ", ")),
	)
}

func (t *StokTool) tambahMassal(ctx context.Context, args map[string]any, kasir string) *tools.ToolResult {
	items := argItems(args)
	if len(items) == 0 {
		return tools.ErrorResult("items wajib diisi (daftar produk + qty [+ harga]).")
	}
	var b strings.Builder
	ok := 0
	for _, it := range items {
		if _, has := it["auto_create"]; !has {
			it["auto_create"] = true // bulk restock auto-creates unknown products by default
		}
		res := t.tambah(ctx, it, kasir)
		fmt.Fprintf(&b, "- %s\n", res.ForLLM)
		if !res.IsError {
			ok++
		}
	}
	return tools.NewToolResult(fmt.Sprintf("Tambah massal: %d/%d berhasil.\n%s", ok, len(items), b.String()))
}

func (t *StokTool) editMassal(ctx context.Context, args map[string]any) *tools.ToolResult {
	items := argItems(args)
	if len(items) == 0 {
		return tools.ErrorResult("items wajib diisi (tiap item: produk + field yang diubah).")
	}
	var b strings.Builder
	ok := 0
	for _, it := range items {
		res := t.editProduk(ctx, it)
		fmt.Fprintf(&b, "- %s\n", res.ForLLM)
		if !res.IsError {
			ok++
		}
	}
	return tools.NewToolResult(fmt.Sprintf("Edit massal: %d/%d berhasil.\n%s", ok, len(items), b.String()))
}

func (t *StokTool) hapus(ctx context.Context, args map[string]any) *tools.ToolResult {
	item, err := findOne(ctx, t.store, argStr(args, "produk"))
	if err != nil {
		return tools.ErrorResult("Aduh, ada kendala pas cari produk kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	if item == nil {
		return tools.NewToolResult("Hmm, produknya nggak ketemu kak 🔍 Coba ketik /stok buat lihat daftar produk ya.")
	}
	if err := t.store.DelProduk(ctx, item.ID); err != nil {
		return tools.ErrorResult("Aduh, gagal hapus produk kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	return tools.NewToolResult(fmt.Sprintf("Produk %s ([%s]) dihapus.", item.Nama, item.ID))
}

func (t *StokTool) setStok(ctx context.Context, args map[string]any) *tools.ToolResult {
	baru := argInt(args, "stok_baru")
	if baru < 0 {
		return tools.ErrorResult("Stok baru nggak boleh minus ya kak 😊")
	}
	item, err := findOne(ctx, t.store, argStr(args, "produk"))
	if err != nil {
		return tools.ErrorResult("Aduh, ada kendala pas cari produk kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	if item == nil {
		return tools.NewToolResult("Hmm, produknya nggak ketemu kak 🔍 Coba ketik /stok buat lihat daftar produk ya.")
	}
	lama := item.Stok
	item.Stok = baru
	item.LastUpdate = NowWITA().Format("2006-01-02")
	if err := t.store.SetProduk(ctx, item); err != nil {
		return tools.ErrorResult("Aduh, gagal set stok kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	return tools.NewToolResult(fmt.Sprintf("Stok %s: %d → %d.", item.Nama, lama, baru))
}

func (t *StokTool) updateExp(ctx context.Context, args map[string]any) *tools.ToolResult {
	exp := argStr(args, "exp_date")
	if exp == "" {
		return tools.ErrorResult("exp_date-nya diisi dulu ya kak 🙏")
	}
	item, err := findOne(ctx, t.store, argStr(args, "produk"))
	if err != nil {
		return tools.ErrorResult("Aduh, ada kendala pas cari produk kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	if item == nil {
		return tools.NewToolResult("Hmm, produknya nggak ketemu kak 🔍 Coba ketik /stok buat lihat daftar produk ya.")
	}
	item.HasExp = true
	item.ExpDate = exp
	item.LastUpdate = NowWITA().Format("2006-01-02")
	if err := t.store.SetProduk(ctx, item); err != nil {
		return tools.ErrorResult("Aduh, gagal update exp kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	return tools.NewToolResult(fmt.Sprintf("Tanggal kedaluwarsa %s = %s.", item.Nama, exp))
}

func (t *StokTool) batalkanTx(ctx context.Context, args map[string]any) *tools.ToolResult {
	id := strings.ToUpper(argStr(args, "id"))
	if id == "" {
		return tools.ErrorResult("ID transaksi-nya diisi dulu ya kak 🙏")
	}

	// Cek dahulu sebelum remove (irreversible)
	allTx, err := t.store.GetAllTransaksi(ctx)
	if err != nil {
		return tools.ErrorResult("Gagal baca transaksi kak 😣").WithError(err)
	}
	var tx *Transaksi
	for _, t2 := range allTx {
		if t2.ID == id {
			tx = t2
			break
		}
	}
	if tx == nil {
		return tools.NewToolResult(fmt.Sprintf("Transaksi %s nggak ketemu kak 🔍", id))
	}

	// Guard: bon dengan cicilan tidak bisa dibatalkan
	if tx.MetodeBayar == "bon" {
		allPiu, _ := t.store.GetAllPiutang(ctx)
		for _, piu := range allPiu {
			if piu.TransaksiID == id && piu.Dibayar > 0 {
				return tools.ErrorResult(fmt.Sprintf(
					"Transaksi %s tidak bisa dibatalkan kak 🙏 — piutang %s sudah ada cicilan %s. Gunakan write-off.",
					id, piu.ID, FormatRupiah(piu.Dibayar),
				))
			}
		}
	}

	// Aman untuk remove
	removed, err := t.store.RemoveTransaksi(ctx, id)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal batalkan transaksi kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	if removed == nil {
		return tools.NewToolResult(fmt.Sprintf("Transaksi %s nggak ketemu kak 🔍", id))
	}

	// Void piutang bila bon dan belum ada cicilan (dilakukan sebelum restore stok agar konsisten)
	if removed.MetodeBayar == "bon" {
		allPiu, _ := t.store.GetAllPiutang(ctx)
		for _, piu := range allPiu {
			if piu.TransaksiID == id {
				piu.Status = "dihapus"
				_ = t.store.SetPiutang(ctx, piu)
			}
		}
	}

	// Restore stok/saldo berdasarkan jenis produk
	item, _ := t.store.GetProduk(ctx, removed.ProdukID)
	if item == nil {
		return tools.NewToolResult(fmt.Sprintf("Transaksi %s dibatalkan (produk tidak ditemukan, stok tidak dikembalikan).", removed.ID))
	}

	switch item.JenisOrDefault() {
	case "pulsa":
		if removed.Modal > 0 {
			_ = t.store.IncrSaldoModal(ctx, item.ID, removed.Modal)
			reloaded, _ := t.store.GetProduk(ctx, item.ID)
			saldoKini := 0
			if reloaded != nil {
				saldoKini = reloaded.SaldoModal
			}
			return tools.NewToolResult(fmt.Sprintf("Transaksi %s dibatalkan, saldo modal pulsa dikembalikan (+%s, saldo kini %s).",
				removed.ID, FormatRupiah(removed.Modal), FormatRupiah(saldoKini)))
		}
		return tools.NewToolResult(fmt.Sprintf("Transaksi %s dibatalkan (modal tidak tercatat, saldo tidak dikembalikan).", removed.ID))

	case "bensin":
		if removed.Liter > 0 {
			mlKembali := int(math.Round(removed.Liter * 1000))
			item.StokMl += mlKembali
			item.Stok = item.StokMl / 1000
			item.LastUpdate = NowWITA().Format("2006-01-02")
			_ = t.store.SetProduk(ctx, item)
			return tools.NewToolResult(fmt.Sprintf("Transaksi %s dibatalkan, stok bensin %s dikembalikan (+%.3fL, sisa %.1fL).",
				removed.ID, item.Nama, removed.Liter, float64(item.StokMl)/1000))
		}
		return tools.NewToolResult(fmt.Sprintf("Transaksi %s dibatalkan (volume tidak tercatat, stok tidak dikembalikan).", removed.ID))

	default:
		item.Stok += removed.Qty
		item.LastUpdate = NowWITA().Format("2006-01-02")
		_ = t.store.SetProduk(ctx, item)
		return tools.NewToolResult(
			fmt.Sprintf(
				"Transaksi %s dibatalkan, stok %s dikembalikan (+%d).",
				removed.ID,
				removed.NamaProduk,
				removed.Qty,
			),
		)
	}
}

func (t *StokTool) stokMenipis(ctx context.Context) *tools.ToolResult {
	all, err := t.store.GetAllProduk(ctx)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal baca data stok kak 😣 Coba lagi sebentar ya.").WithError(err)
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
