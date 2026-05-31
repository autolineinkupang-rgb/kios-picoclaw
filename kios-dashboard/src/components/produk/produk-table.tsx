"use client";

import { useMemo, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import {
  ArrowUpDown,
  CheckCircle2,
  Loader2,
  Package,
  Pencil,
  Plus,
  Search,
  Trash2,
  TriangleAlert,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input, Select } from "@/components/ui/input";
import { Modal } from "@/components/ui/modal";
import { EmptyState } from "@/components/ui/empty-state";
import { ProdukForm } from "./produk-form";
import { deleteProdukAction, type ActionResult } from "@/app/(app)/produk/actions";
import { formatRupiah, formatTanggal } from "@/lib/format";
import { produkImage } from "@/lib/produk-image";
import { STATUS_META, marginPct, stokStatus, type StokStatus } from "@/lib/produk-status";
import { cn } from "@/lib/utils";
import type { Produk } from "@/lib/types";

type SortKey = "nama" | "stok" | "harga_jual";

export function ProdukTable({
  produk,
  canManage,
}: {
  produk: Produk[];
  canManage: boolean;
}) {
  const router = useRouter();
  const [query, setQuery] = useState("");
  const [kategori, setKategori] = useState("");
  const [status, setStatus] = useState<StokStatus | "">("");
  const [sortKey, setSortKey] = useState<SortKey>("nama");
  const [sortAsc, setSortAsc] = useState(true);

  const [dialog, setDialog] = useState<{ mode: "add" | "edit"; produk?: Produk } | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Produk | null>(null);
  const [toast, setToast] = useState<ActionResult | null>(null);
  const [pendingDelete, startDelete] = useTransition();

  const categories = useMemo(
    () => [...new Set(produk.map((p) => p.kategori).filter(Boolean))].sort(),
    [produk],
  );

  const rows = useMemo(() => {
    const q = query.trim().toLowerCase();
    let list = produk.filter((p) => {
      if (kategori && p.kategori !== kategori) return false;
      if (status && stokStatus(p) !== status) return false;
      if (!q) return true;
      return (
        (p.nama ?? "").toLowerCase().includes(q) ||
        (p.id ?? "").toLowerCase().includes(q) ||
        (p.barcode ?? "").toLowerCase().includes(q)
      );
    });
    list = [...list].sort((a, b) => {
      let cmp = 0;
      if (sortKey === "nama") cmp = a.nama.localeCompare(b.nama);
      else cmp = (a[sortKey] as number) - (b[sortKey] as number);
      return sortAsc ? cmp : -cmp;
    });
    return list;
  }, [produk, query, kategori, status, sortKey, sortAsc]);

  function showToast(r: ActionResult) {
    setToast(r);
    window.setTimeout(() => setToast(null), 4000);
  }

  function onMutated(r: ActionResult) {
    setDialog(null);
    showToast(r);
    router.refresh();
  }

  function toggleSort(key: SortKey) {
    if (sortKey === key) setSortAsc((v) => !v);
    else {
      setSortKey(key);
      setSortAsc(true);
    }
  }

  function runDelete() {
    if (!deleteTarget) return;
    const target = deleteTarget;
    startDelete(async () => {
      const r = await deleteProdukAction(target.id);
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
            placeholder="Cari nama, ID, atau barcode…"
            className="pl-9"
            aria-label="Cari produk"
          />
        </div>
        <Select
          value={kategori}
          onChange={(e) => setKategori(e.target.value)}
          aria-label="Filter kategori"
          className="sm:w-40"
        >
          <option value="">Semua kategori</option>
          {categories.map((c) => (
            <option key={c} value={c}>
              {c}
            </option>
          ))}
        </Select>
        <Select
          value={status}
          onChange={(e) => setStatus(e.target.value as StokStatus | "")}
          aria-label="Filter status stok"
          className="sm:w-36"
        >
          <option value="">Semua status</option>
          <option value="aman">Aman</option>
          <option value="menipis">Menipis</option>
          <option value="kritis">Kritis</option>
          <option value="habis">Habis</option>
        </Select>
        {canManage && (
          <Button variant="accent" size="md" onClick={() => setDialog({ mode: "add" })}>
            <Plus className="size-4" />
            Tambah
          </Button>
        )}
      </div>

      <p className="text-xs text-muted-foreground">
        Menampilkan {rows.length} dari {produk.length} produk
      </p>

      {rows.length === 0 ? (
        <EmptyState
          icon={Package}
          title={produk.length === 0 ? "Belum ada produk" : "Tidak ada hasil"}
          description={
            produk.length === 0
              ? "Tambahkan produk pertama, atau biarkan bot Telegram mengisinya."
              : "Coba ubah kata kunci atau filter."
          }
          action={
            canManage && produk.length === 0 ? (
              <Button variant="accent" size="sm" onClick={() => setDialog({ mode: "add" })}>
                <Plus className="size-4" /> Tambah Produk
              </Button>
            ) : undefined
          }
        />
      ) : (
        <div className="overflow-x-auto rounded-xl border bg-card">
          <table className="w-full min-w-[760px] text-sm">
            <thead>
              <tr className="border-b text-left text-xs text-muted-foreground">
                <th className="p-3 font-medium">
                  <SortButton label="Produk" active={sortKey === "nama"} asc={sortAsc} onClick={() => toggleSort("nama")} />
                </th>
                <th className="p-3 font-medium">
                  <SortButton label="Stok" active={sortKey === "stok"} asc={sortAsc} onClick={() => toggleSort("stok")} />
                </th>
                <th className="p-3 font-medium">Status</th>
                <th className="p-3 text-right font-medium">Harga Beli</th>
                <th className="p-3 text-right font-medium">
                  <SortButton
                    label="Harga Jual"
                    active={sortKey === "harga_jual"}
                    asc={sortAsc}
                    onClick={() => toggleSort("harga_jual")}
                    align="right"
                  />
                </th>
                <th className="p-3 text-right font-medium">Margin</th>
                <th className="p-3 font-medium">Update</th>
                {canManage && <th className="p-3 text-right font-medium">Aksi</th>}
              </tr>
            </thead>
            <tbody>
              {rows.map((p) => {
                const st = STATUS_META[stokStatus(p)];
                const margin = marginPct(p);
                return (
                  <tr key={p.id} className="border-b transition-colors last:border-0 hover:bg-muted/40">
                    <td className="p-3">
                      <div className="flex items-center gap-3">
                        {/* eslint-disable-next-line @next/next/no-img-element */}
                        <img
                          src={produkImage(p)}
                          alt={p.nama}
                          loading="lazy"
                          className="size-10 shrink-0 rounded-md border bg-muted/40 object-cover"
                        />
                        <div className="min-w-0">
                          <p className="font-medium">{p.nama}</p>
                          <p className="font-mono text-xs text-muted-foreground">
                            {p.id}
                            {p.kategori ? ` · ${p.kategori}` : ""}
                          </p>
                        </div>
                      </div>
                    </td>
                    <td className="p-3 font-mono tabular-nums">
                      {p.stok} <span className="text-xs text-muted-foreground">{p.satuan}</span>
                    </td>
                    <td className="p-3">
                      <Badge variant={st.variant}>{st.label}</Badge>
                    </td>
                    <td className="p-3 text-right font-mono tabular-nums text-muted-foreground">
                      {formatRupiah(p.harga_beli)}
                    </td>
                    <td className="p-3 text-right font-mono tabular-nums">
                      {formatRupiah(p.harga_jual)}
                    </td>
                    <td className="p-3 text-right font-mono tabular-nums text-muted-foreground">
                      {margin === null ? "–" : `${margin}%`}
                    </td>
                    <td className="p-3 text-xs text-muted-foreground">
                      {formatTanggal(p.last_update)}
                    </td>
                    {canManage && (
                      <td className="p-3">
                        <div className="flex justify-end gap-1">
                          <button
                            type="button"
                            onClick={() => setDialog({ mode: "edit", produk: p })}
                            aria-label={`Edit ${p.nama}`}
                            className="flex size-8 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
                          >
                            <Pencil className="size-4" />
                          </button>
                          <button
                            type="button"
                            onClick={() => setDeleteTarget(p)}
                            aria-label={`Hapus ${p.nama}`}
                            className="flex size-8 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                          >
                            <Trash2 className="size-4" />
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

      {/* Add / edit dialog */}
      <Modal
        open={dialog !== null}
        onClose={() => setDialog(null)}
        title={dialog?.mode === "edit" ? "Edit Produk" : "Tambah Produk"}
        description={dialog?.mode === "edit" ? dialog.produk?.nama : "Lengkapi detail produk baru."}
      >
        {dialog && (
          <ProdukForm produk={dialog.produk} onResult={onMutated} onCancel={() => setDialog(null)} />
        )}
      </Modal>

      {/* Delete confirmation */}
      <Modal
        open={deleteTarget !== null}
        onClose={() => !pendingDelete && setDeleteTarget(null)}
        title="Hapus produk?"
        description="Tindakan ini tidak bisa dibatalkan."
        className="max-w-md"
      >
        <div className="space-y-4">
          <p className="text-sm">
            Yakin mau menghapus <span className="font-medium">{deleteTarget?.nama}</span> (
            {deleteTarget?.id})? Data stok produk ini akan hilang.
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
          {toast.ok ? toast.message : toast.error}
        </div>
      )}
    </div>
  );
}

function SortButton({
  label,
  active,
  asc,
  onClick,
  align = "left",
}: {
  label: string;
  active: boolean;
  asc: boolean;
  onClick: () => void;
  align?: "left" | "right";
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-sort={active ? (asc ? "ascending" : "descending") : "none"}
      className={cn(
        "inline-flex cursor-pointer items-center gap-1 font-medium hover:text-foreground",
        active && "text-foreground",
        align === "right" && "flex-row-reverse",
      )}
    >
      {label}
      <ArrowUpDown className="size-3" />
    </button>
  );
}
