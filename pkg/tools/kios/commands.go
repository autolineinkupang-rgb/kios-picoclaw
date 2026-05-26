package kios

import (
	"context"
	"strings"

	"github.com/sipeed/picoclaw/pkg/commands"
)

// Commands returns deterministic Telegram slash-commands backed by the kios
// store. They call the tools' logic directly and reply without invoking the
// LLM, so core data stays accessible even when Groq/Gemini hit rate limits.
func Commands(store *Store) []commands.Definition {
	stok := NewStokTool(store)
	laporan := NewLaporanTool(store)
	harga := NewHargaTool(store)

	reply := func(req commands.Request, text string) error {
		if strings.TrimSpace(text) == "" {
			text = "(tidak ada data)"
		}
		return req.Reply(text)
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
	}
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
