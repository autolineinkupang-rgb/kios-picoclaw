package kios

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// --- Piutang ---

func (s *Store) NextPiutangID(ctx context.Context) (string, error) {
	n, err := s.rdb.Incr(ctx, keySeqPiu).Result()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("PIU-%04d", n), nil
}

func (s *Store) SetPiutang(ctx context.Context, p *Piutang) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return s.rdb.HSet(ctx, keyPiutang, p.ID, string(b)).Err()
}

func (s *Store) GetPiutang(ctx context.Context, id string) (*Piutang, error) {
	val, err := s.rdb.HGet(ctx, keyPiutang, id).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p Piutang
	if err := json.Unmarshal([]byte(val), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) GetAllPiutang(ctx context.Context) ([]*Piutang, error) {
	m, err := s.rdb.HGetAll(ctx, keyPiutang).Result()
	if err != nil {
		return nil, err
	}
	out := make([]*Piutang, 0, len(m))
	for _, v := range m {
		var p Piutang
		if json.Unmarshal([]byte(v), &p) == nil {
			out = append(out, &p)
		}
	}
	return out, nil
}

func (s *Store) DelPiutang(ctx context.Context, id string) error {
	return s.rdb.HDel(ctx, keyPiutang, id).Err()
}

// --- Hutang ---

func (s *Store) NextHutangID(ctx context.Context) (string, error) {
	n, err := s.rdb.Incr(ctx, keySeqHut).Result()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("HUT-%04d", n), nil
}

func (s *Store) SetHutang(ctx context.Context, h *Hutang) error {
	b, err := json.Marshal(h)
	if err != nil {
		return err
	}
	return s.rdb.HSet(ctx, keyHutang, h.ID, string(b)).Err()
}

func (s *Store) GetHutang(ctx context.Context, id string) (*Hutang, error) {
	val, err := s.rdb.HGet(ctx, keyHutang, id).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var h Hutang
	if err := json.Unmarshal([]byte(val), &h); err != nil {
		return nil, err
	}
	return &h, nil
}

func (s *Store) GetAllHutang(ctx context.Context) ([]*Hutang, error) {
	m, err := s.rdb.HGetAll(ctx, keyHutang).Result()
	if err != nil {
		return nil, err
	}
	out := make([]*Hutang, 0, len(m))
	for _, v := range m {
		var h Hutang
		if json.Unmarshal([]byte(v), &h) == nil {
			out = append(out, &h)
		}
	}
	return out, nil
}

func (s *Store) DelHutang(ctx context.Context, id string) error {
	return s.rdb.HDel(ctx, keyHutang, id).Err()
}

// --- Pembayaran ---

func (s *Store) NextPayID(ctx context.Context) (string, error) {
	n, err := s.rdb.Incr(ctx, keySeqPay).Result()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("PAY-%04d", n), nil
}

func (s *Store) AppendPembayaran(ctx context.Context, p *Pembayaran) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return s.rdb.RPush(ctx, keyPembayaran, string(b)).Err()
}

func (s *Store) GetAllPembayaran(ctx context.Context) ([]*Pembayaran, error) {
	vals, err := s.rdb.LRange(ctx, keyPembayaran, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	out := make([]*Pembayaran, 0, len(vals))
	for _, v := range vals {
		var p Pembayaran
		if json.Unmarshal([]byte(v), &p) == nil {
			out = append(out, &p)
		}
	}
	return out, nil
}

// --- Helpers untuk bon tool ---

// GetPembelianByID scans the pembelian list for a matching ID.
func (s *Store) GetPembelianByID(ctx context.Context, id string) (*Pembelian, error) {
	all, err := s.GetAllPembelian(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range all {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, nil
}

// GetSupplierByID returns the supplier with the given ID, or nil.
func (s *Store) GetSupplierByID(ctx context.Context, id string) (*Supplier, error) {
	all, err := s.GetAllSupplier(ctx)
	if err != nil {
		return nil, err
	}
	for _, sup := range all {
		if sup.ID == id {
			return sup, nil
		}
	}
	return nil, nil
}
