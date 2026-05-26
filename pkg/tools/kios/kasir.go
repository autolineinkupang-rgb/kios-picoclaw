package kios

import (
	"context"
	"fmt"
	"strings"

	tools "github.com/sipeed/picoclaw/pkg/tools"
)

const lokasiKios = "Rote Barat Laut, Rote Ndao"

// KasirTool handles point-of-sale: receipts and cashier shifts.
type KasirTool struct{ store *Store }

func (t *KasirTool) Name() string { return "kios_kasir" }

func (t *KasirTool) Description() string {
	return "Kasir kios: jual barang dengan struk lengkap + kembalian, serta buka/tutup/cek shift kasir."
}

func (t *KasirTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"jual", "jual_massal", "buka_shift", "tutup_shift", "status_shift"},
				"description": "Aksi kasir.",
			},
			"produk": map[string]any{"type": "string"},
			"qty":    map[string]any{"type": "integer"},
			"items": map[string]any{
				"type":        "array",
				"description": "daftar item untuk jual_massal",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"produk": map[string]any{"type": "string"},
						"qty":    map[string]any{"type": "integer"},
					},
				},
			},
			"metode":      map[string]any{"type": "string", "enum": []string{"tunai", "transfer", "qris"}},
			"bayar":       map[string]any{"type": "integer", "description": "nominal uang yang dibayar pelanggan"},
			"saldo_awal":  map[string]any{"type": "integer", "description": "modal kas saat buka shift"},
			"saldo_akhir": map[string]any{"type": "integer", "description": "kas terhitung saat tutup shift"},
		},
		"required": []string{"action"},
	}
}

func (t *KasirTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	role, kasir, refusal := resolveRole(ctx, t.store)
	if refusal != nil {
		return refusal
	}
	_ = role
	switch argStr(args, "action") {
	case "jual":
		return t.jual(ctx, args, kasir)
	case "jual_massal":
		return t.jualMassal(ctx, args, kasir)
	case "buka_shift":
		return t.bukaShift(ctx, args, kasir)
	case "tutup_shift":
		return t.tutupShift(ctx, args)
	case "status_shift":
		return t.statusShift(ctx)
	default:
		return tools.ErrorResult("Aksi kasir tidak dikenal.")
	}
}

func (t *KasirTool) jual(ctx context.Context, args map[string]any, kasir string) *tools.ToolResult {
	qty := argInt(args, "qty")
	bayarPtr := argIntPtr(args, "bayar")

	// Apply an active promo automatically (before recording the sale).
	diskon, promoID := 0, ""
	if pre, _ := findOne(ctx, t.store, argStr(args, "produk")); pre != nil {
		diskon, promoID = activePromoDiskon(ctx, t.store, pre.ID, qty, pre.HargaJual)
	}

	tx, item, sisa, err := performJual(ctx, t.store, argStr(args, "produk"), qty, argStr(args, "metode"), kasir, diskon)
	if err != nil {
		return tools.ErrorResult(err.Error())
	}
	if bayarPtr != nil && *bayarPtr < tx.Total {
		// Sale already recorded; warn about underpayment but keep the receipt.
		kurang := tx.Total - *bayarPtr
		return tools.UserResult(fmt.Sprintf("%s\n\n⚠️ Uang kurang %s kak!", t.struk(tx, item, bayarPtr, promoID), FormatRupiah(kurang)))
	}
	out := t.struk(tx, item, bayarPtr, promoID)
	if sisa <= 0 {
		out += fmt.Sprintf("\n⚠️ %s HABIS!", item.Nama)
	} else if sisa <= item.StokKritis {
		out += fmt.Sprintf("\n⚠️ Stok %s menipis (sisa %d).", item.Nama, sisa)
	}
	return tools.UserResult(out)
}

func (t *KasirTool) jualMassal(ctx context.Context, args map[string]any, kasir string) *tools.ToolResult {
	items := argItems(args)
	if len(items) == 0 {
		return tools.ErrorResult("items wajib diisi (daftar produk + qty).")
	}
	metode := argStr(args, "metode")
	div := strings.Repeat("━", 30)
	var b strings.Builder
	fmt.Fprintf(&b, "🧾 *STRUK KIOS CERDAS*\n📍 %s\n%s\n", lokasiKios, div)
	grand := 0
	var errs []string
	for _, it := range items {
		produk := argStr(it, "produk")
		qty := argInt(it, "qty")
		diskon := 0
		if pre, _ := findOne(ctx, t.store, produk); pre != nil {
			diskon, _ = activePromoDiskon(ctx, t.store, pre.ID, qty, pre.HargaJual)
		}
		tx, item, _, err := performJual(ctx, t.store, produk, qty, metode, kasir, diskon)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %s", produk, err.Error()))
			continue
		}
		sub := tx.Qty * item.HargaJual
		fmt.Fprintf(&b, "%s x%d  %s\n", item.Nama, tx.Qty, FormatRupiah(sub))
		if d := sub - tx.Total; d > 0 {
			fmt.Fprintf(&b, "  promo -%s\n", FormatRupiah(d))
		}
		grand += tx.Total
	}
	fmt.Fprintf(&b, "%s\nTotal: %s\n", div, FormatRupiah(grand))
	if bayar := argIntPtr(args, "bayar"); bayar != nil && *bayar >= grand {
		fmt.Fprintf(&b, "Bayar: %s\nKembalian: %s\n", FormatRupiah(*bayar), FormatRupiah(*bayar-grand))
	}
	fmt.Fprintf(&b, "%s\n%s WITA\nTerima kasih! 🙏", div, NowWITA().Format("02/01/2006 15:04"))
	out := b.String()
	if len(errs) > 0 {
		out += "\n⚠️ Gagal: " + strings.Join(errs, "; ")
	}
	return tools.UserResult(out)
}

func (t *KasirTool) struk(tx *Transaksi, item *Produk, bayar *int, promoID string) string {
	div := strings.Repeat("━", 30)
	subtotal := tx.Qty * item.HargaJual
	diskonTotal := subtotal - tx.Total
	var b strings.Builder
	fmt.Fprintf(&b, "🧾 *STRUK KIOS CERDAS*\n📍 %s\n%s\n", lokasiKios, div)
	fmt.Fprintf(&b, "%s x%d  %s\n", item.Nama, tx.Qty, FormatRupiah(subtotal))
	if diskonTotal > 0 {
		fmt.Fprintf(&b, "Promo %s: -%s\n", promoID, FormatRupiah(diskonTotal))
	}
	fmt.Fprintf(&b, "%s\nTotal: %s\n", div, FormatRupiah(tx.Total))
	if bayar != nil && *bayar >= tx.Total {
		fmt.Fprintf(&b, "Bayar: %s\nKembalian: %s\n", FormatRupiah(*bayar), FormatRupiah(*bayar-tx.Total))
	}
	fmt.Fprintf(&b, "%s\n✅ #%s | %s WITA\nTerima kasih! 🙏", div, tx.ID, NowWITA().Format("02/01/2006 15:04"))
	return b.String()
}

func (t *KasirTool) bukaShift(ctx context.Context, args map[string]any, kasir string) *tools.ToolResult {
	sh, err := t.store.GetShift(ctx)
	if err != nil {
		return tools.ErrorResult("Gagal baca shift.").WithError(err)
	}
	if sh != nil && sh.Status == "buka" {
		return tools.ErrorResult(fmt.Sprintf("Shift sudah dibuka oleh %s sejak %s WITA.", sh.Kasir, sh.WaktuBuka))
	}
	now := NowWITA()
	newShift := &Shift{
		Kasir: kasir, SaldoAwal: argInt(args, "saldo_awal"),
		WaktuBuka: now.Format("2006-01-02 15:04"), Status: "buka",
	}
	if err := t.store.SetShift(ctx, newShift); err != nil {
		return tools.ErrorResult("Gagal buka shift.").WithError(err)
	}
	return tools.NewToolResult(fmt.Sprintf("Shift dibuka oleh %s, saldo awal %s, jam %s WITA.",
		kasir, FormatRupiah(newShift.SaldoAwal), newShift.WaktuBuka))
}

func (t *KasirTool) tutupShift(ctx context.Context, args map[string]any) *tools.ToolResult {
	sh, err := t.store.GetShift(ctx)
	if err != nil {
		return tools.ErrorResult("Gagal baca shift.").WithError(err)
	}
	if sh == nil || sh.Status != "buka" {
		return tools.ErrorResult("Belum ada shift yang dibuka kak. Ketik buka shift dulu ya.")
	}
	omzet, jumlah := t.omzetSejak(ctx, sh.WaktuBuka)
	now := NowWITA()
	sh.Status = "tutup"
	sh.WaktuTutup = now.Format("2006-01-02 15:04")
	sh.SaldoAkhir = argInt(args, "saldo_akhir")
	if err := t.store.SetShift(ctx, sh); err != nil {
		return tools.ErrorResult("Gagal tutup shift.").WithError(err)
	}
	return tools.NewToolResult(fmt.Sprintf("Shift ditutup. Kasir: %s. Transaksi: %d. Omzet: %s. Saldo akhir: %s.",
		sh.Kasir, jumlah, FormatRupiah(omzet), FormatRupiah(sh.SaldoAkhir)))
}

func (t *KasirTool) statusShift(ctx context.Context) *tools.ToolResult {
	sh, err := t.store.GetShift(ctx)
	if err != nil {
		return tools.ErrorResult("Gagal baca shift.").WithError(err)
	}
	if sh == nil || sh.Status != "buka" {
		return tools.NewToolResult("Tidak ada shift yang sedang buka.")
	}
	omzet, jumlah := t.omzetSejak(ctx, sh.WaktuBuka)
	return tools.NewToolResult(fmt.Sprintf("Shift BUKA — kasir %s sejak %s WITA. Transaksi berjalan: %d, omzet: %s.",
		sh.Kasir, sh.WaktuBuka, jumlah, FormatRupiah(omzet)))
}

// omzetSejak sums transaction totals at/after the shift open time.
func (t *KasirTool) omzetSejak(ctx context.Context, waktuBuka string) (omzet, jumlah int) {
	txs, err := t.store.GetAllTransaksi(ctx)
	if err != nil {
		return 0, 0
	}
	for _, tx := range txs {
		stamp := tx.Tanggal + " " + tx.Jam
		if stamp >= waktuBuka {
			omzet += tx.Total
			jumlah++
		}
	}
	return omzet, jumlah
}
