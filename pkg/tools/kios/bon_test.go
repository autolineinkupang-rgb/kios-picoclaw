package kios

import (
	"context"
	"strings"
	"testing"

	"github.com/sipeed/picoclaw/pkg/commands"
	toolshared "github.com/sipeed/picoclaw/pkg/tools/shared"
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

// ---- Test helpers ----
// withKasirCtx: tambah user kasir ke store, pakai ID-nya di context
func withKasirCtx(t *testing.T, s *Store, ctx context.Context) context.Context {
	t.Helper()
	id := "kasir-test-001"
	_ = s.SetUser(ctx, &UserKios{Phone: id, Nama: "TestKasir", Role: "kasir", Aktif: true})
	return toolshared.WithToolContext(ctx, "telegram", id)
}

func TestJualBon(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "001", "Mie Goreng", 20, 2000, 3000, 3)
	if _, err := s.UpsertPelanggan(ctx, "Budi", "08123456789"); err != nil {
		t.Fatalf("upsert pelanggan: %v", err)
	}

	bon := NewBonTool(s)
	result := bon.Execute(ctx, map[string]any{
		"action": "jual_bon", "produk": "mie", "qty": float64(2), "pelanggan": "08123456789",
	})
	if result.IsError {
		t.Fatalf("jual_bon error: %s", result.ForLLM)
	}
	all, _ := s.GetAllPiutang(ctx)
	if len(all) != 1 || all[0].Pokok != 6000 || all[0].Status != "terbuka" {
		t.Fatalf("piutang: %+v", all)
	}
	p, _ := s.GetPelanggan(ctx, "628123456789")
	if p == nil || p.TotalBelanja != 6000 {
		t.Errorf("pelanggan total_belanja=%d want 6000", p.TotalBelanja)
	}
}

func TestBayarPiutang(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "001", "Mie Goreng", 20, 2000, 3000, 3)
	_, _ = s.UpsertPelanggan(ctx, "Budi", "08123456789")

	bon := NewBonTool(s)
	bon.Execute(ctx, map[string]any{"action": "jual_bon", "produk": "mie", "qty": float64(2), "pelanggan": "08123456789"})
	all, _ := s.GetAllPiutang(ctx)
	piuID := all[0].ID

	r := bon.Execute(ctx, map[string]any{"action": "bayar", "id": piuID, "jumlah": float64(3000), "metode": "tunai"})
	if r.IsError {
		t.Fatalf("bayar error: %s", r.ForLLM)
	}
	piu, _ := s.GetPiutang(ctx, piuID)
	if piu.Dibayar != 3000 || piu.Sisa != 3000 || piu.Status != "terbuka" {
		t.Errorf("setelah cicil: %+v", piu)
	}

	bon.Execute(ctx, map[string]any{"action": "bayar", "id": piuID, "jumlah": float64(3000), "metode": "tunai"})
	piu, _ = s.GetPiutang(ctx, piuID)
	if piu.Status != "lunas" {
		t.Errorf("setelah lunas status=%q want lunas", piu.Status)
	}
}

func TestBayarOverpaymentDitolak(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "001", "Mie Goreng", 20, 2000, 3000, 3)
	_, _ = s.UpsertPelanggan(ctx, "Budi", "08123456789")

	bon := NewBonTool(s)
	bon.Execute(ctx, map[string]any{"action": "jual_bon", "produk": "mie", "qty": float64(1), "pelanggan": "08123456789"})
	all, _ := s.GetAllPiutang(ctx)

	r := bon.Execute(ctx, map[string]any{"action": "bayar", "id": all[0].ID, "jumlah": float64(99999), "metode": "tunai"})
	if !r.IsError {
		t.Error("overpayment harus ditolak")
	}
}

func TestJualBonBypassBayarGuard(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "001", "Mie Goreng", 20, 2000, 3000, 3)
	_, _ = s.UpsertPelanggan(ctx, "Budi", "08123456789")

	// Jual bon tanpa arg bayar harus tetap berhasil (bypass guard bayar<total)
	bon := NewBonTool(s)
	r := bon.Execute(ctx, map[string]any{
		"action": "jual_bon", "produk": "mie", "qty": float64(2), "pelanggan": "08123456789",
	})
	if r.IsError {
		t.Errorf("jual_bon harus bypass guard bayar<total: %s", r.ForLLM)
	}
}

func TestBatalkanTxBonTanpaCicilan(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "001", "Mie Goreng", 20, 2000, 3000, 3)
	_, _ = s.UpsertPelanggan(ctx, "Budi", "08123456789")

	bon := NewBonTool(s)
	bon.Execute(ctx, map[string]any{"action": "jual_bon", "produk": "mie", "qty": float64(2), "pelanggan": "08123456789"})
	allTx, _ := s.GetAllTransaksi(ctx)
	txID := allTx[0].ID
	allPiu, _ := s.GetAllPiutang(ctx)
	piuID := allPiu[0].ID

	// Batal tanpa cicilan -> piutang harus void (Status="dihapus")
	stok := NewStokTool(s)
	r := stok.Execute(ctx, map[string]any{"action": "batalkan_tx", "id": txID})
	if r.IsError {
		t.Fatalf("batalkan_tx error: %s", r.ForLLM)
	}
	piu, _ := s.GetPiutang(ctx, piuID)
	if piu == nil || piu.Status != "dihapus" {
		t.Errorf("piutang harus dihapus, got: %+v", piu)
	}
}

func TestBatalkanTxBonDenganCicilan(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "001", "Mie Goreng", 20, 2000, 3000, 3)
	_, _ = s.UpsertPelanggan(ctx, "Budi", "08123456789")

	bon := NewBonTool(s)
	bon.Execute(ctx, map[string]any{"action": "jual_bon", "produk": "mie", "qty": float64(2), "pelanggan": "08123456789"})
	allTx, _ := s.GetAllTransaksi(ctx)
	txID := allTx[0].ID
	allPiu, _ := s.GetAllPiutang(ctx)

	// Cicil dulu
	bon.Execute(ctx, map[string]any{"action": "bayar", "id": allPiu[0].ID, "jumlah": float64(1000), "metode": "tunai"})

	// Batal dengan cicilan -> harus error
	stok := NewStokTool(s)
	r := stok.Execute(ctx, map[string]any{"action": "batalkan_tx", "id": txID})
	if !r.IsError {
		t.Error("batalkan dengan cicilan harus error")
	}
}

func TestDelPelangganSafeDenganPiutangDiblokir(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "001", "Mie Goreng", 20, 2000, 3000, 3)
	_, _ = s.UpsertPelanggan(ctx, "Budi", "08123456789")

	bon := NewBonTool(s)
	bon.Execute(ctx, map[string]any{"action": "jual_bon", "produk": "mie", "qty": float64(1), "pelanggan": "08123456789"})

	err := s.DelPelangganSafe(ctx, "628123456789")
	if err == nil {
		t.Error("harus error — pelanggan masih punya piutang terbuka")
	}
}

func TestSlashUtangCommand(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "001", "Mie Goreng", 20, 2000, 3000, 3)
	_, _ = s.UpsertPelanggan(ctx, "Budi", "08123456789")
	NewBonTool(s).Execute(ctx, map[string]any{
		"action": "jual_bon", "produk": "mie", "qty": float64(2), "pelanggan": "08123456789",
	})

	defs := CommandsBon(s)
	found := false
	for _, d := range defs {
		if d.Name == "utang" {
			found = true
			var out string
			req := commands.Request{
				Channel: "telegram", SenderID: "owner1", Text: "/utang",
				Reply: func(s string) error { out = s; return nil },
			}
			if err := d.Handler(ctx, req, nil); err != nil {
				t.Fatalf("/utang error: %v", err)
			}
			if !strings.Contains(out, "PIU-") {
				t.Errorf("/utang output: %q", out)
			}
		}
	}
	if !found {
		t.Fatal("command /utang tidak ditemukan di CommandsBon")
	}
}

func TestDelPelangganSafeTanpaPiutangOK(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_, _ = s.UpsertPelanggan(ctx, "Budi", "08123456789")

	err := s.DelPelangganSafe(ctx, "628123456789")
	if err != nil {
		t.Errorf("hapus tanpa piutang harus OK: %v", err)
	}
	got, _ := s.GetPelanggan(ctx, "628123456789")
	if got != nil {
		t.Error("pelanggan harus sudah terhapus")
	}
}

func TestHapusPiutangOwnerOnly(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedProduct(t, s, "001", "Mie Goreng", 20, 2000, 3000, 3)
	_, _ = s.UpsertPelanggan(ctx, "Budi", "08123456789")

	bon := NewBonTool(s)
	bon.Execute(ctx, map[string]any{"action": "jual_bon", "produk": "mie", "qty": float64(1), "pelanggan": "08123456789"})
	all, _ := s.GetAllPiutang(ctx)

	// kasir tidak bisa hapus
	ctxKasir := withKasirCtx(t, s, ctx)
	r := bon.Execute(ctxKasir, map[string]any{"action": "hapus", "id": all[0].ID})
	if !r.IsError {
		t.Error("kasir tidak boleh hapus piutang")
	}
	// owner (ctx default) bisa hapus
	r2 := bon.Execute(ctx, map[string]any{"action": "hapus", "id": all[0].ID})
	if r2.IsError {
		t.Errorf("owner harus bisa hapus: %s", r2.ForLLM)
	}
}
