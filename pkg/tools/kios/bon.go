package kios

import (
	"context"
	"fmt"
	"strings"

	tools "github.com/sipeed/picoclaw/pkg/tools"
)

// BonTool handles credit sales (piutang pembeli) and supplier payables (hutang).
type BonTool struct{ store *Store }

func NewBonTool(store *Store) *BonTool { return &BonTool{store: store} }

func (t *BonTool) Name() string { return "kios_bon" }

func (t *BonTool) Description() string {
	return "Kelola bon & hutang kios: jual_bon (kredit pembeli), catat_hutang_supplier, bayar, lunasi, hapus, daftar_piutang, daftar_hutang, detail."
}

func (t *BonTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "Aksi: jual_bon|catat_hutang_supplier|bayar|lunasi|hapus|daftar_piutang|daftar_hutang|detail",
			},
			"produk":       map[string]any{"type": "string"},
			"qty":          map[string]any{"type": "integer"},
			"pelanggan":    map[string]any{"type": "string", "description": "nomor WA atau nama pelanggan"},
			"id":           map[string]any{"type": "string", "description": "PIU-xxxx atau HUT-xxxx"},
			"jumlah":       map[string]any{"type": "integer"},
			"metode":       map[string]any{"type": "string"},
			"supplier_id":  map[string]any{"type": "string"},
			"pembelian_id": map[string]any{"type": "string"},
			"pokok":        map[string]any{"type": "integer"},
			"filter":       map[string]any{"type": "string"},
			"jatuh_tempo":  map[string]any{"type": "string"},
			"catatan":      map[string]any{"type": "string"},
		},
		"required": []string{"action"},
	}
}

func (t *BonTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	role, kasir, refusal := resolveRole(ctx, t.store)
	if refusal != nil {
		return refusal
	}
	switch argStr(args, "action") {
	case "jual_bon":
		if r := requireStaff(role); r != nil {
			return r
		}
		return t.jualBon(ctx, args, kasir)
	case "catat_hutang_supplier":
		if r := requireOwner(role); r != nil {
			return r
		}
		return t.catatHutangSupplier(ctx, args, kasir)
	case "bayar":
		if r := requireStaff(role); r != nil {
			return r
		}
		return t.bayar(ctx, args, kasir)
	case "lunasi":
		if r := requireStaff(role); r != nil {
			return r
		}
		return t.lunasi(ctx, args, kasir)
	case "hapus":
		if r := requireOwner(role); r != nil {
			return r
		}
		return t.hapus(ctx, args)
	case "daftar_piutang":
		return t.daftarPiutang(ctx, args)
	case "daftar_hutang":
		return t.daftarHutang(ctx, args)
	case "detail":
		return t.detail(ctx, args)
	default:
		return tools.ErrorResult("Aksi bon belum dikenal kak 🤔")
	}
}

func (t *BonTool) jualBon(ctx context.Context, args map[string]any, kasir string) *tools.ToolResult {
	qty := argInt(args, "qty")
	if qty <= 0 {
		return tools.ErrorResult("Qty harus > 0 kak 🙏")
	}
	pelangganArg := argStr(args, "pelanggan")
	phone := NormalizePhone(pelangganArg)
	if phone == "" {
		return tools.ErrorResult("Nomor WA atau nama pelanggan tidak valid kak 🙏 (contoh: 08123456789)")
	}
	pelanggan, err := t.store.UpsertPelanggan(ctx, pelangganArg, pelangganArg)
	if err != nil {
		return tools.ErrorResult("Gagal daftarkan pelanggan kak 😣").WithError(err)
	}
	tx, _, _, err := performJual(ctx, t.store, argStr(args, "produk"), qty, "bon", kasir, 0)
	if err != nil {
		return tools.ErrorResult(err.Error())
	}
	piuID, err := t.store.NextPiutangID(ctx)
	if err != nil {
		return tools.ErrorResult("Gagal buat ID piutang kak 😣").WithError(err)
	}
	now := NowWITA()
	piu := &Piutang{
		ID: piuID, PelangganID: pelanggan.ID, Phone: pelanggan.Phone,
		TransaksiID: tx.ID, Pokok: tx.Total, Dibayar: 0, Sisa: tx.Total,
		Status: "terbuka", Tanggal: now.Format("2006-01-02"), Jam: now.Format("15:04:05"),
		Kasir: kasir, Catatan: argStr(args, "catatan"),
	}
	if err := t.store.SetPiutang(ctx, piu); err != nil {
		return tools.ErrorResult("Gagal simpan piutang kak 😣").WithError(err)
	}
	pelanggan.TotalBelanja += tx.Total
	pelanggan.TotalUtang += tx.Total
	_ = t.store.SetPelanggan(ctx, pelanggan)

	return tools.NewToolResult(fmt.Sprintf(
		"Bon kredit dicatat kak ✅\n%s [%s] %s × %d = %s\nPiutang: %s (sisa %s). Pembayaran nanti ya.",
		tx.NamaProduk, tx.ID, FormatRupiah(tx.HargaSatuan), qty, FormatRupiah(tx.Total),
		piuID, FormatRupiah(piu.Sisa),
	))
}

func (t *BonTool) catatHutangSupplier(ctx context.Context, args map[string]any, kasir string) *tools.ToolResult {
	supID := argStr(args, "supplier_id")
	pokok := argInt(args, "pokok")
	pemID := argStr(args, "pembelian_id")
	if supID == "" {
		return tools.ErrorResult("supplier_id wajib diisi kak 🙏")
	}
	if pokok <= 0 {
		return tools.ErrorResult("Pokok hutang harus > 0 kak 🙏")
	}
	sup, err := t.store.GetSupplierByID(ctx, supID)
	if err != nil || sup == nil {
		return tools.ErrorResult(fmt.Sprintf("Supplier %q tidak ditemukan kak 🔍", supID))
	}
	hutID, err := t.store.NextHutangID(ctx)
	if err != nil {
		return tools.ErrorResult("Gagal buat ID hutang kak 😣").WithError(err)
	}
	now := NowWITA()
	hut := &Hutang{
		ID: hutID, SupplierID: supID, PembelianID: pemID,
		Pokok: pokok, Dibayar: 0, Sisa: pokok, Status: "terbuka",
		JatuhTempo: argStr(args, "jatuh_tempo"),
		Tanggal:    now.Format("2006-01-02"), Catatan: argStr(args, "catatan"),
	}
	if err := t.store.SetHutang(ctx, hut); err != nil {
		return tools.ErrorResult("Gagal simpan hutang kak 😣").WithError(err)
	}
	_ = kasir
	return tools.NewToolResult(fmt.Sprintf(
		"Hutang ke %s dicatat: %s — %s\nID: %s. Bayar nanti sesuai jadwal.",
		sup.Nama, hutID, FormatRupiah(pokok), hutID,
	))
}

func (t *BonTool) bayar(ctx context.Context, args map[string]any, kasir string) *tools.ToolResult {
	id := strings.ToUpper(argStr(args, "id"))
	jumlah := argInt(args, "jumlah")
	if jumlah <= 0 {
		return tools.ErrorResult("Jumlah pembayaran harus > 0 kak 🙏")
	}
	metode := argStr(args, "metode")
	if metode == "" {
		metode = "tunai"
	}
	if strings.HasPrefix(id, "PIU-") {
		return t.bayarPiutang(ctx, id, jumlah, metode, kasir)
	} else if strings.HasPrefix(id, "HUT-") {
		return t.bayarHutang(ctx, id, jumlah, metode, kasir)
	}
	return tools.ErrorResult("ID harus PIU-xxxx atau HUT-xxxx kak 🙏")
}

func (t *BonTool) bayarPiutang(ctx context.Context, id string, jumlah int, metode, kasir string) *tools.ToolResult {
	piu, err := t.store.GetPiutang(ctx, id)
	if err != nil {
		return tools.ErrorResult("Gagal baca piutang kak 😣").WithError(err)
	}
	if piu == nil {
		return tools.ErrorResult(fmt.Sprintf("Piutang %s tidak ditemukan kak 🔍", id))
	}
	if piu.Status != "terbuka" {
		return tools.ErrorResult(fmt.Sprintf("Piutang %s sudah %s kak.", id, piu.Status))
	}
	if jumlah > piu.Sisa {
		return tools.ErrorResult(fmt.Sprintf("Overpayment kak 🙏 Sisa hanya %s.", FormatRupiah(piu.Sisa)))
	}

	piu.Dibayar += jumlah
	piu.Sisa -= jumlah
	if piu.Sisa == 0 {
		piu.Status = "lunas"
	}
	if err := t.store.SetPiutang(ctx, piu); err != nil {
		return tools.ErrorResult("Gagal update piutang kak 😣").WithError(err)
	}

	payID, _ := t.store.NextPayID(ctx)
	now := NowWITA()
	_ = t.store.AppendPembayaran(ctx, &Pembayaran{
		ID: payID, LedgerID: id, Jenis: "piutang", Jumlah: jumlah,
		Metode: metode, Tanggal: now.Format("2006-01-02"), Jam: now.Format("15:04:05"), Kasir: kasir,
	})

	if p, _ := t.store.GetPelanggan(ctx, piu.Phone); p != nil {
		if p.TotalUtang >= jumlah {
			p.TotalUtang -= jumlah
		} else {
			p.TotalUtang = 0
		}
		_ = t.store.SetPelanggan(ctx, p)
	}

	msg := fmt.Sprintf("Pembayaran %s diterima: %s (%s)\nDibayar: %s | Sisa: %s",
		payID, FormatRupiah(jumlah), metode, FormatRupiah(piu.Dibayar), FormatRupiah(piu.Sisa))
	if piu.Status == "lunas" {
		msg += "\n✅ Piutang LUNAS!"
	}
	return tools.NewToolResult(msg)
}

func (t *BonTool) bayarHutang(ctx context.Context, id string, jumlah int, metode, kasir string) *tools.ToolResult {
	hut, err := t.store.GetHutang(ctx, id)
	if err != nil {
		return tools.ErrorResult("Gagal baca hutang kak 😣").WithError(err)
	}
	if hut == nil {
		return tools.ErrorResult(fmt.Sprintf("Hutang %s tidak ditemukan kak 🔍", id))
	}
	if hut.Status != "terbuka" {
		return tools.ErrorResult(fmt.Sprintf("Hutang %s sudah %s kak.", id, hut.Status))
	}
	if jumlah > hut.Sisa {
		return tools.ErrorResult(fmt.Sprintf("Overpayment kak 🙏 Sisa hanya %s.", FormatRupiah(hut.Sisa)))
	}

	hut.Dibayar += jumlah
	hut.Sisa -= jumlah
	if hut.Sisa == 0 {
		hut.Status = "lunas"
	}
	if err := t.store.SetHutang(ctx, hut); err != nil {
		return tools.ErrorResult("Gagal update hutang kak 😣").WithError(err)
	}

	payID, _ := t.store.NextPayID(ctx)
	now := NowWITA()
	_ = t.store.AppendPembayaran(ctx, &Pembayaran{
		ID: payID, LedgerID: id, Jenis: "hutang", Jumlah: jumlah,
		Metode: metode, Tanggal: now.Format("2006-01-02"), Jam: now.Format("15:04:05"), Kasir: kasir,
	})

	msg := fmt.Sprintf("Bayar hutang %s: %s (%s)\nDibayar: %s | Sisa: %s",
		payID, FormatRupiah(jumlah), metode, FormatRupiah(hut.Dibayar), FormatRupiah(hut.Sisa))
	if hut.Status == "lunas" {
		msg += "\n✅ Hutang LUNAS!"
	}
	return tools.NewToolResult(msg)
}

func (t *BonTool) lunasi(ctx context.Context, args map[string]any, kasir string) *tools.ToolResult {
	id := strings.ToUpper(argStr(args, "id"))
	if strings.HasPrefix(id, "PIU-") {
		piu, _ := t.store.GetPiutang(ctx, id)
		if piu == nil {
			return tools.ErrorResult("Piutang tidak ditemukan kak 🔍")
		}
		args["jumlah"] = float64(piu.Sisa)
	} else if strings.HasPrefix(id, "HUT-") {
		hut, _ := t.store.GetHutang(ctx, id)
		if hut == nil {
			return tools.ErrorResult("Hutang tidak ditemukan kak 🔍")
		}
		args["jumlah"] = float64(hut.Sisa)
	}
	return t.bayar(ctx, args, kasir)
}

func (t *BonTool) hapus(ctx context.Context, args map[string]any) *tools.ToolResult {
	id := strings.ToUpper(argStr(args, "id"))
	if strings.HasPrefix(id, "PIU-") {
		piu, _ := t.store.GetPiutang(ctx, id)
		if piu == nil {
			return tools.ErrorResult("Piutang tidak ditemukan kak 🔍")
		}
		piu.Status = "dihapus"
		_ = t.store.SetPiutang(ctx, piu)
		return tools.NewToolResult(fmt.Sprintf("Piutang %s dihapus (write-off).", id))
	} else if strings.HasPrefix(id, "HUT-") {
		hut, _ := t.store.GetHutang(ctx, id)
		if hut == nil {
			return tools.ErrorResult("Hutang tidak ditemukan kak 🔍")
		}
		hut.Status = "dihapus"
		_ = t.store.SetHutang(ctx, hut)
		return tools.NewToolResult(fmt.Sprintf("Hutang %s dihapus (write-off).", id))
	}
	return tools.ErrorResult("ID harus PIU-xxxx atau HUT-xxxx kak 🙏")
}

func (t *BonTool) daftarPiutang(ctx context.Context, args map[string]any) *tools.ToolResult {
	all, err := t.store.GetAllPiutang(ctx)
	if err != nil {
		return tools.ErrorResult("Gagal baca piutang kak 😣").WithError(err)
	}
	filter := strings.ToLower(argStr(args, "filter"))
	var open []*Piutang
	for _, p := range all {
		if p.Status != "terbuka" {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(p.Phone), filter) {
			continue
		}
		open = append(open, p)
	}
	if len(open) == 0 {
		return tools.NewToolResult("Tidak ada piutang terbuka kak.")
	}
	var sb strings.Builder
	total := 0
	for _, p := range open {
		fmt.Fprintf(&sb, "- %s [%s] sisa %s (dari %s)\n", p.ID, p.Phone, FormatRupiah(p.Sisa), p.Tanggal)
		total += p.Sisa
	}
	return tools.NewToolResult(
		fmt.Sprintf("%d piutang terbuka — total %s:\n%s", len(open), FormatRupiah(total), sb.String()),
	)
}

func (t *BonTool) daftarHutang(ctx context.Context, args map[string]any) *tools.ToolResult {
	all, err := t.store.GetAllHutang(ctx)
	if err != nil {
		return tools.ErrorResult("Gagal baca hutang kak 😣").WithError(err)
	}
	filter := strings.ToLower(argStr(args, "filter"))
	var open []*Hutang
	for _, h := range all {
		if h.Status != "terbuka" {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(h.SupplierID), filter) {
			continue
		}
		open = append(open, h)
	}
	if len(open) == 0 {
		return tools.NewToolResult("Tidak ada hutang supplier terbuka kak.")
	}
	var sb strings.Builder
	total := 0
	for _, h := range open {
		jt := ""
		if h.JatuhTempo != "" {
			jt = " (jatuh " + h.JatuhTempo + ")"
		}
		fmt.Fprintf(&sb, "- %s [%s] sisa %s%s\n", h.ID, h.SupplierID, FormatRupiah(h.Sisa), jt)
		total += h.Sisa
	}
	return tools.NewToolResult(
		fmt.Sprintf("%d hutang terbuka — total %s:\n%s", len(open), FormatRupiah(total), sb.String()),
	)
}

func (t *BonTool) detail(ctx context.Context, args map[string]any) *tools.ToolResult {
	id := strings.ToUpper(argStr(args, "id"))
	var ledgerInfo string
	if strings.HasPrefix(id, "PIU-") {
		p, _ := t.store.GetPiutang(ctx, id)
		if p == nil {
			return tools.ErrorResult(fmt.Sprintf("Piutang %s tidak ditemukan kak 🔍", id))
		}
		ledgerInfo = fmt.Sprintf("Piutang %s | %s\nPokok: %s | Dibayar: %s | Sisa: %s | Status: %s",
			p.ID, p.Phone, FormatRupiah(p.Pokok), FormatRupiah(p.Dibayar), FormatRupiah(p.Sisa), p.Status)
	} else if strings.HasPrefix(id, "HUT-") {
		h, _ := t.store.GetHutang(ctx, id)
		if h == nil {
			return tools.ErrorResult(fmt.Sprintf("Hutang %s tidak ditemukan kak 🔍", id))
		}
		ledgerInfo = fmt.Sprintf("Hutang %s | %s\nPokok: %s | Dibayar: %s | Sisa: %s | Status: %s",
			h.ID, h.SupplierID, FormatRupiah(h.Pokok), FormatRupiah(h.Dibayar), FormatRupiah(h.Sisa), h.Status)
	} else {
		return tools.ErrorResult("ID harus PIU-xxxx atau HUT-xxxx kak 🙏")
	}
	pays, _ := t.store.GetAllPembayaran(ctx)
	var hist strings.Builder
	for _, pay := range pays {
		if pay.LedgerID == id {
			fmt.Fprintf(&hist, "  • %s %s (%s) — %s\n", pay.ID, FormatRupiah(pay.Jumlah), pay.Metode, pay.Tanggal)
		}
	}
	if hist.Len() == 0 {
		return tools.NewToolResult(ledgerInfo + "\n(belum ada pembayaran)")
	}
	return tools.NewToolResult(ledgerInfo + "\nHistori pembayaran:\n" + hist.String())
}
