package kios

import (
	"context"
	"fmt"
	"sort"
	"strings"

	tools "github.com/sipeed/picoclaw/pkg/tools"
)

// LaporanTool produces sales reports and analytics (read-only).
type LaporanTool struct{ store *Store }

func (t *LaporanTool) Name() string { return "kios_laporan" }

func (t *LaporanTool) Description() string {
	return "Laporan kios: ringkas harian, mingguan, bulanan, laba, riwayat transaksi, " +
		"produk terlaris, dan riwayat perubahan harga."
}

func (t *LaporanTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type": "string",
				"enum": []string{
					"ringkas",
					"mingguan",
					"bulanan",
					"laba",
					"riwayat",
					"terlaris",
					"riwayat_harga",
				},
				"description": "Jenis laporan.",
			},
			"periode": map[string]any{
				"type":        "string",
				"enum":        []string{"hari_ini", "minggu", "bulan"},
				"description": "periode untuk laba/riwayat/terlaris",
			},
			"produk": map[string]any{"type": "string", "description": "filter produk (riwayat_harga)"},
			"top":    map[string]any{"type": "integer", "description": "jumlah produk terlaris (default 10)"},
		},
		"required": []string{"action"},
	}
}

func (t *LaporanTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	if _, _, refusal := resolveRole(ctx, t.store); refusal != nil {
		return refusal
	}
	// Automatic learning: note when reports are requested (builds report-time habit).
	_ = t.store.TrackHabit(ctx, "report_request", "")
	switch argStr(args, "action") {
	case "ringkas":
		return t.ringkas(ctx, "hari_ini", "Harian")
	case "mingguan":
		return t.ringkas(ctx, "minggu", "Mingguan")
	case "bulanan":
		return t.ringkas(ctx, "bulan", "Bulanan")
	case "laba":
		return t.laba(ctx, periodeOf(args))
	case "riwayat":
		return t.riwayat(ctx, periodeOf(args))
	case "terlaris":
		return t.terlaris(ctx, args)
	case "riwayat_harga":
		return t.riwayatHarga(ctx, argStr(args, "produk"))
	default:
		return tools.ErrorResult("Hmm, aksi laporan belum dikenal kak 🤔")
	}
}

func periodeOf(args map[string]any) string {
	p := argStr(args, "periode")
	if p == "" {
		return "hari_ini"
	}
	return p
}

// txPeriode returns transactions within the named period.
func (t *LaporanTool) txPeriode(ctx context.Context, periode string) ([]*Transaksi, error) {
	all, err := t.store.GetAllTransaksi(ctx)
	if err != nil {
		return nil, err
	}
	now := NowWITA()
	var out []*Transaksi
	switch periode {
	case "minggu":
		cutoff := now.AddDate(0, 0, -6).Format("2006-01-02")
		for _, tx := range all {
			if tx.Tanggal >= cutoff {
				out = append(out, tx)
			}
		}
	case "bulan":
		prefix := now.Format("2006-01")
		for _, tx := range all {
			if strings.HasPrefix(tx.Tanggal, prefix) {
				out = append(out, tx)
			}
		}
	default: // hari_ini
		today := now.Format("2006-01-02")
		for _, tx := range all {
			if tx.Tanggal == today {
				out = append(out, tx)
			}
		}
	}
	return out, nil
}

// hitungLaba computes omzet, modal, laba for a transaction set.
func (t *LaporanTool) hitungLaba(ctx context.Context, txs []*Transaksi) (omzet, modal, laba int) {
	all, _ := t.store.GetAllProduk(ctx)
	beli := make(map[string]int, len(all))
	for _, p := range all {
		beli[p.ID] = p.HargaBeli
	}
	for _, tx := range txs {
		omzet += tx.Total
		modal += tx.Qty * beli[tx.ProdukID]
	}
	return omzet, modal, omzet - modal
}

func (t *LaporanTool) stokKritis(ctx context.Context) []string {
	all, _ := t.store.GetAllProduk(ctx)
	var out []string
	for _, p := range all {
		if p.Stok <= p.StokKritis {
			out = append(out, p.Nama)
		}
	}
	return out
}

func topProduk(txs []*Transaksi, n int) []string {
	qty := map[string]int{}
	for _, tx := range txs {
		qty[tx.NamaProduk] += tx.Qty
	}
	type kv struct {
		nama string
		q    int
	}
	arr := make([]kv, 0, len(qty))
	for k, v := range qty {
		arr = append(arr, kv{k, v})
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i].q > arr[j].q })
	out := make([]string, 0, min(len(arr), n))
	for i := 0; i < len(arr) && i < n; i++ {
		out = append(out, arr[i].nama)
	}
	return out
}

func (t *LaporanTool) ringkas(ctx context.Context, periode, label string) *tools.ToolResult {
	txs, err := t.txPeriode(ctx, periode)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal baca transaksi kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	omzet, _, laba := t.hitungLaba(ctx, txs)
	top := topProduk(txs, 3)
	kritis := t.stokKritis(ctx)
	var b strings.Builder
	fmt.Fprintf(&b, "📊 Laporan %s\n", label)
	fmt.Fprintf(&b, "Transaksi: %d\nOmzet: %s\nLaba: %s\n", len(txs), FormatRupiah(omzet), FormatRupiah(laba))
	if len(top) > 0 {
		fmt.Fprintf(&b, "Terlaris: %s\n", strings.Join(top, ", "))
	}
	if len(kritis) > 0 {
		fmt.Fprintf(&b, "⚠️ Stok kritis: %s\n", strings.Join(kritis, ", "))
	}
	return tools.NewToolResult(b.String())
}

func (t *LaporanTool) laba(ctx context.Context, periode string) *tools.ToolResult {
	txs, err := t.txPeriode(ctx, periode)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal baca transaksi kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	omzet, modal, laba := t.hitungLaba(ctx, txs)
	return tools.NewToolResult(fmt.Sprintf("Laba %s — transaksi %d, omzet %s, modal %s, laba %s.",
		periode, len(txs), FormatRupiah(omzet), FormatRupiah(modal), FormatRupiah(laba)))
}

func (t *LaporanTool) riwayat(ctx context.Context, periode string) *tools.ToolResult {
	txs, err := t.txPeriode(ctx, periode)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal baca transaksi kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	if len(txs) == 0 {
		return tools.NewToolResult("Belum ada transaksi pada periode ini.")
	}
	start := 0
	if len(txs) > 20 {
		start = len(txs) - 20
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Riwayat transaksi (%d terbaru):\n", len(txs)-start)
	for _, tx := range txs[start:] {
		fmt.Fprintf(
			&b,
			"- %s %s: %s x%d = %s (%s)\n",
			tx.ID,
			tx.Jam,
			tx.NamaProduk,
			tx.Qty,
			FormatRupiah(tx.Total),
			tx.MetodeBayar,
		)
	}
	return tools.NewToolResult(b.String())
}

func (t *LaporanTool) terlaris(ctx context.Context, args map[string]any) *tools.ToolResult {
	periode := periodeOf(args)
	topN := argInt(args, "top")
	if topN <= 0 {
		topN = 10
	}
	txs, err := t.txPeriode(ctx, periode)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal baca transaksi kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	type agg struct {
		nama  string
		qty   int
		omzet int
	}
	m := map[string]*agg{}
	for _, tx := range txs {
		a := m[tx.NamaProduk]
		if a == nil {
			a = &agg{nama: tx.NamaProduk}
			m[tx.NamaProduk] = a
		}
		a.qty += tx.Qty
		a.omzet += tx.Total
	}
	arr := make([]*agg, 0, len(m))
	for _, a := range m {
		arr = append(arr, a)
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i].qty > arr[j].qty })
	var b strings.Builder
	fmt.Fprintf(&b, "🏆 Produk terlaris (%s):\n", periode)
	for i := 0; i < len(arr) && i < topN; i++ {
		fmt.Fprintf(&b, "%d. %s — %d terjual, %s\n", i+1, arr[i].nama, arr[i].qty, FormatRupiah(arr[i].omzet))
	}
	if len(arr) == 0 {
		return tools.NewToolResult("Belum ada penjualan pada periode ini.")
	}
	return tools.NewToolResult(b.String())
}

func (t *LaporanTool) riwayatHarga(ctx context.Context, produk string) *tools.ToolResult {
	hist, err := t.store.GetAllPriceHistory(ctx)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal baca riwayat harga kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	q := strings.ToLower(strings.TrimSpace(produk))
	var filtered []*PriceHistory
	for _, h := range hist {
		if q == "" || strings.Contains(strings.ToLower(h.NamaProduk), q) {
			filtered = append(filtered, h)
		}
	}
	if len(filtered) == 0 {
		return tools.NewToolResult("Belum ada riwayat perubahan harga.")
	}
	start := 0
	if len(filtered) > 20 {
		start = len(filtered) - 20
	}
	var b strings.Builder
	b.WriteString("Riwayat perubahan harga:\n")
	for _, h := range filtered[start:] {
		fmt.Fprintf(
			&b,
			"- %s %s: %s → %s (%+d)\n",
			h.Tanggal,
			h.NamaProduk,
			FormatRupiah(h.HargaLama),
			FormatRupiah(h.HargaBaru),
			h.Selisih,
		)
	}
	return tools.NewToolResult(b.String())
}
