"use client";

import { useMemo, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import {
  CheckCircle2,
  Loader2,
  Pencil,
  Plus,
  Search,
  Trash2,
  Truck,
  TriangleAlert,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Modal } from "@/components/ui/modal";
import { EmptyState } from "@/components/ui/empty-state";
import { SuplierForm } from "./suplier-form";
import { deleteSuplierAction } from "@/app/(app)/suplier/actions";
import { cn } from "@/lib/utils";
import type { Supplier } from "@/lib/types";

type ActionResult = { ok: true } | { ok: false; error: string };

export function SuplierTable({
  suplier,
  canManage,
  canDelete,
}: {
  suplier: Supplier[];
  canManage: boolean;
  canDelete: boolean;
}) {
  const router = useRouter();
  const [query, setQuery] = useState("");

  const [dialog, setDialog] = useState<{ mode: "add" | "edit"; suplier?: Supplier } | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Supplier | null>(null);
  const [toast, setToast] = useState<ActionResult | null>(null);
  const [pendingDelete, startDelete] = useTransition();

  const rows = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return suplier;
    return suplier.filter(
      (s) =>
        (s.nama ?? "").toLowerCase().includes(q) ||
        (s.kontak ?? "").toLowerCase().includes(q) ||
        (s.pic ?? "").toLowerCase().includes(q) ||
        (s.produk_utama ?? "").toLowerCase().includes(q) ||
        (s.id ?? "").toLowerCase().includes(q),
    );
  }, [suplier, query]);

  function showToast(r: ActionResult) {
    setToast(r);
    window.setTimeout(() => setToast(null), 4000);
  }

  function onMutated(r: ActionResult) {
    setDialog(null);
    showToast(r);
    router.refresh();
  }

  function runDelete() {
    if (!deleteTarget) return;
    const target = deleteTarget;
    startDelete(async () => {
      const r = await deleteSuplierAction(target.id);
      setDeleteTarget(null);
      showToast(r);
      if (r.ok) router.refresh();
    });
  }

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Cari nama, kontak, atau PIC…"
            className="pl-9"
            aria-label="Cari supplier"
          />
        </div>
        {canManage && (
          <Button variant="accent" size="md" onClick={() => setDialog({ mode: "add" })}>
            <Plus className="size-4" />
            Tambah
          </Button>
        )}
      </div>

      <p className="text-xs text-muted-foreground">
        Menampilkan {rows.length} dari {suplier.length} supplier
      </p>

      {rows.length === 0 ? (
        <EmptyState
          icon={Truck}
          title={suplier.length === 0 ? "Belum ada supplier" : "Tidak ada hasil"}
          description={
            suplier.length === 0
              ? "Tambahkan supplier pertama untuk mencatat mitra pengiriman barang."
              : "Coba ubah kata kunci pencarian."
          }
          action={
            canManage && suplier.length === 0 ? (
              <Button variant="accent" size="sm" onClick={() => setDialog({ mode: "add" })}>
                <Plus className="size-4" /> Tambah Supplier
              </Button>
            ) : undefined
          }
        />
      ) : (
        <div className="overflow-x-auto rounded-xl border bg-card">
          <table className="w-full min-w-[700px] text-sm">
            <thead>
              <tr className="border-b text-left text-xs text-muted-foreground">
                <th className="p-3 font-medium">Nama</th>
                <th className="p-3 font-medium">Kontak</th>
                <th className="p-3 font-medium">Alamat</th>
                <th className="p-3 font-medium">PIC</th>
                <th className="p-3 font-medium">Produk Utama</th>
                <th className="p-3 font-medium">Catatan</th>
                {(canManage || canDelete) && (
                  <th className="p-3 text-right font-medium">Aksi</th>
                )}
              </tr>
            </thead>
            <tbody>
              {rows.map((s) => (
                <tr
                  key={s.id}
                  className="border-b transition-colors last:border-0 hover:bg-muted/40"
                >
                  <td className="p-3">
                    <p className="font-medium">{s.nama}</p>
                    <p className="font-mono text-xs text-muted-foreground">{s.id}</p>
                  </td>
                  <td className="p-3 text-muted-foreground">{s.kontak || "–"}</td>
                  <td className="p-3 text-muted-foreground">{s.alamat || "–"}</td>
                  <td className="p-3 text-muted-foreground">{s.pic || "–"}</td>
                  <td className="p-3 text-muted-foreground">{s.produk_utama || "–"}</td>
                  <td className="p-3 text-muted-foreground">{s.catatan || "–"}</td>
                  {(canManage || canDelete) && (
                    <td className="p-3">
                      <div className="flex justify-end gap-1">
                        {canManage && (
                          <button
                            type="button"
                            onClick={() => setDialog({ mode: "edit", suplier: s })}
                            aria-label={`Edit ${s.nama}`}
                            className="flex size-8 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
                          >
                            <Pencil className="size-4" />
                          </button>
                        )}
                        {canDelete && (
                          <button
                            type="button"
                            onClick={() => setDeleteTarget(s)}
                            aria-label={`Hapus ${s.nama}`}
                            className="flex size-8 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                          >
                            <Trash2 className="size-4" />
                          </button>
                        )}
                      </div>
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Add / edit dialog */}
      <Modal
        open={dialog !== null}
        onClose={() => setDialog(null)}
        title={dialog?.mode === "edit" ? "Edit Supplier" : "Tambah Supplier"}
        description={
          dialog?.mode === "edit" ? dialog.suplier?.nama : "Lengkapi detail supplier baru."
        }
      >
        {dialog && (
          <SuplierForm
            suplier={dialog.suplier}
            onResult={onMutated}
            onCancel={() => setDialog(null)}
          />
        )}
      </Modal>

      {/* Delete confirmation */}
      <Modal
        open={deleteTarget !== null}
        onClose={() => !pendingDelete && setDeleteTarget(null)}
        title="Hapus supplier?"
        description="Tindakan ini tidak bisa dibatalkan."
        className="max-w-md"
      >
        <div className="space-y-4">
          <p className="text-sm">
            Yakin mau menghapus supplier{" "}
            <span className="font-medium">{deleteTarget?.nama}</span> ({deleteTarget?.id})?
          </p>
          <div className="flex justify-end gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setDeleteTarget(null)}
              disabled={pendingDelete}
            >
              Batal
            </Button>
            <Button variant="destructive" size="sm" onClick={runDelete} disabled={pendingDelete}>
              {pendingDelete && <Loader2 className="size-4 animate-spin" />}
              Hapus
            </Button>
          </div>
        </div>
      </Modal>

      {/* Toast */}
      {toast && (
        <div
          role="status"
          aria-live="polite"
          className={cn(
            "fixed bottom-4 left-1/2 z-[60] flex -translate-x-1/2 items-center gap-2 rounded-lg border px-4 py-2.5 text-sm shadow-lg",
            toast.ok
              ? "border-success/30 bg-card text-foreground"
              : "border-destructive/30 bg-card text-destructive",
          )}
        >
          {toast.ok ? (
            <CheckCircle2 className="size-4 text-success" />
          ) : (
            <TriangleAlert className="size-4" />
          )}
          {toast.ok ? "Berhasil disimpan." : toast.error}
        </div>
      )}
    </div>
  );
}
