package kios

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/sipeed/picoclaw/pkg/commands"
	"github.com/sipeed/picoclaw/pkg/cron"
	tools "github.com/sipeed/picoclaw/pkg/tools"
)

func TestSeedFromOldData(t *testing.T) {
	dir := "/home/kevinman/kios-openclaw/data"
	if _, err := os.Stat(filepath.Join(dir, "stok.csv")); err != nil {
		t.Skip("legacy data not present; skipping seed integration test")
	}
	t.Setenv("KIOS_SEED_DIR", dir)
	s := newTestStore(t)
	ctx := context.Background()
	if err := SeedFromOldData(ctx, s); err != nil {
		t.Fatalf("seed: %v", err)
	}
	produk, _ := s.GetAllProduk(ctx)
	if len(produk) == 0 {
		t.Error("expected products to be seeded from legacy stok.csv")
	}
	// Idempotent: second run is a no-op (seed:done set).
	if err := SeedFromOldData(ctx, s); err != nil {
		t.Fatalf("second seed: %v", err)
	}
}

func TestSlashCommands(t *testing.T) {
	s := newTestStore(t)
	seedProduct(t, s, "002", "Beras Medium 5kg", 4, 55000, 62000, 3)

	byName := map[string]commands.Definition{}
	for _, d := range Commands(s) {
		byName[d.Name] = d
	}
	for _, n := range []string{"stok", "menipis", "laporan", "harga"} {
		if _, ok := byName[n]; !ok {
			t.Fatalf("slash-command /%s missing", n)
		}
	}

	run := func(name, text string) string {
		var out string
		req := commands.Request{Text: text, Reply: func(s string) error { out = s; return nil }}
		if err := byName[name].Handler(context.Background(), req, nil); err != nil {
			t.Fatalf("/%s handler error: %v", name, err)
		}
		return out
	}
	if got := run("stok", "/stok"); !strings.Contains(got, "Beras") {
		t.Errorf("/stok should list Beras, got: %s", got)
	}
	if got := run("harga", "/harga beras"); !strings.Contains(got, "62.000") {
		t.Errorf("/harga beras should show price, got: %s", got)
	}
	if got := run("menipis", "/menipis"); !strings.Contains(got, "Beras") {
		t.Errorf("/menipis should flag Beras (stok 4 <= min 5), got: %s", got)
	}
}

func TestToolsRegister(t *testing.T) {
	s := newTestStore(t)
	reg := tools.NewToolRegistry()
	for _, tool := range AllTools(s) {
		reg.Register(tool)
	}
	for _, name := range []string{"kios_stok", "kios_kasir", "kios_laporan", "kios_harga", "kios_user", "kios_supplier", "kios_promo"} {
		if !reg.HasRegistered(name) {
			t.Errorf("tool %q not registered (registry: %v)", name, reg.List())
		}
	}
}

func TestSupplierTool(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	tool := NewSupplierTool(s)

	if res := tool.Execute(ctx, map[string]any{"action": "tambah", "nama": "UD Maju", "kontak": "0812", "produk_utama": "gula"}); res.IsError {
		t.Fatalf("tambah: %s", res.ForLLM)
	}
	if res := tool.Execute(ctx, map[string]any{"action": "tambah", "nama": "UD Maju"}); !res.IsError {
		t.Error("expected duplicate supplier error")
	}
	if res := tool.Execute(ctx, map[string]any{"action": "daftar"}); !strings.Contains(res.ForLLM, "UD Maju") {
		t.Errorf("daftar missing UD Maju: %s", res.ForLLM)
	}

	// Price comparison from purchase history.
	s.AppendPembelian(ctx, &Pembelian{NamaProduk: "Gula Pasir 1kg", HargaBeli: 13500, Supplier: "UD Maju"})
	s.AppendPembelian(ctx, &Pembelian{NamaProduk: "Gula Pasir 1kg", HargaBeli: 13000, Supplier: "Toko Beta"})
	res := tool.Execute(ctx, map[string]any{"action": "banding_harga", "produk": "gula"})
	if !strings.Contains(res.ForLLM, "termurah") || !strings.Contains(res.ForLLM, "Rp 13.000") {
		t.Errorf("banding_harga should mark Rp 13.000 cheapest, got: %s", res.ForLLM)
	}
	if res := tool.Execute(ctx, map[string]any{"action": "hapus", "nama": "UD Maju"}); res.IsError {
		t.Errorf("hapus: %s", res.ForLLM)
	}
}

func TestPromoTool(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "003", "Gula Pasir 1kg", 10, 13500, 15000, 2)
	tool := NewPromoTool(s)

	if res := tool.Execute(ctx, map[string]any{"action": "buat", "produk": "gula", "tipe": "persen", "nilai": float64(10), "min_qty": float64(2)}); res.IsError {
		t.Fatalf("buat: %s", res.ForLLM)
	}
	// qty below min_qty -> no promo.
	if res := tool.Execute(ctx, map[string]any{"action": "cek", "produk": "gula", "qty": float64(1)}); !strings.Contains(res.ForLLM, "Tidak ada promo") {
		t.Errorf("qty<min should yield no promo: %s", res.ForLLM)
	}
	// qty meets min -> 10%% of 15000 = 1500, final 13500.
	if res := tool.Execute(ctx, map[string]any{"action": "cek", "produk": "gula", "qty": float64(2)}); !strings.Contains(res.ForLLM, "Rp 13.500") {
		t.Errorf("expected final price Rp 13.500: %s", res.ForLLM)
	}
	if res := tool.Execute(ctx, map[string]any{"action": "daftar"}); !strings.Contains(res.ForLLM, "Gula") {
		t.Errorf("daftar missing promo: %s", res.ForLLM)
	}
}

func TestKasirPromoApplied(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "003", "Gula Pasir 1kg", 10, 13500, 15000, 2)
	NewPromoTool(s).Execute(ctx, map[string]any{"action": "buat", "produk": "gula", "tipe": "persen", "nilai": float64(10), "min_qty": float64(2)})

	res := NewKasirTool(s).Execute(ctx, map[string]any{"action": "jual", "produk": "gula", "qty": float64(2), "bayar": float64(30000)})
	if res.IsError {
		t.Fatalf("kasir jual: %s", res.ForLLM)
	}
	// subtotal 30.000, promo 10%%/unit (3.000 total) -> total 27.000, kembalian 3.000
	for _, want := range []string{"Promo", "Total: Rp 27.000", "Kembalian: Rp 3.000"} {
		if !strings.Contains(res.ForUser, want) {
			t.Errorf("struk missing %q, got: %s", want, res.ForUser)
		}
	}
	// Recorded transaction reflects the discounted total.
	txs, _ := s.GetAllTransaksi(ctx)
	if len(txs) != 1 || txs[0].Total != 27000 {
		t.Errorf("recorded tx total should be 27000, got %+v", txs)
	}
}

func TestUserTool(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	tool := NewUserTool(s)

	// Default role is owner, so management works.
	if res := tool.Execute(ctx, map[string]any{"action": "tambah", "id": "555", "nama": "Ken", "role": "kasir"}); res.IsError {
		t.Fatalf("tambah error: %s", res.ForLLM)
	}
	u, _ := s.GetUser(ctx, "555")
	if u == nil || u.Role != "kasir" || !u.Aktif {
		t.Fatalf("user 555 not stored correctly: %+v", u)
	}
	if res := tool.Execute(ctx, map[string]any{"action": "list"}); !strings.Contains(res.ForLLM, "Ken") {
		t.Errorf("list should include Ken, got: %s", res.ForLLM)
	}
	if res := tool.Execute(ctx, map[string]any{"action": "set_role", "id": "555", "role": "owner"}); res.IsError {
		t.Errorf("set_role error: %s", res.ForLLM)
	}
	if u, _ := s.GetUser(ctx, "555"); u.Role != "owner" {
		t.Errorf("role not updated to owner: %+v", u)
	}
	if res := tool.Execute(ctx, map[string]any{"action": "nonaktif", "id": "555"}); res.IsError {
		t.Errorf("nonaktif error: %s", res.ForLLM)
	}
	if u, _ := s.GetUser(ctx, "555"); u.Aktif {
		t.Errorf("user should be inactive after nonaktif")
	}
}

func TestDailyReportText(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "002", "Beras Medium 5kg", 10, 55000, 62000, 3)
	NewStokTool(s).Execute(ctx, map[string]any{"action": "jual", "produk": "beras", "qty": float64(2)})

	text := DailyReportText(ctx, s)
	if !strings.Contains(text, "Laporan") {
		t.Errorf("report should contain a heading, got: %s", text)
	}
	if !strings.Contains(text, "Rp 124.000") {
		t.Errorf("report should reflect omzet Rp 124.000, got: %s", text)
	}
}

func TestEnsureDailyReportJob(t *testing.T) {
	tmp := t.TempDir()
	mk := func() *cron.CronService { return cron.NewCronService(filepath.Join(tmp, "jobs.json"), nil) }

	// Not configured -> no job.
	cs := mk()
	if err := EnsureDailyReportJob(cs); err != nil {
		t.Fatal(err)
	}
	if n := len(cs.ListJobs(true)); n != 0 {
		t.Fatalf("expected 0 jobs when KIOS_REPORT_CHAT unset, got %d", n)
	}

	// Configured -> exactly one job, idempotent across calls.
	t.Setenv("KIOS_REPORT_CHAT", "123456")
	cs = mk()
	for i := 0; i < 2; i++ {
		if err := EnsureDailyReportJob(cs); err != nil {
			t.Fatal(err)
		}
	}
	jobs := cs.ListJobs(true)
	if len(jobs) != 1 || jobs[0].Name != DailyReportJobName {
		t.Fatalf("expected exactly one %q job, got %+v", DailyReportJobName, jobs)
	}
	if jobs[0].Payload.To != "123456" || jobs[0].Payload.Channel != "telegram" {
		t.Errorf("job payload target wrong: %+v", jobs[0].Payload)
	}
}

func TestUserToolOwnerOnly(t *testing.T) {
	t.Setenv("KIOS_DEFAULT_ROLE", "kasir")
	s := newTestStore(t)
	res := NewUserTool(s).Execute(context.Background(), map[string]any{"action": "list"})
	if !res.IsError || !strings.Contains(res.ForLLM, "owner") {
		t.Errorf("kasir must be refused on kios_user, got: %s (err=%v)", res.ForLLM, res.IsError)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return NewStoreWithClient(rdb)
}

func seedProduct(t *testing.T, s *Store, id, nama string, stok, beli, jual, kritis int) {
	t.Helper()
	if err := s.SetProduk(context.Background(), &Produk{
		ID: id, Nama: nama, Kategori: "umum", Satuan: "pcs",
		Stok: stok, HargaBeli: beli, HargaJual: jual, StokMinimum: 5, StokKritis: kritis,
	}); err != nil {
		t.Fatalf("seed product: %v", err)
	}
}

func TestFormatRupiah(t *testing.T) {
	cases := map[int]string{0: "Rp 0", 500: "Rp 500", 15000: "Rp 15.000", 62000: "Rp 62.000", 1500000: "Rp 1.500.000"}
	for in, want := range cases {
		if got := FormatRupiah(in); got != want {
			t.Errorf("FormatRupiah(%d)=%q want %q", in, got, want)
		}
	}
}

func TestParseRupiah(t *testing.T) {
	cases := map[string]int{"15000": 15000, "Rp 15.000": 15000, "1.500.000": 1500000, "15,000": 15000, "": 0, "abc": 0}
	for in, want := range cases {
		if got := parseRupiah(in); got != want {
			t.Errorf("parseRupiah(%q)=%d want %d", in, got, want)
		}
	}
}

func TestCariProduk(t *testing.T) {
	list := []*Produk{
		{ID: "002", Nama: "Beras Medium 5kg"},
		{ID: "003", Nama: "Gula Pasir 1kg"},
	}
	if got := CariProduk(list, "002"); len(got) != 1 || got[0].ID != "002" {
		t.Errorf("exact id match failed: %v", got)
	}
	if got := CariProduk(list, "beras"); len(got) != 1 || got[0].ID != "002" {
		t.Errorf("substring match failed: %v", got)
	}
	if got := CariProduk(list, "gula pasir"); len(got) != 1 || got[0].ID != "003" {
		t.Errorf("all-words match failed: %v", got)
	}
}

func TestPerformJual(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "002", "Beras Medium 5kg", 10, 55000, 62000, 3)

	tx, item, sisa, err := performJual(ctx, s, "beras", 2, "tunai", "ken", 0)
	if err != nil {
		t.Fatalf("performJual: %v", err)
	}
	if tx.Total != 124000 {
		t.Errorf("total=%d want 124000", tx.Total)
	}
	if sisa != 8 || item.Stok != 8 {
		t.Errorf("sisa=%d item.Stok=%d want 8", sisa, item.Stok)
	}
	if !strings.HasPrefix(tx.ID, "TRX-") {
		t.Errorf("tx id=%q want TRX- prefix", tx.ID)
	}
	// Persisted decrement
	got, _ := s.GetProduk(ctx, "002")
	if got.Stok != 8 {
		t.Errorf("persisted stok=%d want 8", got.Stok)
	}
	// Insufficient stock
	if _, _, _, err := performJual(ctx, s, "beras", 999, "tunai", "ken", 0); err == nil {
		t.Error("expected error for insufficient stock")
	}
	// Non-positive qty
	if _, _, _, err := performJual(ctx, s, "beras", 0, "tunai", "ken", 0); err == nil {
		t.Error("expected error for qty<=0")
	}
}

func TestStokToolJualAndMenipis(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "002", "Beras Medium 5kg", 4, 55000, 62000, 3)
	tool := NewStokTool(s)

	res := tool.Execute(ctx, map[string]any{"action": "jual", "produk": "beras", "qty": float64(2)})
	if res.IsError {
		t.Fatalf("jual error: %s", res.ForLLM)
	}
	if !strings.Contains(res.ForLLM, "menipis") {
		t.Errorf("expected low-stock warning, got: %s", res.ForLLM)
	}

	res = tool.Execute(ctx, map[string]any{"action": "stok_menipis"})
	if !strings.Contains(res.ForLLM, "Beras") {
		t.Errorf("stok_menipis should list Beras, got: %s", res.ForLLM)
	}
}

func TestStokToolTambahAutoCreate(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	tool := NewStokTool(s)

	res := tool.Execute(ctx, map[string]any{"action": "tambah", "produk": "Indomie Goreng", "qty": float64(20), "harga": float64(2800), "auto_create": true})
	if res.IsError {
		t.Fatalf("auto-create restock error: %s", res.ForLLM)
	}
	all, _ := s.GetAllProduk(ctx)
	if len(all) != 1 || all[0].Stok != 20 {
		t.Fatalf("expected 1 product stok 20, got %+v", all)
	}
	// margin 15%: 2800*1.15 = 3220
	if all[0].HargaJual != 3220 {
		t.Errorf("auto harga_jual=%d want 3220", all[0].HargaJual)
	}
	// Without auto_create on unknown product -> error
	res = tool.Execute(ctx, map[string]any{"action": "tambah", "produk": "Barang Asing", "qty": float64(1)})
	if !res.IsError {
		t.Error("expected error restocking unknown product without auto_create")
	}
}

func TestKasirStrukAndShift(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "003", "Gula Pasir 1kg", 10, 13500, 15000, 2)
	tool := NewKasirTool(s)

	res := tool.Execute(ctx, map[string]any{"action": "jual", "produk": "gula", "qty": float64(2), "bayar": float64(50000)})
	if res.IsError {
		t.Fatalf("kasir jual error: %s", res.ForLLM)
	}
	if res.ForUser == "" || !strings.Contains(res.ForUser, "STRUK") {
		t.Errorf("expected receipt in ForUser, got: %q", res.ForUser)
	}
	if !strings.Contains(res.ForUser, "Kembalian: Rp 20.000") {
		t.Errorf("expected kembalian 20.000 (50000-30000), got: %s", res.ForUser)
	}

	// Shift open/close
	if res := tool.Execute(ctx, map[string]any{"action": "buka_shift", "saldo_awal": float64(100000)}); res.IsError {
		t.Fatalf("buka_shift error: %s", res.ForLLM)
	}
	if res := tool.Execute(ctx, map[string]any{"action": "buka_shift"}); !res.IsError {
		t.Error("expected error opening a second shift")
	}
	res = tool.Execute(ctx, map[string]any{"action": "status_shift"})
	if !strings.Contains(res.ForLLM, "BUKA") {
		t.Errorf("status should report open shift, got: %s", res.ForLLM)
	}
}

func TestLaporanLaba(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "002", "Beras Medium 5kg", 10, 55000, 62000, 3)
	stok := NewStokTool(s)
	stok.Execute(ctx, map[string]any{"action": "jual", "produk": "beras", "qty": float64(2)})

	lap := NewLaporanTool(s)
	res := lap.Execute(ctx, map[string]any{"action": "laba", "periode": "hari_ini"})
	if res.IsError {
		t.Fatalf("laba error: %s", res.ForLLM)
	}
	// omzet 124000, modal 110000, laba 14000
	if !strings.Contains(res.ForLLM, "Rp 14.000") {
		t.Errorf("expected laba Rp 14.000, got: %s", res.ForLLM)
	}
}

func TestHargaUpdateLogsHistory(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "003", "Gula Pasir 1kg", 10, 13500, 15000, 2)
	tool := NewHargaTool(s)

	res := tool.Execute(ctx, map[string]any{"action": "update", "produk": "gula", "harga_jual": float64(16000)})
	if res.IsError {
		t.Fatalf("harga update error: %s", res.ForLLM)
	}
	hist, _ := s.GetAllPriceHistory(ctx)
	if len(hist) != 1 || hist[0].HargaBaru != 16000 || hist[0].Selisih != 1000 {
		t.Errorf("expected 1 price-history entry (16000, +1000), got %+v", hist)
	}
	got, _ := s.GetProduk(ctx, "003")
	if got.HargaJual != 16000 {
		t.Errorf("harga_jual not updated, got %d", got.HargaJual)
	}
}

func TestOwnerOnlyGate(t *testing.T) {
	t.Setenv("KIOS_DEFAULT_ROLE", "kasir")
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "002", "Beras Medium 5kg", 10, 55000, 62000, 3)
	tool := NewStokTool(s)

	// kasir (default) may sell
	if res := tool.Execute(ctx, map[string]any{"action": "jual", "produk": "beras", "qty": float64(1)}); res.IsError {
		t.Errorf("kasir should be allowed to sell, got: %s", res.ForLLM)
	}
	// kasir may NOT delete
	res := tool.Execute(ctx, map[string]any{"action": "hapus", "produk": "beras"})
	if !res.IsError || !strings.Contains(res.ForLLM, "owner") {
		t.Errorf("kasir must be refused on hapus, got: %s (err=%v)", res.ForLLM, res.IsError)
	}
}
