"use client";

import { useRef, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { CheckCircle2, Download, FileSpreadsheet, Loader2, TriangleAlert, Upload } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { parseTableFile, type TableRow } from "@/lib/parse-table";
import {
  importProdukAction,
  importStokAction,
  type ImportResult,
} from "@/app/(app)/impor/actions";
import type { Role } from "@/lib/types";

const OWNER_COLS = [
  "nama",
  "kategori",
  "satuan",
  "harga_beli",
  "harga_jual",
  "stok",
  "stok_minimum",
  "stok_kritis",
  "supplier",
  "barcode",
];
const KASIR_COLS = ["id", "nama", "stok"];

function templateCsv(cols: string[], example: string[]): string {
  return `${cols.join(",")}\n${example.join(",")}\n`;
}

export function ImporView({ role }: { role: Role }) {
  const router = useRouter();
  const isOwner = role === "owner";
  const cols = isOwner ? OWNER_COLS : KASIR_COLS;
  const fileRef = useRef<HTMLInputElement>(null);

  const [fileName, setFileName] = useState("");
  const [rows, setRows] = useState<TableRow[]>([]);
  const [headers, setHeaders] = useState<string[]>([]);
  const [parseError, setParseError] = useState<string | null>(null);
  const [parsing, setParsing] = useState(false);
  const [result, setResult] = useState<ImportResult | null>(null);
  const [applying, start] = useTransition();

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
      const r = isOwner ? await importProdukAction(rows) : await importStokAction(rows);
      setResult(r);
      if (r.ok) {
        router.refresh();
      }
    });
  }

  function downloadTemplate() {
    const example = isOwner
      ? ["Beras Premium 5kg", "sembako", "sak", "60000", "68000", "20", "5", "2", "Toko Tani", ""]
      : ["001", "Beras Premium 5kg", "18"];
    const blob = new Blob([templateCsv(cols, example)], { type: "text/csv;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = isOwner ? "template-produk.csv" : "template-stok.csv";
    a.click();
    URL.revokeObjectURL(url);
  }

  return (
    <div className="max-w-3xl space-y-4">
      <Card>
        <CardHeader className="flex-row items-start justify-between gap-3">
          <div>
            <CardTitle>{isOwner ? "Impor Produk" : "Stok Opname"}</CardTitle>
            <p className="mt-1 text-sm text-muted-foreground">
              {isOwner
                ? "Unggah Excel/CSV untuk membuat & memperbarui produk secara massal."
                : "Unggah Excel/CSV berisi jumlah stok terkini untuk produk yang sudah ada."}
            </p>
          </div>
          <Button variant="outline" size="sm" onClick={downloadTemplate}>
            <Download className="size-4" /> Template
          </Button>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="rounded-lg border border-dashed bg-muted/30 p-3 text-xs text-muted-foreground">
            Kolom yang dikenali:{" "}
            {cols.map((c) => (
              <code key={c} className="mr-1 rounded bg-muted px-1.5 py-0.5 font-mono">
                {c}
              </code>
            ))}
            {isOwner && <p className="mt-1.5">Produk baru wajib punya kolom <b>harga_jual</b>. Produk dicocokkan via id / barcode / nama.</p>}
            {!isOwner && <p className="mt-1.5">Hanya stok produk yang sudah ada yang diperbarui. Dicocokkan via id / barcode / nama.</p>}
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
                {isOwner ? "Impor / Perbarui Produk" : "Perbarui Stok"}
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
