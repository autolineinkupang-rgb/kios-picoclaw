"use client";

import { useState, useTransition } from "react";
import { Download, FileSpreadsheet, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { exportCsv, exportXlsx } from "@/lib/export-table";
import {
  exportTransaksiAction,
  type PeriodeExport,
} from "@/app/(app)/impor/actions";
import type { Transaksi } from "@/lib/types";

const HEADERS: string[] = [
  "id",
  "tanggal",
  "jam",
  "nama_produk",
  "kategori",
  "qty",
  "harga_satuan",
  "total",
  "metode_bayar",
  "kasir",
  "catatan",
  "modal",
];

function toRows(data: Transaksi[]): string[][] {
  return data.map((t) => [
    t.id,
    t.tanggal,
    t.jam,
    t.nama_produk,
    t.kategori,
    String(t.qty),
    String(t.harga_satuan),
    String(t.total),
    t.metode_bayar,
    t.kasir,
    t.catatan,
    String(t.modal ?? 0),
  ]);
}

const PERIODE_OPTIONS: { value: PeriodeExport; label: string }[] = [
  { value: "hari_ini", label: "Hari Ini" },
  { value: "7hari", label: "7 Hari Terakhir" },
  { value: "30hari", label: "30 Hari Terakhir" },
  { value: "semua", label: "Semua Data" },
];

export function ImporPenjualanTab() {
  const [periode, setPeriode] = useState<PeriodeExport>("30hari");
  const [exporting, startExport] = useTransition();

  function handleExportCsv() {
    startExport(async () => {
      const data = await exportTransaksiAction(periode);
      exportCsv("penjualan.csv", HEADERS, toRows(data));
    });
  }

  function handleExportXlsx() {
    startExport(async () => {
      const data = await exportTransaksiAction(periode);
      await exportXlsx("penjualan.xlsx", "Penjualan", HEADERS, toRows(data));
    });
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Ekspor Penjualan</CardTitle>
        <p className="mt-1 text-sm text-muted-foreground">
          Unduh riwayat transaksi penjualan ke file CSV atau Excel.
        </p>
      </CardHeader>
      <CardContent className="space-y-4">
        <div>
          <label
            htmlFor="periode-select"
            className="mb-1.5 block text-sm font-medium text-foreground"
          >
            Periode
          </label>
          <select
            id="periode-select"
            value={periode}
            onChange={(e) => setPeriode(e.target.value as PeriodeExport)}
            className="rounded-md border bg-background px-3 py-2 text-sm text-foreground shadow-sm focus:outline-none focus:ring-2 focus:ring-ring"
          >
            {PERIODE_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </div>

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
  );
}
