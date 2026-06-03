package kios

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/sipeed/picoclaw/pkg/cron"
)

// BackupData is a full point-in-time snapshot of every kios dataset in Redis,
// exported as JSON so it can be re-imported for disaster recovery. Upstash's
// free tier wipes data 30 days after credits run out, so the only durable copy
// is the file we send to Telegram.
type BackupData struct {
	Versi         string          `json:"versi"`
	Dibuat        string          `json:"dibuat"` // WITA timestamp
	Produk        []*Produk       `json:"produk"`
	Transaksi     []*Transaksi    `json:"transaksi"`
	Pembelian     []*Pembelian    `json:"pembelian"`
	PriceHistory  []*PriceHistory `json:"price_history"`
	Supplier      []*Supplier     `json:"supplier"`
	Promo         []*Promo        `json:"promo"`
	Pustaka       []*Pustaka      `json:"pustaka"`
	Users         []*UserKios     `json:"users"`
	Shift         *Shift          `json:"shift,omitempty"`
	HargaSupplier map[string]int  `json:"harga_supplier,omitempty"`
	Pelanggan     []*Pelanggan    `json:"pelanggan,omitempty"`
}

// BuildBackup reads every kios dataset from Redis into a single snapshot.
func BuildBackup(ctx context.Context, store *Store) (*BackupData, error) {
	b := &BackupData{Versi: "1.1", Dibuat: NowWITA().Format("2006-01-02 15:04:05")}
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
	if b.HargaSupplier, err = store.GetAllHargaSupplier(ctx); err != nil {
		return nil, err
	}
	if b.Pelanggan, err = store.GetAllPelanggan(ctx); err != nil {
		return nil, err
	}
	return b, nil
}

// Ringkas returns a short human summary of the snapshot's contents.
func (b *BackupData) Ringkas() string {
	return fmt.Sprintf("%d produk, %d transaksi, %d pembelian, %d riwayat harga, %d supplier, %d promo, %d pustaka, %d pengguna, %d harga supplier, %d pelanggan",
		len(b.Produk), len(b.Transaksi), len(b.Pembelian), len(b.PriceHistory),
		len(b.Supplier), len(b.Promo), len(b.Pustaka), len(b.Users), len(b.HargaSupplier), len(b.Pelanggan))
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

// ParseBackup decodes a backup JSON document, validating that it is a kios
// snapshot (has a versi field).
func ParseBackup(raw []byte) (*BackupData, error) {
	var b BackupData
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, fmt.Errorf("file bukan JSON yang valid: %w", err)
	}
	if b.Versi == "" {
		return nil, fmt.Errorf("file ini bukan backup kios (tidak ada field \"versi\")")
	}
	return &b, nil
}

// HasAnyData reports whether any core dataset currently holds records. Used to
// guard the destructive restore so existing data isn't clobbered by accident.
func (s *Store) HasAnyData(ctx context.Context) (bool, error) {
	for _, k := range []string{keyProduk, keySupplier, keyPromo, keyPustaka, keyUsers, keyHargaSupplier, keyPelanggan} {
		n, err := s.rdb.HLen(ctx, k).Result()
		if err != nil {
			return false, err
		}
		if n > 0 {
			return true, nil
		}
	}
	for _, k := range []string{keyTransaksi, keyPembelian, keyPriceHist} {
		n, err := s.rdb.LLen(ctx, k).Result()
		if err != nil {
			return false, err
		}
		if n > 0 {
			return true, nil
		}
	}
	return false, nil
}

// RestoreBackup replaces ALL kios data with the snapshot's contents: it clears
// existing keys, reloads records preserving their original IDs, and fixes the
// sequence counters so newly generated IDs won't collide with restored ones.
// Destructive — callers must confirm with the owner first.
func (s *Store) RestoreBackup(ctx context.Context, b *BackupData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	keys := []string{
		keyProduk, keyTransaksi, keySeqTrx, keyPembelian, keySeqPem,
		keyPriceHist, keySeqPhg, keyShift, keyUsers,
		keySupplier, keySeqSup, keyPromo, keySeqPromo, keyPustaka, keySeqPus,
		keyHargaSupplier,
		keyPelanggan,
	}
	if err := s.rdb.Del(ctx, keys...).Err(); err != nil {
		return err
	}

	hsetAll := func(key string, items []hashItem) error {
		for _, it := range items {
			raw, err := json.Marshal(it.val)
			if err != nil {
				return err
			}
			if err := s.rdb.HSet(ctx, key, it.field, string(raw)).Err(); err != nil {
				return err
			}
		}
		return nil
	}
	rpushAll := func(key string, vals []any) error {
		for _, v := range vals {
			raw, err := json.Marshal(v)
			if err != nil {
				return err
			}
			if err := s.rdb.RPush(ctx, key, string(raw)).Err(); err != nil {
				return err
			}
		}
		return nil
	}

	if err := hsetAll(keyProduk, hashItemsProduk(b.Produk)); err != nil {
		return err
	}
	if err := hsetAll(keySupplier, hashItemsSupplier(b.Supplier)); err != nil {
		return err
	}
	if err := hsetAll(keyPromo, hashItemsPromo(b.Promo)); err != nil {
		return err
	}
	if err := hsetAll(keyPustaka, hashItemsPustaka(b.Pustaka)); err != nil {
		return err
	}
	if err := hsetAll(keyUsers, hashItemsUser(b.Users)); err != nil {
		return err
	}
	if err := rpushAll(keyTransaksi, anySlice(b.Transaksi)); err != nil {
		return err
	}
	if err := rpushAll(keyPembelian, anySlice(b.Pembelian)); err != nil {
		return err
	}
	if err := rpushAll(keyPriceHist, anySlice(b.PriceHistory)); err != nil {
		return err
	}
	// harga_supplier is a flat map[field]int (not JSON structs), so it doesn't
	// fit the hsetAll helper; restore each override field directly, encoding the
	// value exactly as SetHargaSupplier does (strconv.Itoa).
	for field, harga := range b.HargaSupplier {
		if err := s.rdb.HSet(ctx, keyHargaSupplier, field, strconv.Itoa(harga)).Err(); err != nil {
			return err
		}
	}
	for _, p := range b.Pelanggan {
		if err := s.SetPelanggan(ctx, p); err != nil {
			return err
		}
	}
	if b.Shift != nil {
		raw, err := json.Marshal(b.Shift)
		if err != nil {
			return err
		}
		if err := s.rdb.Set(ctx, keyShift, string(raw), 0).Err(); err != nil {
			return err
		}
	}

	setSeq := func(key string, max int64) error {
		if max <= 0 {
			return nil
		}
		return s.rdb.Set(ctx, key, strconv.FormatInt(max, 10), 0).Err()
	}
	for key, ids := range map[string][]string{
		keySeqTrx:   idsTransaksi(b.Transaksi),
		keySeqPem:   idsPembelian(b.Pembelian),
		keySeqPhg:   idsPriceHist(b.PriceHistory),
		keySeqSup:   idsSupplier(b.Supplier),
		keySeqPromo: idsPromo(b.Promo),
		keySeqPus:   idsPustaka(b.Pustaka),
	} {
		if err := setSeq(key, maxNumericSuffix(ids)); err != nil {
			return err
		}
	}
	return nil
}

type hashItem struct {
	field string
	val   any
}

func hashItemsProduk(xs []*Produk) []hashItem {
	out := make([]hashItem, len(xs))
	for i, x := range xs {
		out[i] = hashItem{field: x.ID, val: x}
	}
	return out
}
func hashItemsSupplier(xs []*Supplier) []hashItem {
	out := make([]hashItem, len(xs))
	for i, x := range xs {
		out[i] = hashItem{field: x.ID, val: x}
	}
	return out
}
func hashItemsPromo(xs []*Promo) []hashItem {
	out := make([]hashItem, len(xs))
	for i, x := range xs {
		out[i] = hashItem{field: x.ID, val: x}
	}
	return out
}
func hashItemsPustaka(xs []*Pustaka) []hashItem {
	out := make([]hashItem, len(xs))
	for i, x := range xs {
		out[i] = hashItem{field: x.ID, val: x}
	}
	return out
}
func hashItemsUser(xs []*UserKios) []hashItem {
	out := make([]hashItem, len(xs))
	for i, x := range xs {
		out[i] = hashItem{field: x.Phone, val: x}
	}
	return out
}

func anySlice[T any](xs []T) []any {
	out := make([]any, len(xs))
	for i, x := range xs {
		out[i] = x
	}
	return out
}

func idsTransaksi(xs []*Transaksi) []string {
	ids := make([]string, len(xs))
	for i, x := range xs {
		ids[i] = x.ID
	}
	return ids
}
func idsPembelian(xs []*Pembelian) []string {
	ids := make([]string, len(xs))
	for i, x := range xs {
		ids[i] = x.ID
	}
	return ids
}
func idsPriceHist(xs []*PriceHistory) []string {
	ids := make([]string, len(xs))
	for i, x := range xs {
		ids[i] = x.ID
	}
	return ids
}
func idsSupplier(xs []*Supplier) []string {
	ids := make([]string, len(xs))
	for i, x := range xs {
		ids[i] = x.ID
	}
	return ids
}
func idsPromo(xs []*Promo) []string {
	ids := make([]string, len(xs))
	for i, x := range xs {
		ids[i] = x.ID
	}
	return ids
}
func idsPustaka(xs []*Pustaka) []string {
	ids := make([]string, len(xs))
	for i, x := range xs {
		ids[i] = x.ID
	}
	return ids
}

// maxNumericSuffix returns the largest integer trailing an "PREFIX-NNN" id.
func maxNumericSuffix(ids []string) int64 {
	var max int64
	for _, s := range ids {
		i := strings.LastIndex(s, "-")
		if i < 0 {
			continue
		}
		if n, err := strconv.ParseInt(s[i+1:], 10, 64); err == nil && n > max {
			max = n
		}
	}
	return max
}
