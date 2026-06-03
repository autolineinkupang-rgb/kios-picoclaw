package kios

import (
	"context"
	"testing"
)

func TestNextPiutangID(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	id1, err := s.NextPiutangID(ctx)
	if err != nil || id1 != "PIU-0001" {
		t.Fatalf("NextPiutangID: %v %q", err, id1)
	}
	id2, _ := s.NextPiutangID(ctx)
	if id2 != "PIU-0002" {
		t.Errorf("second id=%q want PIU-0002", id2)
	}
}

func TestSetGetPiutang(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	p := &Piutang{ID: "PIU-0001", Phone: "628123456789", Pokok: 50000, Sisa: 50000, Status: "terbuka"}
	if err := s.SetPiutang(ctx, p); err != nil {
		t.Fatalf("SetPiutang: %v", err)
	}
	got, err := s.GetPiutang(ctx, "PIU-0001")
	if err != nil || got == nil || got.Pokok != 50000 {
		t.Fatalf("GetPiutang: %v %+v", err, got)
	}
	all, _ := s.GetAllPiutang(ctx)
	if len(all) != 1 {
		t.Errorf("GetAllPiutang len=%d want 1", len(all))
	}
}

func TestAppendGetPembayaran(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	id, _ := s.NextPayID(ctx)
	if err := s.AppendPembayaran(ctx, &Pembayaran{ID: id, LedgerID: "PIU-0001", Jumlah: 10000}); err != nil {
		t.Fatalf("AppendPembayaran: %v", err)
	}
	all, err := s.GetAllPembayaran(ctx)
	if err != nil || len(all) != 1 || all[0].Jumlah != 10000 {
		t.Fatalf("GetAllPembayaran: %v %+v", err, all)
	}
}
