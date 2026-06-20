package kios

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"os"
	"strings"
)

// ReadTableFile reads a .csv or .xlsx file into header→value maps (header
// lowercased), so the same import logic serves both formats. The format is
// detected by content (xlsx files are ZIP archives starting with "PK"), which
// works even when a downloaded file has no extension. XLSX is parsed with the
// standard library (archive/zip + encoding/xml) — no external dependency,
// keeping the bot binary lean.
func ReadTableFile(path string) ([]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	magic := make([]byte, 2)
	n, _ := f.Read(magic)
	f.Close()
	if n == 2 && magic[0] == 'P' && magic[1] == 'K' {
		return readXLSXFile(path)
	}
	return readCSVRows(path)
}

// --- minimal XLSX reader (first worksheet only) ---

type xlsxSST struct {
	SI []xlsxSI `xml:"si"`
}

type xlsxSI struct {
	T string   `xml:"t"`
	R []xlsxRT `xml:"r"`
}

type xlsxRT struct {
	T string `xml:"t"`
}

func (si xlsxSI) text() string {
	if len(si.R) == 0 {
		return si.T
	}
	var b strings.Builder
	for _, r := range si.R {
		b.WriteString(r.T)
	}
	return b.String()
}

type xlsxSheet struct {
	Rows []xlsxRow `xml:"sheetData>row"`
}

type xlsxRow struct {
	Cells []xlsxCell `xml:"c"`
}

type xlsxCell struct {
	Ref     string `xml:"r,attr"`
	Type    string `xml:"t,attr"`
	V       string `xml:"v"`
	InlineT string `xml:"is>t"`
}

func readXLSXFile(path string) ([]map[string]string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("buka xlsx: %w", err)
	}
	defer zr.Close()

	var shared []string
	var sheetName string
	for _, f := range zr.File {
		switch {
		case f.Name == "xl/sharedStrings.xml":
			shared, err = parseSharedStrings(f)
			if err != nil {
				return nil, err
			}
		case strings.HasPrefix(f.Name, "xl/worksheets/sheet") && strings.HasSuffix(f.Name, ".xml"):
			if sheetName == "" || f.Name < sheetName {
				sheetName = f.Name
			}
		}
	}
	if sheetName == "" {
		return nil, fmt.Errorf("xlsx tidak punya worksheet")
	}
	var sheetFile *zip.File
	for _, f := range zr.File {
		if f.Name == sheetName {
			sheetFile = f
			break
		}
	}
	grid, err := parseSheet(sheetFile, shared)
	if err != nil {
		return nil, err
	}
	if len(grid) < 2 {
		return nil, nil
	}
	headers := make([]string, len(grid[0]))
	for i, h := range grid[0] {
		header := strings.TrimSpace(h)
		// Remove common annotation characters (e.g. trailing '*') and
		// normalize surrounding whitespace so templates with markers
		// like "Nama *" map to the canonical header "nama".
		header = strings.TrimSuffix(header, "*")
		header = strings.TrimSpace(header)
		headers[i] = strings.ToLower(header)
	}
	out := make([]map[string]string, 0, len(grid)-1)
	for _, row := range grid[1:] {
		m := make(map[string]string, len(headers))
		for i, v := range row {
			if i < len(headers) && headers[i] != "" {
				m[headers[i]] = strings.TrimSpace(v)
			}
		}
		out = append(out, m)
	}
	return out, nil
}

func parseSharedStrings(f *zip.File) ([]string, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	var sst xlsxSST
	if err := xml.NewDecoder(rc).Decode(&sst); err != nil {
		return nil, fmt.Errorf("parse sharedStrings: %w", err)
	}
	out := make([]string, len(sst.SI))
	for i, si := range sst.SI {
		out[i] = si.text()
	}
	return out, nil
}

func parseSheet(f *zip.File, shared []string) ([][]string, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	var sheet xlsxSheet
	if err := xml.NewDecoder(rc).Decode(&sheet); err != nil {
		return nil, fmt.Errorf("parse worksheet: %w", err)
	}
	grid := make([][]string, 0, len(sheet.Rows))
	for _, row := range sheet.Rows {
		var cells []string
		for _, c := range row.Cells {
			col := colIndex(c.Ref)
			for len(cells) <= col {
				cells = append(cells, "")
			}
			cells[col] = cellValue(c, shared)
		}
		grid = append(grid, cells)
	}
	return grid, nil
}

func cellValue(c xlsxCell, shared []string) string {
	switch c.Type {
	case "s": // shared string: V is the index
		idx := 0
		fmt.Sscanf(c.V, "%d", &idx)
		if idx >= 0 && idx < len(shared) {
			return shared[idx]
		}
		return ""
	case "inlineStr":
		return c.InlineT
	default: // number, boolean, formula result — raw value
		return c.V
	}
}

// colIndex converts a cell ref like "B7" to a 0-based column index (B → 1).
func colIndex(ref string) int {
	idx := 0
	for i := 0; i < len(ref); i++ {
		ch := ref[i]
		if ch >= 'A' && ch <= 'Z' {
			idx = idx*26 + int(ch-'A'+1)
		} else if ch >= 'a' && ch <= 'z' {
			idx = idx*26 + int(ch-'a'+1)
		} else {
			break
		}
	}
	if idx > 0 {
		idx--
	}
	return idx
}
