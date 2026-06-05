"use client";

import { useRef, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import {
  CheckCircle2,
  Download,
  FileSpreadsheet,
  Loader2,
  TriangleAlert,
  Upload,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { parseTableFile, type TableRow } from "@/lib/parse-table";
import { exportCsv, exportXlsx } from "@/lib/export-table";
import {
  exportHutangAction,
  importHutangAction,
  type ImportResult,
} from "@/app/(app)/impor/actions";
import type { Hutang } from "@/lib/types";

const HEADERS: string[] = [
  "id",
  "tanggal",
  "supplier_id",
  "pokok",
  "dibayar",
  "sisa",
  "status",
  "jatuh_tempo",
  "catatan",
];

function toRows(data: Hutang[]): string[][] {
  return data.map((h) => [
    h.id,
    h.tanggal,
    h.supplier_id,
    String(h.pokok),
    String(h.dibayar),
    String(h.sisa),
    h.status,
    h.jatuh_tempo ?? "",
    h.catatan,
  ]);
}

const TEMPLATE_COLS = "supplier_id,supplier,pokok,dibayar,catatan,tanggal,jatuh_tempo";
const TEMPLATE_EXAMPLE = "SUP-001,Toko Grosir,500000,0,utang gula,2026-06-01,2026-07-01";

function templateCsvContent(): string {
  return `${TEMPLATE_COLS}\n${TEMPLATE_EXAMPLE}\n`;
}

export function ImporHutangTab() {
  const router = useRouter();
  const fileRef = useRef<HTMLInputElement>(null);

  const [fileName, setFileName] = useState("");
  const [rows, setRows] = useState<TableRow[]>([]);
  const [headers, setHeaders] = useState<string[]>([]);
  const [parseError, setParseError] = useState<string | null>(null);
  const [parsing, setParsing] = useState(false);
  const [result, setResult] = useState<ImportResult | null>(null);
  const [applying, start] = useTransition();
  const [exporting, startExport] = useTransition();

  async function onFile(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    setResult(null);
    setParseError(null);
    setRows([]);
    setHeaders([]);
    if (!file) return;
    setFileName(file.name);
    setParsing(true);
    try {
      const parsed = await parseTableFile(file);
      if (parsed.length === 0) {
        setParseError("Tidak ada baris data yang terbaca. Pastikan baris pertama adalah judul kolom.");
        return;
      }
      setRows(parsed);
      setHeaders(Object.keys(parsed[0]));
    } catch {
      setParseError("Gagal membaca file. Pastikan format .xlsx atau .csv yang benar.");
    } finally {
      setParsing(false);
    }
  }

  function apply() {
    if (rows.length === 0) return;
    start(async () => {
      const r = await importHutangAction(rows);
      setResult(r);
      if (r.ok) {
        router.refresh();
      }
    });
  }

  function downloadTemplate() {
    const blob = new Blob([templateCsvContent()], { type: "text/csv;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "template-hutang.csv";
    a.click();
    URL.revokeObjectURL(url);
  }

  function handleExportCsv() {
    startExport(async () => {
      const data = await exportHutangAction();
      exportCsv("hutang.csv", HEADERS, toRows(data));
    });
  }

  function handleExportXlsx() {
    startExport(async () => {
      const data = await exportHutangAction();
      await exportXlsx("hutang.xlsx", "Hutang", HEADERS, toRows(data));
    });
  }

  return (
    <div className="space-y-4">
      {/* Export Card */}
      <Card>
        <CardHeader>
          <CardTitle>Ekspor Hutang</CardTitle>
          <p className="mt-1 text-sm text-muted-foreground">
            Unduh semua data hutang ke supplier ke file CSV atau Excel.
          </p>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap gap-3">
            <Button variant="outline" size="sm" onClick={handleExportCsv} disabled={exporting}>
              {exporting ? <Loader2 className="size-4 animate-spin" /> : <Download className="size-4" />}
              CSV
            </Button>
            <Button variant="outline" size="sm" onClick={handleExportXlsx} disabled={exporting}>
              {exporting ? <Loader2 className="size-4 animate-spin" /> : <FileSpreadsheet className="size-4" />}
              Excel
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Import Card */}
      <Card>
        <CardHeader className="flex-row items-start justify-between gap-3">
          <div>
            <CardTitle>Impor Hutang</CardTitle>
            <p className="mt-1 text-sm text-muted-foreground">
              Unggah Excel/CSV untuk membuat data hutang ke supplier secara massal.
            </p>
          </div>
          <Button variant="outline" size="sm" onClick={downloadTemplate}>
            <Download className="size-4" /> Template
          </Button>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="rounded-lg border border-dashed bg-muted/30 p-3 text-xs text-muted-foreground">
            Kolom yang dikenali:{" "}
            {["supplier_id", "supplier", "pokok", "dibayar", "catatan", "tanggal", "jatuh_tempo"].map((c) => (
              <code key={c} className="mr-1 rounded bg-muted px-1.5 py-0.5 font-mono">
                {c}
              </code>
            ))}
            <p className="mt-1.5">Kolom <b>supplier_id</b> atau <b>supplier</b> dan <b>pokok</b> wajib diisi. Supplier harus sudah terdaftar.</p>
          </div>

          <div className="flex flex-wrap items-center gap-3">
            <input
              ref={fileRef}
              type="file"
              accept=".xlsx,.xls,.csv"
              onChange={onFile}
              className="hidden"
            />
            <Button variant="default" size="md" onClick={() => fileRef.current?.click()} disabled={parsing}>
              {parsing ? <Loader2 className="size-4 animate-spin" /> : <Upload className="size-4" />}
              Pilih File
            </Button>
            {fileName && (
              <span className="flex items-center gap-1.5 text-sm text-muted-foreground">
                <FileSpreadsheet className="size-4" /> {fileName}
              </span>
            )}
          </div>

          {parseError && (
            <p role="alert" className="flex items-center gap-1.5 text-sm text-destructive">
              <TriangleAlert className="size-4" /> {parseError}
            </p>
          )}

          {rows.length > 0 && (
            <>
              <div className="overflow-x-auto rounded-lg border">
                <table className="w-full text-xs">
                  <thead>
                    <tr className="border-b bg-muted/50 text-left text-muted-foreground">
                      {headers.map((h) => (
                        <th key={h} className="p-2 font-medium whitespace-nowrap">
                          {h}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {rows.slice(0, 8).map((r, i) => (
                      <tr key={i} className="border-b last:border-0">
                        {headers.map((h) => (
                          <td key={h} className="p-2 whitespace-nowrap">
                            {r[h]}
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <p className="text-xs text-muted-foreground">
                {rows.length} baris terbaca{rows.length > 8 ? " (menampilkan 8 pertama)" : ""}.
              </p>

              <Button variant="accent" size="md" onClick={apply} disabled={applying}>
                {applying && <Loader2 className="size-4 animate-spin" />}
                Impor Hutang
              </Button>
            </>
          )}

          {result && (
            <div
              role="status"
              aria-live="polite"
              className={
                result.ok
                  ? "rounded-lg border border-success/30 bg-success/10 p-3 text-sm"
                  : "rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive"
              }
            >
              {result.ok ? (
                <div className="space-y-1.5">
                  <p className="flex items-center gap-1.5 font-medium text-success">
                    <CheckCircle2 className="size-4" /> Impor selesai
                  </p>
                  <p className="text-foreground">
                    {result.created > 0 && `${result.created} dibuat · `}
                    {result.updated} diperbarui
                    {result.skipped > 0 && ` · ${result.skipped} dilewati`}
                  </p>
                  {result.errors.length > 0 && (
                    <ul className="mt-1 list-inside list-disc text-xs text-muted-foreground">
                      {result.errors.map((e, i) => (
                        <li key={i}>{e}</li>
                      ))}
                    </ul>
                  )}
                </div>
              ) : (
                <p className="flex items-center gap-1.5">
                  <TriangleAlert className="size-4" /> {result.error}
                </p>
              )}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
