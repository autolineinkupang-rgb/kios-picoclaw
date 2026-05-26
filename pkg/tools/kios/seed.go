package kios

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sipeed/picoclaw/pkg/logger"
)

// seedDir returns the directory holding the legacy CSV/JSON to import, from
// $KIOS_SEED_DIR. Empty means "no seed source" (the normal case on Railway).
func seedDir() string {
	return os.Getenv("KIOS_SEED_DIR")
}

// SeedFromOldData imports CSV/JSON data from the old project when Redis keys are empty.
// Idempotent: skips if seed already done. No-op when $KIOS_SEED_DIR is unset.
func SeedFromOldData(ctx context.Context, s *Store) error {
	dir := seedDir()
	if dir == "" {
		return nil
	}
	done, err := s.IsSeedDone(ctx)
	if err != nil {
		return fmt.Errorf("seed check failed: %w", err)
	}
	if done {
		return nil
	}

	logger.InfoCF("kios-seed", "Seeding Redis from old project data", map[string]any{
		"source": dir,
	})

	if err := seedProduk(ctx, s); err != nil {
		logger.WarnCF("kios-seed", "produk seed failed", map[string]any{"error": err.Error()})
	}
	if err := seedTransaksi(ctx, s); err != nil {
		logger.WarnCF("kios-seed", "transaksi seed failed", map[string]any{"error": err.Error()})
	}
	if err := seedPembelian(ctx, s); err != nil {
		logger.WarnCF("kios-seed", "pembelian seed failed", map[string]any{"error": err.Error()})
	}
	if err := seedPriceHistory(ctx, s); err != nil {
		logger.WarnCF("kios-seed", "price_history seed failed", map[string]any{"error": err.Error()})
	}
	if err := seedUsers(ctx, s); err != nil {
		logger.WarnCF("kios-seed", "users seed failed", map[string]any{"error": err.Error()})
	}

	if err := s.MarkSeedDone(ctx); err != nil {
		return fmt.Errorf("mark seed done: %w", err)
	}
	logger.InfoCF("kios-seed", "Seed complete", nil)
	return nil
}

func openCSV(name string) ([][]string, error) {
	f, err := os.Open(filepath.Join(seedDir(), name))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, nil
	}
	return rows[1:], nil // skip header
}

func parseInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, _ := strconv.Atoi(s)
	return n
}

func parseBool(s string) bool {
	return s == "1" || strings.ToLower(s) == "true"
}

func seedProduk(ctx context.Context, s *Store) error {
	rows, err := openCSV("stok.csv")
	if err != nil {
		return err
	}
	for _, row := range rows {
		if len(row) < 13 {
			continue
		}
		expDate := ""
		if len(row) >= 14 {
			expDate = strings.TrimSpace(row[13])
		}
		p := &Produk{
			ID:          strings.TrimSpace(row[0]),
			Nama:        strings.TrimSpace(row[1]),
			Kategori:    strings.TrimSpace(row[2]),
			Satuan:      strings.TrimSpace(row[3]),
			Stok:        parseInt(row[4]),
			HargaBeli:   parseInt(row[5]),
			HargaJual:   parseInt(row[6]),
			StokMinimum: parseInt(row[7]),
			StokKritis:  parseInt(row[8]),
			Supplier:    strings.TrimSpace(row[9]),
			LastUpdate:  strings.TrimSpace(row[10]),
			HasExp:      parseBool(row[11]),
			ExpDate:     expDate,
		}
		if err := s.SetProduk(ctx, p); err != nil {
			logger.WarnCF("kios-seed", "produk set failed", map[string]any{
				"id": p.ID, "error": err.Error(),
			})
		}
	}
	return nil
}

func seedTransaksi(ctx context.Context, s *Store) error {
	rows, err := openCSV("transaksi.csv")
	if err != nil {
		return err
	}
	for _, row := range rows {
		if len(row) < 12 {
			continue
		}
		sessID := ""
		if len(row) >= 13 {
			sessID = strings.TrimSpace(row[12])
		}
		t := &Transaksi{
			ID:          strings.TrimSpace(row[0]),
			Tanggal:     strings.TrimSpace(row[1]),
			Jam:         strings.TrimSpace(row[2]),
			ProdukID:    strings.TrimSpace(row[3]),
			NamaProduk:  strings.TrimSpace(row[4]),
			Kategori:    strings.TrimSpace(row[5]),
			Qty:         parseInt(row[6]),
			HargaSatuan: parseInt(row[7]),
			Total:       parseInt(row[8]),
			MetodeBayar: strings.TrimSpace(row[9]),
			Kasir:       strings.TrimSpace(row[10]),
			Catatan:     strings.TrimSpace(row[11]),
			SessionID:   sessID,
		}
		// Use RPush directly with the existing ID (don't auto-increment).
		if err := seedAppendTransaksi(ctx, s, t); err != nil {
			logger.WarnCF("kios-seed", "transaksi append failed", map[string]any{
				"id": t.ID, "error": err.Error(),
			})
		}
	}
	// Sync seq counter with the highest existing ID.
	syncSeq(ctx, s, keySeqTrx, "TRX-", rows)
	return nil
}

func seedAppendTransaksi(ctx context.Context, s *Store, t *Transaksi) error {
	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return s.rdb.RPush(ctx, keyTransaksi, string(b)).Err()
}

func seedPembelian(ctx context.Context, s *Store) error {
	rows, err := openCSV("pembelian.csv")
	if err != nil {
		return err
	}
	for _, row := range rows {
		if len(row) < 11 {
			continue
		}
		catatan := ""
		if len(row) >= 12 {
			catatan = strings.TrimSpace(row[11])
		}
		p := &Pembelian{
			ID:         strings.TrimSpace(row[0]),
			SessionID:  strings.TrimSpace(row[1]),
			Tanggal:    strings.TrimSpace(row[2]),
			Jam:        strings.TrimSpace(row[3]),
			ProdukID:   strings.TrimSpace(row[4]),
			NamaProduk: strings.TrimSpace(row[5]),
			Qty:        parseInt(row[6]),
			HargaBeli:  parseInt(row[7]),
			Subtotal:   parseInt(row[8]),
			Supplier:   strings.TrimSpace(row[9]),
			Kasir:      strings.TrimSpace(row[10]),
			Catatan:    catatan,
		}
		b, err := json.Marshal(p)
		if err != nil {
			continue
		}
		s.rdb.RPush(ctx, keyPembelian, string(b))
	}
	syncSeq(ctx, s, keySeqPem, "PEM-", rows)
	return nil
}

func seedPriceHistory(ctx context.Context, s *Store) error {
	rows, err := openCSV("price-history.csv")
	if err != nil {
		return err
	}
	for _, row := range rows {
		if len(row) < 10 {
			continue
		}
		ph := &PriceHistory{
			ID:         strings.TrimSpace(row[0]),
			Tanggal:    strings.TrimSpace(row[1]),
			Jam:        strings.TrimSpace(row[2]),
			ProdukID:   strings.TrimSpace(row[3]),
			NamaProduk: strings.TrimSpace(row[4]),
			HargaLama:  parseInt(row[5]),
			HargaBaru:  parseInt(row[6]),
			Selisih:    parseInt(row[7]),
			Supplier:   strings.TrimSpace(row[8]),
			Kasir:      strings.TrimSpace(row[9]),
		}
		b, err := json.Marshal(ph)
		if err != nil {
			continue
		}
		s.rdb.RPush(ctx, keyPriceHist, string(b))
	}
	syncSeq(ctx, s, keySeqPhg, "PHG-", rows)
	return nil
}

func seedUsers(ctx context.Context, s *Store) error {
	f, err := os.Open(filepath.Join(seedDir(), "users.json"))
	if err != nil {
		return err
	}
	defer f.Close()

	var raw map[string]struct {
		Phone       string `json:"phone"`
		Nama        string `json:"nama"`
		Role        string `json:"role"`
		Aktif       bool   `json:"aktif"`
		Ditambahkan string `json:"ditambahkan"`
	}
	if err := json.NewDecoder(f).Decode(&raw); err != nil {
		return err
	}
	for phone, u := range raw {
		user := &UserKios{
			Phone:       phone,
			Nama:        u.Nama,
			Role:        u.Role,
			Aktif:       u.Aktif,
			Ditambahkan: u.Ditambahkan,
		}
		if err := s.SetUser(ctx, user); err != nil {
			logger.WarnCF("kios-seed", "user set failed", map[string]any{
				"phone": phone, "error": err.Error(),
			})
		}
	}
	return nil
}

// syncSeq advances the INCR counter so new IDs don't collide with seeded ones.
func syncSeq(ctx context.Context, s *Store, seqKey, prefix string, rows [][]string) {
	max := 0
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		id := strings.TrimPrefix(strings.TrimSpace(row[0]), prefix)
		if n, err := strconv.Atoi(id); err == nil && n > max {
			max = n
		}
	}
	if max > 0 {
		// SET only if the current value is less than max.
		cur, err := s.rdb.Get(ctx, seqKey).Int()
		if err != nil || cur < max {
			s.rdb.Set(ctx, seqKey, max, 0)
		}
	}
}
