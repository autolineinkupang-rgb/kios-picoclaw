package kios

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	tools "github.com/sipeed/picoclaw/pkg/tools"
)

// MarketRef is a reference market price range for a product.
type MarketRef struct {
	Min     int    `json:"min"`
	Max     int    `json:"max"`
	Updated string `json:"updated"`
}

// SetMarketRef stores/extends the market reference range for a product key.
func (s *Store) SetMarketRef(ctx context.Context, key string, harga int) (*MarketRef, error) {
	key = strings.ToLower(strings.TrimSpace(key))
	ref := &MarketRef{Min: harga, Max: harga}
	if v, err := s.rdb.HGet(ctx, keyPasarRef, key).Result(); err == nil {
		_ = json.Unmarshal([]byte(v), ref)
	}
	if harga < ref.Min {
		ref.Min = harga
	}
	if harga > ref.Max {
		ref.Max = harga
	}
	ref.Updated = NowWITA().Format("2006-01-02")
	b, _ := json.Marshal(ref)
	return ref, s.rdb.HSet(ctx, keyPasarRef, key, string(b)).Err()
}

// GetAllMarketRef returns all market references keyed by product key.
func (s *Store) GetAllMarketRef(ctx context.Context) (map[string]*MarketRef, error) {
	m, err := s.rdb.HGetAll(ctx, keyPasarRef).Result()
	if err != nil {
		return nil, err
	}
	out := make(map[string]*MarketRef, len(m))
	for k, v := range m {
		var r MarketRef
		if json.Unmarshal([]byte(v), &r) == nil {
			out[k] = &r
		}
	}
	return out, nil
}

const sumberMonitoring = `📡 SUMBER MONITOR HARGA — Rote Ndao & NTT
⚡ Real-time (harian):
• Panel Badan Pangan — panelharga.badanpangan.go.id
• PIHPS Bank Indonesia — bi.go.id
• ANTARA Kupang — kupang.antaranews.com
📊 Bulanan:
• BPS NTT — ntt.bps.go.id
📰 Berita & Pemda:
• Operasi pasar/bazar — pemda NTT/Rote Ndao
Tips: minta aku "cari harga <produk> di internet" untuk riset langsung (web search), lalu simpan via /pasar.`

// PasarTool provides market price intelligence.
type PasarTool struct{ store *Store }

func (t *PasarTool) Name() string { return "kios_pasar" }

func (t *PasarTool) Description() string {
	return "Intelijen harga pasar: simpan harga referensi pasar sebuah produk (set_pasar), " +
		"analisa harga kita vs pasar (terlalu murah/mahal/kompetitif + saran), dan lihat sumber " +
		"pemantauan harga. Untuk riset harga online, gunakan web search lalu simpan hasilnya."
}

func (t *PasarTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"set_pasar", "analisa", "sumber"},
				"description": "Aksi pasar.",
			},
			"produk": map[string]any{"type": "string", "description": "nama produk"},
			"harga":  map[string]any{"type": "integer", "description": "harga pasar (set_pasar)"},
		},
		"required": []string{"action"},
	}
}

func (t *PasarTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	role, _, refusal := resolveRole(ctx, t.store)
	if refusal != nil {
		return refusal
	}
	switch argStr(args, "action") {
	case "sumber":
		return tools.NewToolResult(sumberMonitoring)
	case "set_pasar":
		if r := requireOwner(role); r != nil {
			return r
		}
		return t.setPasar(ctx, args)
	case "analisa":
		return t.analisa(ctx, args)
	default:
		return tools.ErrorResult("Hmm, aksi pasar belum dikenal kak 🤔")
	}
}

func (t *PasarTool) setPasar(ctx context.Context, args map[string]any) *tools.ToolResult {
	produk := argStr(args, "produk")
	harga := argInt(args, "harga")
	if produk == "" || harga <= 0 {
		return tools.ErrorResult("Produk dan harga pasar-nya diisi dulu ya kak 🙏")
	}
	ref, err := t.store.SetMarketRef(ctx, produk, harga)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal simpan harga pasar kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	return tools.NewToolResult(fmt.Sprintf("Harga pasar %s tersimpan: rentang %s–%s.",
		produk, FormatRupiah(ref.Min), FormatRupiah(ref.Max)))
}

// matchRef finds the market reference for a product name via substring match.
func matchRef(refs map[string]*MarketRef, nama string) *MarketRef {
	key := strings.ToLower(nama)
	if r, ok := refs[key]; ok {
		return r
	}
	first := strings.Fields(key)
	for k, v := range refs {
		if strings.Contains(key, k) || (len(first) > 0 && strings.Contains(k, first[0])) {
			return v
		}
	}
	return nil
}

func (t *PasarTool) analisa(ctx context.Context, args map[string]any) *tools.ToolResult {
	refs, err := t.store.GetAllMarketRef(ctx)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal baca harga pasar kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	if len(refs) == 0 {
		return tools.NewToolResult("Belum ada data harga pasar. Set dulu: action set_pasar.")
	}
	produk := argStr(args, "produk")
	var items []*Produk
	if produk != "" {
		if it, _ := findOne(ctx, t.store, produk); it != nil {
			items = []*Produk{it}
		}
	} else {
		items, _ = t.store.GetAllProduk(ctx)
	}
	if len(items) == 0 {
		return tools.NewToolResult("Produk nggak ketemu kak 🔍")
	}

	var b strings.Builder
	b.WriteString("📊 Analisa harga kita vs pasar:\n")
	count := 0
	for _, it := range items {
		ref := matchRef(refs, it.Nama)
		if ref == nil {
			continue
		}
		kita := it.HargaJual
		var status, saran string
		switch {
		case kita < int(float64(ref.Min)*0.95):
			status = "🔥 terlalu murah"
			saran = "naikkan ke " + FormatRupiah(int(float64(ref.Min)*1.02))
		case kita > int(float64(ref.Max)*1.05):
			status = "⚠️ terlalu mahal"
			saran = "turunkan ke " + FormatRupiah(ref.Max)
		default:
			status = "✅ kompetitif"
		}
		fmt.Fprintf(&b, "- %s: kita %s | pasar %s–%s → %s",
			it.Nama, FormatRupiah(kita), FormatRupiah(ref.Min), FormatRupiah(ref.Max), status)
		if saran != "" {
			b.WriteString(" (" + saran + ")")
		}
		b.WriteByte('\n')
		count++
	}
	if count == 0 {
		return tools.NewToolResult("Belum ada produk yang cocok dengan data harga pasar.")
	}
	return tools.NewToolResult(b.String())
}
