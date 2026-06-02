package kios

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/sipeed/picoclaw/pkg/commands"
	"github.com/sipeed/picoclaw/pkg/cron"
	"github.com/sipeed/picoclaw/pkg/media"
	tools "github.com/sipeed/picoclaw/pkg/tools"
	toolshared "github.com/sipeed/picoclaw/pkg/tools/shared"
)

// fakeMediaStore maps a single ref to a local file for upload tests.
type fakeMediaStore struct {
	ref  string
	path string
	meta media.MediaMeta
}

func (f *fakeMediaStore) Store(string, media.MediaMeta, string) (string, error) { return f.ref, nil }
func (f *fakeMediaStore) Resolve(ref string) (string, error) {
	if ref == f.ref {
		return f.path, nil
	}
	return "", fmt.Errorf("not found")
}
func (f *fakeMediaStore) ResolveWithMeta(ref string) (string, media.MediaMeta, error) {
	if ref == f.ref {
		return f.path, f.meta, nil
	}
	return "", media.MediaMeta{}, fmt.Errorf("not found")
}
func (f *fakeMediaStore) ReleaseAll(string) error { return nil }

func TestReadTableFileXLSX(t *testing.T) {
	p, _ := filepath.Abs("../../../templates/produk-template.xlsx")
	if _, err := os.Stat(p); err != nil {
		t.Skip("template xlsx not present")
	}
	rows, err := ReadTableFile(p)
	if err != nil {
		t.Fatalf("read xlsx: %v", err)
	}
	if len(rows) < 4 {
		t.Fatalf("expected >=4 rows, got %d", len(rows))
	}
	if rows[0]["nama"] == "" || rows[0]["harga_jual"] == "" {
		t.Errorf("xlsx headers/values not parsed: %+v", rows[0])
	}
}

func TestUploadTool(t *testing.T) {
	p, _ := filepath.Abs("../../../templates/produk-template.xlsx")
	if _, err := os.Stat(p); err != nil {
		t.Skip("template xlsx not present")
	}
	s := newTestStore(t)
	tool := NewUploadTool(s)
	tool.SetMediaStore(&fakeMediaStore{
		ref:  "media://x",
		path: p,
		meta: media.MediaMeta{Filename: "daftar-produk.xlsx", ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
	})
	ctx := tools.WithToolMedia(context.Background(), []string{"media://x"})
	res := tool.Execute(ctx, map[string]any{}) // tipe auto-detected
	if res.IsError {
		t.Fatalf("upload import: %s", res.ForLLM)
	}
	if !strings.Contains(res.ForUser, "Dibuat") {
		t.Errorf("expected import summary, got: %s", res.ForUser)
	}
	all, _ := s.GetAllProduk(context.Background())
	if len(all) < 4 {
		t.Errorf("expected >=4 products imported from xlsx, got %d", len(all))
	}

	// No uploaded file -> friendly hint, not an error.
	res2 := tool.Execute(tools.WithToolMedia(context.Background(), nil), map[string]any{})
	if res2.IsError || !strings.Contains(res2.ForLLM, "unggah") {
		t.Errorf("expected upload hint when no file, got: %s (err=%v)", res2.ForLLM, res2.IsError)
	}
}

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
	for _, n := range []string{"stok", "menipis", "laporan", "harga", "jual", "jualmassal", "shift", "promo", "pasar", "produk", "suplier", "template"} {
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
	// /jual validation: missing qty -> usage hint.
	if got := run("jual", "/jual beras"); !strings.Contains(got, "Pakai: /jual") {
		t.Errorf("/jual without qty should show usage, got: %s", got)
	}
	// /jual functional: sells and returns a receipt.
	if got := run("jual", "/jual beras 2"); !strings.Contains(got, "STRUK") {
		t.Errorf("/jual beras 2 should return a struk, got: %s", got)
	}
	// /produk lists products.
	if got := run("produk", "/produk"); !strings.Contains(got, "Beras") {
		t.Errorf("/produk should list Beras, got: %s", got)
	}
	// /suplier: add a supplier first, then list + comparison subcommand.
	NewSupplierTool(s).Execute(context.Background(), map[string]any{"action": "tambah", "nama": "UD Maju", "kontak": "0812"})
	s.AppendPembelian(context.Background(), &Pembelian{NamaProduk: "Beras Medium 5kg", HargaBeli: 55000, Supplier: "UD Maju"})
	if got := run("suplier", "/suplier"); !strings.Contains(got, "UD Maju") {
		t.Errorf("/suplier should list UD Maju, got: %s", got)
	}
	if got := run("suplier", "/suplier banding beras"); !strings.Contains(got, "termurah") {
		t.Errorf("/suplier banding should compare prices, got: %s", got)
	}
}

func TestMenuDefinitionsIncludesKios(t *testing.T) {
	s := newTestStore(t)
	commands.SetExtraDefinitions(Commands(s))
	names := map[string]bool{}
	for _, d := range commands.MenuDefinitions() {
		names[d.Name] = true
	}
	for _, n := range []string{"help", "stok", "jual", "pasar"} { // builtin + kios
		if !names[n] {
			t.Errorf("MenuDefinitions missing /%s", n)
		}
	}
}

func TestImportProdukCSV(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "produk.csv")
	csvData := "nama,barcode,kategori,satuan,stok,harga_beli,harga_jual,stok_minimum,stok_kritis,supplier\n" +
		"Beras Medium 5kg,8991234500015,sembako,karung,20,55000,62000,10,3,UD Maju\n" +
		"Gula Pasir 1kg,,sembako,bungkus,15,13500,15000,8,2,Distributor\n" +
		",,,,,,,,,\n" // blank name -> skipped
	if err := os.WriteFile(path, []byte(csvData), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := ImportProdukCSV(ctx, s, path)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.Created != 2 || res.Skipped != 1 {
		t.Errorf("expected 2 created / 1 skipped, got %+v", res)
	}
	all, _ := s.GetAllProduk(ctx)
	if len(all) != 2 {
		t.Fatalf("expected 2 products, got %d", len(all))
	}
	var beras *Produk
	for _, p := range all {
		if p.Nama == "Beras Medium 5kg" {
			beras = p
		}
	}
	if beras == nil || beras.HargaJual != 62000 || beras.Stok != 20 || beras.Supplier != "UD Maju" || beras.Barcode != "8991234500015" {
		t.Errorf("beras fields wrong: %+v", beras)
	}
	// Re-import updates existing (matched by name), no duplicates.
	res2, _ := ImportProdukCSV(ctx, s, path)
	if res2.Updated != 2 || res2.Created != 0 {
		t.Errorf("re-import should update 2, got %+v", res2)
	}
	if all2, _ := s.GetAllProduk(ctx); len(all2) != 2 {
		t.Errorf("re-import should not duplicate, got %d", len(all2))
	}
}

func TestImportPustakaRows(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	rows := []map[string]string{
		{"judul": "Panel Harga", "info": "harga harian", "url": "https://panelharga.badanpangan.go.id", "kategori": "harga"},
		{"judul": "Sumber Jahat", "url": "http://1.2.3.4/malware?exec=1"}, // unsafe -> skipped
		{"judul": "Catatan tanpa URL", "info": "info lokal"},              // ok, no url
		{"info": "tanpa judul tanpa url"},                                 // skipped
	}
	res, err := ImportPustakaRows(ctx, s, rows)
	if err != nil {
		t.Fatalf("import pustaka: %v", err)
	}
	if res.Created != 2 || res.Skipped != 2 {
		t.Errorf("expected 2 created / 2 skipped, got %+v", res)
	}
	all, _ := s.GetAllPustaka(ctx)
	if len(all) != 2 {
		t.Errorf("expected 2 stored entries (unsafe rejected), got %d", len(all))
	}
}

func TestImportCuratedSources(t *testing.T) {
	p, _ := filepath.Abs("../../../templates/pustaka-sumber.csv")
	if _, err := os.Stat(p); err != nil {
		t.Skip("curated sources file not present")
	}
	s := newTestStore(t)
	res, err := ImportPustakaCSV(context.Background(), s, p)
	if err != nil {
		t.Fatalf("import curated: %v", err)
	}
	if res.Skipped != 0 {
		t.Errorf("curated sources must all pass safety, but %d were skipped", res.Skipped)
	}
	if res.Created < 8 {
		t.Errorf("expected >=8 curated sources imported, got %d", res.Created)
	}
}

func TestImportSupplierCSV(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "sup.csv")
	if err := os.WriteFile(path, []byte("nama,kontak,produk_utama\nUD Maju,0812,beras\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := ImportSupplierCSV(ctx, s, path)
	if err != nil || res.Created != 1 {
		t.Fatalf("supplier import: %+v err=%v", res, err)
	}
	all, _ := s.GetAllSupplier(ctx)
	if sup := CariSupplier(all, "UD Maju"); sup == nil || sup.Kontak != "0812" {
		t.Errorf("supplier not imported correctly: %+v", sup)
	}
}

func TestUploadAllowedForKasir(t *testing.T) {
	p, _ := filepath.Abs("../../../templates/produk-template.xlsx")
	if _, err := os.Stat(p); err != nil {
		t.Skip("template xlsx not present")
	}
	t.Setenv("KIOS_DEFAULT_ROLE", "kasir") // non-owner
	s := newTestStore(t)
	tool := NewUploadTool(s)
	tool.SetMediaStore(&fakeMediaStore{ref: "m", path: p, meta: media.MediaMeta{Filename: "produk.xlsx"}})
	res := tool.Execute(tools.WithToolMedia(context.Background(), []string{"m"}), map[string]any{})
	if res.IsError {
		t.Errorf("kasir (active user) should be allowed to upload, got: %s", res.ForLLM)
	}
}

func TestTemplateCommand(t *testing.T) {
	dir, _ := filepath.Abs("../../../templates")
	if _, err := os.Stat(filepath.Join(dir, "produk-template.xlsx")); err != nil {
		t.Skip("template files not present")
	}
	t.Setenv("KIOS_TEMPLATE_DIR", dir)
	s := newTestStore(t)
	var def commands.Definition
	for _, d := range Commands(s) {
		if d.Name == "template" {
			def = d
		}
	}
	if def.Handler == nil {
		t.Fatal("/template command missing")
	}
	var sent []string
	rt := &commands.Runtime{
		SendFile: func(_ context.Context, _, _, _, name string) error {
			sent = append(sent, name)
			return nil
		},
	}
	var replyText string
	req := commands.Request{Channel: "telegram", ChatID: "1", Reply: func(s string) error { replyText = s; return nil }}
	if err := def.Handler(context.Background(), req, rt); err != nil {
		t.Fatalf("/template: %v", err)
	}
	if len(sent) < 3 {
		t.Errorf("expected 3 template files sent (produk/supplier/pustaka), got %v", sent)
	}
	if !strings.Contains(replyText, "Template terkirim") {
		t.Errorf("unexpected reply: %s", replyText)
	}
}

// /qris must send the QR as an actual scannable photo (via SendImageURL, which
// has Telegram fetch the URL) — not a raw URL in a text reply.
func TestQrisCommandSendsImage(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	cfg := s.GetConfig(ctx)
	cfg.QrisEnabled = true
	cfg.QrisNama = "Kios Test"
	cfg.QrisImageURL = "https://cdn.example.com/qris.png"
	if err := s.SaveConfig(ctx, cfg); err != nil {
		t.Fatal(err)
	}

	var def commands.Definition
	for _, d := range Commands(s) {
		if d.Name == "qris" {
			def = d
		}
	}
	if def.Handler == nil {
		t.Fatal("/qris command missing")
	}

	var gotURL, gotCaption, replyText string
	rt := &commands.Runtime{
		SendImageURL: func(_ context.Context, _, _, url, caption string) error {
			gotURL, gotCaption = url, caption
			return nil
		},
	}
	req := commands.Request{Channel: "telegram", ChatID: "1", Reply: func(s string) error { replyText = s; return nil }}
	if err := def.Handler(ctx, req, rt); err != nil {
		t.Fatalf("/qris: %v", err)
	}
	if gotURL != "https://cdn.example.com/qris.png" {
		t.Errorf("QR image not sent as photo, gotURL=%q", gotURL)
	}
	if !strings.Contains(gotCaption, "Kios Test") {
		t.Errorf("caption missing merchant name: %q", gotCaption)
	}
	if strings.Contains(replyText, "https://") {
		t.Errorf("raw URL leaked into text reply: %q", replyText)
	}
}

// When no image is set (or no media-capable runtime), /qris must fall back to a
// plain text message — never panic, never send a broken photo.
func TestQrisCommandTextFallback(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	cfg := s.GetConfig(ctx)
	cfg.QrisEnabled = true
	cfg.QrisImageURL = "" // enabled but image not set yet
	if err := s.SaveConfig(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	var def commands.Definition
	for _, d := range Commands(s) {
		if d.Name == "qris" {
			def = d
		}
	}
	var replyText string
	req := commands.Request{Channel: "telegram", ChatID: "1", Reply: func(s string) error { replyText = s; return nil }}
	if err := def.Handler(ctx, req, nil); err != nil { // rt nil must be safe
		t.Fatalf("/qris: %v", err)
	}
	if !strings.Contains(replyText, "belum di-set") {
		t.Errorf("expected text fallback about missing QR image, got %q", replyText)
	}
}

func TestParseJualArgs(t *testing.T) {
	cases := []struct {
		text   string
		produk string
		qty    int
		ok     bool
	}{
		{"/jual beras 2", "beras", 2, true},
		{"/jual beras medium 5kg 3", "beras medium 5kg", 3, true},
		{"/jual beras", "", 0, false},
		{"/jual beras abc", "", 0, false},
		{"/jual beras 0", "", 0, false},
	}
	for _, c := range cases {
		p, q, ok := parseJualArgs(c.text)
		if ok != c.ok || (ok && (p != c.produk || q != c.qty)) {
			t.Errorf("parseJualArgs(%q)=(%q,%d,%v) want (%q,%d,%v)", c.text, p, q, ok, c.produk, c.qty, c.ok)
		}
	}
}

func TestBatalCommand(t *testing.T) {
	s := newTestStore(t) // default role owner
	seedProduct(t, s, "002", "Beras Medium 5kg", 10, 55000, 62000, 3)
	ctx := context.Background()

	byName := map[string]commands.Definition{}
	for _, d := range Commands(s) {
		byName[d.Name] = d
	}
	if _, ok := byName["batal"]; !ok {
		t.Fatal("slash-command /batal missing")
	}

	run := func(text string) string {
		var out string
		req := commands.Request{
			Channel:  "telegram",
			SenderID: "owner1",
			Text:     text,
			Reply:    func(s string) error { out = s; return nil },
		}
		if err := byName["batal"].Handler(ctx, req, nil); err != nil {
			t.Fatalf("/batal handler error: %v", err)
		}
		return out
	}

	// Empty arg -> usage hint.
	if got := run("/batal"); !strings.Contains(got, "Pakai: /batal") {
		t.Errorf("/batal without arg should show usage, got: %s", got)
	}

	// Sell 2 to create TRX-0001 (stok 10 -> 8).
	sell := byName["jual"]
	var sellOut string
	sellReq := commands.Request{Channel: "telegram", SenderID: "owner1", Text: "/jual beras 2",
		Reply: func(s string) error { sellOut = s; return nil }}
	if err := sell.Handler(ctx, sellReq, nil); err != nil {
		t.Fatalf("/jual: %v", err)
	}
	if !strings.Contains(sellOut, "STRUK") {
		t.Fatalf("/jual setup failed, got: %s", sellOut)
	}

	// Owner cancels TRX-0001 -> transaction gone, stok restored to 10.
	got := run("/batal TRX-0001")
	if !strings.Contains(got, "dibatalkan") {
		t.Errorf("/batal TRX-0001 should confirm cancellation, got: %s", got)
	}
	tx, _ := s.RemoveTransaksi(ctx, "TRX-0001")
	if tx != nil {
		t.Errorf("transaction TRX-0001 should be removed after /batal")
	}
	p, _ := s.GetProduk(ctx, "002")
	if p == nil || p.Stok != 10 {
		t.Errorf("stok should be restored to 10 after /batal, got: %+v", p)
	}
}

func TestBatalCommandKasirRefused(t *testing.T) {
	t.Setenv("KIOS_DEFAULT_ROLE", "kasir")
	s := newTestStore(t)
	seedProduct(t, s, "002", "Beras Medium 5kg", 10, 55000, 62000, 3)

	var def commands.Definition
	for _, d := range Commands(s) {
		if d.Name == "batal" {
			def = d
		}
	}
	if def.Handler == nil {
		t.Fatal("/batal command missing")
	}
	var out string
	req := commands.Request{
		Channel:  "telegram",
		SenderID: "kasir1",
		Text:     "/batal TRX-0001",
		Reply:    func(s string) error { out = s; return nil },
	}
	if err := def.Handler(context.Background(), req, nil); err != nil {
		t.Fatalf("/batal handler error: %v", err)
	}
	if !strings.Contains(out, "owner") {
		t.Errorf("kasir must be refused on /batal, got: %s", out)
	}
}

func TestToolsRegister(t *testing.T) {
	s := newTestStore(t)
	reg := tools.NewToolRegistry()
	for _, tool := range AllTools(s) {
		reg.Register(tool)
	}
	for _, name := range []string{"kios_stok", "kios_kasir", "kios_laporan", "kios_harga", "kios_user", "kios_supplier", "kios_promo", "kios_pustaka", "kios_pasar", "kios_belajar", "kios_import_upload"} {
		if !reg.HasRegistered(name) {
			t.Errorf("tool %q not registered (registry: %v)", name, reg.List())
		}
	}
}

func TestAutoHabitTracking(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "002", "Beras Medium 5kg", 10, 55000, 62000, 3)

	// A sale auto-records the habit — no explicit habit_track call.
	if res := NewStokTool(s).Execute(ctx, map[string]any{"action": "jual", "produk": "beras", "qty": float64(1)}); res.IsError {
		t.Fatalf("jual: %s", res.ForLLM)
	}
	if res := NewBelajarTool(s).Execute(ctx, map[string]any{"action": "habit"}); !strings.Contains(res.ForLLM, "Beras") {
		t.Errorf("sale should auto-populate habit, got: %s", res.ForLLM)
	}

	// A report request is auto-tracked too.
	NewLaporanTool(s).Execute(ctx, map[string]any{"action": "ringkas"})
	if len(s.GetHabits(ctx).ReportTimes) == 0 {
		t.Error("report request should be auto-tracked")
	}
}

func TestPasarTool(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "003", "Gula Pasir 1kg", 10, 13500, 15000, 2)
	tool := NewPasarTool(s)

	tool.Execute(ctx, map[string]any{"action": "set_pasar", "produk": "Gula Pasir 1kg", "harga": float64(20000)})
	tool.Execute(ctx, map[string]any{"action": "set_pasar", "produk": "Gula Pasir 1kg", "harga": float64(22000)})
	res := tool.Execute(ctx, map[string]any{"action": "analisa", "produk": "gula"})
	if !strings.Contains(res.ForLLM, "terlalu murah") {
		t.Errorf("our 15000 vs market 20-22k should be 'terlalu murah', got: %s", res.ForLLM)
	}
	if res := tool.Execute(ctx, map[string]any{"action": "sumber"}); !strings.Contains(res.ForLLM, "Badan Pangan") {
		t.Errorf("sumber should list monitoring sources, got: %s", res.ForLLM)
	}
}

func TestBelajarTool(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	tool := NewBelajarTool(s)

	tool.Execute(ctx, map[string]any{"action": "alias_set", "alias": "im", "target": "Indomie Goreng"})
	if res := tool.Execute(ctx, map[string]any{"action": "alias_get", "alias": "im"}); !strings.Contains(res.ForLLM, "Indomie") {
		t.Errorf("alias_get should resolve to Indomie, got: %s", res.ForLLM)
	}
	tool.Execute(ctx, map[string]any{"action": "shortcut_set", "nama": "paket hemat", "items": "beras, gula, minyak"})
	if res := tool.Execute(ctx, map[string]any{"action": "shortcut_get", "nama": "paket hemat"}); !strings.Contains(res.ForLLM, "minyak") {
		t.Errorf("shortcut_get should list items, got: %s", res.ForLLM)
	}
	tool.Execute(ctx, map[string]any{"action": "habit_track", "tipe": "sale", "value": "Beras"})
	tool.Execute(ctx, map[string]any{"action": "habit_track", "tipe": "sale", "value": "Beras"})
	if res := tool.Execute(ctx, map[string]any{"action": "habit"}); !strings.Contains(res.ForLLM, "Beras") {
		t.Errorf("habit summary should mention Beras, got: %s", res.ForLLM)
	}
	tool.Execute(ctx, map[string]any{"action": "unknown_add", "cmd": "xyzcmd"})
	if res := tool.Execute(ctx, map[string]any{"action": "unknown_list"}); !strings.Contains(res.ForLLM, "xyzcmd") {
		t.Errorf("unknown_list should include xyzcmd, got: %s", res.ForLLM)
	}
}

func TestSkorAman(t *testing.T) {
	cases := []struct {
		url  string
		aman bool
	}{
		{"https://bps.go.id/data", true},        // trusted + https + .go.id
		{"https://shopee.co.id/produk", true},   // trusted + .id
		{"http://192.168.1.1/login", false},     // IP host + non-https
		{"https://bit.ly/abc", false},           // shortener
		{"https://x.com/malware?exec=1", false}, // suspicious path
		{"ftp://example.com/file", false},       // non-http scheme
		{"", false},                             // empty
	}
	for _, c := range cases {
		got := SkorAman(c.url)
		if got.Aman != c.aman {
			t.Errorf("SkorAman(%q).Aman=%v (skor %d, %s), want %v", c.url, got.Aman, got.Skor, got.Alasan, c.aman)
		}
	}
}

func TestPustakaTool(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	tool := NewPustakaTool(s)

	// Safe URL is accepted.
	if res := tool.Execute(ctx, map[string]any{"action": "tambah", "judul": "Harga pangan NTT", "url": "https://bps.go.id/harga", "kategori": "harga"}); res.IsError {
		t.Fatalf("tambah safe url: %s", res.ForLLM)
	}
	// Unsafe URL is rejected and not stored.
	if res := tool.Execute(ctx, map[string]any{"action": "tambah", "judul": "Jahat", "url": "http://1.2.3.4/malware?exec=1"}); !res.IsError {
		t.Error("expected unsafe URL to be rejected")
	}
	all, _ := s.GetAllPustaka(ctx)
	if len(all) != 1 {
		t.Fatalf("expected exactly 1 stored entry (unsafe rejected), got %d", len(all))
	}
	if res := tool.Execute(ctx, map[string]any{"action": "daftar"}); !strings.Contains(res.ForLLM, "Harga pangan") {
		t.Errorf("daftar missing entry: %s", res.ForLLM)
	}
	if res := tool.Execute(ctx, map[string]any{"action": "cek_url", "url": "https://bit.ly/x"}); !strings.Contains(res.ForLLM, "TIDAK AMAN") {
		t.Errorf("cek_url should flag shortener unsafe: %s", res.ForLLM)
	}
	if res := tool.Execute(ctx, map[string]any{"action": "hapus", "id": all[0].ID}); res.IsError {
		t.Errorf("hapus: %s", res.ForLLM)
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

func TestJualMassal(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "002", "Beras Medium 5kg", 10, 55000, 62000, 3)
	seedProduct(t, s, "003", "Gula Pasir 1kg", 10, 13500, 15000, 2)

	res := NewKasirTool(s).Execute(ctx, map[string]any{
		"action": "jual_massal",
		"items": []any{
			map[string]any{"produk": "beras", "qty": float64(1)},
			map[string]any{"produk": "gula", "qty": float64(2)},
		},
		"bayar": float64(100000),
	})
	if res.IsError {
		t.Fatalf("jual_massal: %s", res.ForLLM)
	}
	// 62000 + 2*15000 = 92000; kembalian 8000
	if !strings.Contains(res.ForUser, "Total: Rp 92.000") || !strings.Contains(res.ForUser, "Kembalian: Rp 8.000") {
		t.Errorf("combined struk wrong: %s", res.ForUser)
	}
	if it, _ := s.GetProduk(ctx, "003"); it.Stok != 8 {
		t.Errorf("gula stock should be 8 after selling 2, got %d", it.Stok)
	}
}

// TestJualMassalSlashItems guards the /jualmassal slash command path: the
// command parser yields []map[string]any (not []any), which argItems must
// accept. Regression: previously argItems only handled []any, so every
// /jualmassal slash invocation failed with "items wajib diisi".
func TestJualMassalSlashItems(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "002", "Beras Medium 5kg", 10, 55000, 62000, 3)
	seedProduct(t, s, "003", "Gula Pasir 1kg", 10, 13500, 15000, 2)

	items := parseMassalItems("beras 1, gula 2") // []map[string]any, slash shape
	res := NewKasirTool(s).Execute(ctx, map[string]any{
		"action": "jual_massal",
		"items":  items,
		"bayar":  float64(100000),
	})
	if res.IsError {
		t.Fatalf("jual_massal (slash items): %s", res.ForLLM)
	}
	if !strings.Contains(res.ForUser, "Total: Rp 92.000") {
		t.Errorf("slash jual_massal struk wrong: %s", res.ForUser)
	}
	if it, _ := s.GetProduk(ctx, "003"); it.Stok != 8 {
		t.Errorf("gula stock should be 8 after selling 2, got %d", it.Stok)
	}
}

func TestTambahMassalAndEditMassal(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	tool := NewStokTool(s)

	// Bulk add (auto-create by default).
	res := tool.Execute(ctx, map[string]any{
		"action": "tambah_massal",
		"items": []any{
			map[string]any{"produk": "Indomie Goreng", "qty": float64(20), "harga": float64(2800)},
			map[string]any{"produk": "Teh Botol", "qty": float64(12), "harga": float64(3000)},
		},
	})
	if res.IsError || !strings.Contains(res.ForLLM, "2/2 berhasil") {
		t.Fatalf("tambah_massal: %s", res.ForLLM)
	}
	all, _ := s.GetAllProduk(ctx)
	if len(all) != 2 {
		t.Fatalf("expected 2 products created, got %d", len(all))
	}

	// Bulk edit categories.
	res = tool.Execute(ctx, map[string]any{
		"action": "edit_massal",
		"items": []any{
			map[string]any{"produk": "Indomie", "kategori": "mie"},
			map[string]any{"produk": "Teh", "kategori": "minuman"},
		},
	})
	if res.IsError || !strings.Contains(res.ForLLM, "2/2 berhasil") {
		t.Fatalf("edit_massal: %s", res.ForLLM)
	}
}

func TestParseMassalItems(t *testing.T) {
	items := parseMassalItems("beras medium 2, gula 3, minyak")
	if len(items) != 2 {
		t.Fatalf("expected 2 valid items (minyak has no qty), got %d: %+v", len(items), items)
	}
	if items[0]["produk"] != "beras medium" || items[0]["qty"].(float64) != 2 {
		t.Errorf("first item wrong: %+v", items[0])
	}
}

func TestEditProduk(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "002", "Beras Medium 5kg", 10, 55000, 62000, 3)
	tool := NewStokTool(s)

	if res := tool.Execute(ctx, map[string]any{"action": "edit_produk", "produk": "beras", "kategori": "sembako", "harga_jual": float64(65000), "stok_minimum": float64(8)}); res.IsError {
		t.Fatalf("edit_produk: %s", res.ForLLM)
	}
	it, _ := s.GetProduk(ctx, "002")
	if it.Kategori != "sembako" || it.HargaJual != 65000 || it.StokMinimum != 8 {
		t.Errorf("edit not applied: %+v", it)
	}
	if res := tool.Execute(ctx, map[string]any{"action": "edit_produk", "produk": "beras"}); !res.IsError {
		t.Error("expected error when no fields given")
	}
}

func TestSupplierEdit(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	tool := NewSupplierTool(s)
	tool.Execute(ctx, map[string]any{"action": "tambah", "nama": "UD Maju", "kontak": "0812"})

	if res := tool.Execute(ctx, map[string]any{"action": "edit", "nama": "UD Maju", "kontak": "0899", "alamat": "Baa"}); res.IsError {
		t.Fatalf("edit: %s", res.ForLLM)
	}
	all, _ := s.GetAllSupplier(ctx)
	sup := CariSupplier(all, "UD Maju")
	if sup == nil || sup.Kontak != "0899" || sup.Alamat != "Baa" {
		t.Errorf("supplier edit not applied: %+v", sup)
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

func TestBuildBackup(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "002", "Beras Medium 5kg", 10, 55000, 62000, 3)
	NewStokTool(s).Execute(ctx, map[string]any{"action": "jual", "produk": "beras", "qty": float64(2)})
	if err := s.SetSupplier(ctx, &Supplier{ID: "SUP-001", Nama: "Toko Grosir"}); err != nil {
		t.Fatal(err)
	}

	b, err := BuildBackup(ctx, s)
	if err != nil {
		t.Fatalf("BuildBackup: %v", err)
	}
	if len(b.Produk) != 1 {
		t.Errorf("expected 1 produk, got %d", len(b.Produk))
	}
	if len(b.Transaksi) != 1 {
		t.Errorf("expected 1 transaksi, got %d", len(b.Transaksi))
	}
	if len(b.Supplier) != 1 {
		t.Errorf("expected 1 supplier, got %d", len(b.Supplier))
	}
	if b.Versi == "" || b.Dibuat == "" {
		t.Errorf("backup missing versi/dibuat: %+v", b)
	}
	if !strings.Contains(b.Ringkas(), "1 produk") {
		t.Errorf("ringkas should mention produk count, got: %s", b.Ringkas())
	}
}

func TestWriteBackupFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KIOS_BACKUP_DIR", dir)
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "002", "Beras Medium 5kg", 10, 55000, 62000, 3)

	path, filename, ringkas, err := WriteBackupFile(ctx, s)
	if err != nil {
		t.Fatalf("WriteBackupFile: %v", err)
	}
	if !strings.HasPrefix(filename, "kios-backup-") || !strings.HasSuffix(filename, ".json") {
		t.Errorf("unexpected filename: %s", filename)
	}
	if filepath.Dir(path) != dir {
		t.Errorf("file not written to backup dir: %s", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read backup file: %v", err)
	}
	var decoded BackupData
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("backup file is not valid JSON: %v", err)
	}
	if len(decoded.Produk) != 1 {
		t.Errorf("decoded backup should have 1 produk, got %d", len(decoded.Produk))
	}
	if !strings.Contains(ringkas, "1 produk") {
		t.Errorf("ringkas wrong: %s", ringkas)
	}
}

func TestPruneBackups(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < backupKeep+5; i++ {
		name := fmt.Sprintf("kios-backup-2026-05-%02d-1200.json", i+1)
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pruneBackups(dir, backupKeep)
	left, _ := filepath.Glob(filepath.Join(dir, "kios-backup-*.json"))
	if len(left) != backupKeep {
		t.Fatalf("expected %d files after prune, got %d", backupKeep, len(left))
	}
	// Oldest must be gone, newest kept.
	for _, p := range left {
		if strings.Contains(p, "2026-05-01-") {
			t.Errorf("oldest backup should have been pruned: %s", p)
		}
	}
}

func TestEnsureDailyBackupJob(t *testing.T) {
	tmp := t.TempDir()
	mk := func() *cron.CronService { return cron.NewCronService(filepath.Join(tmp, "jobs.json"), nil) }

	cs := mk()
	if err := EnsureDailyBackupJob(cs); err != nil {
		t.Fatal(err)
	}
	if n := len(cs.ListJobs(true)); n != 0 {
		t.Fatalf("expected 0 jobs when KIOS_BACKUP_CHAT unset, got %d", n)
	}

	t.Setenv("KIOS_BACKUP_CHAT", "999888")
	cs = mk()
	for i := 0; i < 2; i++ {
		if err := EnsureDailyBackupJob(cs); err != nil {
			t.Fatal(err)
		}
	}
	jobs := cs.ListJobs(true)
	if len(jobs) != 1 || jobs[0].Name != DailyBackupJobName {
		t.Fatalf("expected exactly one %q job, got %+v", DailyBackupJobName, jobs)
	}
	if jobs[0].Payload.To != "999888" || jobs[0].Payload.Channel != "telegram" {
		t.Errorf("job payload target wrong: %+v", jobs[0].Payload)
	}
}

func TestParseBackup(t *testing.T) {
	if _, err := ParseBackup([]byte("{not json")); err == nil {
		t.Error("expected error for invalid JSON")
	}
	if _, err := ParseBackup([]byte(`{"produk":[]}`)); err == nil {
		t.Error("expected error when versi missing")
	}
	b, err := ParseBackup([]byte(`{"versi":"1.0","produk":[]}`))
	if err != nil || b.Versi != "1.0" {
		t.Errorf("valid backup should parse, got b=%+v err=%v", b, err)
	}
}

func TestHasAnyData(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if has, err := s.HasAnyData(ctx); err != nil || has {
		t.Fatalf("fresh store should have no data, has=%v err=%v", has, err)
	}
	seedProduct(t, s, "002", "Beras Medium 5kg", 10, 55000, 62000, 3)
	if has, err := s.HasAnyData(ctx); err != nil || !has {
		t.Fatalf("seeded store should report data, has=%v err=%v", has, err)
	}
}

func TestRestoreBackupRoundTrip(t *testing.T) {
	ctx := context.Background()
	src := newTestStore(t)
	seedProduct(t, src, "002", "Beras Medium 5kg", 10, 55000, 62000, 3)
	if _, err := src.AppendTransaksi(ctx, &Transaksi{NamaProduk: "Beras", Qty: 2, Total: 124000}); err != nil {
		t.Fatal(err)
	}
	if err := src.SetSupplier(ctx, &Supplier{ID: "SUP-003", Nama: "Toko Grosir"}); err != nil {
		t.Fatal(err)
	}

	data, err := BuildBackup(ctx, src)
	if err != nil {
		t.Fatal(err)
	}

	dst := newTestStore(t)
	if err := dst.RestoreBackup(ctx, data); err != nil {
		t.Fatalf("RestoreBackup: %v", err)
	}

	prods, _ := dst.GetAllProduk(ctx)
	if len(prods) != 1 || prods[0].ID != "002" {
		t.Errorf("produk not restored: %+v", prods)
	}
	trx, _ := dst.GetAllTransaksi(ctx)
	if len(trx) != 1 || trx[0].ID != "TRX-0001" {
		t.Errorf("transaksi not restored with original ID: %+v", trx)
	}
	sup, _ := dst.GetAllSupplier(ctx)
	if len(sup) != 1 || sup[0].ID != "SUP-003" {
		t.Errorf("supplier not restored: %+v", sup)
	}

	// Sequence counters must continue past restored IDs (no collision).
	newTrxID, err := dst.AppendTransaksi(ctx, &Transaksi{NamaProduk: "Gula", Qty: 1})
	if err != nil {
		t.Fatal(err)
	}
	if newTrxID != "TRX-0002" {
		t.Errorf("next transaksi ID should be TRX-0002, got %s", newTrxID)
	}
	newSupID, err := dst.NextSupplierID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if newSupID != "SUP-004" {
		t.Errorf("next supplier ID should be SUP-004, got %s", newSupID)
	}
}

func TestRestoreBackupReplaces(t *testing.T) {
	ctx := context.Background()
	dst := newTestStore(t)
	seedProduct(t, dst, "099", "Produk Lama", 5, 1000, 2000, 1)

	data := &BackupData{Versi: "1.0", Produk: []*Produk{{ID: "002", Nama: "Produk Baru", Stok: 7}}}
	if err := dst.RestoreBackup(ctx, data); err != nil {
		t.Fatal(err)
	}
	prods, _ := dst.GetAllProduk(ctx)
	if len(prods) != 1 || prods[0].ID != "002" {
		t.Errorf("restore should replace old data, got %+v", prods)
	}
}

func TestBootstrapOwner(t *testing.T) {
	t.Setenv("KIOS_DEFAULT_ROLE", "kasir")
	t.Setenv("KIOS_OWNER_IDS", "111, 222")
	s := newTestStore(t)

	// Listed ID is owner even with default kasir and no user record.
	ctxOwner := toolshared.WithToolContext(context.Background(), "telegram", "111")
	if role, _, refusal := resolveRole(ctxOwner, s); refusal != nil || role != "owner" {
		t.Errorf("bootstrap owner should be owner, got role=%q refusal=%v", role, refusal)
	}

	// Bootstrap owner overrides an inactive/kasir record (no lockout), and the
	// display name comes from the record when present.
	if err := s.SetUser(ctxOwner, &UserKios{Phone: "111", Nama: "Ruflo", Role: "kasir", Aktif: false}); err != nil {
		t.Fatal(err)
	}
	role, nama, refusal := resolveRole(ctxOwner, s)
	if refusal != nil || role != "owner" {
		t.Errorf("bootstrap owner must override inactive record, got role=%q refusal=%v", role, refusal)
	}
	if nama != "Ruflo" {
		t.Errorf("bootstrap owner name should come from record, got %q", nama)
	}

	// A non-listed caller falls back to the (tightened) default role.
	ctxOther := toolshared.WithToolContext(context.Background(), "telegram", "999")
	if role, _, _ := resolveRole(ctxOther, s); role != "kasir" {
		t.Errorf("non-listed id should default to kasir, got %q", role)
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

func TestCariByBarcode(t *testing.T) {
	list := []*Produk{
		{ID: "002", Nama: "Beras Medium 5kg", Barcode: "8991234500015"},
		{ID: "003", Nama: "Gula Pasir 1kg"},
	}
	got := CariProduk(list, "8991234500015")
	if len(got) != 1 || got[0].ID != "002" {
		t.Errorf("find by barcode failed: %v", got)
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

// Spec (KIOS_BUILD_SPEC.md:81): "error if bayar<total". A sale with insufficient
// payment must be REJECTED and must NOT be recorded (no stock decrement, no tx).
func TestKasirJualBayarKurangDitolak(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "003", "Gula Pasir 1kg", 10, 13500, 15000, 2)
	tool := NewKasirTool(s)

	// total = 2 * 15000 = 30000; bayar 20000 < total -> reject.
	res := tool.Execute(ctx, map[string]any{"action": "jual", "produk": "gula", "qty": float64(2), "bayar": float64(20000)})
	if !res.IsError {
		t.Fatalf("bayar 20000 < total 30000 must be an error, got (err=%v): %s", res.IsError, res.ForLLM)
	}
	// Stock must be untouched.
	if it, _ := s.GetProduk(ctx, "003"); it.Stok != 10 {
		t.Errorf("stock must stay 10 when sale rejected, got %d", it.Stok)
	}
	// No transaction must be recorded.
	if txs, _ := s.GetAllTransaksi(ctx); len(txs) != 0 {
		t.Errorf("no transaksi should be recorded on rejected sale, got %d", len(txs))
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

func TestSupplierTool_PicField(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	tool := NewSupplierTool(s)
	tool.Execute(ctx, map[string]any{"action": "tambah", "nama": "UD Maju", "pic": "Pak Budi"})
	res := tool.Execute(ctx, map[string]any{"action": "cari", "nama": "UD Maju"})
	if !strings.Contains(res.ForLLM, "Pak Budi") {
		t.Errorf("expected PIC 'Pak Budi' in cari output, got: %s", res.ForLLM)
	}
}

func TestHargaSupplierOverride(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.SetHargaSupplier(ctx, "P1", "UD Maju", 54000); err != nil {
		t.Fatalf("set: %v", err)
	}
	all, err := s.GetAllHargaSupplier(ctx)
	if err != nil {
		t.Fatalf("getall: %v", err)
	}
	if all["P1|UD Maju"] != 54000 {
		t.Errorf("expected 54000, got %d", all["P1|UD Maju"])
	}
}

func TestBandingHarga_OverrideWins(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_ = s.SetProduk(ctx, &Produk{ID: "P1", Nama: "Beras Medium 5kg", HargaBeli: 55000, Supplier: "Toko Beta"})
	s.AppendPembelian(ctx, &Pembelian{ProdukID: "P1", NamaProduk: "Beras Medium 5kg", HargaBeli: 55000, Supplier: "Toko Beta"})
	// Override manual: UD Maju menawarkan 50000 (termurah)
	_ = s.SetHargaSupplier(ctx, "P1", "UD Maju", 50000)

	res := NewSupplierTool(s).Execute(ctx, map[string]any{"action": "banding_harga", "produk": "Beras Medium 5kg"})
	if !strings.Contains(res.ForLLM, "UD Maju") || !strings.Contains(res.ForLLM, "termurah") {
		t.Errorf("expected UD Maju as termurah via override, got: %s", res.ForLLM)
	}
}

func TestSupplierTool_KasirCanAddNotDelete(t *testing.T) {
	t.Setenv("KIOS_DEFAULT_ROLE", "kasir")
	s := newTestStore(t)
	tool := NewSupplierTool(s)
	ctx := context.Background()
	if res := tool.Execute(ctx, map[string]any{"action": "tambah", "nama": "UD Sinar"}); res.IsError {
		t.Fatalf("kasir should add supplier, got: %s", res.ForLLM)
	}
	if res := tool.Execute(ctx, map[string]any{"action": "hapus", "nama": "UD Sinar"}); !res.IsError {
		t.Errorf("kasir should NOT delete supplier")
	}
}

func TestJenisOrDefault(t *testing.T) {
	cases := map[string]string{"": "biasa", "biasa": "biasa", "pulsa": "pulsa", "bensin": "bensin"}
	for in, want := range cases {
		p := &Produk{Jenis: in}
		if got := p.JenisOrDefault(); got != want {
			t.Errorf("JenisOrDefault(%q)=%q want %q", in, got, want)
		}
	}
}
