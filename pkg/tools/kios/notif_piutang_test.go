package kios

import (
	"context"
	"strings"
	"testing"
)

func TestBuildPiutangMessage_empty(t *testing.T) {
	s := newTestStore(t)
	n := NewNotifService(s, nil, "telegram")
	_, ok := n.buildPiutangMessage(context.Background())
	if ok {
		t.Fatal("expected ok=false when no piutang")
	}
}

func TestBuildPiutangMessage_withPiutang(t *testing.T) {
	s := newTestStore(t)
	n := NewNotifService(s, nil, "telegram")
	ctx := context.Background()

	// Create one terbuka and one lunas piutang
	_ = s.SetPiutang(ctx, &Piutang{
		ID: "PIU-0001", Phone: "628123456789", PelangganID: "PLG-628123456789",
		Pokok: 150000, Dibayar: 0, Sisa: 150000, Status: "terbuka", Tanggal: "2026-06-01",
	})
	_ = s.SetPiutang(ctx, &Piutang{
		ID: "PIU-0002", Phone: "628999000111", PelangganID: "PLG-628999000111",
		Pokok: 50000, Dibayar: 50000, Sisa: 0, Status: "lunas", Tanggal: "2026-06-01",
	})

	msg, ok := n.buildPiutangMessage(ctx)
	if !ok {
		t.Fatal("expected ok=true with 1 terbuka piutang")
	}
	if msg == "" {
		t.Fatal("expected non-empty message")
	}
	// Message should reference PIU-0001 amount or phone
	if !strings.Contains(msg, "150.000") && !strings.Contains(msg, "628123456789") {
		t.Errorf("expected message to contain piutang info, got: %s", msg)
	}
	// Lunas piutang must not appear
	if strings.Contains(msg, "628999000111") {
		t.Errorf("lunas piutang should not appear, got: %s", msg)
	}
}
