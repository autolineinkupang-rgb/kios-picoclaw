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
	return "Kelola supplier kios: tambah, edit, daftar, cari, hapus, dan bandingkan harga beli " +
		"sebuah produk antar supplier (riwayat pembelian + override). Tambah/edit owner+kasir; hapus khusus owner."
}

func (t *SupplierTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"tambah", "edit", "daftar", "cari", "hapus", "banding_harga"},
				"description": "Aksi supplier.",
			},
			"nama":         map[string]any{"type": "string", "description": "nama/id supplier"},
			"nama_baru":    map[string]any{"type": "string", "description": "nama baru (edit/ganti nama)"},
			"kontak":       map[string]any{"type": "string"},
			"alamat":       map[string]any{"type": "string"},
			"produk_utama": map[string]any{"type": "string"},
			"pic":          map[string]any{"type": "string", "description": "nama PIC/sales"},
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
		if r := requireStaff(role); r != nil {
			return r
		}
		return t.tambah(ctx, args)
	case "edit":
		if r := requireStaff(role); r != nil {
			return r
		}
		return t.edit(ctx, args)
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
		return tools.ErrorResult("Hmm, aksi supplier belum dikenal kak 🤔")
	}
}

func (t *SupplierTool) tambah(ctx context.Context, args map[string]any) *tools.ToolResult {
	nama := argStr(args, "nama")
	if nama == "" {
		return tools.ErrorResult("Nama supplier-nya diisi dulu ya kak 🙏")
	}
	all, err := t.store.GetAllSupplier(ctx)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal baca supplier kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	if CariSupplier(all, nama) != nil {
		return tools.ErrorResult(fmt.Sprintf("Supplier %q sudah terdaftar kak.", nama))
	}
	id, err := t.store.NextSupplierID(ctx)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal buat id supplier kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	sup := &Supplier{
		ID: id, Nama: nama, Kontak: argStr(args, "kontak"), Alamat: argStr(args, "alamat"),
		ProdukUtama: argStr(args, "produk_utama"), Pic: argStr(args, "pic"), Catatan: argStr(args, "catatan"),
	}
	if err := t.store.SetSupplier(ctx, sup); err != nil {
		return tools.ErrorResult("Aduh, gagal simpan supplier kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	return tools.NewToolResult(fmt.Sprintf("Supplier ditambahkan: [%s] %s (kontak: %s).", sup.ID, sup.Nama, sup.Kontak))
}

func (t *SupplierTool) edit(ctx context.Context, args map[string]any) *tools.ToolResult {
	all, err := t.store.GetAllSupplier(ctx)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal baca supplier kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	sup := CariSupplier(all, argStr(args, "nama"))
	if sup == nil {
		return tools.NewToolResult("Supplier nggak ketemu kak 🔍")
	}
	var changed []string
	if v := argStr(args, "nama_baru"); v != "" {
		sup.Nama = v
		changed = append(changed, "nama")
	}
	if v := argStr(args, "kontak"); v != "" {
		sup.Kontak = v
		changed = append(changed, "kontak")
	}
	if v := argStr(args, "alamat"); v != "" {
		sup.Alamat = v
		changed = append(changed, "alamat")
	}
	if v := argStr(args, "produk_utama"); v != "" {
		sup.ProdukUtama = v
		changed = append(changed, "produk_utama")
	}
	if v := argStr(args, "pic"); v != "" {
		sup.Pic = v
		changed = append(changed, "pic")
	}
	if v := argStr(args, "catatan"); v != "" {
		sup.Catatan = v
		changed = append(changed, "catatan")
	}
	if len(changed) == 0 {
		return tools.ErrorResult("Tidak ada field yang diubah. Sebutkan nama_baru/kontak/alamat/produk_utama/pic/catatan.")
	}
	if err := t.store.SetSupplier(ctx, sup); err != nil {
		return tools.ErrorResult("Aduh, gagal simpan supplier kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	return tools.NewToolResult(fmt.Sprintf("Supplier %s ([%s]) diperbarui: %s.", sup.Nama, sup.ID, strings.Join(changed, ", ")))
}

func (t *SupplierTool) daftar(ctx context.Context) *tools.ToolResult {
	all, err := t.store.GetAllSupplier(ctx)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal baca supplier kak 😣 Coba lagi sebentar ya.").WithError(err)
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
		return tools.ErrorResult("Aduh, gagal baca supplier kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	sup := CariSupplier(all, argStr(args, "nama"))
	if sup == nil {
		return tools.NewToolResult("Supplier nggak ketemu kak 🔍")
	}
	produk, _ := t.store.GetAllProduk(ctx)
	var supplied []string
	for _, p := range produk {
		if strings.EqualFold(p.Supplier, sup.Nama) {
			supplied = append(supplied, p.Nama)
		}
	}
	msg := fmt.Sprintf("[%s] %s\nKontak: %s\nAlamat: %s\nPIC: %s\nProduk utama: %s\nCatatan: %s",
		sup.ID, sup.Nama, sup.Kontak, sup.Alamat, sup.Pic, sup.ProdukUtama, sup.Catatan)
	if len(supplied) > 0 {
		msg += "\nMenyuplai: " + strings.Join(supplied, ", ")
	}
	return tools.NewToolResult(msg)
}

func (t *SupplierTool) hapus(ctx context.Context, args map[string]any) *tools.ToolResult {
	all, err := t.store.GetAllSupplier(ctx)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal baca supplier kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	sup := CariSupplier(all, argStr(args, "nama"))
	if sup == nil {
		return tools.NewToolResult("Supplier nggak ketemu kak 🔍")
	}
	// Guard: blokir hapus bila ada hutang terbuka ke supplier ini
	allHut, err := t.store.GetAllHutang(ctx)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal baca hutang kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	for _, h := range allHut {
		if h.SupplierID == sup.ID && h.Status == "terbuka" {
			return tools.ErrorResult(fmt.Sprintf(
				"Supplier %s masih punya hutang terbuka %s (%s) — lunasi atau write-off dulu kak.",
				sup.Nama, h.ID, FormatRupiah(h.Sisa)))
		}
	}
	if err := t.store.DelSupplier(ctx, sup.ID); err != nil {
		return tools.ErrorResult("Aduh, gagal hapus supplier kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	return tools.NewToolResult(fmt.Sprintf("Supplier %s ([%s]) dihapus.", sup.Nama, sup.ID))
}

// bandingHarga compares the best purchase price per supplier for a product,
// using purchase history plus the product's current supplier.
func (t *SupplierTool) bandingHarga(ctx context.Context, args map[string]any) *tools.ToolResult {
	produk := argStr(args, "produk")
	if produk == "" {
		return tools.ErrorResult("Nama produk-nya diisi dulu ya kak 🙏")
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
	// Override harga manual diutamakan (mengalahkan riwayat pembelian).
	if produkID != "" {
		if overrides, err := t.store.GetAllHargaSupplier(ctx); err == nil {
			for field, harga := range overrides {
				parts := strings.SplitN(field, "|", 2)
				if len(parts) == 2 && parts[0] == produkID && harga > 0 {
					best[parts[1]] = harga
				}
			}
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
