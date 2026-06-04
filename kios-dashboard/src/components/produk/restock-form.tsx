"use client";

import { useState, useMemo, useTransition } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { formatRupiah } from "@/lib/format";
import type { Produk, Supplier } from "@/lib/types";

interface RestockFormProps {
  produkList: Produk[];
  suplierList: Supplier[];
  onSuccess?: () => void;
}

export function RestockForm({ produkList, suplierList, onSuccess }: RestockFormProps) {
  const [selectedProdukId, setSelectedProdukId] = useState("");
  const [selectedSuplierId, setSelectedSuplierId] = useState("");
  const [selectedKemasan, setSelectedKemasan] = useState("");
  const [qtyPack, setQtyPack] = useState<string>("1");
  const [hargaPack, setHargaPack] = useState<string>("");
  const [isiManual, setIsiManual] = useState<string>("");
  const [pending, startTransition] = useTransition();

  const selectedProduk = produkList.find((p) => p.id === selectedProdukId);
  const packDefs = selectedProduk?.pack_defs ?? [];

  const resolvedIsi = useMemo(() => {
    if (!selectedKemasan) return 0;
    const def = packDefs.find((k) => k.nama.toLowerCase() === selectedKemasan.toLowerCase());
    if (def) return def.isi;
    return isiManual ? parseInt(isiManual, 10) : 0;
  }, [selectedKemasan, packDefs, isiManual]);

  const preview = useMemo(() => {
    const qty = parseInt(qtyPack, 10) || 0;
    const harga = parseInt(hargaPack, 10) || 0;
    if (!resolvedIsi || !qty || !harga) return null;
    return {
      totalQty: qty * resolvedIsi,
      hargaPerPcs: Math.round(harga / resolvedIsi),
    };
  }, [qtyPack, hargaPack, resolvedIsi]);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!preview || !selectedProdukId) return;
    startTransition(async () => {
      // Placeholder: implementasi aktual via endpoint restock dashboard
      console.log("restock", {
        produk_id: selectedProdukId,
        supplier_id: selectedSuplierId,
        kemasan: selectedKemasan,
        qty_pack: parseInt(qtyPack, 10),
        harga_pack: parseInt(hargaPack, 10),
        isi: resolvedIsi,
      });
      onSuccess?.();
    });
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-1.5">
          <Label>Produk</Label>
          <Select value={selectedProdukId} onChange={(e) => { setSelectedProdukId(e.target.value); setSelectedKemasan(""); }}>
            <option value="">-- Pilih produk --</option>
            {produkList.map((p) => (
              <option key={p.id} value={p.id}>{p.nama} ({p.id})</option>
            ))}
          </Select>
        </div>
        <div className="space-y-1.5">
          <Label>Suplier</Label>
          <Select value={selectedSuplierId} onChange={(e) => setSelectedSuplierId(e.target.value)}>
            <option value="">-- Pilih suplier --</option>
            {suplierList.map((s) => (
              <option key={s.id} value={s.id}>{s.nama}</option>
            ))}
          </Select>
        </div>
      </div>

      <div className="grid grid-cols-3 gap-4">
        <div className="space-y-1.5">
          <Label>Kemasan</Label>
          <Select value={selectedKemasan} onChange={(e) => setSelectedKemasan(e.target.value)}>
            <option value="">-- Per pcs --</option>
            {packDefs.map((k) => (
              <option key={k.nama} value={k.nama}>{k.nama} ({k.isi} pcs)</option>
            ))}
          </Select>
        </div>
        <div className="space-y-1.5">
          <Label>Qty Kemasan</Label>
          <Input type="number" min={1} value={qtyPack} onChange={(e) => setQtyPack(e.target.value)} />
        </div>
        <div className="space-y-1.5">
          <Label>Harga/Kemasan (Rp)</Label>
          <Input type="number" min={0} value={hargaPack} onChange={(e) => setHargaPack(e.target.value)} className="font-mono" />
        </div>
      </div>

      {selectedKemasan && !packDefs.find((k) => k.nama.toLowerCase() === selectedKemasan.toLowerCase()) && (
        <div className="space-y-1.5">
          <Label>Isi per kemasan (pcs)</Label>
          <Input type="number" min={1} value={isiManual} onChange={(e) => setIsiManual(e.target.value)} placeholder="belum dikenal otomatis" />
        </div>
      )}

      {preview && (
        <div className="rounded-xl border bg-muted/30 p-3 text-sm space-y-1">
          <p>Stok bertambah: <strong>+{preview.totalQty} pcs</strong></p>
          <p>Harga beli/pcs: <strong>{formatRupiah(preview.hargaPerPcs)}</strong></p>
        </div>
      )}

      <div className="flex justify-end">
        <Button type="submit" disabled={pending || !preview}>
          {pending && <Loader2 className="size-4 animate-spin mr-2" />}
          Restock
        </Button>
      </div>
    </form>
  );
}
