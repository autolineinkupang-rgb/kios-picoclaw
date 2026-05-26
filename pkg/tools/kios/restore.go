package kios

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sipeed/picoclaw/pkg/media"
	tools "github.com/sipeed/picoclaw/pkg/tools"
	toolshared "github.com/sipeed/picoclaw/pkg/tools/shared"
)

// RestoreTool restores all kios data from a backup JSON file the owner uploaded
// in chat (the file produced by /backup). Owner-only and destructive: it
// replaces ALL current data, so it requires explicit confirmation (paksa=true).
type RestoreTool struct {
	store      *Store
	mediaStore media.MediaStore
}

// SetMediaStore is called by the tool registry to inject the media store.
func (t *RestoreTool) SetMediaStore(s media.MediaStore) { t.mediaStore = s }

func (t *RestoreTool) Name() string { return "kios_restore" }

func (t *RestoreTool) Description() string {
	return "Pulihkan SEMUA data kios dari file backup JSON yang DIUNGGAH pengguna di chat " +
		"(file hasil /backup). KHUSUS OWNER & menimpa seluruh data sekarang. Panggil saat owner " +
		"mengirim file backup .json dan minta pulihkan/restore. Langkah aman: panggil dulu tanpa " +
		"paksa untuk melihat ringkasan isi file + data sekarang, lalu setelah owner menyetujui, " +
		"panggil lagi dengan paksa=true untuk mengeksekusi."
}

func (t *RestoreTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"paksa": map[string]any{
				"type":        "boolean",
				"description": "true untuk benar-benar menimpa data sekarang (setelah owner mengonfirmasi).",
			},
		},
	}
}

func (t *RestoreTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	role, _, refusal := resolveRole(ctx, t.store)
	if refusal != nil {
		return refusal
	}
	if r := requireOwner(role); r != nil {
		return r
	}
	if t.mediaStore == nil {
		return tools.ErrorResult("Maaf kak, fitur baca file belum siap di server 😣")
	}

	path, fname := t.findUploadedJSON(toolshared.ToolMedia(ctx))
	if path == "" {
		return tools.NewToolResult("Belum ada file backup (.json) yang diunggah kak 📎 Kirim dulu file hasil /backup sebagai lampiran, lalu minta pulihkan ya.")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal baca file backup-nya kak 😣 Coba kirim ulang ya.").WithError(err)
	}
	data, err := ParseBackup(raw)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("File *%s* tidak bisa dipakai kak 🙏 %v", fname, err))
	}

	if !argBool(args, "paksa") {
		var b strings.Builder
		fmt.Fprintf(&b, "⚠️ Konfirmasi restore dari *%s*\n\n", fname)
		fmt.Fprintf(&b, "Isi file: %s.\nDibuat: %s.\n\n", data.Ringkas(), data.Dibuat)
		if existing, err := BuildBackup(ctx, t.store); err == nil {
			fmt.Fprintf(&b, "Data sekarang: %s.\n\n", existing.Ringkas())
		}
		b.WriteString("Restore akan MENIMPA SELURUH data kios sekarang dengan isi file ini — tidak bisa dibatalkan. " +
			"Kalau yakin, balas konfirmasi (mis. \"ya, restore paksa\").")
		return tools.NewToolResult(b.String())
	}

	if err := t.store.RestoreBackup(ctx, data); err != nil {
		return tools.ErrorResult("Aduh, restore gagal di tengah jalan kak 😣 Coba lagi ya.").WithError(err)
	}
	return tools.UserResult(fmt.Sprintf("✅ Data kios berhasil dipulihkan dari *%s*\n%s.\n(backup dibuat: %s)",
		fname, data.Ringkas(), data.Dibuat))
}

// findUploadedJSON returns the local path + filename of the first uploaded
// JSON file among the inbound media refs.
func (t *RestoreTool) findUploadedJSON(refs []string) (path, filename string) {
	for _, ref := range refs {
		p, meta, err := t.mediaStore.ResolveWithMeta(ref)
		if err != nil || p == "" {
			continue
		}
		name := strings.ToLower(meta.Filename)
		ct := strings.ToLower(meta.ContentType)
		if strings.HasSuffix(name, ".json") || strings.HasSuffix(strings.ToLower(p), ".json") ||
			strings.Contains(ct, "json") {
			fn := meta.Filename
			if fn == "" {
				fn = "backup.json"
			}
			return p, fn
		}
	}
	return "", ""
}
