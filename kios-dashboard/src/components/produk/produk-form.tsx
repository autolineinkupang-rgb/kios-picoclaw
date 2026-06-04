"use client";

import { useRef, useState, useTransition } from "react";
import { Loader2, Upload } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { fileToCompressedDataUrl } from "@/lib/image-upload";
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
    image_url: "",
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
    image_url: p.image_url ?? "",
    pack_defs: p.pack_defs ?? [],
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
  const [uploading, setUploading] = useState(false);
  const [pending, startTransition] = useTransition();
  const fileRef = useRef<HTMLInputElement>(null);
  const isEdit = Boolean(produk);

  function set<K extends keyof ProdukInput>(key: K, value: ProdukInput[K]) {
    setForm((f) => ({ ...f, [key]: value }));
  }

  async function onPickImage(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = ""; // allow re-picking the same file
    if (!file) return;
    setError(null);
    setUploading(true);
    try {
      set("image_url", await fileToCompressedDataUrl(file, { maxDim: 600, quality: 0.72 }));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal memproses gambar.");
    } finally {
      setUploading(false);
    }
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

      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
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

      <div className="space-y-1.5">
        <Label htmlFor="image_url">Gambar Produk</Label>
        <div className="flex items-start gap-3">
          {form.image_url.trim() ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={form.image_url}
              alt="Pratinjau"
              className="size-16 shrink-0 rounded-lg border object-cover"
            />
          ) : (
            <div className="flex size-16 shrink-0 items-center justify-center rounded-lg border bg-muted/40 text-muted-foreground">
              <Upload className="size-5" />
            </div>
          )}
          <div className="flex-1 space-y-2">
            <input
              ref={fileRef}
              type="file"
              accept="image/*"
              className="hidden"
              onChange={onPickImage}
            />
            <div className="flex gap-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => fileRef.current?.click()}
                disabled={uploading}
              >
                {uploading ? <Loader2 className="size-4 animate-spin" /> : <Upload className="size-4" />}
                {form.image_url.trim() ? "Ganti gambar" : "Unggah gambar"}
              </Button>
              {form.image_url.trim() && (
                <Button type="button" variant="ghost" size="sm" onClick={() => set("image_url", "")}>
                  Hapus
                </Button>
              )}
            </div>
            <Input
              id="image_url"
              value={form.image_url.startsWith("data:") ? "" : form.image_url}
              onChange={(e) => set("image_url", e.target.value)}
              placeholder="atau tempel URL gambar…"
              inputMode="url"
              className="font-mono"
              disabled={form.image_url.startsWith("data:")}
            />
          </div>
        </div>
        <p className="text-xs text-muted-foreground">
          Unggah foto dari HP (otomatis dikecilkan) atau tempel tautan gambar. Kosongkan bila tidak ada.
        </p>
      </div>

      {/* Kemasan Restock */}
      <div className="space-y-2">
        <Label>Kemasan Restock (opsional)</Label>
        {(form.pack_defs ?? []).map((k, i) => (
          <div key={i} className="flex gap-2 items-center">
            <Input
              value={k.nama}
              placeholder="dos / lusin / box"
              onChange={(e) =>
                setForm((f) => {
                  const defs = [...(f.pack_defs ?? [])];
                  defs[i] = { ...defs[i], nama: e.target.value };
                  return { ...f, pack_defs: defs };
                })
              }
            />
            <Input
              type="number"
              min={1}
              value={k.isi || ""}
              placeholder="isi"
              className="w-24"
              onChange={(e) =>
                setForm((f) => {
                  const defs = [...(f.pack_defs ?? [])];
                  defs[i] = { ...defs[i], isi: Number(e.target.value) };
                  return { ...f, pack_defs: defs };
                })
              }
            />
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() =>
                setForm((f) => ({
                  ...f,
                  pack_defs: (f.pack_defs ?? []).filter((_, j) => j !== i),
                }))
              }
            >
              Hapus
            </Button>
          </div>
        ))}
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() =>
            setForm((f) => ({
              ...f,
              pack_defs: [...(f.pack_defs ?? []), { nama: "", isi: 0 }],
            }))
          }
        >
          + Tambah Kemasan
        </Button>
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
