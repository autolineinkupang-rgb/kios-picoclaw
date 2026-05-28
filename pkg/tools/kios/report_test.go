package kios

import (
	"context"
	"strings"
	"testing"
)

func TestDailyReportText_UsesSummaryWhenAvailable(t *testing.T) {
	s := newTestStore(t)
	orig := summaryFetcher
	t.Cleanup(func() { summaryFetcher = orig })
	summaryFetcher = func(ctx context.Context) (*DashboardSummary, error) {
		sum := &DashboardSummary{OK: true, Waktu: "2026-05-28 18:00"}
		sum.PenjualanHariIni.Omzet = 250000
		sum.PenjualanHariIni.Transaksi = 12
		return sum, nil
	}
	got := DailyReportText(context.Background(), s)
	if !strings.Contains(got, "Laporan Harian") || !strings.Contains(got, "Omzet") {
		t.Errorf("expected rich summary with Omzet, got: %s", got)
	}
}

func TestDailyReportText_FallbackOnError(t *testing.T) {
	s := newTestStore(t)
	orig := summaryFetcher
	t.Cleanup(func() { summaryFetcher = orig })
	summaryFetcher = func(ctx context.Context) (*DashboardSummary, error) {
		return nil, context.DeadlineExceeded
	}
	got := DailyReportText(context.Background(), s)
	if !strings.Contains(got, "Laporan Harian") {
		t.Errorf("expected fallback report header, got: %s", got)
	}
}
