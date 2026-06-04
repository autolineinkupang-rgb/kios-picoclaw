package kios

import (
	"context"
	"strings"
	"testing"

	"github.com/sipeed/picoclaw/pkg/commands"
)

// --- store_special tests ---

func TestGetSetPulsaDenom(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	got, err := s.GetPulsaDenom(ctx, 5000)
	if err != nil {
		t.Fatalf("GetPulsaDenom empty: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}

	d := &PulsaDenom{Nominal: 5000, HargaModal: 4800, HargaJual: 5500, Aktif: true}
	if err := s.SetPulsaDenom(ctx, d); err != nil {
		t.Fatalf("SetPulsaDenom: %v", err)
	}
	got, err = s.GetPulsaDenom(ctx, 5000)
	if err != nil {
		t.Fatalf("GetPulsaDenom after set: %v", err)
	}
	if got == nil || got.HargaJual != 5500 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestGetAllPulsaDenom(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for _, n := range []int{5000, 10000, 25000} {
		_ = s.SetPulsaDenom(ctx, &PulsaDenom{Nominal: n, HargaModal: n - 200, HargaJual: n + 500, Aktif: true})
	}
	all, err := s.GetAllPulsaDenom(ctx)
	if err != nil {
		t.Fatalf("GetAllPulsaDenom: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 denom, got %d", len(all))
	}
}

func TestAppendPulsaTopup(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	pt := &PulsaTopup{Jumlah: 100000, SaldoSesudah: 100000, Kasir: "owner", Catatan: ""}
	if err := s.AppendPulsaTopup(ctx, pt); err != nil {
		t.Fatalf("AppendPulsaTopup: %v", err)
	}
	if pt.ID == "" {
		t.Fatal("ID harus diisi setelah append")
	}
	all, err := s.GetAllPulsaTopup(ctx)
	if err != nil {
		t.Fatalf("GetAllPulsaTopup: %v", err)
	}
	if len(all) != 1 || all[0].ID != pt.ID {
		t.Fatalf("topup mismatch: %+v", all)
	}
}

func TestNextPtuID(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	id1, _ := s.nextPtuID(ctx)
	id2, _ := s.nextPtuID(ctx)
	if id1 != "PTU-0001" {
		t.Errorf("first id want PTU-0001, got %s", id1)
	}
	if id2 != "PTU-0002" {
		t.Errorf("second id want PTU-0002, got %s", id2)
	}
}

func TestIncrDecrSaldoModal(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	p := &Produk{ID: "P01", Nama: "Pulsa Anchor", Jenis: "pulsa", SaldoModal: 0}
	_ = s.SetProduk(ctx, p)

	if err := s.IncrSaldoModal(ctx, "P01", 50000); err != nil {
		t.Fatalf("IncrSaldoModal: %v", err)
	}
	got, _ := s.GetProduk(ctx, "P01")
	if got.SaldoModal != 50000 {
		t.Errorf("want 50000, got %d", got.SaldoModal)
	}

	if err := s.DecrSaldoModal(ctx, "P01", 4800); err != nil {
		t.Fatalf("DecrSaldoModal: %v", err)
	}
	got, _ = s.GetProduk(ctx, "P01")
	if got.SaldoModal != 45200 {
		t.Errorf("want 45200, got %d", got.SaldoModal)
	}

	err := s.DecrSaldoModal(ctx, "P01", 999999)
	if err == nil {
		t.Error("expected error when decr > saldo")
	}
}

// --- sellPulsa tests ---

func seedPulsaAnchor(t *testing.T, s *Store, saldo int) *Produk {
	t.Helper()
	p := &Produk{
		ID: "P99", Nama: "Pulsa", Jenis: "pulsa", Kategori: "pulsa",
		SaldoModal: saldo, HargaBeli: 0, HargaJual: 0,
		Stok: 0, StokMinimum: 0, StokKritis: 0,
	}
	if err := s.SetProduk(context.Background(), p); err != nil {
		t.Fatalf("seedPulsaAnchor: %v", err)
	}
	return p
}

func seedDenom(t *testing.T, s *Store, nominal, modal, jual int) {
	t.Helper()
	d := &PulsaDenom{Nominal: nominal, HargaModal: modal, HargaJual: jual, Aktif: true}
	if err := s.SetPulsaDenom(context.Background(), d); err != nil {
		t.Fatalf("seedDenom: %v", err)
	}
}

func TestSellPulsaOK(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	anchor := seedPulsaAnchor(t, s, 100000)
	seedDenom(t, s, 10000, 9500, 11000)

	tx, item, sisa, err := sellPulsa(ctx, s, anchor, 10000, "tunai", "kasir1")
	if err != nil {
		t.Fatalf("sellPulsa: %v", err)
	}
	if tx.Total != 11000 {
		t.Errorf("total want 11000, got %d", tx.Total)
	}
	if tx.Modal != 9500 {
		t.Errorf("modal want 9500, got %d", tx.Modal)
	}
	if tx.Qty != 1 {
		t.Errorf("qty want 1, got %d", tx.Qty)
	}
	if tx.Kategori != "pulsa" {
		t.Errorf("kategori want pulsa, got %s", tx.Kategori)
	}
	if item.SaldoModal != 90500 {
		t.Errorf("SaldoModal want 90500, got %d", item.SaldoModal)
	}
	if sisa != 90500 {
		t.Errorf("sisa want 90500, got %d", sisa)
	}
	reloaded, _ := s.GetProduk(ctx, anchor.ID)
	if reloaded.SaldoModal != 90500 {
		t.Errorf("persisted SaldoModal want 90500, got %d", reloaded.SaldoModal)
	}
}

func TestSellPulsaModalKurang(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	anchor := seedPulsaAnchor(t, s, 5000)
	seedDenom(t, s, 10000, 9500, 11000)

	_, _, _, err := sellPulsa(ctx, s, anchor, 10000, "tunai", "kasir1")
	if err == nil {
		t.Error("expected error saldo kurang")
	}
}

func TestSellPulsaDenomTidakAda(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	anchor := seedPulsaAnchor(t, s, 100000)

	_, _, _, err := sellPulsa(ctx, s, anchor, 99999, "tunai", "kasir1")
	if err == nil {
		t.Error("expected error denom tidak ada")
	}
}

func TestSellPulsaDenomTidakAktif(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	anchor := seedPulsaAnchor(t, s, 100000)
	d := &PulsaDenom{Nominal: 10000, HargaModal: 9500, HargaJual: 11000, Aktif: false}
	_ = s.SetPulsaDenom(ctx, d)

	_, _, _, err := sellPulsa(ctx, s, anchor, 10000, "tunai", "kasir1")
	if err == nil {
		t.Error("expected error denom tidak aktif")
	}
}

// --- sellBensin tests ---

func seedBensinProduk(t *testing.T, s *Store, stokMl, kritisMl, beli, jual int) *Produk {
	t.Helper()
	p := &Produk{
		ID: "B01", Nama: "Pertalite", Jenis: "bensin", Kategori: "bensin",
		StokMl: stokMl, StokKritisMl: kritisMl,
		HargaBeli: beli, HargaJual: jual,
		Stok: stokMl / 1000,
	}
	if err := s.SetProduk(context.Background(), p); err != nil {
		t.Fatalf("seedBensin: %v", err)
	}
	return p
}

func TestSellBensinOK(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	anchor := seedBensinProduk(t, s, 50000, 40000, 10000, 12000)

	tx, item, sisaMl, err := sellBensin(ctx, s, anchor, 2000, "tunai", "kasir1")
	if err != nil {
		t.Fatalf("sellBensin: %v", err)
	}
	if tx.Total != 24000 {
		t.Errorf("total want 24000, got %d", tx.Total)
	}
	if tx.Modal != 20000 {
		t.Errorf("modal want 20000, got %d", tx.Modal)
	}
	if tx.Liter != 2.0 {
		t.Errorf("liter want 2.0, got %f", tx.Liter)
	}
	if item.StokMl != 48000 {
		t.Errorf("StokMl want 48000, got %d", item.StokMl)
	}
	if sisaMl != 48000 {
		t.Errorf("sisaMl want 48000, got %d", sisaMl)
	}
	if item.Stok != 48 {
		t.Errorf("Stok want 48, got %d", item.Stok)
	}
	reloaded, _ := s.GetProduk(ctx, anchor.ID)
	if reloaded.StokMl != 48000 {
		t.Errorf("persisted StokMl want 48000, got %d", reloaded.StokMl)
	}
}

func TestSellBensinStokKurang(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	anchor := seedBensinProduk(t, s, 1000, 40000, 10000, 12000)

	_, _, _, err := sellBensin(ctx, s, anchor, 2000, "tunai", "kasir1")
	if err == nil {
		t.Error("expected error stok kurang")
	}
}

func TestSellBensinFraksi(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	anchor := seedBensinProduk(t, s, 50000, 40000, 10000, 12000)

	tx, _, _, err := sellBensin(ctx, s, anchor, 1500, "tunai", "kasir1")
	if err != nil {
		t.Fatalf("sellBensin 1.5L: %v", err)
	}
	if tx.Total != 18000 {
		t.Errorf("total 1.5L want 18000, got %d", tx.Total)
	}
	if tx.Liter != 1.5 {
		t.Errorf("liter want 1.5, got %f", tx.Liter)
	}
}

func TestPerformJualPulsa(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedPulsaAnchor(t, s, 100000)
	seedDenom(t, s, 10000, 9500, 11000)

	tx, item, sisa, err := performJual(ctx, s, "Pulsa", 1, "tunai", "kasir1", 0, map[string]int{"nominal": 10000})
	if err != nil {
		t.Fatalf("performJual pulsa: %v", err)
	}
	if tx.Total != 11000 {
		t.Errorf("total want 11000, got %d", tx.Total)
	}
	if item.SaldoModal != 90500 {
		t.Errorf("SaldoModal want 90500, got %d", item.SaldoModal)
	}
	if sisa != 90500 {
		t.Errorf("sisa want 90500, got %d", sisa)
	}
}

func TestPerformJualBensin(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedBensinProduk(t, s, 50000, 40000, 10000, 12000)

	tx, item, sisaMl, err := performJual(ctx, s, "Pertalite", 1, "tunai", "kasir1", 0, map[string]int{"ml": 2000})
	if err != nil {
		t.Fatalf("performJual bensin: %v", err)
	}
	if tx.Total != 24000 {
		t.Errorf("total want 24000, got %d", tx.Total)
	}
	if item.StokMl != 48000 {
		t.Errorf("StokMl want 48000, got %d", item.StokMl)
	}
	if sisaMl != 48000 {
		t.Errorf("sisaMl want 48000, got %d", sisaMl)
	}
}

func TestBatalkanTxPulsa(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	anchor := seedPulsaAnchor(t, s, 100000)
	seedDenom(t, s, 10000, 9500, 11000)

	tx, _, _, _ := sellPulsa(ctx, s, anchor, 10000, "tunai", "kasir1")

	tool := &StokTool{store: s}
	result := tool.batalkanTx(ctx, map[string]any{"id": tx.ID})
	if result.IsError {
		t.Fatalf("batalkanTx pulsa error: %s", result.ForLLM)
	}
	reloaded, _ := s.GetProduk(ctx, anchor.ID)
	if reloaded.SaldoModal != 100000 {
		t.Errorf("SaldoModal after batal want 100000, got %d", reloaded.SaldoModal)
	}
}

func TestBatalkanTxBensin(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	anchor := seedBensinProduk(t, s, 50000, 40000, 10000, 12000)

	tx, _, _, _ := sellBensin(ctx, s, anchor, 2000, "tunai", "kasir1")

	tool := &StokTool{store: s}
	result := tool.batalkanTx(ctx, map[string]any{"id": tx.ID})
	if result.IsError {
		t.Fatalf("batalkanTx bensin error: %s", result.ForLLM)
	}
	reloaded, _ := s.GetProduk(ctx, anchor.ID)
	if reloaded.StokMl != 50000 {
		t.Errorf("StokMl after batal want 50000, got %d", reloaded.StokMl)
	}
	if reloaded.Stok != 50 {
		t.Errorf("Stok after batal want 50, got %d", reloaded.Stok)
	}
}

func TestHitungLabaPulsa(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	anchor := seedPulsaAnchor(t, s, 100000)
	seedDenom(t, s, 10000, 9500, 11000)
	tx, _, _, _ := sellPulsa(ctx, s, anchor, 10000, "tunai", "kasir1")

	tool := &LaporanTool{store: s}
	omzet, modal, laba := tool.hitungLaba(ctx, []*Transaksi{tx})
	if omzet != 11000 {
		t.Errorf("omzet want 11000, got %d", omzet)
	}
	if modal != 9500 {
		t.Errorf("modal want 9500, got %d", modal)
	}
	if laba != 1500 {
		t.Errorf("laba want 1500, got %d", laba)
	}
}

func TestHitungLabaBensin(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	anchor := seedBensinProduk(t, s, 50000, 40000, 10000, 12000)
	tx, _, _, _ := sellBensin(ctx, s, anchor, 2000, "tunai", "kasir1")

	tool := &LaporanTool{store: s}
	omzet, modal, laba := tool.hitungLaba(ctx, []*Transaksi{tx})
	if omzet != 24000 {
		t.Errorf("omzet want 24000, got %d", omzet)
	}
	if modal != 20000 {
		t.Errorf("modal want 20000, got %d", modal)
	}
	if laba != 4000 {
		t.Errorf("laba want 4000, got %d", laba)
	}
}

func TestNotifBensinKritis(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	// 30L stok below 40L kritis threshold
	_ = seedBensinProduk(t, s, 30000, 40000, 10000, 12000)

	svc := &NotifService{store: s}
	msg, ok := svc.buildLowStockMessage(ctx)
	if !ok {
		t.Fatal("expected low stock message for bensin")
	}
	if !strings.Contains(msg, "Pertalite") && !strings.Contains(msg, "bensin") {
		t.Errorf("message should mention bensin product, got: %s", msg)
	}
}

func TestNotifPulsaLowBalance(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	// SaldoModal 4000 below StokMinimum 5000
	p := &Produk{
		ID: "P99", Nama: "Pulsa", Jenis: "pulsa",
		SaldoModal: 4000, StokMinimum: 5000,
	}
	_ = s.SetProduk(ctx, p)

	svc := &NotifService{store: s}
	msg, ok := svc.buildLowStockMessage(ctx)
	if !ok {
		t.Fatal("expected low balance message for pulsa")
	}
	if !strings.Contains(msg, "saldo modal") {
		t.Errorf("message should mention saldo modal, got: %s", msg)
	}
}

func TestHandlePulsaCmdInfo(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedPulsaAnchor(t, s, 75000)
	seedDenom(t, s, 10000, 9500, 11000)

	defs := CommandsSpecial(s)
	var pulsaDef *commands.Definition
	for i := range defs {
		if defs[i].Name == "pulsa" {
			pulsaDef = &defs[i]
			break
		}
	}
	if pulsaDef == nil {
		t.Fatal("command /pulsa tidak ditemukan di CommandsSpecial")
	}

	var out string
	req := commands.Request{
		Channel: "telegram", SenderID: "owner1", Text: "/pulsa",
		Reply: func(s string) error { out = s; return nil },
	}
	if err := pulsaDef.Handler(ctx, req, nil); err != nil {
		t.Fatalf("/pulsa error: %v", err)
	}
	if !strings.Contains(out, "75.000") {
		t.Errorf("/pulsa output should show saldo 75000, got: %s", out)
	}
	if !strings.Contains(out, "10.000") {
		t.Errorf("/pulsa output should show nominal 10000, got: %s", out)
	}
}

func TestBackupRestorePulsa(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	anchor := seedPulsaAnchor(t, s, 150000)
	_ = anchor
	seedDenom(t, s, 10000, 9500, 11000)
	seedDenom(t, s, 25000, 24500, 26500)
	pt := &PulsaTopup{
		Tanggal: "2026-06-03", Jam: "10:00:00",
		Jumlah: 150000, SaldoSesudah: 150000, Kasir: "owner",
	}
	_ = s.AppendPulsaTopup(ctx, pt)

	b, err := BuildBackup(ctx, s)
	if err != nil {
		t.Fatalf("BuildBackup: %v", err)
	}
	if len(b.PulsaDenom) != 2 {
		t.Errorf("want 2 denom in backup, got %d", len(b.PulsaDenom))
	}
	if len(b.PulsaTopup) != 1 {
		t.Errorf("want 1 topup in backup, got %d", len(b.PulsaTopup))
	}

	s2 := newTestStore(t)
	if err := s2.RestoreBackup(ctx, b); err != nil {
		t.Fatalf("RestoreBackup: %v", err)
	}

	d10k, _ := s2.GetPulsaDenom(ctx, 10000)
	if d10k == nil || d10k.HargaJual != 11000 {
		t.Errorf("denom 10000 after restore: %+v", d10k)
	}

	tops, _ := s2.GetAllPulsaTopup(ctx)
	if len(tops) != 1 {
		t.Errorf("want 1 topup after restore, got %d", len(tops))
	}

	// Counter PTU should be at the max seen ID
	// After AppendPulsaTopup (1 topup = PTU-0001), counter = 1
	// After RestoreBackup it should be reset to 1 so next = PTU-0002
	nextID, _ := s2.nextPtuID(ctx)
	if nextID != "PTU-0002" {
		t.Errorf("next PTU after restore want PTU-0002, got %s", nextID)
	}

	anchor2, _ := s2.GetProduk(ctx, "P99")
	if anchor2 == nil || anchor2.SaldoModal != 150000 {
		t.Errorf("SaldoModal after restore want 150000, got %v", anchor2)
	}
}

func TestHandleBensinCmdInfo(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedBensinProduk(t, s, 30000, 40000, 10000, 12000)

	defs := CommandsSpecial(s)
	var def *commands.Definition
	for i := range defs {
		if defs[i].Name == "bensin" {
			def = &defs[i]
			break
		}
	}
	if def == nil {
		t.Fatal("command /bensin tidak ditemukan di CommandsSpecial")
	}

	var out string
	req := commands.Request{
		Channel: "telegram", SenderID: "owner1", Text: "/bensin",
		Reply: func(s string) error { out = s; return nil },
	}
	if err := def.Handler(ctx, req, nil); err != nil {
		t.Fatalf("/bensin error: %v", err)
	}
	if !strings.Contains(out, "KRITIS") {
		t.Errorf("/bensin output should show KRITIS for 30L below 40L, got: %s", out)
	}
}
