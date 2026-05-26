package kios

import (
	"context"
	"fmt"
	"strings"

	tools "github.com/sipeed/picoclaw/pkg/tools"
)

// PustakaTool manages a knowledge base of info + malware-safe source URLs.
type PustakaTool struct{ store *Store }

func (t *PustakaTool) Name() string { return "kios_pustaka" }

func (t *PustakaTool) Description() string {
	return "Pustaka informasi kios: simpan catatan/sumber (URL) yang sudah dicek aman dari malware/phishing, " +
		"cari, daftar, hapus, serta cek keamanan sebuah URL. URL tidak aman akan ditolak. " +
		"Simpan/hapus khusus owner."
}

func (t *PustakaTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"tambah", "cari", "daftar", "hapus", "cek_url"},
				"description": "Aksi pustaka.",
			},
			"judul":    map[string]any{"type": "string"},
			"info":     map[string]any{"type": "string", "description": "isi catatan/informasi"},
			"url":      map[string]any{"type": "string", "description": "sumber URL (akan dicek keamanannya)"},
			"kategori": map[string]any{"type": "string"},
			"q":        map[string]any{"type": "string", "description": "kata kunci pencarian"},
			"id":       map[string]any{"type": "string", "description": "id entri (hapus)"},
		},
		"required": []string{"action"},
	}
}

func (t *PustakaTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	role, _, refusal := resolveRole(ctx, t.store)
	if refusal != nil {
		return refusal
	}
	switch argStr(args, "action") {
	case "cek_url":
		return t.cekURL(args)
	case "tambah":
		if r := requireOwner(role); r != nil {
			return r
		}
		return t.tambah(ctx, args)
	case "cari":
		return t.cari(ctx, args)
	case "daftar":
		return t.daftar(ctx)
	case "hapus":
		if r := requireOwner(role); r != nil {
			return r
		}
		return t.hapus(ctx, args)
	default:
		return tools.ErrorResult("Hmm, aksi pustaka belum dikenal kak 🤔")
	}
}

func (t *PustakaTool) cekURL(args map[string]any) *tools.ToolResult {
	u := argStr(args, "url")
	if u == "" {
		return tools.ErrorResult("URL-nya diisi dulu ya kak 🙏")
	}
	s := SkorAman(u)
	verdict := "AMAN"
	if !s.Aman {
		verdict = "TIDAK AMAN"
	}
	return tools.NewToolResult(fmt.Sprintf("%s — skor %d/100 (%s). Alasan: %s", verdict, s.Skor, u, s.Alasan))
}

func (t *PustakaTool) tambah(ctx context.Context, args map[string]any) *tools.ToolResult {
	judul := argStr(args, "judul")
	info := argStr(args, "info")
	u := argStr(args, "url")
	if judul == "" {
		return tools.ErrorResult("Judul-nya diisi dulu ya kak 🙏")
	}
	if info == "" && u == "" {
		return tools.ErrorResult("Isi info atau url minimal salah satu.")
	}
	skor := 0
	if u != "" {
		s := SkorAman(u)
		if !s.Aman {
			return tools.ErrorResult(fmt.Sprintf("URL ditolak (tidak aman, skor %d/100): %s. Sumber tidak disimpan.", s.Skor, s.Alasan))
		}
		skor = s.Skor
	}
	id, err := t.store.NextPustakaID(ctx)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal buat id pustaka kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	p := &Pustaka{
		ID: id, Judul: judul, Info: info, URL: u, Skor: skor,
		Kategori: argStr(args, "kategori"), Ditambahkan: NowWITA().Format("2006-01-02 15:04"),
	}
	if err := t.store.SetPustaka(ctx, p); err != nil {
		return tools.ErrorResult("Aduh, gagal simpan pustaka kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	msg := fmt.Sprintf("Tersimpan: [%s] %s.", p.ID, p.Judul)
	if u != "" {
		msg += fmt.Sprintf(" Sumber aman (skor %d/100).", skor)
	}
	return tools.NewToolResult(msg)
}

func (t *PustakaTool) cari(ctx context.Context, args map[string]any) *tools.ToolResult {
	q := strings.ToLower(strings.TrimSpace(argStr(args, "q")))
	all, err := t.store.GetAllPustaka(ctx)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal baca pustaka kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	var b strings.Builder
	count := 0
	for _, p := range all {
		hay := strings.ToLower(p.Judul + " " + p.Info + " " + p.Kategori)
		if q != "" && !strings.Contains(hay, q) {
			continue
		}
		t.writeEntry(&b, p)
		count++
	}
	if count == 0 {
		return tools.NewToolResult("Tidak ada entri pustaka yang cocok.")
	}
	return tools.NewToolResult(fmt.Sprintf("Hasil pustaka (%d):\n%s", count, b.String()))
}

func (t *PustakaTool) daftar(ctx context.Context) *tools.ToolResult {
	all, err := t.store.GetAllPustaka(ctx)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal baca pustaka kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	if len(all) == 0 {
		return tools.NewToolResult("Pustaka masih kosong.")
	}
	var b strings.Builder
	for _, p := range all {
		t.writeEntry(&b, p)
	}
	return tools.NewToolResult(fmt.Sprintf("Pustaka (%d entri):\n%s", len(all), b.String()))
}

func (t *PustakaTool) writeEntry(b *strings.Builder, p *Pustaka) {
	fmt.Fprintf(b, "- [%s] %s", p.ID, p.Judul)
	if p.Kategori != "" {
		fmt.Fprintf(b, " (%s)", p.Kategori)
	}
	if p.Info != "" {
		fmt.Fprintf(b, ": %s", p.Info)
	}
	if p.URL != "" {
		fmt.Fprintf(b, " — %s [aman %d/100]", p.URL, p.Skor)
	}
	b.WriteByte('\n')
}

func (t *PustakaTool) hapus(ctx context.Context, args map[string]any) *tools.ToolResult {
	id := strings.ToUpper(argStr(args, "id"))
	if id == "" {
		return tools.ErrorResult("ID entri-nya diisi dulu ya kak 🙏")
	}
	all, err := t.store.GetAllPustaka(ctx)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal baca pustaka kak 😣 Coba lagi sebentar ya.").WithError(err)
	}
	for _, p := range all {
		if strings.EqualFold(p.ID, id) {
			if err := t.store.DelPustaka(ctx, p.ID); err != nil {
				return tools.ErrorResult("Aduh, gagal hapus entri kak 😣 Coba lagi sebentar ya.").WithError(err)
			}
			return tools.NewToolResult(fmt.Sprintf("Entri %s (%s) dihapus.", p.ID, p.Judul))
		}
	}
	return tools.NewToolResult(fmt.Sprintf("Entri %s nggak ketemu kak 🔍", id))
}
