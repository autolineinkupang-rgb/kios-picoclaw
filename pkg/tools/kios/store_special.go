package kios

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

// GetPulsaDenom returns the config for one nominal, or nil if not set.
func (s *Store) GetPulsaDenom(ctx context.Context, nominal int) (*PulsaDenom, error) {
	val, err := s.rdb.HGet(ctx, keyPulsaDenom, strconv.Itoa(nominal)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var d PulsaDenom
	if err := json.Unmarshal([]byte(val), &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// SetPulsaDenom saves or overwrites a nominal config.
func (s *Store) SetPulsaDenom(ctx context.Context, d *PulsaDenom) error {
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return s.rdb.HSet(ctx, keyPulsaDenom, strconv.Itoa(d.Nominal), string(b)).Err()
}

// GetAllPulsaDenom returns all configured nominals.
func (s *Store) GetAllPulsaDenom(ctx context.Context) ([]*PulsaDenom, error) {
	m, err := s.rdb.HGetAll(ctx, keyPulsaDenom).Result()
	if err != nil {
		return nil, err
	}
	out := make([]*PulsaDenom, 0, len(m))
	for _, v := range m {
		var d PulsaDenom
		if err := json.Unmarshal([]byte(v), &d); err == nil {
			out = append(out, &d)
		}
	}
	return out, nil
}

// nextPtuID returns the next top-up ID (PTU-0001, ...).
func (s *Store) nextPtuID(ctx context.Context) (string, error) {
	n, err := s.rdb.Incr(ctx, keySeqPtu).Result()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("PTU-%04d", n), nil
}

// AppendPulsaTopup records a top-up event; ID is filled automatically.
func (s *Store) AppendPulsaTopup(ctx context.Context, pt *PulsaTopup) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, err := s.nextPtuID(ctx)
	if err != nil {
		return err
	}
	pt.ID = id
	b, err := json.Marshal(pt)
	if err != nil {
		return err
	}
	return s.rdb.RPush(ctx, keyPulsaTopup, string(b)).Err()
}

// GetAllPulsaTopup returns all top-up history in append order.
func (s *Store) GetAllPulsaTopup(ctx context.Context) ([]*PulsaTopup, error) {
	vals, err := s.rdb.LRange(ctx, keyPulsaTopup, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	out := make([]*PulsaTopup, 0, len(vals))
	for _, v := range vals {
		var pt PulsaTopup
		if err := json.Unmarshal([]byte(v), &pt); err == nil {
			out = append(out, &pt)
		}
	}
	return out, nil
}

// IncrSaldoModal adds delta to a product's SaldoModal (pulsa anchor).
func (s *Store) IncrSaldoModal(ctx context.Context, produkID string, delta int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.GetProduk(ctx, produkID)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("produk %s tidak ditemukan", produkID)
	}
	p.SaldoModal += delta
	return s.SetProduk(ctx, p)
}

// DecrSaldoModal reduces SaldoModal by delta. Returns error if delta > SaldoModal.
func (s *Store) DecrSaldoModal(ctx context.Context, produkID string, delta int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.GetProduk(ctx, produkID)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("produk %s tidak ditemukan", produkID)
	}
	if p.SaldoModal < delta {
		return fmt.Errorf("saldo modal pulsa tidak cukup (saldo %s, butuh %s)",
			FormatRupiah(p.SaldoModal), FormatRupiah(delta))
	}
	p.SaldoModal -= delta
	return s.SetProduk(ctx, p)
}
