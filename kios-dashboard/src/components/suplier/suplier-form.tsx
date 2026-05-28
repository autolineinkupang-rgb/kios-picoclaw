"use client";

import { useState, useTransition } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { createSuplierAction, updateSuplierAction } from "@/app/(app)/suplier/actions";
import type { Supplier } from "@/lib/types";

type ActionResult = { ok: true } | { ok: false; error: string };

function emptyForm(): Partial<Supplier> {
  return {
    nama: "",
    kontak: "",
    alamat: "",
    pic: "",
    produk_utama: "",
    catatan: "",
  };
}

function fromSuplier(s: Supplier): Partial<Supplier> {
  return {
    id: s.id,
    nama: s.nama,
    kontak: s.kontak,
    alamat: s.alamat,
    pic: s.pic,
    produk_utama: s.produk_utama,
    catatan: s.catatan,
  };
}

export function SuplierForm({
  suplier,
  onResult,
  onCancel,
}: {
  suplier?: Supplier;
  onResult: (r: ActionResult) => void;
  onCancel: () => void;
}) {
  const [form, setForm] = useState<Partial<Supplier>>(
    suplier ? fromSuplier(suplier) : emptyForm(),
  );
  const [error, setError] = useState<string | null>(null);
  const [pending, startTransition] = useTransition();
  const isEdit = Boolean(suplier);

  function set<K extends keyof Supplier>(key: K, value: Supplier[K]) {
    setForm((f) => ({ ...f, [key]: value }));
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    if (!form.nama?.trim()) {
      setError("Nama supplier wajib diisi.");
      return;
    }
    startTransition(async () => {
      const result = isEdit
        ? await updateSuplierAction(form)
        : await createSuplierAction(form);
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
          Nama Supplier <span className="text-destructive">*</span>
        </Label>
        <Input
          id="nama"
          value={form.nama ?? ""}
          onChange={(e) => set("nama", e.target.value)}
          placeholder="mis. CV Maju Jaya"
          required
          autoFocus
        />
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-1.5">
          <Label htmlFor="kontak">Kontak</Label>
          <Input
            id="kontak"
            value={form.kontak ?? ""}
            onChange={(e) => set("kontak", e.target.value)}
            placeholder="mis. 0812-3456-7890"
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="pic">PIC (Penanggung Jawab)</Label>
          <Input
            id="pic"
            value={form.pic ?? ""}
            onChange={(e) => set("pic", e.target.value)}
            placeholder="mis. Pak Budi"
          />
        </div>
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="alamat">Alamat</Label>
        <Input
          id="alamat"
          value={form.alamat ?? ""}
          onChange={(e) => set("alamat", e.target.value)}
          placeholder="mis. Jl. Pasar Baru No. 12"
        />
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="produk_utama">Produk Utama</Label>
        <Input
          id="produk_utama"
          value={form.produk_utama ?? ""}
          onChange={(e) => set("produk_utama", e.target.value)}
          placeholder="mis. Beras, Gula, Minyak"
        />
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="catatan">Catatan</Label>
        <Input
          id="catatan"
          value={form.catatan ?? ""}
          onChange={(e) => set("catatan", e.target.value)}
          placeholder="opsional"
        />
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
          {isEdit ? "Simpan Perubahan" : "Tambah Supplier"}
        </Button>
      </div>
    </form>
  );
}
