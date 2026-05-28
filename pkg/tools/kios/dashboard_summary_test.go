package kios

import (
	"encoding/json"
	"testing"
	"time"
)

func TestServiceAuthHeader(t *testing.T) {
	got := serviceAuthHeader("test-secret-32-chars-minimum-abc", time.Unix(1700000000, 0))
	want := "1700000000.2921ba316deb8d4c36b55729661097c6f8b3af2e2c02f3c85e3c2051153d922c"
	if got != want {
		t.Fatalf("serviceAuthHeader = %q, want %q", got, want)
	}
}

func TestParseDashboardSummary(t *testing.T) {
	raw := `{"ok":true,"waktu":"2026-05-28 14:05",` +
		`"penjualan_hari_ini":{"omzet":50000,"laba":12000,"transaksi":4},` +
		`"terlaris":[{"nama":"Beras","qty":3,"omzet":18000}],` +
		`"jam_ramai":[{"jam":"10","transaksi":5}],` +
		`"pesanan_pending":2,` +
		`"stok":{"menipis":3,"kritis":1,"habis":0,"daftar_kritis":["Gula"]},` +
		`"sistem":{"redis":true}}`
	var s DashboardSummary
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatal(err)
	}
	if s.PenjualanHariIni.Omzet != 50000 || s.PesananPending != 2 || len(s.Terlaris) != 1 {
		t.Fatalf("parse mismatch: %+v", s)
	}
	if s.Stok.Kritis != 1 || len(s.Stok.DaftarKritis) != 1 {
		t.Fatalf("stok parse mismatch: %+v", s.Stok)
	}
}
