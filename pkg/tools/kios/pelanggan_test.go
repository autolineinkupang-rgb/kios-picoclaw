package kios

import (
	"context"
	"encoding/json"
	"testing"
)

func TestNormalizePhone(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"08123456789", "628123456789"},
		{"+628123456789", "628123456789"},
		{"628123456789", "628123456789"},
		{"8123456789", "628123456789"},
		{"", ""},
		{"abc", ""},
		{"+16504004000", ""},       // non-Indonesian country code must be rejected
		{"628123456789012345", ""}, // too long (>15 digits) must be rejected
	}
	for _, c := range cases {
		got := NormalizePhone(c.in)
		if got != c.want {
			t.Errorf("NormalizePhone(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestUpsertPelanggan(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	p, err := s.UpsertPelanggan(ctx, "Budi", "08123456789")
	if err != nil {
		t.Fatalf("upsert baru: %v", err)
	}
	if p.Phone != "628123456789" {
		t.Errorf("phone=%q want 628123456789", p.Phone)
	}
	if p.ID != "PLG-628123456789" {
		t.Errorf("id=%q want PLG-628123456789", p.ID)
	}
	if p.Nama != "Budi" {
		t.Errorf("nama=%q want Budi", p.Nama)
	}
	if p.TotalPesanan != 1 {
		t.Errorf("TotalPesanan=%d want 1", p.TotalPesanan)
	}

	p2, err := s.UpsertPelanggan(ctx, "Budi Santoso", "628123456789")
	if err != nil {
		t.Fatalf("upsert ulang: %v", err)
	}
	if p2.TotalPesanan != 2 {
		t.Errorf("TotalPesanan setelah upsert ulang=%d want 2", p2.TotalPesanan)
	}
	if p2.Nama != "Budi Santoso" {
		t.Errorf("nama setelah update=%q want Budi Santoso", p2.Nama)
	}
	if p2.ID != p.ID {
		t.Errorf("id berubah: %q → %q (tidak boleh)", p.ID, p2.ID)
	}
}

func TestGetPelanggan(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	got, err := s.GetPelanggan(ctx, "628123456789")
	if err != nil {
		t.Fatalf("get kosong: %v", err)
	}
	if got != nil {
		t.Errorf("got non-nil want nil (belum ada)")
	}

	_, err = s.UpsertPelanggan(ctx, "Siti", "08123456789")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err = s.GetPelanggan(ctx, "628123456789")
	if err != nil {
		t.Fatalf("get setelah upsert: %v", err)
	}
	if got == nil || got.Nama != "Siti" {
		t.Errorf("got=%v want Siti", got)
	}
}

func TestGetAllPelanggan(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.UpsertPelanggan(ctx, "Adi", "08111111111")  //nolint:errcheck
	s.UpsertPelanggan(ctx, "Budi", "08222222222") //nolint:errcheck
	s.UpsertPelanggan(ctx, "Cici", "08333333333") //nolint:errcheck

	all, err := s.GetAllPelanggan(ctx)
	if err != nil {
		t.Fatalf("getall: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("len=%d want 3", len(all))
	}
}

func TestUpsertPelangganInvalidPhone(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, err := s.UpsertPelanggan(ctx, "Ghost", "abc")
	if err == nil {
		t.Error("expected error for invalid phone")
	}
}

func TestBackupRestorePelanggan(t *testing.T) {
	ctx := context.Background()
	src := newTestStore(t)

	if _, err := src.UpsertPelanggan(ctx, "Rina", "08512345678"); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	b, err := BuildBackup(ctx, src)
	if err != nil {
		t.Fatalf("build backup: %v", err)
	}
	if len(b.Pelanggan) != 1 {
		t.Fatalf("pelanggan in backup=%d want 1", len(b.Pelanggan))
	}

	dst := newTestStore(t)
	err = dst.RestoreBackup(ctx, b)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	phone := NormalizePhone("08512345678")
	got, err := dst.GetPelanggan(ctx, phone)
	if err != nil {
		t.Fatalf("get setelah restore: %v", err)
	}
	if got == nil || got.Nama != "Rina" {
		t.Errorf("restored pelanggan=%v want Rina", got)
	}
}

func TestPesananPelangganIDRoundtrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_ = s
	_ = ctx

	original := &Pesanan{
		ID:          "PSN-0001",
		Tanggal:     "2026-06-03",
		Jam:         "08:00:00",
		NamaPembeli: "Budi",
		Kontak:      "628123456789",
		Items:       []PesananItem{{ProdukID: "P001", NamaProduk: "Beras", Qty: 1, HargaSatuan: 10000, Subtotal: 10000}},
		Total:       10000,
		Status:      "pending",
		CreatedAt:   1748916000,
		PelangganID: "PLG-628123456789",
	}

	b, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Pesanan
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.PelangganID != "PLG-628123456789" {
		t.Errorf("PelangganID=%q want PLG-628123456789", decoded.PelangganID)
	}

	oldJSON := `{"id":"PSN-0000","tanggal":"2026-01-01","jam":"07:00","nama_pembeli":"Anon","kontak":"","items":[],"total":0,"metode_bayar":"tunai","status":"pending","created_at":0}`
	var old Pesanan
	if err := json.Unmarshal([]byte(oldJSON), &old); err != nil {
		t.Fatalf("decode old format: %v", err)
	}
	if old.PelangganID != "" {
		t.Errorf("old format PelangganID=%q want empty", old.PelangganID)
	}
}
