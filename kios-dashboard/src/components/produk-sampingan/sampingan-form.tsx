"use client";

import { useRef, useState, useTransition } from "react";
import { Loader2, Upload } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { fileToCompressedDataUrl } from "@/lib/image-upload";
import {
  createSampinganAction,
  updateSampinganAction,
  type ActionResult,
  type JenisSampingan,
  type SampinganInput,
} from "@/app/(app)/produk-sampingan/actions";
import type { Produk } from "@/lib/types";

const JENIS_OPTIONS: { value: JenisSampingan; label: string }[] = [
  { value: "pulsa", label: "Pulsa" },
  { value: "pertalite", label: "Pertalite" },
  { value: "pertamax", label: "Pertamax" },
  { value: "solar", label: "Solar" },
  { value: "minyak_tanah", label: "Minyak Tanah" },
];

function emptyForm(jenis: JenisSampingan): SampinganInput {
  return {
    jenis,
    nama: "",
    barcode: "",
    kategori: "",
    satuan: jenis === "pulsa" ? "paket" : "liter",
    stok: 0,
    harga_beli: 0,
    harga_jual: 0,
    stok_minimum: 5,
    stok_kritis: 2,
    supplier: "",
    exp_date: "",
    image_url: "",
    saldo_modal: 0,
    stok_ml: 0,
    stok_kritis_ml: 40000,
  };
}

function fromProduk(p: Produk): SampinganInput {
  return {
    id: p.id,
    jenis: (p.jenis as JenisSampingan) ?? "pulsa",
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
    saldo_modal: p.saldo_modal ?? 0,
    stok_ml: p.stok_ml ?? 0,
    stok_kritis_ml: p.stok_kritis_ml ?? 40000,
  };
}

export function SampinganForm({
  produk,
  onResult,
  onCancel,
}: {
  produk?: Produk;
  onResult: (r: ActionResult) => void;
  onCancel: () => void;
}) {
  const [form, setForm] = useState<SampinganInput>(
    produk ? fromProduk(produk) : emptyForm("pulsa"),
  );
  const [error, setError] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);
  const [pending, startTransition] = useTransition();
  const fileRef = useRef<HTMLInputElement>(null);
  const isEdit = Boolean(produk);

  function set<K extends keyof SampinganInput>(key: K, value: SampinganInput[K]) {
    setForm((f) => ({ ...f, [key]: value }));
  }

  async function onPickImage(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = "";
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

  function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    if (!form.nama.trim()) {
      setError("Nama produk wajib diisi.");
      return;
    }
    startTransition(async () => {
      const result = isEdit
        ? await updateSampinganAction(form)
        : await createSampinganAction(form);
      if (!result.ok) {
        setError(result.error);
        return;
      }
      onResult(result);
    });
  }

  return (
    <form onSubmit={submit} className="space-y-4">
      {/* Jenis — locked on edit */}
      <div className="space-y-1.5">
        <Label htmlFor="jenis">
          Jenis Produk <span className="text-destructive">*</span>
        </Label>
        <Select
          id="jenis"
          value={form.jenis}
          onChange={(e) => !isEdit && set("jenis", e.target.value as JenisSampingan)}
          disabled={isEdit}
        >
          {JENIS_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </Select>
        {isEdit && (
          <p className="text-xs text-muted-foreground">Jenis tidak bisa diubah setelah disimpan.</p>
        )}
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="nama">
          Nama Produk <span className="text-destructive">*</span>
        </Label>
        <Input
          id="nama"
          value={form.nama}
          onChange={(e) => set("nama", e.target.value)}
          placeholder={
            form.jenis === "pulsa"
              ? "mis. Pulsa Telkomsel 25rb"
              : form.jenis === "pertalite"
                ? "mis. Pertalite 1 Liter"
                : form.jenis === "pertamax"
                  ? "mis. Pertamax 1 Liter"
                  : form.jenis === "solar"
                    ? "mis. Solar Bio"
                    : "mis. Minyak Tanah"
          }
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
            placeholder={form.jenis}
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="satuan">Satuan</Label>
          <Input
            id="satuan"
            value={form.satuan}
            onChange={(e) => set("satuan", e.target.value)}
            placeholder={form.jenis === "pulsa" ? "paket" : "liter"}
          />
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-1.5">
          <Label htmlFor="harga_beli">Harga Beli / Modal (Rp)</Label>
          <Input
            id="harga_beli"
            type="number"
            inputMode="numeric"
            min={0}
            value={String(form.harga_beli)}
            onChange={(e) => set("harga_beli", Number(e.target.value))}
            className="font-mono tabular-nums"
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="harga_jual">Harga Jual (Rp)</Label>
          <Input
            id="harga_jual"
            type="number"
            inputMode="numeric"
            min={0}
            value={String(form.harga_jual)}
            onChange={(e) => set("harga_jual", Number(e.target.value))}
            className="font-mono tabular-nums"
          />
        </div>
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="barcode">Barcode (opsional)</Label>
        <Input
          id="barcode"
          value={form.barcode}
          onChange={(e) => set("barcode", e.target.value)}
          placeholder="opsional"
          className="font-mono"
        />
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="image_url">Gambar Produk</Label>
        <div className="flex items-start gap-3">
          {form.image_url.trim() ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={form.image_url} alt="Pratinjau" className="size-16 shrink-0 rounded-lg border object-cover" />
          ) : (
            <div className="flex size-16 shrink-0 items-center justify-center rounded-lg border bg-muted/40 text-muted-foreground">
              <Upload className="size-5" />
            </div>
          )}
          <div className="flex-1 space-y-2">
            <input ref={fileRef} type="file" accept="image/*" className="hidden" onChange={onPickImage} />
            <div className="flex gap-2">
              <Button type="button" variant="outline" size="sm" onClick={() => fileRef.current?.click()} disabled={uploading}>
                {uploading ? <Loader2 className="size-4 animate-spin" /> : <Upload className="size-4" />}
                {form.image_url.trim() ? "Ganti gambar" : "Unggah gambar"}
              </Button>
              {form.image_url.trim() && (
                <Button type="button" variant="ghost" size="sm" onClick={() => set("image_url", "")}>Hapus</Button>
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
      </div>

      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}

      <div className="flex justify-end gap-2 pt-2">
        <Button type="button" variant="outline" size="sm" onClick={onCancel} disabled={pending}>Batal</Button>
        <Button type="submit" variant="accent" size="sm" disabled={pending}>
          {pending && <Loader2 className="size-4 animate-spin" />}
          {isEdit ? "Simpan Perubahan" : "Tambah Produk"}
        </Button>
      </div>
    </form>
  );
}
