"use client";

import { useState, useTransition } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import {
  createProdukAction,
  updateProdukAction,
  type ActionResult,
  type ProdukInput,
} from "@/app/(app)/produk/actions";
import type { Produk } from "@/lib/types";

function emptyForm(): ProdukInput {
  return {
    nama: "",
    barcode: "",
    kategori: "",
    satuan: "pcs",
    stok: 0,
    harga_beli: 0,
    harga_jual: 0,
    stok_minimum: 5,
    stok_kritis: 2,
    supplier: "",
    exp_date: "",
  };
}

function fromProduk(p: Produk): ProdukInput {
  return {
    id: p.id,
    nama: p.nama,
    barcode: p.barcode,
    kategori: p.kategori,
    satuan: p.satuan,
    stok: p.stok,
    harga_beli: p.harga_beli,
    harga_jual: p.harga_jual,
    stok_minimum: p.stok_minimum,
    stok_kritis: p.stok_kritis,
    supplier: p.supplier,
    exp_date: p.exp_date,
  };
}

export function ProdukForm({
  produk,
  onResult,
  onCancel,
}: {
  produk?: Produk;
  onResult: (r: ActionResult) => void;
  onCancel: () => void;
}) {
  const [form, setForm] = useState<ProdukInput>(produk ? fromProduk(produk) : emptyForm());
  const [error, setError] = useState<string | null>(null);
  const [pending, startTransition] = useTransition();
  const isEdit = Boolean(produk);

  function set<K extends keyof ProdukInput>(key: K, value: ProdukInput[K]) {
    setForm((f) => ({ ...f, [key]: value }));
  }

  function numField(key: keyof ProdukInput, label: string, hint?: string) {
    return (
      <div className="space-y-1.5">
        <Label htmlFor={key}>{label}</Label>
        <Input
          id={key}
          type="number"
          inputMode="numeric"
          min={0}
          value={String(form[key] ?? 0)}
          onChange={(e) => set(key, Number(e.target.value) as never)}
          className="font-mono tabular-nums"
        />
        {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
      </div>
    );
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    if (!form.nama.trim()) {
      setError("Nama produk wajib diisi.");
      return;
    }
    startTransition(async () => {
      const result = isEdit
        ? await updateProdukAction(form)
        : await createProdukAction(form);
      if (!result.ok) {
        setError(result.error);
        return;
      }
      onResult(result);
    });
  }

  return (
    <form onSubmit={submit} className="space-y-4">
      <div className="space-y-1.5">
        <Label htmlFor="nama">
          Nama Produk <span className="text-destructive">*</span>
        </Label>
        <Input
          id="nama"
          value={form.nama}
          onChange={(e) => set("nama", e.target.value)}
          placeholder="mis. Beras Premium 5kg"
          required
          autoFocus
        />
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-1.5">
          <Label htmlFor="kategori">Kategori</Label>
          <Input
            id="kategori"
            value={form.kategori}
            onChange={(e) => set("kategori", e.target.value)}
            placeholder="umum"
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="satuan">Satuan</Label>
          <Input
            id="satuan"
            value={form.satuan}
            onChange={(e) => set("satuan", e.target.value)}
            placeholder="pcs"
          />
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4">
        {numField("harga_beli", "Harga Beli (Rp)")}
        {numField("harga_jual", "Harga Jual (Rp)")}
      </div>

      <div className="grid grid-cols-3 gap-4">
        {numField("stok", "Stok")}
        {numField("stok_minimum", "Stok Min")}
        {numField("stok_kritis", "Stok Kritis")}
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-1.5">
          <Label htmlFor="supplier">Supplier</Label>
          <Input
            id="supplier"
            value={form.supplier}
            onChange={(e) => set("supplier", e.target.value)}
            placeholder="opsional"
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="barcode">Barcode</Label>
          <Input
            id="barcode"
            value={form.barcode}
            onChange={(e) => set("barcode", e.target.value)}
            placeholder="opsional"
            className="font-mono"
          />
        </div>
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="exp_date">Tanggal Kedaluwarsa</Label>
        <Input
          id="exp_date"
          type="date"
          value={form.exp_date}
          onChange={(e) => set("exp_date", e.target.value)}
        />
        <p className="text-xs text-muted-foreground">Kosongkan bila produk tidak kedaluwarsa.</p>
      </div>

      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}

      <div className="flex justify-end gap-2 pt-2">
        <Button type="button" variant="outline" size="sm" onClick={onCancel} disabled={pending}>
          Batal
        </Button>
        <Button type="submit" variant="accent" size="sm" disabled={pending}>
          {pending && <Loader2 className="size-4 animate-spin" />}
          {isEdit ? "Simpan Perubahan" : "Tambah Produk"}
        </Button>
      </div>
    </form>
  );
}
