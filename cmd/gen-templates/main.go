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

const (
	headerBg  = "1B4F72" // navy-blue header background
	evenRowBg = "D6EAF8" // light-blue alternating row fill
	borderClr = "BFC9CA" // soft grey border
)

type colDef struct {
	header   string
	width    float64
	numFmt   int      // 0=General, 3=#,##0 (thousands separator)
	alignH   string   // "left" | "right" | "center"
	dropdown []string // nil = no validation
}

type sheetDef struct {
	file string
	name string
	cols []colDef
	rows [][]any
}

var (
	satuanList     = []string{"pcs", "kg", "liter", "gram", "bungkus", "botol", "karung", "tabung", "renceng", "pax", "lusin", "rim"}
	kategoriList   = []string{"sembako", "minuman", "snack", "rokok", "gas", "kebutuhan", "mie", "bumbu", "sayur", "buah", "obat", "lainnya"}
	pustakaCatList = []string{"harga", "statistik", "berita", "regulasi", "umum"}
)

var allSheets = []sheetDef{
	{
		file: "templates/produk-template.xlsx",
		name: "Produk",
		cols: []colDef{
			{header: "nama", width: 28, numFmt: 0, alignH: "left"},
			{header: "barcode", width: 18, numFmt: 0, alignH: "left"},
			{header: "kategori", width: 15, numFmt: 0, alignH: "left", dropdown: kategoriList},
			{header: "satuan", width: 11, numFmt: 0, alignH: "left", dropdown: satuanList},
			{header: "stok", width: 10, numFmt: 3, alignH: "right"},
			{header: "harga_beli", width: 16, numFmt: 3, alignH: "right"},
			{header: "harga_jual", width: 16, numFmt: 3, alignH: "right"},
			{header: "stok_minimum", width: 14, numFmt: 3, alignH: "right"},
			{header: "stok_kritis", width: 14, numFmt: 3, alignH: "right"},
			{header: "supplier", width: 22, numFmt: 0, alignH: "left"},
			{header: "gambar", width: 42, numFmt: 0, alignH: "left"},
		},
		rows: [][]any{
			{"Beras Medium 5kg", "8991234500015", "sembako", "karung", 20, 55000, 62000, 10, 3, "UD Maju", "https://contoh.com/gambar/beras.jpg"},
			{"Gula Pasir 1kg", "8991234500022", "sembako", "bungkus", 15, 13500, 15000, 8, 2, "Distributor Kupang", ""},
			{"Minyak Goreng 1L", "8991234500039", "sembako", "botol", 24, 16000, 18000, 10, 3, "UD Maju", ""},
			{"Tepung Terigu 1kg", "8991234500046", "sembako", "bungkus", 18, 11000, 13000, 8, 2, "Distributor Kupang", ""},
			{"Indomie Goreng", "089686010947", "mie", "pcs", 50, 2800, 3500, 20, 5, "Grosir Baa", ""},
			{"Teh Botol 350ml", "8992761111118", "minuman", "botol", 36, 3000, 4000, 12, 4, "Grosir Baa", ""},
			{"Kopi Sachet", "", "minuman", "renceng", 30, 9000, 11000, 10, 3, "Grosir Baa", ""},
			{"Rokok Surya 16", "8997011710018", "rokok", "bungkus", 40, 27000, 29000, 15, 5, "Agen Rokok Rote", ""},
			{"Gas LPG 3kg", "", "gas", "tabung", 10, 18000, 22000, 5, 2, "Pangkalan Gas Baa", ""},
			{"Sabun Mandi", "8999999036007", "kebutuhan", "pcs", 25, 3500, 5000, 10, 3, "Grosir Baa", ""},
			{"Biskuit Kelapa", "8990111222333", "snack", "bungkus", 40, 3500, 5000, 15, 5, "Grosir Baa", ""},
		},
	},
	{
		file: "templates/supplier-template.xlsx",
		name: "Supplier",
		cols: []colDef{
			{header: "nama", width: 24, numFmt: 0, alignH: "left"},
			{header: "kontak", width: 18, numFmt: 0, alignH: "left"},
			{header: "alamat", width: 30, numFmt: 0, alignH: "left"},
			{header: "produk_utama", width: 28, numFmt: 0, alignH: "left"},
			{header: "catatan", width: 38, numFmt: 0, alignH: "left"},
		},
		rows: [][]any{
			{"UD Maju", "0812xxxxxxx", "Baa Rote Ndao", "beras gula minyak", "MOQ 10 karung; lead time 2 hari"},
			{"Distributor Kupang", "0813xxxxxxx", "Kupang NTT", "sembako", "kirim tiap Senin"},
			{"Grosir Baa", "0852xxxxxxx", "Pasar Baa", "mie minuman snack", "bisa COD"},
			{"Agen Rokok Rote", "0821xxxxxxx", "Ba'a", "rokok", "harga ikut pita cukai"},
			{"Pangkalan Gas Baa", "0838xxxxxxx", "Ba'a", "gas LPG 3kg", "ambil sendiri, antri pagi"},
		},
	},
	{
		file: "templates/pustaka-template.xlsx",
		name: "Pustaka",
		cols: []colDef{
			{header: "judul", width: 32, numFmt: 0, alignH: "left"},
			{header: "info", width: 40, numFmt: 0, alignH: "left"},
			{header: "url", width: 45, numFmt: 0, alignH: "left"},
			{header: "kategori", width: 14, numFmt: 0, alignH: "left", dropdown: pustakaCatList},
		},
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

func makeBorders() []excelize.Border {
	return []excelize.Border{
		{Type: "left", Color: borderClr, Style: 1},
		{Type: "right", Color: borderClr, Style: 1},
		{Type: "top", Color: borderClr, Style: 1},
		{Type: "bottom", Color: borderClr, Style: 1},
	}
}

func main() {
	for _, sh := range allSheets {
		if err := generateSheet(sh); err != nil {
			fmt.Fprintf(os.Stderr, "FATAL %s: %v\n", sh.file, err)
			os.Exit(1)
		}
		fmt.Println("wrote", sh.file)
	}
}

func generateSheet(sh sheetDef) error {
	f := excelize.NewFile()
	if err := f.SetSheetName("Sheet1", sh.name); err != nil {
		return err
	}

	nCols := len(sh.cols)
	nDataRows := len(sh.rows)
	extraRows := 30 // pre-styled empty rows for user convenience

	// ── Style cache ──────────────────────────────────────────────────────────
	type styleKey struct {
		even   bool
		numFmt int
		alignH string
	}
	styleCache := map[styleKey]int{}

	getDataStyle := func(key styleKey) (int, error) {
		if id, ok := styleCache[key]; ok {
			return id, nil
		}
		fill := excelize.Fill{}
		if key.even {
			fill = excelize.Fill{Type: "pattern", Color: []string{evenRowBg}, Pattern: 1}
		}
		id, err := f.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Size: 11},
			Fill:      fill,
			NumFmt:    key.numFmt,
			Alignment: &excelize.Alignment{Horizontal: key.alignH, Vertical: "center"},
			Border:    makeBorders(),
		})
		if err != nil {
			return 0, err
		}
		styleCache[key] = id
		return id, nil
	}

	// Bold white header on navy background.
	headerStyleID, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11},
		Fill: excelize.Fill{Type: "pattern", Color: []string{headerBg}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    makeBorders(),
	})
	if err != nil {
		return fmt.Errorf("header style: %w", err)
	}

	// ── Header row ───────────────────────────────────────────────────────────
	for i, col := range sh.cols {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sh.name, cell, col.header); err != nil {
			return err
		}
		if err := f.SetCellStyle(sh.name, cell, cell, headerStyleID); err != nil {
			return err
		}
	}
	if err := f.SetRowHeight(sh.name, 1, 26); err != nil {
		return err
	}

	// ── Data rows ────────────────────────────────────────────────────────────
	for r, row := range sh.rows {
		excelRow := r + 2
		isEven := r%2 == 1 // rows 1,3,5 (0-indexed) get light-blue fill
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, excelRow)
			if err := f.SetCellValue(sh.name, cell, v); err != nil {
				return err
			}
			sid, err := getDataStyle(styleKey{even: isEven, numFmt: sh.cols[c].numFmt, alignH: sh.cols[c].alignH})
			if err != nil {
				return fmt.Errorf("data style r%d c%d: %w", r, c, err)
			}
			if err := f.SetCellStyle(sh.name, cell, cell, sid); err != nil {
				return err
			}
		}
	}

	// ── Pre-styled empty rows (continue stripe pattern) ───────────────────
	for excelRow := nDataRows + 2; excelRow <= nDataRows+1+extraRows; excelRow++ {
		isEven := (excelRow-2)%2 == 1
		for c := 0; c < nCols; c++ {
			cell, _ := excelize.CoordinatesToCellName(c+1, excelRow)
			sid, err := getDataStyle(styleKey{even: isEven, numFmt: sh.cols[c].numFmt, alignH: sh.cols[c].alignH})
			if err != nil {
				return err
			}
			if err := f.SetCellStyle(sh.name, cell, cell, sid); err != nil {
				return err
			}
		}
	}

	// ── Column widths ────────────────────────────────────────────────────────
	for i, col := range sh.cols {
		colLetter, _ := excelize.ColumnNumberToName(i + 1)
		if err := f.SetColWidth(sh.name, colLetter, colLetter, col.width); err != nil {
			return err
		}
	}

	// ── Data validation (dropdown lists) ─────────────────────────────────────
	lastRow := nDataRows + 1 + extraRows
	for i, col := range sh.cols {
		if len(col.dropdown) == 0 {
			continue
		}
		colLetter, _ := excelize.ColumnNumberToName(i + 1)
		sqref := fmt.Sprintf("%s2:%s%d", colLetter, colLetter, lastRow)
		dv := excelize.NewDataValidation(true)
		dv.Sqref = sqref
		if err := dv.SetDropList(col.dropdown); err != nil {
			return fmt.Errorf("dropdown %s: %w", col.header, err)
		}
		if err := f.AddDataValidation(sh.name, dv); err != nil {
			return fmt.Errorf("add validation %s: %w", col.header, err)
		}
	}

	// ── Freeze header row ────────────────────────────────────────────────────
	_ = f.SetPanes(sh.name, &excelize.Panes{
		Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft",
	})

	return f.SaveAs(sh.file)
}
