package kios

import (
	"context"
	"encoding/json"
	"strings"
)

// HargaSupplierLast adalah snapshot harga beli per-suplier yang diperbarui
// setiap kali ada restock dari suplier tersebut. Disimpan di HASH
// kios:harga_supplier_last dengan field "<produkID>|<supplierID>".
// Berbeda dari kios:harga_supplier (override manual): field ini diisi otomatis
// dari restock, bukan diisi manual oleh owner.
type HargaSupplierLast struct {
	Harga     int    `json:"harga"`      // harga beli per pcs (rupiah)
	Kemasan   string `json:"kemasan"`    // mis. "dos", "lusin"
	Isi       int    `json:"isi"`        // pcs per kemasan
	HargaPack int    `json:"harga_pack"` // harga total satu kemasan (rupiah)
	Tanggal   string `json:"tanggal"`    // YYYY-MM-DD (WITA)
}

// hargaSupplierLastField membentuk field hash: "<produkID>|<supplierID>".
func hargaSupplierLastField(produkID, supplierID string) string {
	return produkID + "|" + supplierID
}

// GetAllHargaSupplierLast mengembalikan semua snapshot harga beli suplier.
// Map key = "<produkID>|<supplierID>".
func (s *Store) GetAllHargaSupplierLast(ctx context.Context) (map[string]HargaSupplierLast, error) {
	m, err := s.rdb.HGetAll(ctx, keyHargaSupplierLast).Result()
	if err != nil {
		return nil, err
	}
	out := make(map[string]HargaSupplierLast, len(m))
	for k, v := range m {
		var h HargaSupplierLast
		if json.Unmarshal([]byte(v), &h) == nil {
			out[k] = h
		}
	}
	return out, nil
}

// SetHargaSupplierLast menyimpan snapshot harga beli untuk kombinasi
// (produkID, supplierID). Dipanggil oleh recordPembelian setelah restock pack.
func (s *Store) SetHargaSupplierLast(ctx context.Context, produkID, supplierID string, v HargaSupplierLast) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.rdb.HSet(ctx, keyHargaSupplierLast, hargaSupplierLastField(produkID, supplierID), string(b)).Err()
}

// DelHargaSupplierLastBySuplier menghapus semua snapshot yang terkait dengan
// supplierID tertentu di semua produk. Dipanggil dari hapus() di supplier.go.
func (s *Store) DelHargaSupplierLastBySuplier(ctx context.Context, supplierID string) error {
	all, err := s.rdb.HGetAll(ctx, keyHargaSupplierLast).Result()
	if err != nil {
		return err
	}
	suffix := "|" + supplierID
	for field := range all {
		if strings.HasSuffix(field, suffix) {
			if err := s.rdb.HDel(ctx, keyHargaSupplierLast, field).Err(); err != nil {
				return err
			}
		}
	}
	return nil
}
