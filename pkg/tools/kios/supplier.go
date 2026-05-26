package kios

import (
	"context"
	"fmt"
	"sort"
	"strings"

	tools "github.com/sipeed/picoclaw/pkg/tools"
)

// SupplierTool manages suppliers and compares purchase prices.
type SupplierTool struct{ store *Store }

func (t *SupplierTool) Name() string { return "kios_supplier" }

func (t *SupplierTool) Description() string {
	return "Kelola supplier kios: tambah, daftar, cari, hapus, dan bandingkan harga beli " +
		"sebuah produk antar supplier (dari riwayat pembelian). Tambah/hapus khusus owner."
}

func (t *SupplierTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"tambah", "daftar", "cari", "hapus", "banding_harga"},
				"description": "Aksi supplier.",
			},
			"nama":         map[string]any{"type": "string", "description": "nama/id supplier"},
			"kontak":       map[string]any{"type": "string"},
			"alamat":       map[string]any{"type": "string"},
			"produk_utama": map[string]any{"type": "string"},
			"catatan":      map[string]any{"type": "string"},
			"produk":       map[string]any{"type": "string", "description": "produk untuk banding_harga"},
		},
		"required": []string{"action"},
	}
}

func (t *SupplierTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	role, _, refusal := resolveRole(ctx, t.store)
	if refusal != nil {
		return refusal
	}
	switch argStr(args, "action") {
	case "tambah":
		if r := requireOwner(role); r != nil {
			return r
		}
		return t.tambah(ctx, args)
	case "daftar":
		return t.daftar(ctx)
	case "cari":
		return t.cari(ctx, args)
	case "hapus":
		if r := requireOwner(role); r != nil {
			return r
		}
		return t.hapus(ctx, args)
	case "banding_harga":
		return t.bandingHarga(ctx, args)
	default:
		return tools.ErrorResult("Aksi supplier tidak dikenal.")
	}
}

func (t *SupplierTool) tambah(ctx context.Context, args map[string]any) *tools.ToolResult {
	nama := argStr(args, "nama")
	if nama == "" {
		return tools.ErrorResult("Nama supplier wajib diisi.")
	}
	all, err := t.store.GetAllSupplier(ctx)
	if err != nil {
		return tools.ErrorResult("Gagal baca supplier.").WithError(err)
	}
	if CariSupplier(all, nama) != nil {
		return tools.ErrorResult(fmt.Sprintf("Supplier %q sudah terdaftar kak.", nama))
	}
	id, err := t.store.NextSupplierID(ctx)
	if err != nil {
		return tools.ErrorResult("Gagal buat id supplier.").WithError(err)
	}
	sup := &Supplier{
		ID: id, Nama: nama, Kontak: argStr(args, "kontak"), Alamat: argStr(args, "alamat"),
		ProdukUtama: argStr(args, "produk_utama"), Catatan: argStr(args, "catatan"),
	}
	if err := t.store.SetSupplier(ctx, sup); err != nil {
		return tools.ErrorResult("Gagal simpan supplier.").WithError(err)
	}
	return tools.NewToolResult(fmt.Sprintf("Supplier ditambahkan: [%s] %s (kontak: %s).", sup.ID, sup.Nama, sup.Kontak))
}

func (t *SupplierTool) daftar(ctx context.Context) *tools.ToolResult {
	all, err := t.store.GetAllSupplier(ctx)
	if err != nil {
		return tools.ErrorResult("Gagal baca supplier.").WithError(err)
	}
	if len(all) == 0 {
		return tools.NewToolResult("Belum ada supplier terdaftar.")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Daftar supplier (%d):\n", len(all))
	for _, s := range all {
		fmt.Fprintf(&b, "- [%s] %s — %s | %s\n", s.ID, s.Nama, s.Kontak, s.ProdukUtama)
	}
	return tools.NewToolResult(b.String())
}

func (t *SupplierTool) cari(ctx context.Context, args map[string]any) *tools.ToolResult {
	all, err := t.store.GetAllSupplier(ctx)
	if err != nil {
		return tools.ErrorResult("Gagal baca supplier.").WithError(err)
	}
	sup := CariSupplier(all, argStr(args, "nama"))
	if sup == nil {
		return tools.NewToolResult("Supplier tidak ditemukan.")
	}
	produk, _ := t.store.GetAllProduk(ctx)
	var supplied []string
	for _, p := range produk {
		if strings.EqualFold(p.Supplier, sup.Nama) {
			supplied = append(supplied, p.Nama)
		}
	}
	msg := fmt.Sprintf("[%s] %s\nKontak: %s\nAlamat: %s\nProduk utama: %s\nCatatan: %s",
		sup.ID, sup.Nama, sup.Kontak, sup.Alamat, sup.ProdukUtama, sup.Catatan)
	if len(supplied) > 0 {
		msg += "\nMenyuplai: " + strings.Join(supplied, ", ")
	}
	return tools.NewToolResult(msg)
}

func (t *SupplierTool) hapus(ctx context.Context, args map[string]any) *tools.ToolResult {
	all, err := t.store.GetAllSupplier(ctx)
	if err != nil {
		return tools.ErrorResult("Gagal baca supplier.").WithError(err)
	}
	sup := CariSupplier(all, argStr(args, "nama"))
	if sup == nil {
		return tools.NewToolResult("Supplier tidak ditemukan.")
	}
	if err := t.store.DelSupplier(ctx, sup.ID); err != nil {
		return tools.ErrorResult("Gagal hapus supplier.").WithError(err)
	}
	return tools.NewToolResult(fmt.Sprintf("Supplier %s ([%s]) dihapus.", sup.Nama, sup.ID))
}

// bandingHarga compares the best purchase price per supplier for a product,
// using purchase history plus the product's current supplier.
func (t *SupplierTool) bandingHarga(ctx context.Context, args map[string]any) *tools.ToolResult {
	produk := argStr(args, "produk")
	if produk == "" {
		return tools.ErrorResult("Nama produk wajib diisi.")
	}
	item, _ := findOne(ctx, t.store, produk)
	namaProduk := produk
	produkID := ""
	if item != nil {
		namaProduk = item.Nama
		produkID = item.ID
	}

	best := map[string]int{} // supplier -> lowest harga_beli
	pembelian, _ := t.store.GetAllPembelian(ctx)
	q := strings.ToLower(produk)
	for _, p := range pembelian {
		match := (produkID != "" && p.ProdukID == produkID) || strings.Contains(strings.ToLower(p.NamaProduk), q)
		if !match || p.HargaBeli <= 0 {
			continue
		}
		sup := p.Supplier
		if sup == "" {
			sup = "(tanpa supplier)"
		}
		if cur, ok := best[sup]; !ok || p.HargaBeli < cur {
			best[sup] = p.HargaBeli
		}
	}
	if item != nil && item.Supplier != "" && item.HargaBeli > 0 {
		if cur, ok := best[item.Supplier]; !ok || item.HargaBeli < cur {
			best[item.Supplier] = item.HargaBeli
		}
	}
	if len(best) == 0 {
		return tools.NewToolResult(fmt.Sprintf("Belum ada data harga beli untuk %s.", namaProduk))
	}

	type row struct {
		sup   string
		harga int
	}
	rows := make([]row, 0, len(best))
	for s, h := range best {
		rows = append(rows, row{s, h})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].harga < rows[j].harga })

	var b strings.Builder
	fmt.Fprintf(&b, "Perbandingan harga beli %s:\n", namaProduk)
	for i, r := range rows {
		mark := ""
		if i == 0 {
			mark = " ⭐ termurah"
		}
		fmt.Fprintf(&b, "- %s: %s%s\n", r.sup, FormatRupiah(r.harga), mark)
	}
	return tools.NewToolResult(b.String())
}
