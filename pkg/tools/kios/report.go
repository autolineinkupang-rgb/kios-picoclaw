package kios

import (
	"context"
	"os"
	"strings"

	"github.com/sipeed/picoclaw/pkg/cron"
)

// DailyReportJobName identifies the auto-registered daily report cron job.
const DailyReportJobName = "kios-laporan-harian"

// DailyReportText builds the daily summary text deterministically (no LLM),
// suitable for pushing to Telegram from a cron job.
func DailyReportText(ctx context.Context, store *Store) string {
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
