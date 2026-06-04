package kios

import (
	"context"
	"strings"
	"testing"
)

func TestComputeFromPack(t *testing.T) {
	cases := []struct {
		kemasan   string
		qtyPack   int
		hargaPack int
		isi       int
		wantQty   int
		wantHarga int
	}{
		{"dos", 2, 24000, 12, 24, 2000},  // 24000/12 = 2000/pcs
		{"lusin", 1, 36000, 12, 12, 3000}, // 36000/12 = 3000/pcs
		// round(30000/3) = 10000/pcs (hargaPack = harga per SATU kemasan)
		{"renteng", 3, 30000, 3, 9, 10000},
		// 50000/10 = 5000/pcs
		{"pak", 5, 50000, 10, 50, 5000},
	}
	for _, c := range cases {
		qty, harga := computeFromPack(c.kemasan, c.qtyPack, c.hargaPack, c.isi)
		if qty != c.wantQty {
			t.Errorf("computeFromPack(%q,%d,%d,%d) qty=%d want %d", c.kemasan, c.qtyPack, c.hargaPack, c.isi, qty, c.wantQty)
		}
		if harga != c.wantHarga {
			t.Errorf("computeFromPack(%q,%d,%d,%d) harga=%d want %d", c.kemasan, c.qtyPack, c.hargaPack, c.isi, harga, c.wantHarga)
		}
	}
}

func TestLookupIsi(t *testing.T) {
	p := &Produk{
		PackDefs: []Kemasan{
			{Nama: "Dos", Isi: 48},
			{Nama: "Lusin", Isi: 12},
		},
	}
	if got := lookupIsi(p, "dos"); got != 48 {
		t.Errorf("lookupIsi(dos)=%d want 48", got)
	}
	if got := lookupIsi(p, "LUSIN"); got != 12 {
		t.Errorf("lookupIsi(LUSIN)=%d want 12", got)
	}
	// fallback ke packVocab
	pEmpty := &Produk{}
	if got := lookupIsi(pEmpty, "lusin"); got != 12 {
		t.Errorf("lookupIsi vocab fallback lusin=%d want 12", got)
	}
	// tidak dikenal → 0
	if got := lookupIsi(pEmpty, "unknownxyz"); got != 0 {
		t.Errorf("lookupIsi unknown=%d want 0", got)
	}
}

func TestHargaSupplierLastRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	v := HargaSupplierLast{Harga: 1500, Kemasan: "dos", Isi: 48, HargaPack: 72000, Tanggal: "2026-06-03"}
	if err := s.SetHargaSupplierLast(ctx, "001", "SUP-001", v); err != nil {
		t.Fatal(err)
	}
	all, err := s.GetAllHargaSupplierLast(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := all["001|SUP-001"]
	if !ok {
		t.Fatal("key 001|SUP-001 tidak ditemukan")
	}
	if got.Harga != 1500 || got.Isi != 48 || got.HargaPack != 72000 {
		t.Errorf("got %+v, bukan harga=1500 isi=48 harga_pack=72000", got)
	}
}

func TestBackupIncludesHargaSupplierLast(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	v := HargaSupplierLast{Harga: 2000, Kemasan: "lusin", Isi: 12, HargaPack: 24000, Tanggal: "2026-06-03"}
	if err := s.SetHargaSupplierLast(ctx, "002", "SUP-002", v); err != nil {
		t.Fatal(err)
	}
	b, err := BuildBackup(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.HargaSupplierLast) == 0 {
		t.Fatal("HargaSupplierLast kosong di backup")
	}
	s2 := newTestStore(t)
	if err := s2.RestoreBackup(ctx, b); err != nil {
		t.Fatal(err)
	}
	all2, _ := s2.GetAllHargaSupplierLast(ctx)
	if all2["002|SUP-002"].Harga != 2000 {
		t.Errorf("setelah restore harga=%d want 2000", all2["002|SUP-002"].Harga)
	}
}

func TestRestockPackUpdatesStokAndHarga(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	seedProduct(t, s, "001", "Mie Goreng", 0, 2000, 3000, 2)

	tool := &StokTool{store: s}
	ctxOwner := withOwnerCtx(t, s, ctx)
	result := tool.Execute(ctxOwner, map[string]any{
		"action":     "tambah",
		"produk":     "Mie Goreng",
		"kemasan":    "dos",
		"qty_pack":   float64(2),
		"harga_pack": float64(24000),
		"isi":        float64(12),
	})
	if result.IsError {
		t.Fatalf("tambah pack error: %s", result.ForLLM)
	}

	p, _ := s.GetProduk(ctx, "001")
	if p.Stok != 24 { // 2 dos × 12 pcs
		t.Errorf("stok=%d want 24", p.Stok)
	}
	if p.HargaBeli != 2000 { // round(24000/12)
		t.Errorf("harga_beli=%d want 2000", p.HargaBeli)
	}

	pembelian, _ := s.GetAllPembelian(ctx)
	if len(pembelian) == 0 {
		t.Fatal("pembelian tidak tersimpan")
	}
	last := pembelian[len(pembelian)-1]
	if last.Kemasan != "dos" || last.QtyPack != 2 || last.HargaPack != 24000 || last.Isi != 12 {
		t.Errorf("pembelian pack fields salah: %+v", last)
	}
}

func TestRestockPackRequiresIsiWhenUnknown(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	seedProduct(t, s, "001", "Produk Aneh", 0, 0, 5000, 2)

	tool := &StokTool{store: s}
	ctxOwner := withOwnerCtx(t, s, ctx)
	result := tool.Execute(ctxOwner, map[string]any{
		"action":     "tambah",
		"produk":     "Produk Aneh",
		"kemasan":    "xyzabc",
		"qty_pack":   float64(1),
		"harga_pack": float64(10000),
	})
	if !result.IsError {
		t.Fatal("harusnya error karena isi tidak diketahui")
	}
	if !strings.Contains(strings.ToLower(result.ForLLM), "isi per kemasan") {
		t.Errorf("pesan error salah: %s", result.ForLLM)
	}
}

func TestAturKemasanOwnerOnly(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	seedProduct(t, s, "001", "Snack A", 10, 1000, 2000, 2)

	tool := &StokTool{store: s}

	// kasir tidak boleh
	ctxKasir := withKasirCtx(t, s, ctx)
	r := tool.Execute(ctxKasir, map[string]any{
		"action":  "atur_kemasan",
		"produk":  "001",
		"kemasan": []any{map[string]any{"nama": "dos", "isi": float64(48)}},
	})
	if !r.IsError {
		t.Fatal("kasir harus ditolak dari atur_kemasan")
	}

	// owner boleh
	ctxOwner := withOwnerCtx(t, s, ctx)
	r2 := tool.Execute(ctxOwner, map[string]any{
		"action":  "atur_kemasan",
		"produk":  "001",
		"kemasan": []any{map[string]any{"nama": "dos", "isi": float64(48)}},
	})
	if r2.IsError {
		t.Fatalf("owner gagal atur_kemasan: %s", r2.ForLLM)
	}
	p, _ := s.GetProduk(ctx, "001")
	if len(p.PackDefs) != 1 || p.PackDefs[0].Nama != "dos" || p.PackDefs[0].Isi != 48 {
		t.Errorf("PackDefs setelah atur_kemasan salah: %+v", p.PackDefs)
	}
}

func TestGetAllHargaSupplierDualRead(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	s.rdb.HSet(ctx, keyHargaSupplier, "001|CV Maju", "5000")
	s.rdb.HSet(ctx, keyHargaSupplier, "001|SUP-001", "4500")

	all, err := s.GetAllHargaSupplier(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if all["001|CV Maju"] != 5000 {
		t.Errorf("dual-read format lama gagal: %d", all["001|CV Maju"])
	}
	if all["001|SUP-001"] != 4500 {
		t.Errorf("dual-read format baru gagal: %d", all["001|SUP-001"])
	}
}

func TestSetHargaSupplierTulisFormatID(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if err := s.SetHargaSupplier(ctx, "001", "SUP-001", 3000); err != nil {
		t.Fatal(err)
	}
	v, _ := s.rdb.HGet(ctx, keyHargaSupplier, "001|SUP-001").Result()
	if v != "3000" {
		t.Errorf("field Redis salah: %q want \"3000\"", v)
	}
}

func TestSupplierHapusCascade(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	sup := &Supplier{ID: "SUP-001", Nama: "CV Maju"}
	_ = s.SetSupplier(ctx, sup)
	_ = s.SetHargaSupplier(ctx, "001", "SUP-001", 4000)
	_ = s.SetHargaSupplier(ctx, "001", "CV Maju", 4200) // format lama
	_ = s.SetHargaSupplierLast(ctx, "001", "SUP-001", HargaSupplierLast{Harga: 4000, Tanggal: "2026-06-03"})

	tool := &SupplierTool{store: s}
	ctxOwner := withOwnerCtx(t, s, ctx)
	r := tool.Execute(ctxOwner, map[string]any{"action": "hapus", "nama": "SUP-001"})
	if r.IsError {
		t.Fatalf("hapus supplier gagal: %s", r.ForLLM)
	}

	all, _ := s.GetAllHargaSupplier(ctx)
	for k := range all {
		if strings.Contains(k, "SUP-001") {
			t.Errorf("harga_supplier masih ada field %q setelah hapus", k)
		}
	}
	allLast, _ := s.GetAllHargaSupplierLast(ctx)
	for k := range allLast {
		if strings.Contains(k, "SUP-001") {
			t.Errorf("harga_supplier_last masih ada field %q setelah hapus", k)
		}
	}
}

func TestBackupRestoreFullPackRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	p := &Produk{
		ID: "001", Nama: "Mie Instan", Kategori: "makanan", Satuan: "pcs",
		Stok: 48, HargaBeli: 1500, HargaJual: 2500, StokMinimum: 5, StokKritis: 2,
		SupplierID: "SUP-001",
		PackDefs:   []Kemasan{{Nama: "dos", Isi: 48}},
	}
	if err := s.SetProduk(ctx, p); err != nil {
		t.Fatal(err)
	}

	s.AppendPembelian(ctx, &Pembelian{
		ID: "PEM-0001", Tanggal: "2026-06-03", ProdukID: "001", NamaProduk: "Mie Instan",
		Qty: 48, HargaBeli: 1500, Subtotal: 72000,
		Kemasan: "dos", Isi: 48, QtyPack: 1, HargaPack: 72000, SupplierID: "SUP-001",
	})

	s.SetHargaSupplierLast(ctx, "001", "SUP-001", HargaSupplierLast{
		Harga: 1500, Kemasan: "dos", Isi: 48, HargaPack: 72000, Tanggal: "2026-06-03",
	})

	b, err := BuildBackup(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.HargaSupplierLast) != 1 {
		t.Errorf("backup HargaSupplierLast len=%d want 1", len(b.HargaSupplierLast))
	}

	s2 := newTestStore(t)
	if err := s2.RestoreBackup(ctx, b); err != nil {
		t.Fatalf("RestoreBackup gagal: %v", err)
	}

	p2, _ := s2.GetProduk(ctx, "001")
	if p2 == nil || len(p2.PackDefs) != 1 || p2.PackDefs[0].Nama != "dos" {
		t.Errorf("PackDefs tidak survive restore: %+v", p2)
	}

	pems, _ := s2.GetAllPembelian(ctx)
	if len(pems) == 0 || pems[0].Kemasan != "dos" || pems[0].QtyPack != 1 {
		t.Errorf("Pembelian pack fields tidak survive restore")
	}

	snaps, _ := s2.GetAllHargaSupplierLast(ctx)
	if snaps["001|SUP-001"].Harga != 1500 {
		t.Errorf("HargaSupplierLast tidak survive restore")
	}

	has, _ := s2.HasAnyData(ctx)
	if !has {
		t.Error("HasAnyData false setelah restore")
	}
}
