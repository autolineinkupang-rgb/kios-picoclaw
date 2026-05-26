// Command kios-import loads a filled-in Excel (.xlsx) or CSV (produk or
// supplier) into Upstash Redis. Edit a template in templates/, then run:
//
//	UPSTASH_REDIS_URL='rediss://...' go run ./cmd/kios-import produk daftar-produk.xlsx
//	UPSTASH_REDIS_URL='rediss://...' go run ./cmd/kios-import supplier daftar-supplier.csv
//
// Rows are matched by name: existing entries are updated, new ones created.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/sipeed/picoclaw/pkg/tools/kios"
)

// readXLSXRows reads the first sheet of an .xlsx into header→value maps,
// mirroring the CSV reader. Headers (first row) are lowercased + trimmed.
func readXLSXRows(path string) ([]map[string]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("file excel kosong")
	}
	recs, err := f.GetRows(sheets[0])
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

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Pakai: kios-import <produk|supplier> <file.csv>")
		os.Exit(1)
	}
	typ, path := os.Args[1], os.Args[2]
	if os.Getenv("UPSTASH_REDIS_URL") == "" {
		fmt.Fprintln(os.Stderr, "FATAL: set UPSTASH_REDIS_URL (rediss://...)")
		os.Exit(1)
	}
	store, err := kios.NewStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}
	ctx := context.Background()
	if err := store.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: tidak bisa konek Redis: %v\n", err)
		os.Exit(1)
	}

	isXLSX := strings.HasSuffix(strings.ToLower(path), ".xlsx")
	var rows []map[string]string
	if isXLSX {
		if rows, err = readXLSXRows(path); err != nil {
			fmt.Fprintf(os.Stderr, "FATAL: gagal baca excel: %v\n", err)
			os.Exit(1)
		}
	}

	var res kios.ImportResult
	switch typ {
	case "produk":
		if isXLSX {
			res, err = kios.ImportProdukRows(ctx, store, rows)
		} else {
			res, err = kios.ImportProdukCSV(ctx, store, path)
		}
	case "supplier", "suplier":
		if isXLSX {
			res, err = kios.ImportSupplierRows(ctx, store, rows)
		} else {
			res, err = kios.ImportSupplierCSV(ctx, store, path)
		}
	default:
		fmt.Fprintln(os.Stderr, "FATAL: tipe harus 'produk' atau 'supplier'")
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: import gagal: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Selesai ✅  Dibuat: %d | Diupdate: %d | Dilewati: %d\n", res.Created, res.Updated, res.Skipped)
}
