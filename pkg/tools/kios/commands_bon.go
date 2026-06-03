package kios

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/sipeed/picoclaw/pkg/commands"
)

// CommandsBon returns 0-token slash commands for bon/hutang ledger.
func CommandsBon(store *Store) []commands.Definition {
	bon := NewBonTool(store)

	replyText := func(req commands.Request, text string) error {
		if strings.TrimSpace(text) == "" {
			text = "(tidak ada data)"
		}
		return req.Reply(text)
	}

	return []commands.Definition{
		{
			Name:        "utang",
			Description: "Daftar piutang pembeli terbuka (0-token)",
			Usage:       "/utang [nama pelanggan]",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				filter := strings.TrimSpace(strings.TrimPrefix(req.Text, "/utang"))
				args := map[string]any{"action": "daftar_piutang"}
				if filter != "" {
					args["filter"] = filter
				}
				return replyText(req, bon.Execute(ctx, args).ForLLM)
			},
		},
		{
			Name:        "hutang",
			Description: "Daftar hutang kios ke supplier (0-token)",
			Usage:       "/hutang [nama supplier]",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				filter := strings.TrimSpace(strings.TrimPrefix(req.Text, "/hutang"))
				args := map[string]any{"action": "daftar_hutang"}
				if filter != "" {
					args["filter"] = filter
				}
				return replyText(req, bon.Execute(ctx, args).ForLLM)
			},
		},
		{
			Name:        "bayar",
			Description: "Catat pembayaran piutang atau hutang (0-token)",
			Usage:       "/bayar <PIU-xxxx|HUT-xxxx> <jumlah>",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				parts := strings.Fields(req.Text)
				if len(parts) < 3 {
					return replyText(req, "Format: /bayar <PIU-xxxx|HUT-xxxx> <jumlah>\nContoh: /bayar PIU-0001 15000")
				}
				id := strings.ToUpper(parts[1])
				jumlah, err := strconv.Atoi(parts[2])
				if err != nil || jumlah <= 0 {
					return replyText(
						req,
						fmt.Sprintf("Jumlah %q tidak valid kak. Contoh: /bayar %s 15000", parts[2], id),
					)
				}
				return replyText(req, bon.Execute(ctx, map[string]any{
					"action": "bayar", "id": id,
					"jumlah": float64(jumlah), "metode": "tunai",
				}).ForLLM)
			},
		},
		{
			Name:        "jualutang",
			Description: "Catat penjualan kredit/bon ke pelanggan (0-token)",
			Usage:       "/jualutang <produk> <qty> <nomor-WA>",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				parts := strings.Fields(req.Text)
				if len(parts) < 4 {
					return replyText(
						req,
						"Format: /jualutang <produk> <qty> <nomor-WA>\nContoh: /jualutang mie 2 08123456789",
					)
				}
				phone := parts[len(parts)-1]
				qtyStr := parts[len(parts)-2]
				produk := strings.Join(parts[1:len(parts)-2], " ")
				qty, err := strconv.Atoi(qtyStr)
				if err != nil || qty <= 0 {
					return replyText(req, fmt.Sprintf("Qty %q tidak valid kak.", qtyStr))
				}
				return replyText(req, bon.Execute(ctx, map[string]any{
					"action": "jual_bon", "produk": produk,
					"qty": float64(qty), "pelanggan": phone,
				}).ForLLM)
			},
		},
	}
}
