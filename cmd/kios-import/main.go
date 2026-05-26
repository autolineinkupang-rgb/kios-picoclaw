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

	"github.com/sipeed/picoclaw/pkg/tools/kios"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Pakai: kios-import <produk|supplier> <file.xlsx|file.csv>")
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

	rows, err := kios.ReadTableFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: gagal baca file: %v\n", err)
		os.Exit(1)
	}

	var res kios.ImportResult
	switch typ {
	case "produk":
		res, err = kios.ImportProdukRows(ctx, store, rows)
	case "supplier", "suplier":
		res, err = kios.ImportSupplierRows(ctx, store, rows)
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
