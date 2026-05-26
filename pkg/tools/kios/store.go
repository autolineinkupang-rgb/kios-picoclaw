// Package kios provides village-shop AI tools for picoclaw.
// Data persists in Upstash Redis (rediss:// URL from $UPSTASH_REDIS_URL).
package kios

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Produk represents a product in the shop.
type Produk struct {
	ID          string `json:"id"`
	Nama        string `json:"nama"`
	Kategori    string `json:"kategori"`
	Satuan      string `json:"satuan"`
	Stok        int    `json:"stok"`
	HargaBeli   int    `json:"harga_beli"`
	HargaJual   int    `json:"harga_jual"`
	StokMinimum int    `json:"stok_minimum"`
	StokKritis  int    `json:"stok_kritis"`
	Supplier    string `json:"supplier"`
	LastUpdate  string `json:"last_update"`
	HasExp      bool   `json:"has_exp"`
	ExpDate     string `json:"exp_date"`
}

// Transaksi represents a sales transaction.
type Transaksi struct {
	ID          string `json:"id"`
	Tanggal     string `json:"tanggal"`
	Jam         string `json:"jam"`
	ProdukID    string `json:"produk_id"`
	NamaProduk  string `json:"nama_produk"`
	Kategori    string `json:"kategori"`
	Qty         int    `json:"qty"`
	HargaSatuan int    `json:"harga_satuan"`
	Total       int    `json:"total"`
	MetodeBayar string `json:"metode_bayar"`
	Kasir       string `json:"kasir"`
	Catatan     string `json:"catatan"`
	SessionID   string `json:"session_id"`
}

// Pembelian represents a restocking purchase.
type Pembelian struct {
	ID         string `json:"id"`
	SessionID  string `json:"session_id"`
	Tanggal    string `json:"tanggal"`
	Jam        string `json:"jam"`
	ProdukID   string `json:"produk_id"`
	NamaProduk string `json:"nama_produk"`
	Qty        int    `json:"qty"`
	HargaBeli  int    `json:"harga_beli"`
	Subtotal   int    `json:"subtotal"`
	Supplier   string `json:"supplier"`
	Kasir      string `json:"kasir"`
	Catatan    string `json:"catatan"`
}

// PriceHistory represents a price change event.
type PriceHistory struct {
	ID         string `json:"id"`
	Tanggal    string `json:"tanggal"`
	Jam        string `json:"jam"`
	ProdukID   string `json:"produk_id"`
	NamaProduk string `json:"nama_produk"`
	HargaLama  int    `json:"harga_lama"`
	HargaBaru  int    `json:"harga_baru"`
	Selisih    int    `json:"selisih"`
	Supplier   string `json:"supplier"`
	Kasir      string `json:"kasir"`
}

// Shift represents the current cashier shift.
type Shift struct {
	Kasir      string `json:"kasir"`
	SaldoAwal  int    `json:"saldo_awal"`
	SaldoAkhir int    `json:"saldo_akhir"`
	WaktuBuka  string `json:"waktu_buka"`
	WaktuTutup string `json:"waktu_tutup"`
	Status     string `json:"status"` // "buka" | "tutup"
}

// UserKios represents an authorized shop user.
type UserKios struct {
	Phone       string `json:"phone"`
	Nama        string `json:"nama"`
	Role        string `json:"role"` // "kasir" | "owner"
	Aktif       bool   `json:"aktif"`
	Ditambahkan string `json:"ditambahkan"`
}

// Redis key constants.
const (
	keyProduk    = "kios:produk"
	keyTransaksi = "kios:transaksi"
	keySeqTrx    = "kios:seq:trx"
	keyPembelian = "kios:pembelian"
	keySeqPem    = "kios:seq:pem"
	keyPriceHist = "kios:price_history"
	keySeqPhg    = "kios:seq:phg"
	keyShift     = "kios:shift"
	keyUsers     = "kios:users"
	keySeedDone  = "kios:seed:done"
	keySupplier  = "kios:supplier"
	keySeqSup    = "kios:seq:sup"
	keyPromo     = "kios:promo"
	keySeqPromo  = "kios:seq:promo"
	keyPustaka   = "kios:pustaka"
	keySeqPus    = "kios:seq:pus"
	keyPasarRef  = "kios:pasar:ref"
	keyLearnPat  = "kios:learn:patterns"
	keyLearnAls  = "kios:learn:aliases"
	keyLearnShc  = "kios:learn:shortcuts"
	keyLearnHab  = "kios:learn:habits"
	keyLearnUnk  = "kios:learn:unknowns"
)

// Store provides all Redis operations for kios tools.
type Store struct {
	rdb *redis.Client
	mu  sync.Mutex
}

// NewStore creates a Store connected to Upstash Redis via $UPSTASH_REDIS_URL.
func NewStore() (*Store, error) {
	url := os.Getenv("UPSTASH_REDIS_URL")
	if url == "" {
		return nil, fmt.Errorf("UPSTASH_REDIS_URL is not set")
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("invalid UPSTASH_REDIS_URL: %w", err)
	}
	rdb := redis.NewClient(opt)
	return &Store{rdb: rdb}, nil
}

// NewStoreWithClient creates a Store with an injected Redis client (for tests).
func NewStoreWithClient(rdb *redis.Client) *Store {
	return &Store{rdb: rdb}
}

// Ping verifies the Redis connection.
func (s *Store) Ping(ctx context.Context) error {
	return s.rdb.Ping(ctx).Err()
}

// --- Produk ---

// GetProduk returns a product by ID or nil if not found.
func (s *Store) GetProduk(ctx context.Context, id string) (*Produk, error) {
	val, err := s.rdb.HGet(ctx, keyProduk, id).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p Produk
	if err := json.Unmarshal([]byte(val), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// GetAllProduk returns all products.
func (s *Store) GetAllProduk(ctx context.Context) ([]*Produk, error) {
	m, err := s.rdb.HGetAll(ctx, keyProduk).Result()
	if err != nil {
		return nil, err
	}
	result := make([]*Produk, 0, len(m))
	for _, v := range m {
		var p Produk
		if err := json.Unmarshal([]byte(v), &p); err != nil {
			continue
		}
		result = append(result, &p)
	}
	return result, nil
}

// SetProduk stores a product.
func (s *Store) SetProduk(ctx context.Context, p *Produk) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return s.rdb.HSet(ctx, keyProduk, p.ID, string(b)).Err()
}

// DelProduk removes a product by ID.
func (s *Store) DelProduk(ctx context.Context, id string) error {
	return s.rdb.HDel(ctx, keyProduk, id).Err()
}

// NextProdukID generates the next product ID (zero-padded 3 digits).
func (s *Store) NextProdukID(ctx context.Context) (string, error) {
	all, err := s.GetAllProduk(ctx)
	if err != nil {
		return "", err
	}
	max := 0
	for _, p := range all {
		n, err := strconv.Atoi(p.ID)
		if err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("%03d", max+1), nil
}

// CariProduk finds products: exact ID, or nama substring/all-words match.
func CariProduk(list []*Produk, query string) []*Produk {
	q := strings.ToLower(strings.TrimSpace(query))
	var results []*Produk
	for _, p := range list {
		if p.ID == query {
			return []*Produk{p}
		}
		namaLower := strings.ToLower(p.Nama)
		if strings.Contains(namaLower, q) {
			results = append(results, p)
			continue
		}
		// all query words present in nama
		words := strings.Fields(q)
		if len(words) > 1 {
			allPresent := true
			for _, w := range words {
				if !strings.Contains(namaLower, w) {
					allPresent = false
					break
				}
			}
			if allPresent {
				results = append(results, p)
			}
		}
	}
	return results
}

// --- Transaksi ---

// AppendTransaksi appends a transaction to the list. Returns auto-generated ID.
func (s *Store) AppendTransaksi(ctx context.Context, t *Transaksi) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.rdb.Incr(ctx, keySeqTrx).Result()
	if err != nil {
		return "", err
	}
	t.ID = fmt.Sprintf("TRX-%04d", n)
	b, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	return t.ID, s.rdb.RPush(ctx, keyTransaksi, string(b)).Err()
}

// GetAllTransaksi returns all transactions.
func (s *Store) GetAllTransaksi(ctx context.Context) ([]*Transaksi, error) {
	vals, err := s.rdb.LRange(ctx, keyTransaksi, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	result := make([]*Transaksi, 0, len(vals))
	for _, v := range vals {
		var t Transaksi
		if err := json.Unmarshal([]byte(v), &t); err != nil {
			continue
		}
		result = append(result, &t)
	}
	return result, nil
}

// RemoveTransaksi removes a transaction by ID, restoring stock.
// Returns the removed transaction, or nil if not found.
func (s *Store) RemoveTransaksi(ctx context.Context, id string) (*Transaksi, error) {
	vals, err := s.rdb.LRange(ctx, keyTransaksi, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	for _, v := range vals {
		var t Transaksi
		if err := json.Unmarshal([]byte(v), &t); err != nil {
			continue
		}
		if t.ID == id {
			if err := s.rdb.LRem(ctx, keyTransaksi, 1, v).Err(); err != nil {
				return nil, err
			}
			return &t, nil
		}
	}
	return nil, nil
}

// --- Pembelian ---

// AppendPembelian appends a purchase record.
func (s *Store) AppendPembelian(ctx context.Context, p *Pembelian) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.rdb.Incr(ctx, keySeqPem).Result()
	if err != nil {
		return "", err
	}
	p.ID = fmt.Sprintf("PEM-%04d", n)
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return p.ID, s.rdb.RPush(ctx, keyPembelian, string(b)).Err()
}

// --- Price History ---

// AppendPriceHistory appends a price change record.
func (s *Store) AppendPriceHistory(ctx context.Context, ph *PriceHistory) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.rdb.Incr(ctx, keySeqPhg).Result()
	if err != nil {
		return "", err
	}
	ph.ID = fmt.Sprintf("PHG-%04d", n)
	b, err := json.Marshal(ph)
	if err != nil {
		return "", err
	}
	return ph.ID, s.rdb.RPush(ctx, keyPriceHist, string(b)).Err()
}

// GetAllPriceHistory returns all price history entries.
func (s *Store) GetAllPriceHistory(ctx context.Context) ([]*PriceHistory, error) {
	vals, err := s.rdb.LRange(ctx, keyPriceHist, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	result := make([]*PriceHistory, 0, len(vals))
	for _, v := range vals {
		var ph PriceHistory
		if err := json.Unmarshal([]byte(v), &ph); err != nil {
			continue
		}
		result = append(result, &ph)
	}
	return result, nil
}

// --- Shift ---

// GetShift returns the current shift or nil.
func (s *Store) GetShift(ctx context.Context) (*Shift, error) {
	val, err := s.rdb.Get(ctx, keyShift).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var sh Shift
	if err := json.Unmarshal([]byte(val), &sh); err != nil {
		return nil, err
	}
	return &sh, nil
}

// SetShift stores the current shift.
func (s *Store) SetShift(ctx context.Context, sh *Shift) error {
	b, err := json.Marshal(sh)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, keyShift, string(b), 0).Err()
}

// --- Users ---

// GetUser returns a user by phone.
func (s *Store) GetUser(ctx context.Context, phone string) (*UserKios, error) {
	val, err := s.rdb.HGet(ctx, keyUsers, phone).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var u UserKios
	if err := json.Unmarshal([]byte(val), &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// SetUser stores a user record (keyed by u.Phone, which holds the user's
// channel identifier — e.g. the Telegram user ID).
func (s *Store) SetUser(ctx context.Context, u *UserKios) error {
	b, err := json.Marshal(u)
	if err != nil {
		return err
	}
	return s.rdb.HSet(ctx, keyUsers, u.Phone, string(b)).Err()
}

// GetAllUsers returns all registered users.
func (s *Store) GetAllUsers(ctx context.Context) ([]*UserKios, error) {
	m, err := s.rdb.HGetAll(ctx, keyUsers).Result()
	if err != nil {
		return nil, err
	}
	result := make([]*UserKios, 0, len(m))
	for _, v := range m {
		var u UserKios
		if err := json.Unmarshal([]byte(v), &u); err != nil {
			continue
		}
		result = append(result, &u)
	}
	return result, nil
}

// --- Seed ---

// IsSeedDone returns true if the one-time seed has already run.
func (s *Store) IsSeedDone(ctx context.Context) (bool, error) {
	n, err := s.rdb.Exists(ctx, keySeedDone).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// MarkSeedDone marks the seed as complete.
func (s *Store) MarkSeedDone(ctx context.Context) error {
	return s.rdb.Set(ctx, keySeedDone, "1", 0).Err()
}

// ResetSeed clears the one-time seed flag so a seed can run again.
func (s *Store) ResetSeed(ctx context.Context) error {
	return s.rdb.Del(ctx, keySeedDone).Err()
}

// --- Helpers ---

// NowWITA returns current time in WITA (UTC+8).
func NowWITA() time.Time {
	loc := time.FixedZone("WITA", 8*3600)
	return time.Now().In(loc)
}

// FormatRupiah formats an integer as "Rp 15.000".
func FormatRupiah(v int) string {
	s := strconv.Itoa(v)
	n := len(s)
	if n <= 3 {
		return "Rp " + s
	}
	var b strings.Builder
	rem := n % 3
	b.WriteString(s[:rem])
	for i := rem; i < n; i += 3 {
		if i > 0 {
			b.WriteByte('.')
		}
		b.WriteString(s[i : i+3])
	}
	result := b.String()
	if strings.HasPrefix(result, ".") {
		result = result[1:]
	}
	return "Rp " + result
}
