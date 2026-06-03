package kios

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	tools "github.com/sipeed/picoclaw/pkg/tools"
	toolshared "github.com/sipeed/picoclaw/pkg/tools/shared"
)

// reDigits strips everything except digits and a leading minus, so that
// "Rp 15.000", "1.500.000" and "15,000" all parse to a plain integer.
var reDigits = regexp.MustCompile(`[^0-9-]`)

// parseRupiah parses a number from any rupiah-ish string format.
func parseRupiah(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	s = reDigits.ReplaceAllString(s, "")
	if s == "" || s == "-" {
		return 0
	}
	n, _ := strconv.Atoi(s)
	return n
}

// argStr reads a trimmed string argument.
func argStr(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// argInt reads an integer argument, tolerating JSON floats and rupiah strings.
func argInt(args map[string]any, key string) int {
	switch x := args[key].(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	case string:
		return parseRupiah(x)
	}
	return 0
}

// argIntPtr returns nil when the key is absent (to distinguish "0" from "unset").
func argIntPtr(args map[string]any, key string) *int {
	if _, ok := args[key]; !ok {
		return nil
	}
	v := argInt(args, key)
	return &v
}

// argBool reads a boolean argument.
func argBool(args map[string]any, key string) bool {
	switch x := args[key].(type) {
	case bool:
		return x
	case string:
		return x == "true" || x == "1" || x == "ya"
	}
	return false
}

// argItems reads an "items" argument as a list of objects (for bulk actions).
func argItems(args map[string]any) []map[string]any {
	// items may arrive as []map[string]any (slash command parser) or as
	// []any of maps (LLM tool call decoded from JSON). Accept both.
	switch raw := args["items"].(type) {
	case []map[string]any:
		return raw
	case []any:
		out := make([]map[string]any, 0, len(raw))
		for _, r := range raw {
			if m, ok := r.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
}

// defaultRole is used for whitelisted callers who are not registered in
// kios:users. Defaults to "owner" so a fresh deploy is fully usable; set
// KIOS_DEFAULT_ROLE=kasir to lock down destructive actions by default. To do
// that safely, also list the owner's Telegram ID in KIOS_OWNER_IDS so they
// stay owner and can't be locked out (see isBootstrapOwner).
func defaultRole() string {
	if r := strings.TrimSpace(os.Getenv("KIOS_DEFAULT_ROLE")); r != "" {
		return r
	}
	return "owner"
}

// isBootstrapOwner reports whether id is in KIOS_OWNER_IDS (comma-separated
// Telegram IDs). These callers are always owner, regardless of KIOS_DEFAULT_ROLE
// or their kios:users record — this prevents an owner lockout when the default
// role is tightened to kasir on a fresh deploy.
func isBootstrapOwner(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	for _, part := range strings.Split(os.Getenv("KIOS_OWNER_IDS"), ",") {
		if strings.TrimSpace(part) == id {
			return true
		}
	}
	return false
}

// resolveRole determines the caller's role from the tool context + kios:users.
// Returns a refusal result (non-nil) when the caller is a known-but-inactive user.
func resolveRole(ctx context.Context, store *Store) (role string, kasir string, refusal *tools.ToolResult) {
	id := toolshared.ToolChatID(ctx)
	if isBootstrapOwner(id) {
		nama := "owner"
		if u, err := store.GetUser(ctx, id); err == nil && u != nil && u.Nama != "" {
			nama = u.Nama
		}
		return "owner", nama, nil
	}
	if id != "" {
		if u, err := store.GetUser(ctx, id); err == nil && u != nil {
			if !u.Aktif {
				return "", "", tools.ErrorResult(
					"Duh, akun kamu lagi nonaktif kak 😔 Coba minta pemilik kios buat aktifin dulu ya.",
				)
			}
			nama := u.Nama
			if nama == "" {
				nama = "kasir"
			}
			return u.Role, nama, nil
		}
	}
	return defaultRole(), "kasir-bot", nil
}

// requireOwner returns a refusal result when role is not owner.
func requireOwner(role string) *tools.ToolResult {
	if role != "owner" {
		return tools.ErrorResult("Maaf ya kak 🙏 aksi ini khusus pemilik (owner). Hubungi pemilik kios kalau perlu.")
	}
	return nil
}

// requireStaff returns a refusal when role is neither owner nor kasir.
func requireStaff(role string) *tools.ToolResult {
	if role != "owner" && role != "kasir" {
		return tools.ErrorResult("Maaf ya kak 🙏 aksi ini khusus owner/kasir.")
	}
	return nil
}

// findOne returns the single best product match for a query, or nil.
func findOne(ctx context.Context, store *Store, query string) (*Produk, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	all, err := store.GetAllProduk(ctx)
	if err != nil {
		return nil, err
	}
	matches := CariProduk(all, query)
	if len(matches) == 0 {
		return nil, nil
	}
	return matches[0], nil
}

// performJual executes a sale. It validates inputs, then dispatches by product
// kind. Ordinary unit-stock products go through sellBiasa (the historical path);
// special kinds (pulsa/bensin) are added in later plans.
func performJual(
	ctx context.Context,
	store *Store,
	query string,
	qty int,
	metode, kasir string,
	diskonPerUnit int,
) (*Transaksi, *Produk, int, error) {
	if qty <= 0 {
		return nil, nil, 0, fmt.Errorf("jumlahnya harus lebih dari 0 ya kak 🙏")
	}
	item, err := findOne(ctx, store, query)
	if err != nil {
		return nil, nil, 0, err
	}
	if item == nil {
		return nil, nil, 0, fmt.Errorf(
			"produk \"%s\" nggak ketemu kak 🔍 coba ketik /stok buat lihat daftarnya ya",
			query,
		)
	}
	if metode == "" {
		metode = "tunai"
	}
	switch item.JenisOrDefault() {
	// Only "biasa" exists today; "pulsa"/"bensin" cases are added in a later
	// plan. Until then every value (including any unrecognised jenis) falls
	// through to the ordinary unit-stock path.
	default:
		return sellBiasa(ctx, store, item, qty, metode, kasir, diskonPerUnit)
	}
}

// sellBiasa records a sale of an ordinary unit-stock product: it validates stock,
// decrements it, and appends the transaction at the effective unit price
// (list price minus diskonPerUnit). Returns the transaction, updated product, and
// remaining stock.
func sellBiasa(
	ctx context.Context,
	store *Store,
	item *Produk,
	qty int,
	metode, kasir string,
	diskonPerUnit int,
) (*Transaksi, *Produk, int, error) {
	if item.Stok < qty {
		return nil, nil, 0, fmt.Errorf(
			"yah, stok %s tinggal %d kak 😅 nggak cukup buat jual segitu",
			item.Nama,
			item.Stok,
		)
	}
	hargaEfektif := item.HargaJual - diskonPerUnit
	if hargaEfektif < 0 {
		hargaEfektif = 0
	}
	catatan := ""
	if diskonPerUnit > 0 {
		catatan = fmt.Sprintf("diskon %s/unit", FormatRupiah(diskonPerUnit))
	}
	now := NowWITA()
	item.Stok -= qty
	item.LastUpdate = now.Format("2006-01-02")
	if err := store.SetProduk(ctx, item); err != nil {
		return nil, nil, 0, err
	}
	tx := &Transaksi{
		Tanggal:     now.Format("2006-01-02"),
		Jam:         now.Format("15:04:05"),
		ProdukID:    item.ID,
		NamaProduk:  item.Nama,
		Kategori:    item.Kategori,
		Qty:         qty,
		HargaSatuan: hargaEfektif,
		Total:       qty * hargaEfektif,
		MetodeBayar: metode,
		Kasir:       kasir,
		Catatan:     catatan,
	}
	if _, err := store.AppendTransaksi(ctx, tx); err != nil {
		return nil, nil, 0, err
	}
	// Automatic learning: record the sale habit (peak hour + top product).
	_ = store.TrackHabit(ctx, "sale", item.Nama)
	return tx, item, item.Stok, nil
}

// activePromoDiskon returns the per-unit discount from the active promo for a
// product at the given qty/list price, or 0 with empty id when none applies.
func activePromoDiskon(ctx context.Context, store *Store, produkID string, qty, hargaJual int) (int, string) {
	if produkID == "" {
		return 0, ""
	}
	today := NowWITA().Format("2006-01-02")
	all, err := store.GetAllPromo(ctx)
	if err != nil {
		return 0, ""
	}
	for _, p := range all {
		if p.ProdukID != produkID || !promoAktif(p, today) || qty < p.MinQty {
			continue
		}
		diskon := int(p.Nilai)
		if p.Tipe == "persen" {
			diskon = int(float64(hargaJual) * p.Nilai / 100)
		}
		if diskon > hargaJual {
			diskon = hargaJual
		}
		return diskon, p.ID
	}
	return 0, ""
}
