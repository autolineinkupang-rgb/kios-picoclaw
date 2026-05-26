package kios

import (
	"context"
	"encoding/csv"
	"os"
	"strings"
)

// ImportResult summarizes a bulk CSV import.
type ImportResult struct {
	Created int
	Updated int
	Skipped int
}

// readCSVRows reads a CSV into a slice of header→value maps (header lowercased).
func readCSVRows(path string) ([]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	recs, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(recs) < 2 {
		return nil, nil
	}
	headers := make([]string, len(recs[0]))
	for i, h := range recs[0] {
		headers[i] = strings.ToLower(strings.TrimSpace(h))
	}
	out := make([]map[string]string, 0, len(recs)-1)
	for _, rec := range recs[1:] {
		m := make(map[string]string, len(headers))
		for i, v := range rec {
			if i < len(headers) {
				m[headers[i]] = strings.TrimSpace(v)
			}
		}
		out = append(out, m)
	}
	return out, nil
}

// csvInt parses an int from a CSV cell tolerating "Rp"/dots/commas, with a default.
func csvInt(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	v := parseRupiah(s)
	if v == 0 && !strings.ContainsAny(s, "0") {
		return def
	}
	return v
}

// ImportProdukCSV loads products from a CSV (headers: nama, kategori, satuan,
// stok, harga_beli, harga_jual, stok_minimum, stok_kritis, supplier). Products
// are matched by exact name (case-insensitive): existing ones are updated,
// new ones created with an auto id.
func ImportProdukCSV(ctx context.Context, s *Store, path string) (ImportResult, error) {
	rows, err := readCSVRows(path)
	if err != nil {
		return ImportResult{}, err
	}
	return ImportProdukRows(ctx, s, rows)
}

// ImportProdukRows imports already-parsed rows (header→value maps). Used by
// both the CSV and XLSX import paths.
func ImportProdukRows(ctx context.Context, s *Store, rows []map[string]string) (ImportResult, error) {
	var res ImportResult
	all, err := s.GetAllProduk(ctx)
	if err != nil {
		return res, err
	}
	byNama := make(map[string]*Produk, len(all))
	for _, p := range all {
		byNama[strings.ToLower(p.Nama)] = p
	}
	now := NowWITA().Format("2006-01-02")
	for _, row := range rows {
		nama := strings.TrimSpace(row["nama"])
		if nama == "" {
			res.Skipped++
			continue
		}
		p, isNew := byNama[strings.ToLower(nama)], false
		if p == nil {
			isNew = true
			id, err := s.NextProdukID(ctx)
			if err != nil {
				return res, err
			}
			p = &Produk{ID: id, Nama: nama, Kategori: "umum", Satuan: "pcs", StokMinimum: 5, StokKritis: 2}
		}
		if v := row["barcode"]; v != "" {
			p.Barcode = v
		}
		if v := row["kategori"]; v != "" {
			p.Kategori = v
		}
		if v := row["satuan"]; v != "" {
			p.Satuan = v
		}
		if v := row["supplier"]; v != "" {
			p.Supplier = v
		}
		p.Stok = csvInt(row["stok"], p.Stok)
		p.HargaBeli = csvInt(row["harga_beli"], p.HargaBeli)
		p.HargaJual = csvInt(row["harga_jual"], p.HargaJual)
		p.StokMinimum = csvInt(row["stok_minimum"], p.StokMinimum)
		p.StokKritis = csvInt(row["stok_kritis"], p.StokKritis)
		p.LastUpdate = now
		if err := s.SetProduk(ctx, p); err != nil {
			return res, err
		}
		if isNew {
			byNama[strings.ToLower(nama)] = p
			res.Created++
		} else {
			res.Updated++
		}
	}
	return res, nil
}

// ImportSupplierCSV loads suppliers from a CSV (headers: nama, kontak, alamat,
// produk_utama, catatan). Matched by exact name (case-insensitive).
func ImportSupplierCSV(ctx context.Context, s *Store, path string) (ImportResult, error) {
	rows, err := readCSVRows(path)
	if err != nil {
		return ImportResult{}, err
	}
	return ImportSupplierRows(ctx, s, rows)
}

// ImportPustakaCSV loads knowledge-base entries from a CSV (headers: judul,
// info, url, kategori). Unsafe URLs (SkorAman < 60) are skipped.
func ImportPustakaCSV(ctx context.Context, s *Store, path string) (ImportResult, error) {
	rows, err := readCSVRows(path)
	if err != nil {
		return ImportResult{}, err
	}
	return ImportPustakaRows(ctx, s, rows)
}

// ImportPustakaRows imports knowledge-base rows. Entries are matched by URL
// (or judul when no URL): existing updated, new created. URLs failing the
// malware-safety check are skipped.
func ImportPustakaRows(ctx context.Context, s *Store, rows []map[string]string) (ImportResult, error) {
	var res ImportResult
	all, err := s.GetAllPustaka(ctx)
	if err != nil {
		return res, err
	}
	byKey := make(map[string]*Pustaka, len(all)*2)
	for _, p := range all {
		if p.URL != "" {
			byKey["u:"+strings.ToLower(p.URL)] = p
		}
		if p.Judul != "" {
			byKey["j:"+strings.ToLower(p.Judul)] = p
		}
	}
	now := NowWITA().Format("2006-01-02 15:04")
	for _, row := range rows {
		judul := strings.TrimSpace(row["judul"])
		info := strings.TrimSpace(row["info"])
		url := strings.TrimSpace(row["url"])
		if judul == "" && url == "" {
			res.Skipped++
			continue
		}
		skor := 0
		if url != "" {
			sf := SkorAman(url)
			if !sf.Aman {
				res.Skipped++ // reject unsafe source
				continue
			}
			skor = sf.Skor
		}
		var p *Pustaka
		if url != "" {
			p = byKey["u:"+strings.ToLower(url)]
		}
		if p == nil && judul != "" {
			p = byKey["j:"+strings.ToLower(judul)]
		}
		isNew := p == nil
		if isNew {
			id, err := s.NextPustakaID(ctx)
			if err != nil {
				return res, err
			}
			p = &Pustaka{ID: id, Ditambahkan: now}
		}
		if judul != "" {
			p.Judul = judul
		}
		if info != "" {
			p.Info = info
		}
		if url != "" {
			p.URL = url
			p.Skor = skor
		}
		if v := row["kategori"]; v != "" {
			p.Kategori = v
		}
		if err := s.SetPustaka(ctx, p); err != nil {
			return res, err
		}
		if isNew {
			if url != "" {
				byKey["u:"+strings.ToLower(url)] = p
			}
			if judul != "" {
				byKey["j:"+strings.ToLower(judul)] = p
			}
			res.Created++
		} else {
			res.Updated++
		}
	}
	return res, nil
}

// ImportSupplierRows imports already-parsed supplier rows.
func ImportSupplierRows(ctx context.Context, s *Store, rows []map[string]string) (ImportResult, error) {
	var res ImportResult
	all, err := s.GetAllSupplier(ctx)
	if err != nil {
		return res, err
	}
	byNama := make(map[string]*Supplier, len(all))
	for _, sup := range all {
		byNama[strings.ToLower(sup.Nama)] = sup
	}
	for _, row := range rows {
		nama := strings.TrimSpace(row["nama"])
		if nama == "" {
			res.Skipped++
			continue
		}
		sup, isNew := byNama[strings.ToLower(nama)], false
		if sup == nil {
			isNew = true
			id, err := s.NextSupplierID(ctx)
			if err != nil {
				return res, err
			}
			sup = &Supplier{ID: id, Nama: nama}
		}
		if v := row["kontak"]; v != "" {
			sup.Kontak = v
		}
		if v := row["alamat"]; v != "" {
			sup.Alamat = v
		}
		if v := row["produk_utama"]; v != "" {
			sup.ProdukUtama = v
		}
		if v := row["catatan"]; v != "" {
			sup.Catatan = v
		}
		if err := s.SetSupplier(ctx, sup); err != nil {
			return res, err
		}
		if isNew {
			byNama[strings.ToLower(nama)] = sup
			res.Created++
		} else {
			res.Updated++
		}
	}
	return res, nil
}
