package kios

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/redis/go-redis/v9"
)

var reNonDigit = regexp.MustCompile(`\D`)

// NormalizePhone converts an Indonesian mobile number to the canonical "62..."
// format used as the HASH field key in kios:pelanggan.
// Accepts "08xx", "8xx", "+62xx", "62xx". Returns "" for empty, non-Indonesian,
// or implausibly short/long inputs.
func NormalizePhone(raw string) string {
	d := reNonDigit.ReplaceAllString(strings.TrimSpace(raw), "")
	if d == "" {
		return ""
	}
	if strings.HasPrefix(d, "0") {
		d = "62" + d[1:]
	} else if strings.HasPrefix(d, "8") {
		d = "62" + d
	}
	// Indonesian numbers: "62" + 8..13 local digits = 10..15 total chars.
	if len(d) < 10 || len(d) > 15 {
		return ""
	}
	// Reject non-Indonesian country codes (e.g. "+1650..." becomes "1650...").
	if !strings.HasPrefix(d, "62") {
		return ""
	}
	return d
}

// UpsertPelanggan creates or updates a Pelanggan record keyed by the normalised
// phone number. Calling it on each new order keeps the registry fresh without
// requiring a separate "register" step. Returns the updated record.
//
// Concurrency note: this is a read-modify-write without distributed locking.
// At single-kiosk scale (low concurrent checkout volume) the risk of a missed
// TotalPesanan increment is accepted as a known trade-off.
func (s *Store) UpsertPelanggan(ctx context.Context, nama, rawPhone string) (*Pelanggan, error) {
	phone := NormalizePhone(rawPhone)
	if phone == "" {
		return nil, fmt.Errorf("nomor WhatsApp tidak valid kak 🙏 (contoh: 08123456789)")
	}

	existing, err := s.GetPelanggan(ctx, phone)
	if err != nil {
		return nil, err
	}
	now := NowWITA()
	if existing == nil {
		existing = &Pelanggan{
			ID:        "PLG-" + phone,
			Phone:     phone,
			CreatedAt: now.Unix(),
		}
	}

	existing.Nama = strings.TrimSpace(nama)
	existing.TotalPesanan++
	existing.LastOrder = now.Format("2006-01-02")

	return existing, s.SetPelanggan(ctx, existing)
}

// SetPelanggan persists a Pelanggan record (field = Phone).
func (s *Store) SetPelanggan(ctx context.Context, p *Pelanggan) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return s.rdb.HSet(ctx, keyPelanggan, p.Phone, string(b)).Err()
}

// GetPelanggan returns the customer keyed by the canonical phone number
// (must already be in "62..." form — call NormalizePhone first if needed),
// or nil when no record exists.
func (s *Store) GetPelanggan(ctx context.Context, phone string) (*Pelanggan, error) {
	val, err := s.rdb.HGet(ctx, keyPelanggan, phone).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p Pelanggan
	if err := json.Unmarshal([]byte(val), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// GetAllPelanggan returns every customer in the registry, unsorted.
func (s *Store) GetAllPelanggan(ctx context.Context) ([]*Pelanggan, error) {
	m, err := s.rdb.HGetAll(ctx, keyPelanggan).Result()
	if err != nil {
		return nil, err
	}
	out := make([]*Pelanggan, 0, len(m))
	for _, v := range m {
		var p Pelanggan
		if err := json.Unmarshal([]byte(v), &p); err == nil {
			out = append(out, &p)
		}
	}
	return out, nil
}

// DelPelanggan removes a customer record by canonical phone ("62...") form.
// Owner-only at the tool layer.
func (s *Store) DelPelanggan(ctx context.Context, phone string) error {
	return s.rdb.HDel(ctx, keyPelanggan, phone).Err()
}
