// Client-side helpers for exporting table data as CSV or Excel.

export function toCsvString(headers: string[], rows: string[][]): string {
  const escape = (s: string): string => {
    const v = String(s ?? "");
    return v.includes(",") || v.includes('"') || v.includes("\n")
      ? `"${v.replace(/"/g, '""')}"`
      : v;
  };
  return [headers, ...rows].map((row) => row.map(escape).join(",")).join("\n");
}

export function exportCsv(filename: string, headers: string[], rows: string[][]): void {
  const blob = new Blob(["﻿" + toCsvString(headers, rows)], {
    type: "text/csv;charset=utf-8",
  });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  setTimeout(() => URL.revokeObjectURL(url), 60_000);
}

export async function exportXlsx(
  filename: string,
  sheetName: string,
  headers: string[],
  rows: string[][],
): Promise<void> {
  const XLSX = await import("xlsx");
  const ws = XLSX.utils.aoa_to_sheet([headers, ...rows]);
  const wb = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(wb, ws, sheetName);
  XLSX.writeFile(wb, filename);
}
