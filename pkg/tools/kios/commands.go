package kios

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sipeed/picoclaw/pkg/commands"
	toolshared "github.com/sipeed/picoclaw/pkg/tools/shared"
)

// templateDir returns the directory holding the downloadable templates,
// from $KIOS_TEMPLATE_DIR (default "templates" for local dev; the Docker image
// sets it to /app/templates).
func templateDir() string {
	if d := strings.TrimSpace(os.Getenv("KIOS_TEMPLATE_DIR")); d != "" {
		return d
	}
	return "templates"
}

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
	supplier := NewSupplierTool(store)

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
		{
			Name:        "jualmassal",
			Description: "Jual beberapa barang sekaligus (tanpa AI)",
			Usage:       "/jualmassal <produk> <jml>, <produk> <jml>",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				items := parseMassalItems(argAfter(req.Text))
				if len(items) == 0 {
					return reply(req, "Pakai: /jualmassal <produk> <jml>, <produk> <jml>. Contoh: /jualmassal beras 2, gula 3")
				}
				res := kasir.Execute(withSender(ctx, req), map[string]any{"action": "jual_massal", "items": items})
				out := res.ForUser
				if out == "" {
					out = res.ForLLM
				}
				return reply(req, out)
			},
		},
		{
			Name:        "template",
			Description: "Kirim file template produk & supplier (Excel) untuk diisi",
			Handler: func(ctx context.Context, req commands.Request, rt *commands.Runtime) error {
				if rt == nil || rt.SendFile == nil {
					return reply(req, "Maaf kak, fitur kirim file belum tersedia di sini.")
				}
				dir := templateDir()
				files := []string{"produk-template.xlsx", "supplier-template.xlsx", "pustaka-template.xlsx"}
				sent := 0
				for _, name := range files {
					path := filepath.Join(dir, name)
					if _, err := os.Stat(path); err != nil {
						continue
					}
					if err := rt.SendFile(ctx, req.Channel, req.ChatID, path, name); err == nil {
						sent++
					}
				}
				if sent == 0 {
					return reply(req, "Maaf kak, file template tidak ditemukan di server 😔")
				}
				return reply(req, "📎 Template terkirim ya kak! Buka di Excel/Google Sheets, isi datanya, "+
					"lalu kirim balik file-nya ke chat ini — nanti aku import otomatis. 🙏")
			},
		},
		{
			Name:        "backup",
			Description: "Export semua data kios jadi file JSON (khusus owner)",
			Handler: func(ctx context.Context, req commands.Request, rt *commands.Runtime) error {
				ctx = withSender(ctx, req)
				role, _, refusal := resolveRole(ctx, store)
				if refusal != nil {
					return reply(req, refusal.ForLLM)
				}
				if r := requireOwner(role); r != nil {
					return reply(req, r.ForLLM)
				}
				if rt == nil || rt.SendFile == nil {
					return reply(req, "Maaf kak, fitur kirim file belum tersedia di sini.")
				}
				path, filename, ringkas, err := WriteBackupFile(ctx, store)
				if err != nil {
					return reply(req, "Aduh, gagal bikin backup kak 😣 Coba lagi sebentar ya.")
				}
				if err := rt.SendFile(ctx, req.Channel, req.ChatID, path, filename); err != nil {
					return reply(req, "Backup-nya jadi tapi gagal terkirim kak 😔 Coba lagi ya.")
				}
				return reply(req, "✅ Backup terkirim kak! Isi: "+ringkas+".\n"+
					"Simpan file JSON ini baik-baik ya — kalau data kios hilang, bisa dipulihkan dari sini. 🙏")
			},
		},
		{
			Name:        "produk",
			Description: "Lihat daftar produk / cari detail (tanpa AI)",
			Usage:       "/produk [nama]",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				if arg := argAfter(req.Text); arg != "" {
					return reply(req, stok.Execute(ctx, map[string]any{"action": "cari", "produk": arg}).ForLLM)
				}
				return reply(req, stok.Execute(ctx, map[string]any{"action": "cek"}).ForLLM)
			},
		},
		{
			Name:        "suplier",
			Description: "Lihat/cari supplier; banding harga antar supplier (tanpa AI)",
			Usage:       "/suplier [nama | banding <produk>]",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				ctx = withSender(ctx, req)
				arg := argAfter(req.Text)
				if arg == "" {
					return reply(req, supplier.Execute(ctx, map[string]any{"action": "daftar"}).ForLLM)
				}
				if rest, ok := strings.CutPrefix(strings.ToLower(arg), "banding "); ok {
					return reply(req, supplier.Execute(ctx, map[string]any{"action": "banding_harga", "produk": strings.TrimSpace(rest)}).ForLLM)
				}
				return reply(req, supplier.Execute(ctx, map[string]any{"action": "cari", "nama": arg}).ForLLM)
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

// parseMassalItems parses "produk jml, produk jml, ..." into [{produk, qty}].
// Each comma-separated entry's last token is the quantity.
func parseMassalItems(text string) []map[string]any {
	var items []map[string]any
	for _, part := range strings.Split(text, ",") {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) < 2 {
			continue
		}
		n, err := strconv.Atoi(fields[len(fields)-1])
		if err != nil || n <= 0 {
			continue
		}
		items = append(items, map[string]any{
			"produk": strings.Join(fields[:len(fields)-1], " "),
			"qty":    float64(n),
		})
	}
	return items
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
