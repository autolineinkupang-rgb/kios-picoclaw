package kios

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sipeed/picoclaw/pkg/cron"
)

// BackupData is a full point-in-time snapshot of every kios dataset in Redis,
// exported as JSON so it can be re-imported for disaster recovery. Upstash's
// free tier wipes data 30 days after credits run out, so the only durable copy
// is the file we send to Telegram.
type BackupData struct {
	Versi        string          `json:"versi"`
	Dibuat       string          `json:"dibuat"` // WITA timestamp
	Produk       []*Produk       `json:"produk"`
	Transaksi    []*Transaksi    `json:"transaksi"`
	Pembelian    []*Pembelian    `json:"pembelian"`
	PriceHistory []*PriceHistory `json:"price_history"`
	Supplier     []*Supplier     `json:"supplier"`
	Promo        []*Promo        `json:"promo"`
	Pustaka      []*Pustaka      `json:"pustaka"`
	Users        []*UserKios     `json:"users"`
	Shift        *Shift          `json:"shift,omitempty"`
}

// BuildBackup reads every kios dataset from Redis into a single snapshot.
func BuildBackup(ctx context.Context, store *Store) (*BackupData, error) {
	b := &BackupData{Versi: "1.0", Dibuat: NowWITA().Format("2006-01-02 15:04:05")}
	var err error
	if b.Produk, err = store.GetAllProduk(ctx); err != nil {
		return nil, err
	}
	if b.Transaksi, err = store.GetAllTransaksi(ctx); err != nil {
		return nil, err
	}
	if b.Pembelian, err = store.GetAllPembelian(ctx); err != nil {
		return nil, err
	}
	if b.PriceHistory, err = store.GetAllPriceHistory(ctx); err != nil {
		return nil, err
	}
	if b.Supplier, err = store.GetAllSupplier(ctx); err != nil {
		return nil, err
	}
	if b.Promo, err = store.GetAllPromo(ctx); err != nil {
		return nil, err
	}
	if b.Pustaka, err = store.GetAllPustaka(ctx); err != nil {
		return nil, err
	}
	if b.Users, err = store.GetAllUsers(ctx); err != nil {
		return nil, err
	}
	if b.Shift, err = store.GetShift(ctx); err != nil {
		return nil, err
	}
	return b, nil
}

// Ringkas returns a short human summary of the snapshot's contents.
func (b *BackupData) Ringkas() string {
	return fmt.Sprintf("%d produk, %d transaksi, %d pembelian, %d riwayat harga, %d supplier, %d promo, %d pustaka, %d pengguna",
		len(b.Produk), len(b.Transaksi), len(b.Pembelian), len(b.PriceHistory),
		len(b.Supplier), len(b.Promo), len(b.Pustaka), len(b.Users))
}

// backupKeep is the number of most-recent backup files retained on disk.
const backupKeep = 10

// backupDir returns the directory backup files are written to, from
// $KIOS_BACKUP_DIR (default a temp subdir — durable copies live in Telegram).
func backupDir() string {
	if d := strings.TrimSpace(os.Getenv("KIOS_BACKUP_DIR")); d != "" {
		return d
	}
	return filepath.Join(os.TempDir(), "kios-backups")
}

// WriteBackupFile builds a snapshot and writes it as pretty JSON to the backup
// dir, returning the path, display filename, and a short summary. Old backups
// are pruned to the most recent backupKeep files. On-disk files are best-effort
// (Railway's filesystem is ephemeral); the durable copy is the Telegram message.
func WriteBackupFile(ctx context.Context, store *Store) (path, filename, summary string, err error) {
	data, err := BuildBackup(ctx, store)
	if err != nil {
		return "", "", "", err
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", "", "", err
	}
	dir := backupDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", "", err
	}
	filename = "kios-backup-" + NowWITA().Format("2006-01-02-1504") + ".json"
	path = filepath.Join(dir, filename)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return "", "", "", err
	}
	pruneBackups(dir, backupKeep)
	return path, filename, data.Ringkas(), nil
}

// pruneBackups removes the oldest kios-backup-*.json files beyond keep. Names
// are timestamped, so lexical order matches chronological order.
func pruneBackups(dir string, keep int) {
	entries, err := filepath.Glob(filepath.Join(dir, "kios-backup-*.json"))
	if err != nil || len(entries) <= keep {
		return
	}
	sort.Strings(entries)
	for _, p := range entries[:len(entries)-keep] {
		_ = os.Remove(p)
	}
}

// DailyBackupJobName identifies the auto-registered daily backup cron job.
const DailyBackupJobName = "kios-backup-harian"

// EnsureDailyBackupJob registers a recurring daily-backup cron job when
// KIOS_BACKUP_CHAT is set. Idempotent: it does nothing if the job already
// exists. Schedule defaults to 22:00 WITA; override with KIOS_BACKUP_CRON.
func EnsureDailyBackupJob(cs *cron.CronService) error {
	chat := strings.TrimSpace(os.Getenv("KIOS_BACKUP_CHAT"))
	if chat == "" {
		return nil // not configured — feature off
	}
	expr := strings.TrimSpace(os.Getenv("KIOS_BACKUP_CRON"))
	if expr == "" {
		expr = "0 22 * * *" // every day at 22:00
	}
	for _, j := range cs.ListJobs(true) {
		if j.Name == DailyBackupJobName {
			return nil // already registered
		}
	}
	_, err := cs.AddJob(
		DailyBackupJobName,
		cron.CronSchedule{Kind: "cron", Expr: expr, TZ: "Asia/Makassar"},
		"backup harian otomatis",
		"telegram",
		chat,
	)
	return err
}
