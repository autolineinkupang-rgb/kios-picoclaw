"use client";

import { useMemo, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { AlertTriangle, CheckCircle2, Loader2, Package, Pencil, Plus, PlusCircle, Search, Settings2, Trash2, TriangleAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input, Select } from "@/components/ui/input";
import { Modal } from "@/components/ui/modal";
import { EmptyState } from "@/components/ui/empty-state";
import { SampinganForm } from "./sampingan-form";
import { deleteSampinganAction, setAmbangBatasAction, topupSaldoAction, topupStokAction, type ActionResult } from "@/app/(app)/produk-sampingan/actions";
import { Label } from "@/components/ui/input";
import { formatRupiah, formatTanggal } from "@/lib/format";
import { produkImage } from "@/lib/produk-image";
import { STATUS_META, stokStatus } from "@/lib/produk-status";
import { cn, matchesQuery } from "@/lib/utils";
import type { Produk } from "@/lib/types";

type JenisFilter = "" | "pulsa" | "bensin" | "solar" | "minyak_tanah";

const JENIS_META: Record<string, { label: string; variant: "accent" | "warning" | "secondary" | "success" }> = {
  pulsa: { label: "Pulsa", variant: "accent" },
  bensin: { label: "Bensin", variant: "warning" },
  solar: { label: "Solar", variant: "secondary" },
  minyak_tanah: { label: "Minyak Tanah", variant: "success" },
};

export function SampinganTable({
  produk,
  canManage,
  labaTersedia = 0,
}: {
  produk: Produk[];
  canManage: boolean;
  labaTersedia?: number;
}) {
  const router = useRouter();
  const [query, setQuery] = useState("");
  const [jenisFilter, setJenisFilter] = useState<JenisFilter>("");
  const [dialog, setDialog] = useState<{ mode: "add" | "edit"; produk?: Produk } | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Produk | null>(null);
  const [toast, setToast] = useState<ActionResult | null>(null);
  const [pendingDelete, startDelete] = useTransition();
  const [topupTarget, setTopupTarget] = useState<Produk | null>(null);
  const [topupJumlah, setTopupJumlah] = useState("");
  const [topupCatatan, setTopupCatatan] = useState("");
  const [topupFromLaba, setTopupFromLaba] = useState(false);
  const [pendingTopup, startTopup] = useTransition();
  const [minTarget, setMinTarget] = useState<Produk | null>(null);
  const [minNilai, setMinNilai] = useState("");
  const [pendingMin, startMin] = useTransition();

  const rows = useMemo(() => {
    const q = query.trim().toLowerCase();
    return produk.filter((p) => {
      if (jenisFilter && p.jenis !== jenisFilter) return false;
      if (!q) return true;
      return matchesQuery(q, p.nama, p.id, p.barcode);
    });
  }, [produk, query, jenisFilter]);

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
      const r = await deleteSampinganAction(target.id);
      setDeleteTarget(null);
      showToast(r);
      if (r.ok) router.refresh();
    });
  }

  function openTopup(p: Produk) {
    setTopupTarget(p);
    setTopupJumlah("");
    setTopupCatatan("");
    setTopupFromLaba(false);
  }

  function runTopup(e: React.FormEvent) {
    e.preventDefault();
    if (!topupTarget) return;
    const jumlah = Number(topupJumlah);
    if (!jumlah || jumlah <= 0) return;
    const target = topupTarget;
    startTopup(async () => {
      const r =
        target.jenis === "pulsa"
          ? await topupSaldoAction(target.id, jumlah, topupCatatan, topupFromLaba)
          : await topupStokAction(target.id, jumlah, topupCatatan, topupFromLaba);
      setTopupTarget(null);
      showToast(r);
      if (r.ok) router.refresh();
    });
  }

  function openMin(p: Produk) {
    setMinTarget(p);
    const current = p.jenis === "pulsa" ? (p.saldo_minimum ?? 0) : p.stok_minimum;
    setMinNilai(current > 0 ? String(current) : "");
  }

  function runSetMin(e: React.FormEvent) {
    e.preventDefault();
    if (!minTarget) return;
    const nilai = Number(minNilai);
    if (!Number.isFinite(nilai) || nilai < 0) return;
    const target = minTarget;
    startMin(async () => {
      const r = await setAmbangBatasAction(target.id, nilai);
      setMinTarget(null);
      showToast(r);
      if (r.ok) router.refresh();
    });
  }

  return (
    <div className="space-y-4">
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
          value={jenisFilter}
          onChange={(e) => setJenisFilter(e.target.value as JenisFilter)}
          aria-label="Filter jenis"
          className="sm:w-44"
        >
          <option value="">Semua jenis</option>
          <option value="pulsa">Pulsa</option>
          <option value="bensin">Bensin</option>
          <option value="solar">Solar</option>
          <option value="minyak_tanah">Minyak Tanah</option>
        </Select>
        {canManage && (
          <Button variant="accent" size="md" onClick={() => setDialog({ mode: "add" })}>
            <Plus className="size-4" /> Tambah
          </Button>
        )}
      </div>

      <p className="text-xs text-muted-foreground">
        Menampilkan {rows.length} dari {produk.length} produk
      </p>

      {rows.length === 0 ? (
        <EmptyState
          icon={Package}
          title={produk.length === 0 ? "Belum ada produk sampingan" : "Tidak ada hasil"}
          description={
            produk.length === 0
              ? "Tambahkan pulsa, bensin, solar, atau minyak tanah."
              : "Coba ubah kata kunci atau filter."
          }
          action={
            canManage && produk.length === 0 ? (
              <Button variant="accent" size="sm" onClick={() => setDialog({ mode: "add" })}>
                <Plus className="size-4" /> Tambah Produk Sampingan
              </Button>
            ) : undefined
          }
        />
      ) : (
        <div className="overflow-x-auto rounded-xl border bg-card">
          <table className="w-full min-w-[760px] text-sm">
            <thead>
              <tr className="border-b text-left text-xs text-muted-foreground">
                <th className="p-3 font-medium">Produk</th>
                <th className="p-3 font-medium">Jenis</th>
                <th className="p-3 font-medium">Stok</th>
                <th className="p-3 font-medium">Status</th>
                <th className="p-3 text-right font-medium">Harga Beli</th>
                <th className="p-3 text-right font-medium">Harga Jual</th>
                <th className="p-3 font-medium">Update</th>
                {canManage && <th className="p-3 text-right font-medium">Aksi</th>}
              </tr>
            </thead>
            <tbody>
              {rows.map((p) => {
                const st = STATUS_META[stokStatus(p)];
                const jenisMeta = JENIS_META[p.jenis ?? ""] ?? null;
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
                            {p.id}{p.kategori ? ` · ${p.kategori}` : ""}
                          </p>
                        </div>
                      </div>
                    </td>
                    <td className="p-3">
                      {jenisMeta && <Badge variant={jenisMeta.variant}>{jenisMeta.label}</Badge>}
                    </td>
                    <td className="p-3 font-mono tabular-nums">
                      {p.jenis === "pulsa" ? (
                        <div>
                          {p.saldo_modal !== undefined ? (
                            <div className="flex items-center gap-1.5">
                              <span>{formatRupiah(p.saldo_modal)}</span>
                              {(p.saldo_minimum ?? 0) > 0 && p.saldo_modal < (p.saldo_minimum ?? 0) && (
                                <span title={`Di bawah ambang batas (min Rp ${(p.saldo_minimum ?? 0).toLocaleString("id-ID")})`}>
                                  <AlertTriangle className="size-3.5 text-destructive" />
                                </span>
                              )}
                            </div>
                          ) : (
                            <span className="text-muted-foreground">—</span>
                          )}
                          {(p.saldo_minimum ?? 0) > 0 && (
                            <p className="text-xs text-muted-foreground">min {formatRupiah(p.saldo_minimum!)}</p>
                          )}
                        </div>
                      ) : (
                        <div>
                          <div className="flex items-center gap-1.5">
                            <span>{p.stok} <span className="text-xs text-muted-foreground">{p.satuan}</span></span>
                            {p.stok_minimum > 0 && p.stok <= p.stok_minimum && (
                              <span title={`Di bawah ambang batas (min ${p.stok_minimum} ${p.satuan})`}>
                                <AlertTriangle className="size-3.5 text-destructive" />
                              </span>
                            )}
                          </div>
                          {p.stok_minimum > 0 && (
                            <p className="text-xs text-muted-foreground">min {p.stok_minimum} {p.satuan}</p>
                          )}
                        </div>
                      )}
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
                    <td className="p-3 text-xs text-muted-foreground">
                      {formatTanggal(p.last_update)}
                    </td>
                    {canManage && (
                      <td className="p-3">
                        <div className="flex justify-end gap-1">
                          <button
                            type="button"
                            onClick={() => openTopup(p)}
                            aria-label={p.jenis === "pulsa" ? `Tambah saldo ${p.nama}` : `Tambah stok ${p.nama}`}
                            title={p.jenis === "pulsa" ? "Tambah Saldo" : "Tambah Stok"}
                            className="flex size-8 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-success/10 hover:text-success"
                          >
                            <PlusCircle className="size-4" />
                          </button>
                          <button
                            type="button"
                            onClick={() => openMin(p)}
                            aria-label={`Atur ambang batas ${p.nama}`}
                            title="Atur Ambang Batas"
                            className="flex size-8 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-amber-500/10 hover:text-amber-600"
                          >
                            <Settings2 className="size-4" />
                          </button>
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

      <Modal
        open={dialog !== null}
        onClose={() => setDialog(null)}
        title={dialog?.mode === "edit" ? "Edit Produk Sampingan" : "Tambah Produk Sampingan"}
        description={dialog?.mode === "edit" ? dialog.produk?.nama : "Pilih jenis dan lengkapi detail."}
      >
        {dialog && (
          <SampinganForm produk={dialog.produk} onResult={onMutated} onCancel={() => setDialog(null)} />
        )}
      </Modal>

      <Modal
        open={deleteTarget !== null}
        onClose={() => !pendingDelete && setDeleteTarget(null)}
        title="Hapus produk sampingan?"
        description="Tindakan ini tidak bisa dibatalkan."
        className="max-w-md"
      >
        <div className="space-y-4">
          <p className="text-sm">
            Yakin mau menghapus <span className="font-medium">{deleteTarget?.nama}</span> ({deleteTarget?.id})?
          </p>
          <div className="flex justify-end gap-2">
            <Button variant="outline" size="sm" onClick={() => setDeleteTarget(null)} disabled={pendingDelete}>Batal</Button>
            <Button variant="destructive" size="sm" onClick={runDelete} disabled={pendingDelete}>
              {pendingDelete && <Loader2 className="size-4 animate-spin" />}
              Hapus
            </Button>
          </div>
        </div>
      </Modal>

      <Modal
        open={minTarget !== null}
        onClose={() => !pendingMin && setMinTarget(null)}
        title={`Atur Ambang Batas — ${minTarget?.nama}`}
        description={
          minTarget?.jenis === "pulsa"
            ? "Alert muncul jika saldo modal turun di bawah nilai ini."
            : "Alert muncul jika stok turun di bawah nilai ini."
        }
        className="max-w-sm"
      >
        {minTarget && (
          <form onSubmit={runSetMin} className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="min-nilai">
                {minTarget.jenis === "pulsa" ? "Minimal Saldo (Rp)" : `Minimal Stok (${minTarget.satuan})`}
              </Label>
              <Input
                id="min-nilai"
                type="number"
                inputMode="numeric"
                min={0}
                value={minNilai}
                onChange={(e) => setMinNilai(e.target.value)}
                placeholder={minTarget.jenis === "pulsa" ? "mis. 50000" : "mis. 20"}
                className="font-mono tabular-nums"
                autoFocus
              />
              <p className="text-xs text-muted-foreground">
                Isi 0 untuk menonaktifkan alert.
              </p>
            </div>
            <div className="flex justify-end gap-2 pt-1">
              <Button type="button" variant="outline" size="sm" onClick={() => setMinTarget(null)} disabled={pendingMin}>
                Batal
              </Button>
              <Button type="submit" variant="accent" size="sm" disabled={pendingMin || minNilai === ""}>
                {pendingMin && <Loader2 className="size-4 animate-spin" />}
                Simpan
              </Button>
            </div>
          </form>
        )}
      </Modal>

      <Modal
        open={topupTarget !== null}
        onClose={() => !pendingTopup && setTopupTarget(null)}
        title={
          topupTarget?.jenis === "pulsa"
            ? `Tambah Saldo — ${topupTarget?.nama}`
            : `Tambah Liter — ${topupTarget?.nama}`
        }
        description={
          topupTarget?.jenis === "pulsa"
            ? `Saldo saat ini: Rp ${(topupTarget?.saldo_modal ?? 0).toLocaleString("id-ID")}`
            : `Stok saat ini: ${topupTarget?.stok ?? 0} liter`
        }
        className="max-w-sm"
      >
        {topupTarget && (
          <form onSubmit={runTopup} className="space-y-4">
            {/* Sumber dana */}
            <div className="space-y-1.5">
              <Label>Sumber Dana</Label>
              <div className="grid grid-cols-2 gap-2">
                <button
                  type="button"
                  onClick={() => setTopupFromLaba(false)}
                  className={`rounded-lg border px-3 py-2.5 text-sm text-left transition-colors ${
                    !topupFromLaba
                      ? "border-accent bg-accent/10 font-medium text-accent"
                      : "border-border text-muted-foreground hover:border-accent/40"
                  }`}
                >
                  <p className="font-medium">Dana Sendiri</p>
                  <p className="text-xs opacity-70">Modal / uang tunai</p>
                </button>
                <button
                  type="button"
                  onClick={() => setTopupFromLaba(true)}
                  className={`rounded-lg border px-3 py-2.5 text-sm text-left transition-colors ${
                    topupFromLaba
                      ? "border-success bg-success/10 font-medium text-success"
                      : "border-border text-muted-foreground hover:border-success/40"
                  }`}
                >
                  <p className="font-medium">Dari Laba</p>
                  <p className="text-xs opacity-70">
                    Tersedia: Rp {labaTersedia.toLocaleString("id-ID")}
                  </p>
                </button>
              </div>
              {topupFromLaba && labaTersedia <= 0 && (
                <p className="text-xs text-destructive">Laba tersedia tidak cukup.</p>
              )}
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="topup-jumlah">
                {topupTarget.jenis === "pulsa" ? "Jumlah Saldo (Rp)" : "Jumlah Liter"}
                <span className="text-destructive"> *</span>
              </Label>
              <Input
                id="topup-jumlah"
                type="number"
                inputMode="numeric"
                min={1}
                value={topupJumlah}
                onChange={(e) => setTopupJumlah(e.target.value)}
                placeholder={topupTarget.jenis === "pulsa" ? "mis. 100000" : "mis. 50"}
                className="font-mono tabular-nums"
                autoFocus
                required
              />
              {topupFromLaba && topupTarget.jenis !== "pulsa" && topupTarget.harga_beli > 0 && Number(topupJumlah) > 0 && (
                <p className="text-xs text-muted-foreground">
                  Biaya modal: Rp {(Number(topupJumlah) * topupTarget.harga_beli).toLocaleString("id-ID")}
                  {" "}(@ Rp {topupTarget.harga_beli.toLocaleString("id-ID")}/L)
                </p>
              )}
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="topup-catatan">Catatan (opsional)</Label>
              <Input
                id="topup-catatan"
                value={topupCatatan}
                onChange={(e) => setTopupCatatan(e.target.value)}
                placeholder={topupFromLaba ? "mis. reinvest laba bulan ini" : "mis. dari agen Telkomsel"}
              />
            </div>
            <div className="flex justify-end gap-2 pt-1">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => setTopupTarget(null)}
                disabled={pendingTopup}
              >
                Batal
              </Button>
              <Button
                type="submit"
                variant="accent"
                size="sm"
                disabled={pendingTopup || !topupJumlah || (topupFromLaba && labaTersedia <= 0)}
              >
                {pendingTopup && <Loader2 className="size-4 animate-spin" />}
                {topupTarget.jenis === "pulsa" ? "Tambah Saldo" : "Tambah Liter"}
              </Button>
            </div>
          </form>
        )}
      </Modal>

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
          {toast.ok ? <CheckCircle2 className="size-4 text-success" /> : <TriangleAlert className="size-4" />}
          {toast.ok ? toast.message : toast.error}
        </div>
      )}
    </div>
  );
}
