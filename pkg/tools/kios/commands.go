package kios

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/commands"
	toolshared "github.com/sipeed/picoclaw/pkg/tools/shared"
)

const panduanText = `📖 Panduan Kios Cerdas

PERINTAH CEPAT (tanpa AI):
/stok [nama] — cek stok semua / cari produk
/harga <produk> — lihat harga jual & modal
/jual <produk> <jml> — catat penjualan + struk
/jualmassal <produk> <jml>, ... — jual banyak barang
/laporan — ringkasan penjualan & laba hari ini
/menipis — produk yang stoknya hampir habis
/shift — buka/cek/tutup shift kasir
/promo — daftar promo & diskon aktif
/pasar [produk] — bandingkan harga kita vs pasar
/produk [nama] — daftar / detail produk
/suplier [nama | banding <produk>] — info supplier
/qris — tampilkan QRIS kios untuk pembayaran

PERINTAH OWNER:
/backup — export data ke JSON (owner)
/template — download template Excel untuk import data
/model [nama|reset] — lihat/ganti model AI utama bot

TANYA AI (ketik bebas):
• "stok beras berapa?"
• "jual sabun 3 biji, bayar 15 ribu"
• "laporan minggu ini"
• "tambah produk baru gula 1 kg harga 14.000"
• "restock minyak 24 botol harga beli 12.000"
• "promo diskon 10% untuk susu bulan ini"

TIPS:
• Sebut nominal bayar saat jual agar dapat kembalian
• Ketik /stok untuk cek stok terkini tanpa menunggu AI
• Transaksi ada kode TRX-xxxx — simpan jika ingin dibatalkan
• Owner bisa ketik "aktifkan belajar otomatis" atau "nonaktifkan belajar"

Butuh bantuan lain? Ketik saja pertanyaanmu kak! 🙏`

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
	return CommandsWithNotif(store, nil)
}

// CommandsWithNotif sama dengan Commands, dengan dukungan perintah /notif
// jika notifSvc tidak nil.
func CommandsWithNotif(store *Store, notifSvc *NotifService) []commands.Definition {
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
		{
			Name:        "model",
			Description: "Lihat atau ganti model AI utama bot (khusus owner)",
			Usage:       "/model [nama-model | reset]",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				ctx = withSender(ctx, req)
				role, _, refusal := resolveRole(ctx, store)
				if refusal != nil {
					return reply(req, refusal.ForLLM)
				}
				if r := requireOwner(role); r != nil {
					return reply(req, r.ForLLM)
				}
				cfg := store.GetConfig(ctx)
				arg := argAfter(req.Text)
				if arg == "" {
					current := cfg.ModelUtama
					if current == "" {
						current = "routing default (dari config.json)"
					}
					return reply(req, fmt.Sprintf(
						"🤖 *Model AI Utama Kios*\n\nSaat ini: *%s*\n\n%s\n\n"+
							"Ketik `/model <nomor>` atau `/model <nama-model>` untuk mengganti.\n"+
							"Ketik `/model reset` untuk kembali ke routing default.",
						current, availableModelsText(),
					))
				}
				// Nomor pilihan dari daftar model
				if n, err2 := strconv.Atoi(strings.TrimSpace(arg)); err2 == nil {
					if name, ok := modelByNumber(n); ok {
						arg = name
					}
				}
				if strings.ToLower(arg) == "reset" || strings.ToLower(arg) == "default" {
					cfg.ModelUtama = ""
					if err := store.SaveConfig(ctx, cfg); err != nil {
						return reply(req, "Aduh, gagal simpan pengaturan kak 😣 Coba lagi sebentar ya.")
					}
					return reply(req, "✅ Model AI dikembalikan ke *routing default* (dari config.json).")
				}
				cfg.ModelUtama = strings.TrimSpace(arg)
				if err := store.SaveConfig(ctx, cfg); err != nil {
					return reply(req, "Aduh, gagal simpan pengaturan kak 😣 Coba lagi sebentar ya.")
				}
				return reply(req, fmt.Sprintf(
					"✅ Model AI utama diubah ke *%s*\n\nPerubahan berlaku untuk semua respons bot mulai sekarang 🙏",
					cfg.ModelUtama,
				))
			},
		},
		{
			Name:        "bantuan",
			Description: "Panduan lengkap cara pakai Kios Cerdas",
			Handler: func(_ context.Context, req commands.Request, _ *commands.Runtime) error {
				return reply(req, panduanText)
			},
		},
		{
			Name:        "login",
			Description: "Dapatkan kode untuk masuk ke Dashboard web",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				ctx = withSender(ctx, req)
				_, nama, refusal := resolveRole(ctx, store)
				if refusal != nil {
					return reply(req, refusal.ForLLM)
				}
				code, err := store.CreateLoginCode(ctx, req.SenderID, nama)
				if err != nil {
					return reply(req, "Aduh, gagal bikin kode masuk kak 😣 Coba lagi sebentar ya.")
				}
				return reply(req, loginMessage(code))
			},
		},
		{
			Name:        "qris",
			Description: "Tampilkan QRIS kios untuk pembayaran (tanpa AI)",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				return reply(req, qrisMessage(store.GetConfig(ctx)))
			},
		},
		{
			Name:        "web",
			Description: "Cek status Dashboard web kios (khusus owner)",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				ctx = withSender(ctx, req)
				role, _, refusal := resolveRole(ctx, store)
				if refusal != nil {
					return reply(req, refusal.ForLLM)
				}
				if r := requireOwner(role); r != nil {
					return reply(req, r.ForLLM)
				}
				return reply(req, checkWebsiteStatus(ctx))
			},
		},
		{
			Name:        "notif",
			Description: "Cek & kirim notif stok menipis sekarang (khusus owner)",
			Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
				ctx = withSender(ctx, req)
				role, _, refusal := resolveRole(ctx, store)
				if refusal != nil {
					return reply(req, refusal.ForLLM)
				}
				if r := requireOwner(role); r != nil {
					return reply(req, r.ForLLM)
				}
				if notifSvc == nil {
					return reply(req, "Layanan notifikasi belum aktif kak 😔")
				}
				return reply(req, notifSvc.CheckNow(ctx))
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

// qrisMessage builds the /qris reply from the kios config: the merchant name
// and a link to the static QRIS image to scan. Telegram previews the image URL.
func qrisMessage(cfg KiosConfig) string {
	if !cfg.QrisEnabled {
		return "Pembayaran QRIS belum diaktifkan kak. Owner bisa mengaktifkannya lewat Pengaturan di Dashboard, " +
			"atau ketik: aktifkan QRIS, nama merchant ..., gambar QR ... 🙏"
	}
	nama := cfg.QrisNama
	if nama == "" {
		nama = "Kios Cerdas"
	}
	if cfg.QrisImageURL == "" {
		return fmt.Sprintf("QRIS %s aktif, tapi gambar QR-nya belum di-set kak. "+
			"Owner bisa menambahkan URL gambar QRIS lewat Pengaturan di Dashboard ya. 🙏", nama)
	}
	return fmt.Sprintf("💳 Pembayaran QRIS — %s\n\nScan QR ini untuk bayar:\n%s\n\n"+
		"Setelah bayar, tunjukkan bukti ke kasir ya kak. 🙏", nama, cfg.QrisImageURL)
}

// loginMessage builds the /login reply: the one-time code plus, when
// $KIOS_DASHBOARD_URL is set, a tap-to-login link with the code prefilled.
func loginMessage(code string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🔐 Kode masuk Dashboard kamu:\n\n*%s*\n\n", code)
	b.WriteString("Masukkan kode ini di halaman login dashboard. Berlaku 5 menit & sekali pakai ya kak.")
	if url := strings.TrimSpace(os.Getenv("KIOS_DASHBOARD_URL")); url != "" {
		link := strings.TrimRight(url, "/") + "/login?code=" + code
		fmt.Fprintf(&b, "\n\nAtau buka langsung dari HP: %s", link)
	}
	return b.String()
}

// checkWebsiteStatus pings the dashboard health endpoint and reports status,
// latency, Redis connectivity, and pending-order count.
func checkWebsiteStatus(ctx context.Context) string {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("KIOS_DASHBOARD_URL")), "/")
	if base == "" {
		return "URL dashboard belum di-set kak (env KIOS_DASHBOARD_URL). Isi dulu ya."
	}
	client := &http.Client{Timeout: 8 * time.Second}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/health", nil)
	if err != nil {
		return "Aduh, gagal menyusun permintaan ke dashboard kak 😣"
	}
	start := time.Now()
	resp, err := client.Do(httpReq)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return fmt.Sprintf("❌ Dashboard tidak bisa dihubungi kak.\n%s\nError: %v", base, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("⚠️ Dashboard membalas status %d (%dms).\n%s", resp.StatusCode, latency, base)
	}
	var h struct {
		OK             bool `json:"ok"`
		Redis          bool `json:"redis"`
		PesananPending int  `json:"pesanan_pending"`
	}
	_ = json.Unmarshal(body, &h)
	// Best-effort: pull the richer authenticated summary. When the service
	// secret isn't set (or verification fails) we fall back to the plain health
	// ping so /web keeps working without the secure channel configured.
	if summary, err := fetchDashboardSummary(ctx); err == nil {
		return formatDashboardSummary(summary, base, latency)
	}
	redisStr := "ok"
	if !h.Redis {
		redisStr = "GAGAL"
	}
	return fmt.Sprintf("✅ Dashboard online (%dms)\n%s\nRedis: %s\nPesanan menunggu: %d",
		latency, base, redisStr, h.PesananPending)
}

// modelEntry mewakili satu model yang bisa dipilih owner.
type modelEntry struct {
	Name  string // model identifier (dipakai untuk set ModelUtama)
	Label string // deskripsi singkat untuk owner
}

// availableModels mengembalikan daftar model yang tersedia berdasarkan
// API key yang di-set di environment variable Railway.
func availableModels() []modelEntry {
	var list []modelEntry

	if os.Getenv("GROQ_API_KEY") != "" {
		list = append(list,
			modelEntry{"groq/llama-3.3-70b-versatile", "Groq — Llama 3.3 70B (cepat & pintar) ⭐"},
			modelEntry{"groq/llama3-8b-8192", "Groq — Llama 3 8B (sangat cepat)"},
			modelEntry{"groq/mixtral-8x7b-32768", "Groq — Mixtral 8x7B (konteks panjang)"},
		)
	}
	if os.Getenv("GEMINI_API_KEY") != "" {
		list = append(list,
			modelEntry{"gemini/gemini-2.0-flash-lite", "Gemini — 2.0 Flash Lite (tercepat)"},
			modelEntry{"gemini/gemini-2.0-flash", "Gemini — 2.0 Flash (seimbang)"},
			modelEntry{"gemini/gemini-1.5-pro", "Gemini — 1.5 Pro (paling canggih)"},
		)
	}
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		list = append(list,
			modelEntry{"anthropic/claude-haiku-4-5-20251001", "Anthropic — Claude Haiku (cepat & hemat)"},
			modelEntry{"anthropic/claude-sonnet-4-6", "Anthropic — Claude Sonnet (pintar & seimbang)"},
		)
	}
	return list
}

// availableModelsText membangun teks daftar bernomor model yang tersedia.
func availableModelsText() string {
	models := availableModels()
	if len(models) == 0 {
		return "_(Tidak ada API key provider yang terdeteksi — set GROQ_API_KEY, GEMINI_API_KEY, atau ANTHROPIC_API_KEY di Railway.)_"
	}
	var b strings.Builder
	b.WriteString("*Pilihan model tersedia:*\n")
	for i, m := range models {
		fmt.Fprintf(&b, "%d. `%s`\n   _%s_\n", i+1, m.Name, m.Label)
	}
	return b.String()
}

// modelByNumber mengembalikan nama model berdasarkan nomor pilihan (1-based).
func modelByNumber(n int) (string, bool) {
	models := availableModels()
	if n < 1 || n > len(models) {
		return "", false
	}
	return models[n-1].Name, true
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
