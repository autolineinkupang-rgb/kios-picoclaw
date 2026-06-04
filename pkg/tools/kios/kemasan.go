package kios

import (
	"math"
	"strings"
)

// Kemasan mendefinisikan satu ukuran kemasan restock untuk sebuah produk.
// Tersimpan sebagai slice di Produk.PackDefs; lihat store.go.
type Kemasan struct {
	Nama string `json:"nama"` // mis. "dos", "lusin", "box"
	Isi  int    `json:"isi"`  // jumlah pcs per satu kemasan
}

// packVocab adalah vocab kemasan bawaan dengan isi default-nya.
// Dipakai sebagai fallback ketika produk belum punya PackDefs.
var packVocab = []struct {
	Nama       string
	DefaultIsi int
}{
	{"dos", 48},
	{"karton", 48},
	{"ball", 48},
	{"box", 24},
	{"lusin", 12},
	{"setengah lusin", 6},
	{"half lusin", 6},
	{"renteng", 10},
	{"slop", 10},
	{"pak", 10},
}

// lookupIsi mencari jumlah isi (pcs) per kemasan untuk produk tertentu.
// Urutan pencarian:
//  1. PackDefs produk — case-insensitive exact match pada Nama.
//  2. packVocab bawaan — case-insensitive contains match.
//  3. 0 bila tidak ditemukan (caller harus error bila isi 0).
func lookupIsi(item *Produk, kemasan string) int {
	k := strings.ToLower(strings.TrimSpace(kemasan))
	if k == "" {
		return 0
	}
	// 1. Cari di PackDefs produk
	for _, pd := range item.PackDefs {
		if strings.EqualFold(pd.Nama, k) && pd.Isi > 0 {
			return pd.Isi
		}
	}
	// 2. Fallback ke vocab bawaan
	for _, v := range packVocab {
		if strings.Contains(strings.ToLower(v.Nama), k) || strings.Contains(k, strings.ToLower(v.Nama)) {
			return v.DefaultIsi
		}
	}
	return 0
}

// computeFromPack menghitung qty pcs dan harga beli per pcs dari input kemasan.
//
//	qty        = qtyPack * isi
//	hargaBeli  = round(hargaPack / isi)
//
// Parameter kemasan hanya untuk dokumentasi; kalkulasi tidak memakainya.
func computeFromPack(kemasan string, qtyPack, hargaPack, isi int) (qty int, hargaBeliPerPcs int) {
	_ = kemasan
	qty = qtyPack * isi
	hargaBeliPerPcs = int(math.Round(float64(hargaPack) / float64(isi)))
	return
}
