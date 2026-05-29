"use client";

import { useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { CheckCircle2, Loader2, Plus, TriangleAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Modal } from "@/components/ui/modal";
import { EmptyState } from "@/components/ui/empty-state";
import { formatRupiah } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { Produk, Promo } from "@/lib/types";
import {
  togglePromoAction,
  deletePromoAction,
  type PromoResult,
} from "@/app/(app)/promo/actions";
import { PromoForm } from "./promo-form";

type PromoStatus = "aktif" | "menunggu" | "nonaktif" | "kedaluwarsa";

function getPromoStatus(p: Promo): PromoStatus {
  const today = new Date().toISOString().slice(0, 10);
  if (p.aktif && p.selesai < today) return "kedaluwarsa";
  if (p.aktif) return "aktif";
  if (p.selesai >= today) return "menunggu";
  return "nonaktif";
}

const STATUS_PROMO: Record<PromoStatus, { label: string; variant: "default" | "destructive" | "outline" | "secondary" }> = {
  aktif: { label: "Aktif", variant: "default" },
  menunggu: { label: "Menunggu", variant: "secondary" },
  nonaktif: { label: "Nonaktif", variant: "outline" },
  kedaluwarsa: { label: "Kedaluwarsa", variant: "destructive" },
};

function formatNilai(p: Promo): string {
  return p.tipe === "persen" ? `${p.nilai}%` : formatRupiah(p.nilai);
}

interface Props {
  promo: Promo[];
  produk: Produk[];
  isOwner: boolean;
}

export function PromoTable({ promo, produk, isOwner }: Props) {
  const router = useRouter();
  const [pending, start] = useTransition();
  const [showForm, setShowForm] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Promo | null>(null);
  const [toast, setToast] = useState<PromoResult | null>(null);

  function showToast(r: PromoResult) {
    setToast(r);
    window.setTimeout(() => setToast(null), 4000);
  }

  function onCreated(r: PromoResult) {
    setShowForm(false);
    showToast(r);
    router.refresh();
  }

  function handleToggle(id: string) {
    start(async () => {
      const r = await togglePromoAction(id);
      showToast(r);
      if (r.ok) router.refresh();
    });
  }

  function handleDelete() {
    if (!deleteTarget) return;
    const id = deleteTarget.id;
    start(async () => {
      const r = await deletePromoAction(id);
      setDeleteTarget(null);
      showToast(r);
      if (r.ok) router.refresh();
    });
  }

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex items-center justify-between">
        <p className="text-xs text-muted-foreground">{promo.length} promo terdaftar</p>
        <Button variant="accent" size="md" onClick={() => setShowForm(true)}>
          <Plus className="size-4" /> Buat Promo
        </Button>
      </div>

      {/* Toast */}
      {toast && (
        <p
          className={cn(
            "flex items-center gap-1.5 text-sm",
            toast.ok ? "text-success" : "text-destructive",
          )}
        >
          {toast.ok ? <CheckCircle2 className="size-4" /> : <TriangleAlert className="size-4" />}
          {toast.ok ? "Berhasil." : toast.error}
        </p>
      )}

      {promo.length === 0 ? (
        <EmptyState
          icon={CheckCircle2}
          title="Belum ada promo"
          description="Buat promo diskon untuk produk kios."
          action={
            <Button variant="accent" size="sm" onClick={() => setShowForm(true)}>
              <Plus className="size-4" /> Buat Promo
            </Button>
          }
        />
      ) : (
        <div className="overflow-x-auto rounded-xl border bg-card">
          <table className="w-full min-w-[700px] text-sm">
            <thead>
              <tr className="border-b text-left text-xs text-muted-foreground">
                <th className="p-3 font-medium">Produk</th>
                <th className="p-3 font-medium">Diskon</th>
                <th className="p-3 font-medium">Min Qty</th>
                <th className="p-3 font-medium">Periode</th>
                <th className="p-3 font-medium">Status</th>
                <th className="p-3 font-medium">Catatan</th>
                {isOwner && <th className="p-3 text-right font-medium">Aksi</th>}
              </tr>
            </thead>
            <tbody>
              {promo.map((p) => {
                const st = getPromoStatus(p);
                const meta = STATUS_PROMO[st];
                return (
                  <tr key={p.id} className="border-b last:border-0 hover:bg-muted/40">
                    <td className="p-3">
                      <p className="font-medium">{p.produk}</p>
                      <p className="font-mono text-xs text-muted-foreground">{p.id}</p>
                    </td>
                    <td className="p-3 font-mono font-medium tabular-nums">
                      {formatNilai(p)}
                    </td>
                    <td className="p-3 text-muted-foreground">{p.min_qty}x</td>
                    <td className="p-3 text-xs text-muted-foreground">
                      {p.mulai} – {p.selesai}
                    </td>
                    <td className="p-3">
                      <Badge variant={meta.variant}>{meta.label}</Badge>
                    </td>
                    <td className="p-3 text-xs text-muted-foreground">
                      {p.catatan || "–"}
                    </td>
                    {isOwner && (
                      <td className="p-3">
                        <div className="flex justify-end gap-1">
                          <button
                            type="button"
                            onClick={() => handleToggle(p.id)}
                            disabled={pending}
                            className="rounded-md px-2 py-1 text-xs font-medium text-muted-foreground hover:bg-muted hover:text-foreground disabled:opacity-50"
                          >
                            {p.aktif ? "Nonaktifkan" : "Aktifkan"}
                          </button>
                          <button
                            type="button"
                            onClick={() => setDeleteTarget(p)}
                            disabled={pending}
                            className="rounded-md px-2 py-1 text-xs font-medium text-muted-foreground hover:bg-destructive/10 hover:text-destructive disabled:opacity-50"
                          >
                            Hapus
                          </button>
                        </div>
                      </td>
                    )}
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Form buat promo */}
      <Modal
        open={showForm}
        onClose={() => setShowForm(false)}
        title="Buat Promo Baru"
        description="Kasir: tersimpan nonaktif, menunggu aktivasi owner. Owner: langsung aktif."
      >
        <PromoForm produk={produk} onResult={onCreated} onCancel={() => setShowForm(false)} />
      </Modal>

      {/* Konfirmasi hapus */}
      <Modal
        open={deleteTarget !== null}
        onClose={() => !pending && setDeleteTarget(null)}
        title="Hapus promo?"
        description="Tindakan ini tidak bisa dibatalkan."
        className="max-w-md"
      >
        <div className="space-y-4">
          <p className="text-sm">
            Yakin mau menghapus promo{" "}
            <span className="font-medium">{deleteTarget?.id}</span> untuk{" "}
            <span className="font-medium">{deleteTarget?.produk}</span>?
          </p>
          <div className="flex justify-end gap-2">
            <Button variant="outline" size="md" onClick={() => setDeleteTarget(null)} disabled={pending}>
              Batal
            </Button>
            <Button variant="destructive" size="md" onClick={handleDelete} disabled={pending}>
              {pending ? <Loader2 className="size-4 animate-spin" /> : null}
              Hapus
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
