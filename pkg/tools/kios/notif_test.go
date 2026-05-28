package kios

import (
	"context"
	"strings"
	"testing"
)

func TestBuildLowStockMessage_ClassifiesCriticalAndOut(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_ = s.SetProduk(ctx, &Produk{ID: "P1", Nama: "Beras", Stok: 0, StokMinimum: 5, StokKritis: 2})
	_ = s.SetProduk(ctx, &Produk{ID: "P2", Nama: "Gula", Stok: 2, StokMinimum: 5, StokKritis: 2})
	_ = s.SetProduk(ctx, &Produk{ID: "P3", Nama: "Kopi", Stok: 50, StokMinimum: 5, StokKritis: 2})

	n := NewNotifService(s, nil, "telegram")
	msg, ok := n.buildLowStockMessage(ctx)
	if !ok {
		t.Fatal("expected a message")
	}
	if !strings.Contains(msg, "Beras [HABIS]") {
		t.Errorf("expected HABIS label, got: %s", msg)
	}
	if !strings.Contains(msg, "Gula [kritis]") {
		t.Errorf("expected kritis label, got: %s", msg)
	}
	if strings.Contains(msg, "Kopi") {
		t.Errorf("Kopi (aman) should not appear, got: %s", msg)
	}
}

func TestShouldAlertPileup(t *testing.T) {
	cases := []struct {
		pending, threshold int
		state              string
		wantAlert          bool
		wantState          string
	}{
		{5, 5, "clear", true, "alerted"},   // mencapai ambang → alert
		{6, 5, "alerted", false, "alerted"}, // masih menumpuk → tidak spam
		{2, 5, "alerted", false, "clear"},   // turun di bawah ambang → reset
		{1, 5, "clear", false, "clear"},     // aman → diam
	}
	for _, c := range cases {
		gotAlert, gotState := shouldAlertPileup(c.pending, c.threshold, c.state)
		if gotAlert != c.wantAlert || gotState != c.wantState {
			t.Errorf("shouldAlertPileup(%d,%d,%q)=(%v,%q) want (%v,%q)",
				c.pending, c.threshold, c.state, gotAlert, gotState, c.wantAlert, c.wantState)
		}
	}
}
