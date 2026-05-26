package kios

import (
	"context"
	"strconv"
	"strings"

	"github.com/sipeed/picoclaw/pkg/commands"
	toolshared "github.com/sipeed/picoclaw/pkg/tools/shared"
)

// Commands returns deterministic Telegram slash-commands backed by the kios
// store. They call the tools' logic directly and reply without invoking the
// LLM, so core data stays accessible even when Groq/Gemini hit rate limits.
func Commands(store *Store) []commands.Definition {
	stok := NewStokTool(store)
	laporan := NewLaporanTool(store)
	harga := NewHargaTool(store)
	kasir := NewKasirTool(store)
	promo := NewPromoTool(store)
	pasar := NewPasarTool(store)

	reply := func(req commands.Request, text string) error {
		if strings.TrimSpace(text) == "" {
			text = "(tidak ada data)"
		}
		return req.Reply(text)
	}
	// withSender attaches the Telegram sender id so RBAC (kasir/owner) applies
	// to slash-commands the same way it does for LLM tool calls.
	withSender := func(ctx context.Context, req commands.Request) context.Context {
		return toolshared.WithToolContext(ctx, req.Channel, req.SenderID)
	}

	return []commands.Definition{
		{
			Name:        "stok",
			Description: "Lihat daftar stok / cari produk (tanpa AI)",
			Usage:       "/stok [nama produk]",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				if arg := argAfter(req.Text); arg != "" {
					return reply(req, stok.Execute(ctx, map[string]any{"action": "cari", "produk": arg}).ForLLM)
				}
				return reply(req, stok.Execute(ctx, map[string]any{"action": "cek"}).ForLLM)
			},
		},
		{
			Name:        "menipis",
			Description: "Produk yang stoknya menipis (tanpa AI)",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				return reply(req, stok.Execute(ctx, map[string]any{"action": "stok_menipis"}).ForLLM)
			},
		},
		{
			Name:        "laporan",
			Description: "Ringkasan penjualan & laba hari ini (tanpa AI)",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				return reply(req, laporan.Execute(ctx, map[string]any{"action": "ringkas"}).ForLLM)
			},
		},
		{
			Name:        "harga",
			Description: "Cek harga produk (tanpa AI)",
			Usage:       "/harga <nama produk>",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				arg := argAfter(req.Text)
				if arg == "" {
					return reply(req, "Pakai: /harga <nama produk>")
				}
				return reply(req, harga.Execute(ctx, map[string]any{"action": "cek", "produk": arg}).ForLLM)
			},
		},
		{
			Name:        "jual",
			Description: "Jual cepat + struk (tanpa AI)",
			Usage:       "/jual <produk> <jumlah>",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				produk, qty, ok := parseJualArgs(req.Text)
				if !ok {
					return reply(req, "Pakai: /jual <produk> <jumlah>. Contoh: /jual beras 2")
				}
				res := kasir.Execute(withSender(ctx, req), map[string]any{"action": "jual", "produk": produk, "qty": float64(qty)})
				out := res.ForUser
				if out == "" {
					out = res.ForLLM
				}
				return reply(req, out)
			},
		},
		{
			Name:        "shift",
			Description: "Status shift kasir berjalan (tanpa AI)",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				return reply(req, kasir.Execute(withSender(ctx, req), map[string]any{"action": "status_shift"}).ForLLM)
			},
		},
		{
			Name:        "promo",
			Description: "Daftar promo aktif (tanpa AI)",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				return reply(req, promo.Execute(ctx, map[string]any{"action": "daftar", "aktif_only": true}).ForLLM)
			},
		},
		{
			Name:        "pasar",
			Description: "Analisa harga kita vs pasar (tanpa AI)",
			Usage:       "/pasar [nama produk]",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				args := map[string]any{"action": "analisa"}
				if arg := argAfter(req.Text); arg != "" {
					args["produk"] = arg
				}
				return reply(req, pasar.Execute(ctx, args).ForLLM)
			},
		},
	}
}

// parseJualArgs parses "/jual <produk multi-kata> <jumlah>": the last token is
// the quantity (positive integer); everything before it is the product name.
func parseJualArgs(text string) (produk string, qty int, ok bool) {
	parts := strings.Fields(strings.TrimSpace(text))
	if len(parts) < 3 { // "/jual" + produk + qty
		return "", 0, false
	}
	last := parts[len(parts)-1]
	n, err := strconv.Atoi(last)
	if err != nil || n <= 0 {
		return "", 0, false
	}
	return strings.Join(parts[1:len(parts)-1], " "), n, true
}

// argAfter returns everything after the first whitespace-separated token
// (i.e. the command name), trimmed.
func argAfter(text string) string {
	parts := strings.Fields(strings.TrimSpace(text))
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts[1:], " ")
}
