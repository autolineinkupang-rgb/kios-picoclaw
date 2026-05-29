"use client";

import { useState, useTransition } from "react";
import { CheckCircle2, Loader2, TriangleAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import type { Produk } from "@/lib/types";
import { createPromoAction, type PromoResult } from "@/app/(app)/promo/actions";

interface Props {
  produk: Produk[];
  onResult: (r: PromoResult) => void;
  onCancel: () => void;
}

export function PromoForm({ produk, onResult, onCancel }: Props) {
  const [pending, start] = useTransition();
  const [toast, setToast] = useState<PromoResult | null>(null);

  const today = new Date().toISOString().slice(0, 10);

  const [produkId, setProdukId] = useState("");
  const [tipe, setTipe] = useState<"persen" | "fixed">("persen");
  const [nilai, setNilai] = useState("");
  const [minQty, setMinQty] = useState("1");
  const [mulai, setMulai] = useState(today);
  const [selesai, setSelesai] = useState(today);
  const [catatan, setCatatan] = useState("");

  const selectedProduk = produk.find((p) => p.id === produkId);

  function handleSubmit() {
    start(async () => {
      const r = await createPromoAction({
        produk_id: produkId,
        produk: selectedProduk?.nama ?? "",
        tipe,
        nilai: parseFloat(nilai) || 0,
        min_qty: parseInt(minQty) || 1,
        mulai,
        selesai,
        catatan,
      });
      if (!r.ok) {
        setToast(r);
        window.setTimeout(() => setToast(null), 4000);
      } else {
        onResult(r);
      }
    });
  }

  return (
    <div className="space-y-4">
      <div className="space-y-1.5">
        <Label htmlFor="promo-produk">Produk</Label>
        <Select
          id="promo-produk"
          value={produkId}
          onChange={(e) => setProdukId(e.target.value)}
        >
          <option value="">— Pilih produk —</option>
          {produk.map((p) => (
            <option key={p.id} value={p.id}>
              {p.nama}
            </option>
          ))}
        </Select>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label htmlFor="promo-tipe">Tipe Diskon</Label>
          <Select
            id="promo-tipe"
            value={tipe}
            onChange={(e) => setTipe(e.target.value as "persen" | "fixed")}
          >
            <option value="persen">Persen (%)</option>
            <option value="fixed">Nominal (Rp)</option>
          </Select>
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="promo-nilai">
            Nilai {tipe === "persen" ? "(%)" : "(Rp)"}
          </Label>
          <Input
            id="promo-nilai"
            value={nilai}
            onChange={(e) => setNilai(e.target.value)}
            placeholder={tipe === "persen" ? "mis. 10" : "mis. 5000"}
            inputMode="decimal"
            className="font-mono"
          />
        </div>
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="promo-minqty">Min Qty (agar promo berlaku)</Label>
        <Input
          id="promo-minqty"
          value={minQty}
          onChange={(e) => setMinQty(e.target.value)}
          placeholder="1"
          inputMode="numeric"
          className="max-w-[100px] font-mono"
        />
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label htmlFor="promo-mulai">Mulai</Label>
          <Input
            id="promo-mulai"
            type="date"
            value={mulai}
            onChange={(e) => setMulai(e.target.value)}
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="promo-selesai">Selesai</Label>
          <Input
            id="promo-selesai"
            type="date"
            value={selesai}
            onChange={(e) => setSelesai(e.target.value)}
          />
        </div>
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="promo-catatan">Catatan (opsional)</Label>
        <Input
          id="promo-catatan"
          value={catatan}
          onChange={(e) => setCatatan(e.target.value)}
          placeholder="mis. Promo hari raya"
        />
      </div>

      {toast && !toast.ok && (
        <p className="flex items-center gap-1.5 text-sm text-destructive">
          <TriangleAlert className="size-4" />
          {toast.error}
        </p>
      )}

      <div className="flex justify-end gap-2 pt-2">
        <Button type="button" variant="outline" size="md" onClick={onCancel} disabled={pending}>
          Batal
        </Button>
        <Button variant="accent" size="md" onClick={handleSubmit} disabled={pending || !produkId}>
          {pending ? <Loader2 className="size-4 animate-spin" /> : <CheckCircle2 className="size-4" />}
          Simpan Promo
        </Button>
      </div>
    </div>
  );
}
