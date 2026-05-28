"use client";

import { useMemo, useState, useTransition } from "react";
import { Loader2, Star } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { setHargaOverrideAction } from "@/app/(app)/suplier/actions";
import { formatRupiah } from "@/lib/format";
import type { Produk, Pembelian } from "@/lib/types";

interface BandingRow {
  supplier: string;
  harga: number;
  isOverride: boolean;
  isCheapest: boolean;
}

export function BandingHarga({
  produkList,
  pembelianList,
  hargaOverrides,
  canManage,
}: {
  produkList: Produk[];
  pembelianList: Pembelian[];
  hargaOverrides: Record<string, number>;
  canManage: boolean;
}) {
  const [selectedProdukId, setSelectedProdukId] = useState<string>("");
  const [overrideSupplier, setOverrideSupplier] = useState("");
  const [overrideHarga, setOverrideHarga] = useState<string>("");
  const [overrideError, setOverrideError] = useState<string | null>(null);
  const [overrideOk, setOverrideOk] = useState(false);
  const [pending, startTransition] = useTransition();

  const selectedProduk = produkList.find((p) => p.id === selectedProdukId);

  const rows = useMemo<BandingRow[]>(() => {
    if (!selectedProdukId) return [];

    // Hitung harga beli terendah per supplier dari pembelianList
    const minPerSupplier: Record<string, number> = {};
    for (const pb of pembelianList) {
      if (!pb.supplier) continue;
      const match =
        pb.produk_id === selectedProdukId ||
        pb.nama_produk.toLowerCase().includes(selectedProduk?.nama.toLowerCase() ?? "");
      if (!match) continue;
      if (pb.harga_beli <= 0) continue;
      if (minPerSupplier[pb.supplier] === undefined || pb.harga_beli < minPerSupplier[pb.supplier]) {
        minPerSupplier[pb.supplier] = pb.harga_beli;
      }
    }

    // Terapkan override
    for (const [key, harga] of Object.entries(hargaOverrides)) {
      const [produkId, supplier] = key.split("|");
      if (produkId !== selectedProdukId || !supplier) continue;
      minPerSupplier[supplier] = harga;
    }

    if (Object.keys(minPerSupplier).length === 0) return [];

    const sorted = Object.entries(minPerSupplier).sort((a, b) => a[1] - b[1]);
    const cheapestPrice = sorted[0][1];

    return sorted.map(([supplier, harga]) => {
      const overrideKey = `${selectedProdukId}|${supplier}`;
      const isOverride = overrideKey in hargaOverrides;
      return {
        supplier,
        harga,
        isOverride,
        isCheapest: harga === cheapestPrice,
      };
    });
  }, [selectedProdukId, pembelianList, hargaOverrides, selectedProduk]);

  function submitOverride(e: React.FormEvent) {
    e.preventDefault();
    setOverrideError(null);
    setOverrideOk(false);
    const harga = parseFloat(overrideHarga);
    if (!selectedProdukId) {
      setOverrideError("Pilih produk terlebih dahulu.");
      return;
    }
    if (!overrideSupplier.trim()) {
      setOverrideError("Nama supplier wajib diisi.");
      return;
    }
    if (!Number.isFinite(harga) || harga < 0) {
      setOverrideError("Harga harus berupa angka positif.");
      return;
    }
    startTransition(async () => {
      const result = await setHargaOverrideAction(
        selectedProdukId,
        overrideSupplier.trim(),
        harga,
      );
      if (!result.ok) {
        setOverrideError(result.error);
      } else {
        setOverrideOk(true);
        setOverrideSupplier("");
        setOverrideHarga("");
        window.setTimeout(() => setOverrideOk(false), 3000);
      }
    });
  }

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-base font-semibold">Banding Harga Supplier</h3>
        <p className="text-sm text-muted-foreground">
          Lihat dan bandingkan harga beli terendah per supplier untuk setiap produk.
        </p>
      </div>

      <div className="max-w-xs">
        <Label htmlFor="produk-select">Pilih Produk</Label>
        <Select
          id="produk-select"
          value={selectedProdukId}
          onChange={(e) => setSelectedProdukId(e.target.value)}
          className="mt-1.5"
          aria-label="Pilih produk untuk banding harga"
        >
          <option value="">-- Pilih produk --</option>
          {produkList.map((p) => (
            <option key={p.id} value={p.id}>
              {p.nama} ({p.id})
            </option>
          ))}
        </Select>
      </div>

      {selectedProdukId && rows.length === 0 && (
        <p className="text-sm text-muted-foreground">
          Belum ada data pembelian untuk produk ini. Tambahkan pembelian atau isi override manual.
        </p>
      )}

      {rows.length > 0 && (
        <div className="overflow-x-auto rounded-xl border bg-card">
          <table className="w-full min-w-[400px] text-sm">
            <thead>
              <tr className="border-b text-left text-xs text-muted-foreground">
                <th className="p-3 font-medium">Supplier</th>
                <th className="p-3 text-right font-medium">Harga Beli</th>
                <th className="p-3 font-medium">Keterangan</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr
                  key={row.supplier}
                  className="border-b transition-colors last:border-0 hover:bg-muted/40"
                >
                  <td className="p-3 font-medium">{row.supplier}</td>
                  <td className="p-3 text-right font-mono tabular-nums">
                    {formatRupiah(row.harga)}
                  </td>
                  <td className="p-3">
                    <span className="flex items-center gap-1 text-xs text-muted-foreground">
                      {row.isCheapest && (
                        <Star className="size-3.5 fill-yellow-400 text-yellow-400" />
                      )}
                      {row.isCheapest && "Termurah"}
                      {row.isOverride && !row.isCheapest && "Override manual"}
                      {row.isOverride && row.isCheapest && " · Override manual"}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {canManage && selectedProdukId && (
        <div className="rounded-xl border bg-card p-4">
          <h4 className="mb-3 text-sm font-semibold">Isi Override Harga Manual</h4>
          <form onSubmit={submitOverride} className="space-y-3">
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="override-supplier">Nama Supplier</Label>
                <Input
                  id="override-supplier"
                  value={overrideSupplier}
                  onChange={(e) => setOverrideSupplier(e.target.value)}
                  placeholder="mis. CV Maju Jaya"
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="override-harga">Harga Beli (Rp)</Label>
                <Input
                  id="override-harga"
                  type="number"
                  inputMode="numeric"
                  min={0}
                  value={overrideHarga}
                  onChange={(e) => setOverrideHarga(e.target.value)}
                  placeholder="0"
                  className="font-mono tabular-nums"
                />
              </div>
            </div>

            {overrideError && (
              <p role="alert" className="text-sm text-destructive">
                {overrideError}
              </p>
            )}
            {overrideOk && (
              <p role="status" className="text-sm text-success">
                Override harga berhasil disimpan.
              </p>
            )}

            <div className="flex justify-end">
              <Button type="submit" variant="accent" size="sm" disabled={pending}>
                {pending && <Loader2 className="size-4 animate-spin" />}
                Simpan Override
              </Button>
            </div>
          </form>
        </div>
      )}
    </div>
  );
}
