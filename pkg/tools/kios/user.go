package kios

import (
	"context"
	"fmt"
	"strings"

	tools "github.com/sipeed/picoclaw/pkg/tools"
)

// UserTool manages authorized users / roles (owner-only).
type UserTool struct{ store *Store }

func (t *UserTool) Name() string { return "kios_user" }

func (t *UserTool) Description() string {
	return "Kelola pengguna kios (khusus owner): tambah/lihat/nonaktifkan/aktifkan pengguna dan " +
		"atur peran (kasir/owner). Pengguna diidentifikasi dengan ID Telegram (angka)."
}

func (t *UserTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"tambah", "list", "nonaktif", "aktifkan", "set_role"},
				"description": "Aksi manajemen pengguna.",
			},
			"id":   map[string]any{"type": "string", "description": "ID Telegram pengguna (angka)"},
			"nama": map[string]any{"type": "string"},
			"role": map[string]any{"type": "string", "enum": []string{"kasir", "owner"}},
		},
		"required": []string{"action"},
	}
}

func (t *UserTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	role, _, refusal := resolveRole(ctx, t.store)
	if refusal != nil {
		return refusal
	}
	if r := requireOwner(role); r != nil {
		return r
	}
	switch argStr(args, "action") {
	case "tambah":
		return t.tambah(ctx, args)
	case "list":
		return t.list(ctx)
	case "nonaktif":
		return t.setAktif(ctx, args, false)
	case "aktifkan":
		return t.setAktif(ctx, args, true)
	case "set_role":
		return t.setRole(ctx, args)
	default:
		return tools.ErrorResult("Hmm, aksi user belum dikenal kak 🤔")
	}
}

func normalizeRole(r string) string {
	r = strings.ToLower(strings.TrimSpace(r))
	if r == "owner" {
		return "owner"
	}
	return "kasir"
}

func (t *UserTool) tambah(ctx context.Context, args map[string]any) *tools.ToolResult {
	id := argStr(args, "id")
	if id == "" {
		return tools.ErrorResult("ID Telegram pengguna-nya diisi dulu ya kak 🙏")
	}
	u := &UserKios{
		Phone:       id,
		Nama:        argStr(args, "nama"),
		Role:        normalizeRole(argStr(args, "role")),
		Aktif:       true,
		Ditambahkan: NowWITA().Format("2006-01-02 15:04"),
	}
	if err := t.store.SetUser(ctx, u); err != nil {
		return tools.ErrorResult("Aduh, gagal simpan pengguna kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	return tools.NewToolResult(fmt.Sprintf("Pengguna ditambahkan: %s (%s), peran %s.", u.Nama, id, u.Role))
}

func (t *UserTool) list(ctx context.Context) *tools.ToolResult {
	users, err := t.store.GetAllUsers(ctx)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal baca pengguna kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	if len(users) == 0 {
		return tools.NewToolResult("Belum ada pengguna terdaftar (semua user whitelist dianggap owner).")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Pengguna terdaftar (%d):\n", len(users))
	for _, u := range users {
		status := "aktif"
		if !u.Aktif {
			status = "NONAKTIF"
		}
		fmt.Fprintf(&b, "- %s (%s) — %s, %s\n", u.Nama, u.Phone, u.Role, status)
	}
	return tools.NewToolResult(b.String())
}

func (t *UserTool) setAktif(ctx context.Context, args map[string]any, aktif bool) *tools.ToolResult {
	id := argStr(args, "id")
	if id == "" {
		return tools.ErrorResult("ID Telegram pengguna-nya diisi dulu ya kak 🙏")
	}
	u, err := t.store.GetUser(ctx, id)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal baca pengguna kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	if u == nil {
		return tools.NewToolResult("Pengguna nggak ketemu kak 🔍")
	}
	u.Aktif = aktif
	if err := t.store.SetUser(ctx, u); err != nil {
		return tools.ErrorResult("Aduh, gagal simpan pengguna kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	state := "diaktifkan"
	if !aktif {
		state = "dinonaktifkan"
	}
	return tools.NewToolResult(fmt.Sprintf("Pengguna %s (%s) %s.", u.Nama, id, state))
}

func (t *UserTool) setRole(ctx context.Context, args map[string]any) *tools.ToolResult {
	id := argStr(args, "id")
	if id == "" {
		return tools.ErrorResult("ID Telegram pengguna-nya diisi dulu ya kak 🙏")
	}
	u, err := t.store.GetUser(ctx, id)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal baca pengguna kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	if u == nil {
		return tools.NewToolResult("Pengguna nggak ketemu kak 🔍. Tambahkan dulu dengan action tambah")
	}
	u.Role = normalizeRole(argStr(args, "role"))
	if err := t.store.SetUser(ctx, u); err != nil {
		return tools.ErrorResult("Aduh, gagal simpan pengguna kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	return tools.NewToolResult(fmt.Sprintf("Peran %s (%s) diatur ke %s.", u.Nama, id, u.Role))
}
