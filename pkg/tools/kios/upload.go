package kios

import (
	"context"
	"fmt"
	"strings"

	"github.com/sipeed/picoclaw/pkg/media"
	tools "github.com/sipeed/picoclaw/pkg/tools"
	toolshared "github.com/sipeed/picoclaw/pkg/tools/shared"
)

// UploadTool imports an Excel/CSV file that the user uploaded in chat into the
// kios data (products or suppliers). Owner-only.
type UploadTool struct {
	store      *Store
	mediaStore media.MediaStore
}

// SetMediaStore is called by the tool registry to inject the media store.
func (t *UploadTool) SetMediaStore(s media.MediaStore) { t.mediaStore = s }

func (t *UploadTool) Name() string { return "kios_import_upload" }

func (t *UploadTool) Description() string {
	return "Import file Excel (.xlsx) atau CSV yang DIUNGGAH pengguna di chat ke data kios " +
		"(produk atau supplier). Panggil saat pengguna mengirim file lampiran dan minta import/" +
		"upload daftar. Khusus owner. Tipe (produk/supplier) bisa otomatis dari kolomnya."
}

func (t *UploadTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tipe": map[string]any{
				"type":        "string",
				"enum":        []string{"produk", "supplier"},
				"description": "jenis data dalam file (opsional; auto-deteksi dari kolom bila kosong)",
			},
		},
	}
}

func (t *UploadTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
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

	path, fname := t.findUploadedTable(toolshared.ToolMedia(ctx))
	if path == "" {
		return tools.NewToolResult("Belum ada file Excel/CSV yang diunggah kak 📎 Kirim filenya dulu sebagai lampiran, lalu minta import ya.")
	}

	rows, err := ReadTableFile(path)
	if err != nil {
		return tools.ErrorResult("Aduh, gagal baca file-nya kak 😣 Pastikan formatnya Excel (.xlsx) atau CSV yang benar.").WithError(err)
	}
	if len(rows) == 0 {
		return tools.NewToolResult("File-nya kosong atau cuma berisi header kak 😊 Isi datanya dulu ya.")
	}

	tipe := strings.ToLower(argStr(args, "tipe"))
	if tipe == "" {
		tipe = detectTipe(rows[0])
	}

	var res ImportResult
	switch tipe {
	case "supplier", "suplier":
		res, err = ImportSupplierRows(ctx, t.store, rows)
		tipe = "supplier"
	default:
		res, err = ImportProdukRows(ctx, t.store, rows)
		tipe = "produk"
	}
	if err != nil {
		return tools.ErrorResult("Aduh, gagal simpan data dari file kak 😣 Coba lagi ya.").WithError(err)
	}
	return tools.UserResult(fmt.Sprintf("File *%s* berhasil di-import ✅ (%s)\nDibuat: %d | Diupdate: %d | Dilewati: %d",
		fname, tipe, res.Created, res.Updated, res.Skipped))
}

// findUploadedTable returns the local path + filename of the first uploaded
// CSV/XLSX among the inbound media refs.
func (t *UploadTool) findUploadedTable(refs []string) (path, filename string) {
	for _, ref := range refs {
		p, meta, err := t.mediaStore.ResolveWithMeta(ref)
		if err != nil || p == "" {
			continue
		}
		name := strings.ToLower(meta.Filename)
		ct := strings.ToLower(meta.ContentType)
		if strings.HasSuffix(name, ".csv") || strings.HasSuffix(name, ".xlsx") ||
			strings.HasSuffix(strings.ToLower(p), ".csv") || strings.HasSuffix(strings.ToLower(p), ".xlsx") ||
			strings.Contains(ct, "spreadsheet") || strings.Contains(ct, "csv") || strings.Contains(ct, "excel") {
			fn := meta.Filename
			if fn == "" {
				fn = "file"
			}
			return p, fn
		}
	}
	return "", ""
}

// detectTipe guesses produk vs supplier from a row's column keys.
func detectTipe(row map[string]string) string {
	if _, ok := row["produk_utama"]; ok {
		return "supplier"
	}
	_, hasJual := row["harga_jual"]
	_, hasStok := row["stok"]
	if _, ok := row["kontak"]; ok && !hasJual && !hasStok {
		return "supplier"
	}
	return "produk"
}
