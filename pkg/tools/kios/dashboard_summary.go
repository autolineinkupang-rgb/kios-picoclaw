package kios

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// DashboardSummary mirrors the JSON returned by the dashboard GET /api/summary.
type DashboardSummary struct {
	OK               bool   `json:"ok"`
	Waktu            string `json:"waktu"`
	PenjualanHariIni struct {
		Omzet     int `json:"omzet"`
		Laba      int `json:"laba"`
		Transaksi int `json:"transaksi"`
	} `json:"penjualan_hari_ini"`
	Terlaris []struct {
		Nama  string `json:"nama"`
		Qty   int    `json:"qty"`
		Omzet int    `json:"omzet"`
	} `json:"terlaris"`
	JamRamai []struct {
		Jam       string `json:"jam"`
		Transaksi int    `json:"transaksi"`
	} `json:"jam_ramai"`
	PesananPending int `json:"pesanan_pending"`
	Stok           struct {
		Menipis      int      `json:"menipis"`
		Kritis       int      `json:"kritis"`
		Habis        int      `json:"habis"`
		DaftarKritis []string `json:"daftar_kritis"`
	} `json:"stok"`
	Sistem struct {
		Redis bool `json:"redis"`
	} `json:"sistem"`
}

// fetchDashboardSummary GETs the dashboard /api/summary endpoint with a freshly
// generated service token. Requires KIOS_DASHBOARD_URL + KIOS_SERVICE_SECRET.
func fetchDashboardSummary(ctx context.Context) (*DashboardSummary, error) {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("KIOS_DASHBOARD_URL")), "/")
	secret := strings.TrimSpace(os.Getenv("KIOS_SERVICE_SECRET"))
	if base == "" || secret == "" {
		return nil, fmt.Errorf("KIOS_DASHBOARD_URL atau KIOS_SERVICE_SECRET belum di-set")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/summary", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Kios-Service", serviceAuthHeader(secret, time.Now()))
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("verifikasi gagal (401) — cek KIOS_SERVICE_SECRET")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dashboard balas status %d", resp.StatusCode)
	}
	var s DashboardSummary
	if err := json.Unmarshal(body, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// formatDashboardSummary renders a DashboardSummary as the owner-facing /web
// reply: today's sales, busiest hours, pending orders, and stock alerts.
func formatDashboardSummary(s *DashboardSummary, base string, latencyMS int64) string {
	var b strings.Builder
	fmt.Fprintf(&b, "✅ Dashboard online (%dms)\n%s\n", latencyMS, base)
	redisStr := "ok"
	if !s.Sistem.Redis {
		redisStr = "GAGAL"
	}
	fmt.Fprintf(&b, "Redis: %s · Diperbarui: %s\n\n", redisStr, s.Waktu)

	p := s.PenjualanHariIni
	fmt.Fprintf(&b, "📊 Penjualan hari ini\nTransaksi: %d\nOmzet: %s\nLaba: %s\n",
		p.Transaksi, FormatRupiah(p.Omzet), FormatRupiah(p.Laba))

	if len(s.Terlaris) > 0 {
		b.WriteString("\n🏆 Terlaris hari ini:\n")
		for i, t := range s.Terlaris {
			fmt.Fprintf(&b, "%d. %s — %d terjual (%s)\n", i+1, t.Nama, t.Qty, FormatRupiah(t.Omzet))
		}
	}
	if len(s.JamRamai) > 0 {
		jam := make([]string, 0, len(s.JamRamai))
		for _, j := range s.JamRamai {
			jam = append(jam, fmt.Sprintf("%s.00 (%dx)", j.Jam, j.Transaksi))
		}
		fmt.Fprintf(&b, "\n⏰ Jam ramai: %s\n", strings.Join(jam, ", "))
	}

	fmt.Fprintf(&b, "\n📦 Stok: %d menipis, %d kritis, %d habis\n",
		s.Stok.Menipis, s.Stok.Kritis, s.Stok.Habis)
	if len(s.Stok.DaftarKritis) > 0 {
		fmt.Fprintf(&b, "⚠️ Kritis: %s\n", strings.Join(s.Stok.DaftarKritis, ", "))
	}
	fmt.Fprintf(&b, "\n🛒 Pesanan menunggu: %d", s.PesananPending)
	return b.String()
}

// serviceAuthHeader builds the "<ts>.<hexsig>" verification token the dashboard
// expects in the X-Kios-Service header. The bot generates a fresh one per
// request: sig = HMAC_SHA256(secret, "kios-service:" + ts).
func serviceAuthHeader(secret string, now time.Time) string {
	ts := now.Unix()
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "kios-service:%d", ts)
	return fmt.Sprintf("%d.%s", ts, hex.EncodeToString(mac.Sum(nil)))
}
