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
		headers: []string{"nama", "kategori", "satuan", "stok", "harga_beli", "harga_jual", "stok_minimum", "stok_kritis", "supplier"},
		rows: [][]any{
			{"Beras Medium 5kg", "sembako", "karung", 20, 55000, 62000, 10, 3, "UD Maju"},
			{"Gula Pasir 1kg", "sembako", "bungkus", 15, 13500, 15000, 8, 2, "Distributor Kupang"},
			{"Minyak Goreng 1L", "sembako", "botol", 24, 16000, 18000, 10, 3, "UD Maju"},
			{"Tepung Terigu 1kg", "sembako", "bungkus", 18, 11000, 13000, 8, 2, "Distributor Kupang"},
			{"Indomie Goreng", "mie", "pcs", 50, 2800, 3500, 20, 5, "Grosir Baa"},
			{"Teh Botol 350ml", "minuman", "botol", 36, 3000, 4000, 12, 4, "Grosir Baa"},
			{"Kopi Sachet", "minuman", "renceng", 30, 9000, 11000, 10, 3, "Grosir Baa"},
			{"Rokok Surya 16", "rokok", "bungkus", 40, 27000, 29000, 15, 5, "Agen Rokok Rote"},
			{"Gas LPG 3kg", "gas", "tabung", 10, 18000, 22000, 5, 2, "Pangkalan Gas Baa"},
			{"Sabun Mandi", "kebutuhan", "pcs", 25, 3500, 5000, 10, 3, "Grosir Baa"},
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
