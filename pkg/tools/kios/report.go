package kios

import (
	"context"
	"os"
	"strings"

	"github.com/sipeed/picoclaw/pkg/cron"
)

// DailyReportJobName identifies the auto-registered daily report cron job.
const DailyReportJobName = "kios-laporan-harian"

// summaryFetcher dipisah agar bisa di-stub di test.
var summaryFetcher = fetchDashboardSummary

// DailyReportText menyusun teks laporan harian. Memakai ringkasan kaya dari
// /api/summary bila tersedia; bila gagal (dashboard mati / env belum diset),
// jatuh ke ringkasan Go dari Redis langsung agar laporan tetap terkirim.
func DailyReportText(ctx context.Context, store *Store) string {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("KIOS_DASHBOARD_URL")), "/")
	if s, err := summaryFetcher(ctx); err == nil && s != nil && s.OK {
		return "📅 Laporan Harian Otomatis\n" + formatDashboardSummary(s, base, 0)
	}
	res := NewLaporanTool(store).Execute(ctx, map[string]any{"action": "ringkas"})
	text := res.ForLLM
	if strings.TrimSpace(text) == "" {
		return "📅 Laporan harian: belum ada data hari ini."
	}
	return "📅 Laporan Harian Otomatis\n" + text
}

// EnsureDailyReportJob registers a recurring daily-report cron job when
// KIOS_REPORT_CHAT is set. Idempotent: it does nothing if the job already
// exists. Schedule defaults to 18:00 WITA; override with KIOS_REPORT_CRON.
func EnsureDailyReportJob(cs *cron.CronService) error {
	chat := strings.TrimSpace(os.Getenv("KIOS_REPORT_CHAT"))
	if chat == "" {
		return nil // not configured — feature off
	}
	expr := strings.TrimSpace(os.Getenv("KIOS_REPORT_CRON"))
	if expr == "" {
		expr = "0 18 * * *" // every day at 18:00
	}
	for _, j := range cs.ListJobs(true) {
		if j.Name == DailyReportJobName {
			return nil // already registered
		}
	}
	_, err := cs.AddJob(
		DailyReportJobName,
		cron.CronSchedule{Kind: "cron", Expr: expr, TZ: "Asia/Makassar"},
		"laporan harian otomatis",
		"telegram",
		chat,
	)
	return err
}
