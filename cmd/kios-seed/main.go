// Command kios-seed imports the legacy kios CSV data into Upstash Redis.
//
// Run it LOCALLY (so your secret URL never leaves your machine):
//
//	UPSTASH_REDIS_URL='rediss://...' \
//	KIOS_SEED_DIR=/home/kevinman/kios-openclaw/data \
//	go run ./cmd/kios-seed
//
// Set KIOS_SEED_FORCE=1 to re-run even if a previous seed was marked done.
// Note: legacy users.json (phone-keyed) is intentionally NOT imported.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/sipeed/picoclaw/pkg/tools/kios"
)

func main() {
	if os.Getenv("UPSTASH_REDIS_URL") == "" {
		fmt.Fprintln(os.Stderr, "FATAL: set UPSTASH_REDIS_URL (rediss://...)")
		os.Exit(1)
	}
	if os.Getenv("KIOS_SEED_DIR") == "" {
		fmt.Fprintln(
			os.Stderr,
			"FATAL: set KIOS_SEED_DIR to the folder with stok.csv, transaksi.csv, pembelian.csv, price-history.csv",
		)
		os.Exit(1)
	}

	ctx := context.Background()
	store, err := kios.NewStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}
	if err := store.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: cannot reach Redis: %v\n", err)
		os.Exit(1)
	}

	if os.Getenv("KIOS_SEED_FORCE") == "1" {
		if err := store.ResetSeed(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "warning: reset seed flag: %v\n", err)
		}
	}

	fmt.Println("Seeding Redis from", os.Getenv("KIOS_SEED_DIR"), "...")
	if err := kios.SeedFromOldData(ctx, store); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: seed failed: %v\n", err)
		os.Exit(1)
	}

	produk, _ := store.GetAllProduk(ctx)
	tx, _ := store.GetAllTransaksi(ctx)
	fmt.Printf("Selesai. Produk: %d, Transaksi: %d.\n", len(produk), len(tx))
	if len(produk) == 0 {
		fmt.Println("(Jika 0, mungkin seed sudah pernah jalan — pakai KIOS_SEED_FORCE=1 untuk ulang.)")
	}
}
