package kios

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Supplier represents a goods supplier.
type Supplier struct {
	ID          string `json:"id"`
	Nama        string `json:"nama"`
	Kontak      string `json:"kontak"`
	Alamat      string `json:"alamat"`
	ProdukUtama string `json:"produk_utama"`
	Catatan     string `json:"catatan"`
}

// Promo represents a discount on a product.
type Promo struct {
	ID       string  `json:"id"`
	Produk   string  `json:"produk"`
	ProdukID string  `json:"produk_id"`
	Tipe     string  `json:"tipe"` // "persen" | "fixed"
	Nilai    float64 `json:"nilai"`
	MinQty   int     `json:"min_qty"`
	Aktif    bool    `json:"aktif"`
	Mulai    string  `json:"mulai"`
	Selesai  string  `json:"selesai"`
	Catatan  string  `json:"catatan"`
}

// --- Pembelian read (for price comparison) ---

// GetAllPembelian returns all purchase records.
func (s *Store) GetAllPembelian(ctx context.Context) ([]*Pembelian, error) {
	vals, err := s.rdb.LRange(ctx, keyPembelian, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	out := make([]*Pembelian, 0, len(vals))
	for _, v := range vals {
		var p Pembelian
		if err := json.Unmarshal([]byte(v), &p); err == nil {
			out = append(out, &p)
		}
	}
	return out, nil
}

// --- Supplier ---

// GetAllSupplier returns all suppliers.
func (s *Store) GetAllSupplier(ctx context.Context) ([]*Supplier, error) {
	m, err := s.rdb.HGetAll(ctx, keySupplier).Result()
	if err != nil {
		return nil, err
	}
	out := make([]*Supplier, 0, len(m))
	for _, v := range m {
		var sup Supplier
		if err := json.Unmarshal([]byte(v), &sup); err == nil {
			out = append(out, &sup)
		}
	}
	return out, nil
}

// SetSupplier stores a supplier.
func (s *Store) SetSupplier(ctx context.Context, sup *Supplier) error {
	b, err := json.Marshal(sup)
	if err != nil {
		return err
	}
	return s.rdb.HSet(ctx, keySupplier, sup.ID, string(b)).Err()
}

// DelSupplier removes a supplier by ID.
func (s *Store) DelSupplier(ctx context.Context, id string) error {
	return s.rdb.HDel(ctx, keySupplier, id).Err()
}

// NextSupplierID generates the next supplier ID (SUP-NNN).
func (s *Store) NextSupplierID(ctx context.Context) (string, error) {
	n, err := s.rdb.Incr(ctx, keySeqSup).Result()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("SUP-%03d", n), nil
}

// CariSupplier finds a supplier by exact ID or nama substring.
func CariSupplier(list []*Supplier, query string) *Supplier {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	for _, sup := range list {
		if strings.EqualFold(sup.ID, query) || strings.Contains(strings.ToLower(sup.Nama), q) {
			return sup
		}
	}
	return nil
}

// --- Promo ---

// GetAllPromo returns all promos.
func (s *Store) GetAllPromo(ctx context.Context) ([]*Promo, error) {
	m, err := s.rdb.HGetAll(ctx, keyPromo).Result()
	if err != nil {
		return nil, err
	}
	out := make([]*Promo, 0, len(m))
	for _, v := range m {
		var p Promo
		if err := json.Unmarshal([]byte(v), &p); err == nil {
			out = append(out, &p)
		}
	}
	return out, nil
}

// SetPromo stores a promo.
func (s *Store) SetPromo(ctx context.Context, p *Promo) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return s.rdb.HSet(ctx, keyPromo, p.ID, string(b)).Err()
}

// NextPromoID generates the next promo ID (PROMO-NNNN).
func (s *Store) NextPromoID(ctx context.Context) (string, error) {
	n, err := s.rdb.Incr(ctx, keySeqPromo).Result()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("PROMO-%04d", n), nil
}

// promoAktif reports whether a promo is active on the given date (YYYY-MM-DD).
func promoAktif(p *Promo, today string) bool {
	if !p.Aktif {
		return false
	}
	if p.Selesai != "" && p.Selesai < today {
		return false
	}
	if p.Mulai != "" && p.Mulai > today {
		return false
	}
	return true
}

// parseQtyDefault parses an int from any value, returning def when absent/zero.
func parseQtyDefault(v any, def int) int {
	switch x := v.(type) {
	case float64:
		if int(x) > 0 {
			return int(x)
		}
	case int:
		if x > 0 {
			return x
		}
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(x)); err == nil && n > 0 {
			return n
		}
	}
	return def
}
