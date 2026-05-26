// Command gen-templates generates the fill-in Excel (.xlsx) templates in
// templates/. Run from the repo root: go run ./cmd/gen-templates
// Note: this tool is NOT part of the picoclaw bot binary; excelize is only a
// build-time dependency of this generator.
package main

import (
	"fmt"
	"os"

	"github.com/xuri/excelize/v2"
)

type sheet struct {
	file    string
	name    string
	headers []string
	rows    [][]any
}

var sheets = []sheet{
	{
		file:    "templates/produk-template.xlsx",
		name:    "Produk",
		headers: []string{"nama", "barcode", "kategori", "satuan", "stok", "harga_beli", "harga_jual", "stok_minimum", "stok_kritis", "supplier"},
		rows: [][]any{
			{"Beras Medium 5kg", "8991234500015", "sembako", "karung", 20, 55000, 62000, 10, 3, "UD Maju"},
			{"Gula Pasir 1kg", "8991234500022", "sembako", "bungkus", 15, 13500, 15000, 8, 2, "Distributor Kupang"},
			{"Minyak Goreng 1L", "8991234500039", "sembako", "botol", 24, 16000, 18000, 10, 3, "UD Maju"},
			{"Tepung Terigu 1kg", "8991234500046", "sembako", "bungkus", 18, 11000, 13000, 8, 2, "Distributor Kupang"},
			{"Indomie Goreng", "089686010947", "mie", "pcs", 50, 2800, 3500, 20, 5, "Grosir Baa"},
			{"Teh Botol 350ml", "8992761111118", "minuman", "botol", 36, 3000, 4000, 12, 4, "Grosir Baa"},
			{"Kopi Sachet", "", "minuman", "renceng", 30, 9000, 11000, 10, 3, "Grosir Baa"},
			{"Rokok Surya 16", "8997011710018", "rokok", "bungkus", 40, 27000, 29000, 15, 5, "Agen Rokok Rote"},
			{"Gas LPG 3kg", "", "gas", "tabung", 10, 18000, 22000, 5, 2, "Pangkalan Gas Baa"},
			{"Sabun Mandi", "8999999036007", "kebutuhan", "pcs", 25, 3500, 5000, 10, 3, "Grosir Baa"},
		},
	},
	{
		file:    "templates/supplier-template.xlsx",
		name:    "Supplier",
		headers: []string{"nama", "kontak", "alamat", "produk_utama", "catatan"},
		rows: [][]any{
			{"UD Maju", "0812xxxxxxx", "Baa Rote Ndao", "beras gula minyak", "MOQ 10 karung; lead time 2 hari"},
			{"Distributor Kupang", "0813xxxxxxx", "Kupang NTT", "sembako", "kirim tiap Senin"},
			{"Grosir Baa", "0852xxxxxxx", "Pasar Baa", "mie minuman snack", "bisa COD"},
			{"Agen Rokok Rote", "0821xxxxxxx", "Ba'a", "rokok", "harga ikut pita cukai"},
			{"Pangkalan Gas Baa", "0838xxxxxxx", "Ba'a", "gas LPG 3kg", "ambil sendiri, antri pagi"},
		},
	},
	{
		file:    "templates/pustaka-template.xlsx",
		name:    "Pustaka",
		headers: []string{"judul", "info", "url", "kategori"},
		rows: [][]any{
			{"Panel Harga Badan Pangan", "Harga pangan harian nasional", "https://panelharga.badanpangan.go.id", "harga"},
			{"PIHPS Bank Indonesia", "Harga pangan strategis antarwilayah", "https://www.bi.go.id/hargapangan", "harga"},
			{"Kemendag SP2KP/EWS", "Harga & pasokan bahan pokok nasional", "https://ews.kemendag.go.id", "harga"},
			{"Distan & Ketahanan Pangan NTT", "Harga komoditi pasar Kupang (mingguan)", "https://distankp.nttprov.go.id", "harga"},
			{"BPS Nusa Tenggara Timur", "Inflasi & statistik harga NTT", "https://ntt.bps.go.id", "statistik"},
			{"ANTARA Kupang", "Berita ekonomi & harga pangan NTT", "https://kupang.antaranews.com", "berita"},
			{"Pemkab Rote Ndao", "Berita & program pangan daerah", "https://rotendaokab.go.id", "berita"},
			{"OnlineNTT", "Berita lokal Rote Ndao & NTT", "https://www.onlinentt.com", "berita"},
		},
	},
}

func main() {
	for _, sh := range sheets {
		f := excelize.NewFile()
		if err := f.SetSheetName("Sheet1", sh.name); err != nil {
			fail(err)
		}
		for i, h := range sh.headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			f.SetCellValue(sh.name, cell, h)
		}
		for r, row := range sh.rows {
			for c, v := range row {
				cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
				f.SetCellValue(sh.name, cell, v)
			}
		}
		// Bold header row + freeze it + widen columns.
		style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
		if err != nil {
			fail(err)
		}
		last, _ := excelize.CoordinatesToCellName(len(sh.headers), 1)
		f.SetCellStyle(sh.name, "A1", last, style)
		f.SetColWidth(sh.name, "A", columnLetter(len(sh.headers)), 18)
		_ = f.SetPanes(sh.name, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
		if err := f.SaveAs(sh.file); err != nil {
			fail(err)
		}
		fmt.Println("wrote", sh.file)
	}
}

func columnLetter(n int) string {
	c, _ := excelize.ColumnNumberToName(n)
	return c
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "FATAL:", err)
	os.Exit(1)
}
