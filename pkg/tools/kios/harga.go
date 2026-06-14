package kios

import (
	"context"
	"fmt"
	"sort"
	"strings"

	tools "github.com/sipeed/picoclaw/pkg/tools"
)

// HargaTool handles price checks, updates, estimation, and trend prediction.
type HargaTool struct{ store *Store }

func (t *HargaTool) Name() string { return "kios_harga" }

func (t *HargaTool) Description() string {
	return "Harga kios: cek harga produk, update harga jual/beli (tercatat di riwayat), " +
		"estimasi harga jual ideal dari margin, dan prediksi tren harga dari riwayat."
}

func (t *HargaTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"cek", "update", "estimasi", "prediksi"},
				"description": "Aksi harga.",
			},
			"produk":          map[string]any{"type": "string", "description": "nama atau id produk"},
			"harga_jual":      map[string]any{"type": "integer", "description": "harga jual baru, rupiah (WAJIB untuk action update)"},
			"harga_beli":      map[string]any{"type": "integer", "description": "harga beli baru, rupiah (opsional saat update: ubah bersamaan dengan harga jual)"},
			"harga_beli_baru": map[string]any{"type": "integer", "description": "harga beli hipotetis untuk simulasi margin (action estimasi)"},
		},
		"required": []string{"action", "produk"},
	}
}

func (t *HargaTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	role, kasir, refusal := resolveRole(ctx, t.store)
	if refusal != nil {
		return refusal
	}
	switch argStr(args, "action") {
	case "cek":
		return t.cek(ctx, args)
	case "update":
		_ = role
		return t.update(ctx, args, kasir)
	case "estimasi":
		return t.estimasi(ctx, args)
	case "prediksi":
		return t.prediksi(ctx, args)
	default:
		return tools.ErrorResult("Hmm, aksi harga belum dikenal kak 🤔")
	}
}

func (t *HargaTool) cek(ctx context.Context, args map[string]any) *tools.ToolResult {
	item, err := findOne(ctx, t.store, argStr(args, "produk"))
	if err != nil {
		return tools.ErrorResult("Aduh, gagal cari produk kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	if item == nil {
		return tools.NewToolResult("Produk nggak ketemu kak 🔍")
	}
	return tools.NewToolResult(
		fmt.Sprintf("%s — jual %s, beli %s.", item.Nama, FormatRupiah(item.HargaJual), FormatRupiah(item.HargaBeli)),
	)
}

func (t *HargaTool) update(ctx context.Context, args map[string]any, kasir string) *tools.ToolResult {
	hargaJual := argInt(args, "harga_jual")
	if hargaJual <= 0 {
		return tools.ErrorResult("harga_jual-nya wajib diisi dan harus lebih dari 0 ya kak 🙏")
	}
	item, err := findOne(ctx, t.store, argStr(args, "produk"))
	if err != nil {
		return tools.ErrorResult("Aduh, gagal cari produk kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	if item == nil {
		return tools.NewToolResult("Produk nggak ketemu kak 🔍")
	}
	lama := item.HargaJual
	item.HargaJual = hargaJual
	if hb := argIntPtr(args, "harga_beli"); hb != nil {
		item.HargaBeli = *hb
	}
	item.LastUpdate = NowWITA().Format("2006-01-02")
	if err := t.store.SetProduk(ctx, item); err != nil {
		return tools.ErrorResult("Aduh, gagal update harga kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	if hargaJual != lama {
		now := NowWITA()
		t.store.AppendPriceHistory(ctx, &PriceHistory{
			Tanggal: now.Format("2006-01-02"), Jam: now.Format("15:04:05"),
			ProdukID: item.ID, NamaProduk: item.Nama,
			HargaLama: lama, HargaBaru: hargaJual, Selisih: hargaJual - lama, Kasir: kasir,
		})
	}
	return tools.NewToolResult(
		fmt.Sprintf("Harga jual %s: %s → %s.", item.Nama, FormatRupiah(lama), FormatRupiah(hargaJual)),
	)
}

func (t *HargaTool) estimasi(ctx context.Context, args map[string]any) *tools.ToolResult {
	item, err := findOne(ctx, t.store, argStr(args, "produk"))
	if err != nil {
		return tools.ErrorResult("Aduh, gagal cari produk kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	if item == nil {
		return tools.NewToolResult("Produk nggak ketemu kak 🔍")
	}
	hargaBeli := item.HargaBeli
	if hb := argIntPtr(args, "harga_beli_baru"); hb != nil && *hb > 0 {
		hargaBeli = *hb
	}
	if hargaBeli <= 0 {
		return tools.ErrorResult("Harga beli belum tersedia untuk estimasi.")
	}
	m10 := int(float64(hargaBeli) * 1.10)
	m15 := int(float64(hargaBeli) * 1.15)
	m20 := int(float64(hargaBeli) * 1.20)
	m25 := int(float64(hargaBeli) * 1.25)
	return tools.NewToolResult(fmt.Sprintf(
		"Estimasi harga jual %s (beli %s):\n- margin 10%%: %s\n- margin 15%%: %s (saran)\n- margin 20%%: %s\n- margin 25%%: %s",
		item.Nama,
		FormatRupiah(hargaBeli),
		FormatRupiah(m10),
		FormatRupiah(m15),
		FormatRupiah(m20),
		FormatRupiah(m25),
	))
}

func (t *HargaTool) prediksi(ctx context.Context, args map[string]any) *tools.ToolResult {
	item, err := findOne(ctx, t.store, argStr(args, "produk"))
	if err != nil {
		return tools.ErrorResult("Aduh, gagal cari produk kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	if item == nil {
		return tools.NewToolResult("Produk nggak ketemu kak 🔍")
	}
	hist, err := t.store.GetAllPriceHistory(ctx)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal baca riwayat harga kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	var riwayat []*PriceHistory
	for _, h := range hist {
		if h.ProdukID == item.ID || strings.EqualFold(h.NamaProduk, item.Nama) {
			riwayat = append(riwayat, h)
		}
	}
	sort.Slice(riwayat, func(i, j int) bool { return riwayat[i].Tanggal < riwayat[j].Tanggal })
	if len(riwayat) < 2 {
		return tools.NewToolResult(
			fmt.Sprintf(
				"Belum cukup data untuk prediksi %s (butuh minimal 2 riwayat, ada %d).",
				item.Nama,
				len(riwayat),
			),
		)
	}
	var sumPct float64
	var cnt int
	for i := 1; i < len(riwayat); i++ {
		lama := riwayat[i-1].HargaBaru
		baru := riwayat[i].HargaBaru
		if lama > 0 {
			sumPct += float64(baru-lama) / float64(lama) * 100
			cnt++
		}
	}
	avg := 0.0
	if cnt > 0 {
		avg = sumPct / float64(cnt)
	}
	hargaKini := riwayat[len(riwayat)-1].HargaBaru
	tren := "stabil"
	proyeksi := hargaKini
	if avg > 2 {
		tren = "naik"
		proyeksi = int(float64(hargaKini) * (1 + avg/100))
	} else if avg < -2 {
		tren = "turun"
		proyeksi = int(float64(hargaKini) * (1 + avg/100))
	}
	return tools.NewToolResult(fmt.Sprintf("Tren harga %s: %s (rata-rata %.1f%%). Harga kini %s, proyeksi %s.",
		item.Nama, tren, avg, FormatRupiah(hargaKini), FormatRupiah(proyeksi)))
}
