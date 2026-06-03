package kios

import (
	"context"
	"testing"
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
