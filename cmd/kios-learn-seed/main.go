// Command kios-learn-seed seeds alias, shortcut, dan pattern belajar ke Redis.
//
// Jalankan sekali saat setup awal atau setelah reset data:
//
//	UPSTASH_REDIS_URL='rediss://...' go run ./cmd/kios-learn-seed
//
// Aman dijalankan berulang (idempotent — tidak hapus data yang sudah ada).
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

	ctx := context.Background()
	store, err := kios.NewStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}
	if err := store.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: tidak bisa konek Redis: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Menyimpan alias produk umum...")
	aliases := map[string]string{
		"beras":      "beras medium 5kg",
		"minyak":     "minyak goreng 1L",
		"gula":       "gula pasir 1kg",
		"sabun":      "sabun mandi",
		"susu":       "susu kental manis",
		"rokok":      "rokok surya 12",
		"aqua":       "air mineral 600ml",
		"indomie":    "mie instan goreng",
		"kopi":       "kopi sachet",
		"detergen":   "detergen bubuk 1kg",
	}
	for alias, target := range aliases {
		if err := store.SaveAlias(ctx, alias, target); err != nil {
			fmt.Fprintf(os.Stderr, "  gagal alias '%s': %v\n", alias, err)
		} else {
			fmt.Printf("  alias: %s → %s\n", alias, target)
		}
	}

	fmt.Println("Menyimpan shortcut paket belanja...")
	shortcuts := map[string][]string{
		"paket dapur":   {"beras medium 5kg", "minyak goreng 1L", "gula pasir 1kg"},
		"paket mandi":   {"sabun mandi", "shampo sachet", "pasta gigi"},
		"paket minuman": {"air mineral 600ml", "kopi sachet", "teh celup"},
		"paket bumbu":   {"garam 250g", "merica bubuk", "penyedap rasa"},
	}
	for nama, items := range shortcuts {
		if err := store.SaveShortcut(ctx, nama, items); err != nil {
			fmt.Fprintf(os.Stderr, "  gagal shortcut '%s': %v\n", nama, err)
		} else {
			fmt.Printf("  shortcut: %s = %v\n", nama, items)
		}
	}

	fmt.Println("Menyimpan pattern perintah umum...")
	patterns := []struct{ input, intent, target string }{
		{"cek stok", "cek", "stok"},
		{"lihat stok", "cek", "stok"},
		{"stok habis", "stok_menipis", ""},
		{"mau beli", "jual", ""},
		{"laporan hari ini", "ringkas", "laporan"},
		{"laporan harian", "ringkas", "laporan"},
		{"laba hari ini", "laba", "laporan"},
		{"harga beras", "cek", "harga"},
		{"berapa harga", "cek", "harga"},
		{"tambah stok", "tambah", "restock"},
		{"restock", "tambah", "restock"},
		{"backup data", "backup", ""},
		{"export data", "backup", ""},
	}
	for _, p := range patterns {
		if err := store.SavePattern(ctx, p.input, p.intent, p.target); err != nil {
			fmt.Fprintf(os.Stderr, "  gagal pattern '%s': %v\n", p.input, err)
		} else {
			fmt.Printf("  pattern: '%s' → intent=%s\n", p.input, p.intent)
		}
	}

	fmt.Println("\nSelesai. Data belajar berhasil disimpan ke Redis.")
	fmt.Println("Cek via: kios_belajar action=alias_get alias=beras")
}
