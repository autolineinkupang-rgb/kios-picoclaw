package kios

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/sipeed/picoclaw/pkg/commands"
	toolshared "github.com/sipeed/picoclaw/pkg/tools/shared"
)

// CommandsSpecial returns 0-token slash commands for pulsa and bensin.
func CommandsSpecial(store *Store) []commands.Definition {
	reply := func(req commands.Request, text string) error {
		if strings.TrimSpace(text) == "" {
			text = "(tidak ada data)"
		}
		return req.Reply(text)
	}

	withSender := func(ctx context.Context, req commands.Request) context.Context {
		return toolshared.WithToolContext(ctx, req.Channel, req.SenderID)
	}

	resolveRoleForCmd := func(ctx context.Context) (role, name string, err error) {
		r, n, refusal := resolveRole(ctx, store)
		if refusal != nil {
			return "", "", fmt.Errorf("%s", refusal.ForLLM)
		}
		return r, n, nil
	}

	return []commands.Definition{
		{
			Name:        "pulsa",
			Description: "Cek saldo modal & daftar nominal pulsa, atau jual pulsa (0-token)",
			Usage:       "/pulsa [nominal]",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				ctx = withSender(ctx, req)
				arg := argAfter(req.Text)

				if arg == "" {
					return reply(req, pulsaInfo(ctx, store))
				}

				// Jual pulsa: /pulsa <nominal>
				nominal := parseRupiah(arg)
				if nominal <= 0 {
					return reply(req, fmt.Sprintf("Nominal %q tidak valid kak. Contoh: /pulsa 10000", arg))
				}

				_, kasirName, err := resolveRoleForCmd(ctx)
				if err != nil {
					return reply(req, err.Error())
				}

				anchor, err := findPulsaAnchor(ctx, store)
				if err != nil {
					return reply(req, fmt.Sprintf("Gagal ambil data pulsa: %v", err))
				}
				if anchor == nil {
					return reply(req, "Produk pulsa belum dikonfigurasi kak. Minta owner tambahkan produk dengan jenis=pulsa ya.")
				}

				tx, _, sisa, err := sellPulsa(ctx, store, anchor, nominal, "tunai", kasirName)
				if err != nil {
					return reply(req, err.Error())
				}
				return reply(req, fmt.Sprintf(
					"Pulsa %s berhasil dijual kak!\nTotal: %s\nSisa saldo modal: %s\nID: %s",
					FormatRupiah(nominal), FormatRupiah(tx.Total), FormatRupiah(sisa), tx.ID,
				))
			},
		},
		{
			Name:        "isipulsa",
			Description: "Top-up saldo modal pulsa (khusus owner)",
			Usage:       "/isipulsa <jumlah>",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				ctx = withSender(ctx, req)
				role, kasirName, err := resolveRoleForCmd(ctx)
				if err != nil {
					return reply(req, err.Error())
				}
				if r := requireOwner(role); r != nil {
					return reply(req, r.ForLLM)
				}

				arg := argAfter(req.Text)
				if arg == "" {
					return reply(req, "Format: /isipulsa <jumlah>\nContoh: /isipulsa 100000")
				}
				jumlah := parseRupiah(arg)
				if jumlah <= 0 {
					return reply(req, fmt.Sprintf("Jumlah %q tidak valid kak.", arg))
				}

				anchor, err := findPulsaAnchor(ctx, store)
				if err != nil {
					return reply(req, fmt.Sprintf("Gagal ambil data pulsa: %v", err))
				}
				if anchor == nil {
					return reply(req, "Produk pulsa belum dikonfigurasi kak. Tambahkan produk dengan jenis=pulsa dulu ya.")
				}

				anchor.SaldoModal += jumlah
				if err := store.SetProduk(ctx, anchor); err != nil {
					return reply(req, fmt.Sprintf("Gagal update saldo modal: %v", err))
				}

				now := NowWITA()
				pt := &PulsaTopup{
					Tanggal:      now.Format("2006-01-02"),
					Jam:          now.Format("15:04:05"),
					Jumlah:       jumlah,
					SaldoSesudah: anchor.SaldoModal,
					Kasir:        kasirName,
				}
				if err := store.AppendPulsaTopup(ctx, pt); err != nil {
					return reply(req, fmt.Sprintf("Saldo diisi tapi gagal catat log: %v", err))
				}
				return reply(req, fmt.Sprintf(
					"Saldo modal pulsa berhasil diisi %s kak!\nSaldo sekarang: %s\nID: %s",
					FormatRupiah(jumlah), FormatRupiah(anchor.SaldoModal), pt.ID,
				))
			},
		},
		{
			Name:        "bensin",
			Description: "Cek stok bensin atau jual bensin (0-token)",
			Usage:       "/bensin [nama liter]",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				ctx = withSender(ctx, req)
				arg := argAfter(req.Text)

				if arg == "" {
					return reply(req, bensinInfo(ctx, store))
				}

				// Jual bensin: /bensin <produk> <liter>
				parts := strings.Fields(arg)
				if len(parts) < 2 {
					return reply(req, "Format: /bensin <nama produk> <liter>\nContoh: /bensin pertalite 2")
				}
				literStr := parts[len(parts)-1]
				namaProduk := strings.Join(parts[:len(parts)-1], " ")

				literArg, err := strconv.ParseFloat(literStr, 64)
				if err != nil || literArg <= 0 {
					return reply(req, fmt.Sprintf("Volume %q tidak valid kak. Contoh: /bensin pertalite 2", literStr))
				}

				_, kasirName, err := resolveRoleForCmd(ctx)
				if err != nil {
					return reply(req, err.Error())
				}

				item, err := findOne(ctx, store, namaProduk)
				if err != nil {
					return reply(req, fmt.Sprintf("Gagal cari produk: %v", err))
				}
				if item == nil {
					return reply(req, fmt.Sprintf("Produk %q tidak ditemukan kak.", namaProduk))
				}
				if item.JenisOrDefault() != "bensin" {
					return reply(req, fmt.Sprintf("Produk %q bukan bensin kak.", item.Nama))
				}

				ml := int(math.Round(literArg * 1000))
				tx, _, sisaMl, err := sellBensin(ctx, store, item, ml, "tunai", kasirName)
				if err != nil {
					return reply(req, err.Error())
				}
				sisaLiter := float64(sisaMl) / 1000
				return reply(req, fmt.Sprintf(
					"Bensin %s %.2fL berhasil dijual kak!\nTotal: %s\nSisa stok: %.1fL\nID: %s",
					item.Nama, literArg, FormatRupiah(tx.Total), sisaLiter, tx.ID,
				))
			},
		},
		{
			Name:        "isibensin",
			Description: "Restock bensin (khusus owner)",
			Usage:       "/isibensin <nama> <liter> <harga_total>",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				ctx = withSender(ctx, req)
				role, _, err := resolveRoleForCmd(ctx)
				if err != nil {
					return reply(req, err.Error())
				}
				if r := requireOwner(role); r != nil {
					return reply(req, r.ForLLM)
				}

				parts := strings.Fields(argAfter(req.Text))
				if len(parts) < 3 {
					return reply(req, "Format: /isibensin <nama> <liter> <harga_total>\nContoh: /isibensin pertalite 40 440000")
				}
				hargaStr := parts[len(parts)-1]
				literStr := parts[len(parts)-2]
				namaProduk := strings.Join(parts[:len(parts)-2], " ")

				literArg, err := strconv.ParseFloat(literStr, 64)
				if err != nil || literArg <= 0 {
					return reply(req, fmt.Sprintf("Volume %q tidak valid kak.", literStr))
				}
				hargaTotal := parseRupiah(hargaStr)
				if hargaTotal <= 0 {
					return reply(req, fmt.Sprintf("Harga %q tidak valid kak.", hargaStr))
				}

				item, err := findOne(ctx, store, namaProduk)
				if err != nil {
					return reply(req, fmt.Sprintf("Gagal cari produk: %v", err))
				}
				if item == nil {
					return reply(req, fmt.Sprintf("Produk %q tidak ditemukan kak.", namaProduk))
				}
				if item.JenisOrDefault() != "bensin" {
					return reply(req, fmt.Sprintf("Produk %q bukan bensin kak.", item.Nama))
				}

				mlTambah := int(math.Round(literArg * 1000))
				hargaPerLiter := int(math.Round(float64(hargaTotal) / literArg))
				now := NowWITA()

				item.StokMl += mlTambah
				item.Stok = item.StokMl / 1000
				item.HargaBeli = hargaPerLiter
				item.LastUpdate = now.Format("2006-01-02")

				if err := store.SetProduk(ctx, item); err != nil {
					return reply(req, fmt.Sprintf("Gagal update stok bensin: %v", err))
				}
				sisaLiter := float64(item.StokMl) / 1000
				return reply(req, fmt.Sprintf(
					"Restock bensin %s berhasil kak!\nTambah: %.1fL\nHarga beli: %s/L\nStok sekarang: %.1fL",
					item.Nama, literArg, FormatRupiah(hargaPerLiter), sisaLiter,
				))
			},
		},
	}
}

// findPulsaAnchor returns the first product with jenis == "pulsa", or nil.
func findPulsaAnchor(ctx context.Context, store *Store) (*Produk, error) {
	all, err := store.GetAllProduk(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range all {
		if p.JenisOrDefault() == "pulsa" {
			return p, nil
		}
	}
	return nil, nil
}

// pulsaInfo renders a summary of pulsa saldo + denom table.
func pulsaInfo(ctx context.Context, store *Store) string {
	anchor, err := findPulsaAnchor(ctx, store)
	if err != nil {
		return fmt.Sprintf("Gagal ambil data pulsa: %v", err)
	}
	if anchor == nil {
		return "Produk pulsa belum dikonfigurasi kak. Minta owner tambahkan produk dengan jenis=pulsa ya."
	}

	denoms, err := store.GetAllPulsaDenom(ctx)
	if err != nil {
		return fmt.Sprintf("Gagal ambil daftar nominal: %v", err)
	}
	sort.Slice(denoms, func(i, j int) bool { return denoms[i].Nominal < denoms[j].Nominal })

	var b strings.Builder
	fmt.Fprintf(&b, "Pulsa — Saldo Modal: %s\n\n", FormatRupiah(anchor.SaldoModal))

	if len(denoms) == 0 {
		b.WriteString("Belum ada nominal yang dikonfigurasi.\nGunakan kios_special action=set_denom untuk menambahkan.")
		return b.String()
	}

	b.WriteString("Nominal     | Modal   | Jual    | Margin\n")
	b.WriteString("------------|---------|---------|-------\n")
	for _, d := range denoms {
		status := ""
		if !d.Aktif {
			status = " (nonaktif)"
		}
		fmt.Fprintf(&b, "%-11s | %-7s | %-7s | %s%s\n",
			FormatRupiah(d.Nominal),
			FormatRupiah(d.HargaModal),
			FormatRupiah(d.HargaJual),
			FormatRupiah(d.Margin()),
			status,
		)
	}
	b.WriteString("\nJual: /pulsa <nominal> — contoh: /pulsa 10000")
	return b.String()
}

// bensinInfo renders a summary of all bensin products.
func bensinInfo(ctx context.Context, store *Store) string {
	all, err := store.GetAllProduk(ctx)
	if err != nil {
		return fmt.Sprintf("Gagal ambil data bensin: %v", err)
	}

	var items []*Produk
	for _, p := range all {
		if p.JenisOrDefault() == "bensin" {
			items = append(items, p)
		}
	}

	if len(items) == 0 {
		return "Belum ada produk bensin yang dikonfigurasi kak."
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Nama < items[j].Nama })

	var b strings.Builder
	b.WriteString("Stok Bensin:\n\n")
	for _, p := range items {
		sisaLiter := float64(p.StokMl) / 1000
		kritisMl := p.StokKritisMl
		status := ""
		if kritisMl > 0 && p.StokMl <= kritisMl {
			status = " ⚠️ KRITIS"
		}
		fmt.Fprintf(&b, "%s\n  Stok: %.1fL | Kritis: %.1fL | Jual: %s/L%s\n",
			p.Nama,
			sisaLiter,
			float64(kritisMl)/1000,
			FormatRupiah(p.HargaJual),
			status,
		)
	}
	b.WriteString("\nJual: /bensin <nama> <liter> — contoh: /bensin pertalite 2")
	return b.String()
}
