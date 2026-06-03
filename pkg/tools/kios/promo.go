package kios

import (
	"context"
	"fmt"
	"strings"

	tools "github.com/sipeed/picoclaw/pkg/tools"
)

// PromoTool manages product discounts.
type PromoTool struct{ store *Store }

func (t *PromoTool) Name() string { return "kios_promo" }

func (t *PromoTool) Description() string {
	return "Kelola promo/diskon produk: buat, cek diskon aktif, daftar, dan hapus. " +
		"Tipe diskon 'persen' atau 'fixed' (rupiah). Buat/hapus khusus owner."
}

func (t *PromoTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"buat", "cek", "daftar", "hapus"},
				"description": "Aksi promo.",
			},
			"produk":     map[string]any{"type": "string"},
			"tipe":       map[string]any{"type": "string", "enum": []string{"persen", "fixed"}},
			"nilai":      map[string]any{"type": "number", "description": "persen (mis. 10) atau rupiah (fixed)"},
			"qty":        map[string]any{"type": "integer"},
			"min_qty":    map[string]any{"type": "integer", "description": "minimal qty agar promo berlaku"},
			"mulai":      map[string]any{"type": "string", "description": "tanggal mulai YYYY-MM-DD"},
			"selesai":    map[string]any{"type": "string", "description": "tanggal selesai YYYY-MM-DD"},
			"catatan":    map[string]any{"type": "string"},
			"id":         map[string]any{"type": "string", "description": "id promo (hapus)"},
			"aktif_only": map[string]any{"type": "boolean", "description": "daftar hanya promo aktif"},
		},
		"required": []string{"action"},
	}
}

func (t *PromoTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	role, _, refusal := resolveRole(ctx, t.store)
	if refusal != nil {
		return refusal
	}
	switch argStr(args, "action") {
	case "buat":
		if r := requireOwner(role); r != nil {
			return r
		}
		return t.buat(ctx, args)
	case "cek":
		return t.cek(ctx, args)
	case "daftar":
		return t.daftar(ctx, args)
	case "hapus":
		if r := requireOwner(role); r != nil {
			return r
		}
		return t.hapus(ctx, args)
	default:
		return tools.ErrorResult("Hmm, aksi promo belum dikenal kak 🤔")
	}
}

func (t *PromoTool) buat(ctx context.Context, args map[string]any) *tools.ToolResult {
	tipe := argStr(args, "tipe")
	if tipe == "" {
		tipe = "persen"
	}
	if tipe != "persen" && tipe != "fixed" {
		return tools.ErrorResult("Tipe harus 'persen' atau 'fixed'.")
	}
	nilai, ok := args["nilai"].(float64)
	if !ok {
		nilai = float64(argInt(args, "nilai"))
	}
	if nilai <= 0 {
		return tools.ErrorResult("Nilai diskon-nya wajib diisi dan harus lebih dari 0 ya kak 🙏")
	}
	item, err := findOne(ctx, t.store, argStr(args, "produk"))
	if err != nil {
		return tools.ErrorResult("Aduh, gagal cari produk kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	if item == nil {
		return tools.ErrorResult("Produk nggak ketemu kak 🔍 di stok")
	}
	// Deactivate existing promos for the same product.
	all, _ := t.store.GetAllPromo(ctx)
	for _, p := range all {
		if p.ProdukID == item.ID && p.Aktif {
			p.Aktif = false
			t.store.SetPromo(ctx, p)
		}
	}
	id, err := t.store.NextPromoID(ctx)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal buat id promo kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	mulai := argStr(args, "mulai")
	if mulai == "" {
		mulai = NowWITA().Format("2006-01-02")
	}
	promo := &Promo{
		ID: id, Produk: item.Nama, ProdukID: item.ID, Tipe: tipe, Nilai: nilai,
		MinQty: parseQtyDefault(args["min_qty"], 1), Aktif: true,
		Mulai: mulai, Selesai: argStr(args, "selesai"), Catatan: argStr(args, "catatan"),
	}
	if err := t.store.SetPromo(ctx, promo); err != nil {
		return tools.ErrorResult("Aduh, gagal simpan promo kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	disc := fmt.Sprintf("%.0f%%", nilai)
	if tipe == "fixed" {
		disc = FormatRupiah(int(nilai))
	}
	return tools.NewToolResult(fmt.Sprintf("Promo dibuat: %s diskon %s untuk %s (min %d). [%s]",
		item.Nama, disc, "pembelian", promo.MinQty, promo.ID))
}

func (t *PromoTool) cek(ctx context.Context, args map[string]any) *tools.ToolResult {
	item, _ := findOne(ctx, t.store, argStr(args, "produk"))
	if item == nil {
		return tools.NewToolResult("Tidak ada promo (produk nggak ketemu kak 🔍)")
	}
	qty := parseQtyDefault(args["qty"], 1)
	hargaJual := argInt(args, "harga_jual")
	if hargaJual <= 0 {
		hargaJual = item.HargaJual
	}
	today := NowWITA().Format("2006-01-02")
	all, _ := t.store.GetAllPromo(ctx)
	var promo *Promo
	for _, p := range all {
		if p.ProdukID == item.ID && promoAktif(p, today) && qty >= p.MinQty {
			promo = p
			break
		}
	}
	if promo == nil {
		return tools.NewToolResult(fmt.Sprintf("Tidak ada promo aktif untuk %s.", item.Nama))
	}
	diskon := int(promo.Nilai)
	if promo.Tipe == "persen" {
		diskon = int(float64(hargaJual) * promo.Nilai / 100)
	}
	final := hargaJual - diskon
	if final < 0 {
		final = 0
	}
	return tools.NewToolResult(fmt.Sprintf("Promo %s: diskon %s/unit → harga %s (dari %s). [%s]",
		item.Nama, FormatRupiah(diskon), FormatRupiah(final), FormatRupiah(hargaJual), promo.ID))
}

func (t *PromoTool) daftar(ctx context.Context, args map[string]any) *tools.ToolResult {
	all, err := t.store.GetAllPromo(ctx)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal baca promo kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	today := NowWITA().Format("2006-01-02")
	aktifOnly := argBool(args, "aktif_only")
	var b strings.Builder
	count := 0
	for _, p := range all {
		if aktifOnly && !promoAktif(p, today) {
			continue
		}
		disc := fmt.Sprintf("%.0f%%", p.Nilai)
		if p.Tipe == "fixed" {
			disc = FormatRupiah(int(p.Nilai))
		}
		status := "aktif"
		if !promoAktif(p, today) {
			status = "nonaktif"
		}
		fmt.Fprintf(&b, "- [%s] %s: %s (min %d) — %s\n", p.ID, p.Produk, disc, p.MinQty, status)
		count++
	}
	if count == 0 {
		return tools.NewToolResult("Belum ada promo.")
	}
	return tools.NewToolResult(fmt.Sprintf("Daftar promo (%d):\n%s", count, b.String()))
}

func (t *PromoTool) hapus(ctx context.Context, args map[string]any) *tools.ToolResult {
	id := strings.ToUpper(argStr(args, "id"))
	if id == "" {
		return tools.ErrorResult("ID promo-nya diisi dulu ya kak 🙏")
	}
	all, err := t.store.GetAllPromo(ctx)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal baca promo kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	for _, p := range all {
		if strings.EqualFold(p.ID, id) {
			p.Aktif = false
			if err := t.store.SetPromo(ctx, p); err != nil {
				return tools.ErrorResult("Aduh, gagal hapus promo kak 😣 Coba lagi sebentar ya.").WithError(err)
			}
			return tools.NewToolResult(fmt.Sprintf("Promo %s (%s) dinonaktifkan.", p.ID, p.Produk))
		}
	}
	return tools.NewToolResult(fmt.Sprintf("Promo %s nggak ketemu kak 🔍", id))
}
