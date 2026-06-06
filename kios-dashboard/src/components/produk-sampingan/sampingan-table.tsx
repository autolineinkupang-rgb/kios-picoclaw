"use client";

import { useMemo, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import {
  AlertTriangle,
  ArrowDownToLine,
  CheckCircle2,
  Loader2,
  Package,
  Pencil,
  Plus,
  PlusCircle,
  Search,
  Settings2,
  Trash2,
  TriangleAlert,
  TrendingUp,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input, Select } from "@/components/ui/input";
import { Modal } from "@/components/ui/modal";
import { EmptyState } from "@/components/ui/empty-state";
import { SampinganForm } from "./sampingan-form";
import {
  deleteSampinganAction,
  setAmbangBatasKategoriAction,
  tarikHasilAction,
  topupSaldoAction,
  topupStokAction,
  type ActionResult,
  type JenisSampingan,
} from "@/app/(app)/produk-sampingan/actions";
import { Label } from "@/components/ui/input";
import { formatRupiah, formatTanggal } from "@/lib/format";
import { produkImage } from "@/lib/produk-image";
import { cn, matchesQuery } from "@/lib/utils";
import type { Produk, SampinganSaldo } from "@/lib/types";

type JenisFilter = "" | JenisSampingan;

const JENIS_META: Record<JenisSampingan, {
  label: string;
  icon: string;
  unit: string;
  isPulsa: boolean;
  variant: "accent" | "warning" | "secondary" | "success";
}> = {
  pulsa: { label: "Pulsa", icon: "📶", unit: "Rp", isPulsa: true, variant: "accent" },
  bensin: { label: "Bensin", icon: "⛽", unit: "liter", isPulsa: false, variant: "warning" },
  solar: { label: "Solar", icon: "🛢️", unit: "liter", isPulsa: false, variant: "secondary" },
  minyak_tanah: { label: "Minyak Tanah", icon: "🪔", unit: "liter", isPulsa: false, variant: "success" },
};

const ALL_JENIS: JenisSampingan[] = ["pulsa", "bensin", "solar", "minyak_tanah"];

function formatSaldo(jenis: JenisSampingan, nilai: number): string {
  return jenis === "pulsa"
    ? formatRupiah(nilai)
    : `${nilai.toLocaleString("id-ID")} L`;
}

function getMinKey(jenis: JenisSampingan): keyof SampinganSaldo {
  return `min_${jenis}` as keyof SampinganSaldo;
}

export function SampinganTable({
  produk,
  canManage,
  labaTersedia = 0,
  saldo,
  omzetPerJenis = {},
  labaTersediaPerJenis = {},
}: {
  produk: Produk[];
  canManage: boolean;
  labaTersedia?: number;
  saldo: SampinganSaldo;
  omzetPerJenis?: Partial<Record<JenisSampingan, number>>;
  labaTersediaPerJenis?: Partial<Record<JenisSampingan, number>>;
}) {
  const router = useRouter();
  const [query, setQuery] = useState("");
  const [jenisFilter, setJenisFilter] = useState<JenisFilter>("");
  const [dialog, setDialog] = useState<{ mode: "add" | "edit"; produk?: Produk } | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Produk | null>(null);
  const [toast, setToast] = useState<ActionResult | null>(null);
  const [pendingDelete, startDelete] = useTransition();

  // category-level topup state
  const [topupJenis, setTopupJenis] = useState<JenisSampingan | null>(null);
  const [topupJumlah, setTopupJumlah] = useState("");
  const [topupHargaBeli, setTopupHargaBeli] = useState("");
  const [topupCatatan, setTopupCatatan] = useState("");
  const [topupFromLaba, setTopupFromLaba] = useState(false);
  const [pendingTopup, startTopup] = useTransition();

  // category-level min threshold state
  const [minJenis, setMinJenis] = useState<JenisSampingan | null>(null);
  const [minNilai, setMinNilai] = useState("");
  const [pendingMin, startMin] = useTransition();

  // category-level tarik hasil state
  const [tarikJenis, setTarikJenis] = useState<JenisSampingan | null>(null);
  const [tarikJumlah, setTarikJumlah] = useState("");
  const [tarikCatatan, setTarikCatatan] = useState("");
  const [pendingTarik, startTarik] = useTransition();

  // jenis present in produk list
  const jenisPresent = useMemo(() => {
    const s = new Set(produk.map((p) => p.jenis as JenisSampingan));
    return ALL_JENIS.filter((j) => s.has(j));
  }, [produk]);

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

  function openTopup(jenis: JenisSampingan) {
    setTopupJenis(jenis);
    setTopupJumlah("");
    setTopupHargaBeli("");
    setTopupCatatan("");
    setTopupFromLaba(false);
  }

  function runTopup(e: React.FormEvent) {
    e.preventDefault();
    if (!topupJenis) return;
    const jumlah = Number(topupJumlah);
    if (!jumlah || jumlah <= 0) return;
    const jenis = topupJenis;
    startTopup(async () => {
      const r =
        jenis === "pulsa"
          ? await topupSaldoAction("pulsa", jumlah, topupCatatan, topupFromLaba)
          : await topupStokAction(jenis, jumlah, topupCatatan, topupFromLaba, Number(topupHargaBeli) || 0);
      setTopupJenis(null);
      showToast(r);
      if (r.ok) router.refresh();
    });
  }

  function openMin(jenis: JenisSampingan) {
    setMinJenis(jenis);
    const current = (saldo[getMinKey(jenis)] as number | undefined) ?? 0;
    setMinNilai(current > 0 ? String(current) : "");
  }

  function runSetMin(e: React.FormEvent) {
    e.preventDefault();
    if (!minJenis) return;
    const nilai = Number(minNilai);
    if (!Number.isFinite(nilai) || nilai < 0) return;
    const jenis = minJenis;
    startMin(async () => {
      const r = await setAmbangBatasKategoriAction(jenis, nilai);
      setMinJenis(null);
      showToast(r);
      if (r.ok) router.refresh();
    });
  }

  function openTarik(jenis: JenisSampingan) {
    setTarikJenis(jenis);
    setTarikJumlah("");
    setTarikCatatan("");
  }

  function runTarik(e: React.FormEvent) {
    e.preventDefault();
    if (!tarikJenis) return;
    const jumlah = Number(tarikJumlah);
    if (!jumlah || jumlah <= 0) return;
    const jenis = tarikJenis;
    startTarik(async () => {
      const r = await tarikHasilAction(jenis, jumlah, tarikCatatan);
      setTarikJenis(null);
      showToast(r);
      if (r.ok) router.refresh();
    });
  }

  return (
    <div className="space-y-5">
      {/* Laba tersedia banner */}
      <div className="flex items-center gap-3 rounded-xl border border-violet-500/25 bg-violet-500/8 px-4 py-3">
        <TrendingUp className="size-5 shrink-0 text-violet-500" />
        <div className="flex-1 min-w-0">
          <p className="text-xs text-muted-foreground">Total Penghasilan Tersedia (bisa ditarik)</p>
          <p className="font-semibold tabular-nums text-violet-700 dark:text-violet-300">
            {formatRupiah(labaTersedia)}
          </p>
        </div>
        {labaTersedia <= 0 && (
          <span className="text-xs text-muted-foreground">Belum ada laba</span>
        )}
      </div>

      {/* Category saldo cards */}
      {jenisPresent.length > 0 && (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {jenisPresent.map((jenis) => {
            const meta = JENIS_META[jenis];
            const nilaiSaldo = saldo[jenis] as number;
            const minKey = getMinKey(jenis);
            const minNilaiSaldo = (saldo[minKey] as number | undefined) ?? 0;
            const belowMin = minNilaiSaldo > 0 && nilaiSaldo < minNilaiSaldo;
            const penghasilan = omzetPerJenis[jenis] ?? 0;
            const labaTersediaKat = labaTersediaPerJenis[jenis] ?? 0;
            return (
              <div
                key={jenis}
                className={cn(
                  "rounded-xl border bg-card p-4 space-y-3",
                  belowMin && "border-destructive/40",
                )}
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="text-lg">{meta.icon}</span>
                    <span className="font-semibold text-sm">{meta.label}</span>
                  </div>
                  {belowMin && (
                    <span title="Saldo/stok di bawah ambang batas">
                      <AlertTriangle className="size-4 text-destructive" />
                    </span>
                  )}
                </div>

                <div>
                  <p className="text-xl font-bold tabular-nums">
                    {formatSaldo(jenis, nilaiSaldo)}
                  </p>
                  {minNilaiSaldo > 0 && (
                    <p className={cn("text-xs", belowMin ? "text-destructive" : "text-muted-foreground")}>
                      min {formatSaldo(jenis, minNilaiSaldo)}
                    </p>
                  )}
                </div>

                <div className="grid grid-cols-2 gap-2 rounded-lg bg-muted/40 p-2.5 text-xs">
                  <div>
                    <p className="text-muted-foreground">Penghasilan</p>
                    <p className="font-semibold tabular-nums">{formatRupiah(penghasilan)}</p>
                  </div>
                  <div>
                    <p className="text-muted-foreground">Penghasilan Tersedia</p>
                    <p className={cn("font-semibold tabular-nums", labaTersediaKat > 0 ? "text-success" : "text-muted-foreground")}>
                      {formatRupiah(labaTersediaKat)}
                    </p>
                  </div>
                </div>

                {canManage && (
                  <div className="flex items-center gap-1.5">
                    <button
                      type="button"
                      onClick={() => openTopup(jenis)}
                      className="flex flex-1 items-center justify-center gap-1 rounded-md bg-success/10 px-2 py-1.5 text-xs font-medium text-success hover:bg-success/20"
                    >
                      <PlusCircle className="size-3.5" />
                      {jenis === "pulsa" ? "Topup" : "Tambah"}
                    </button>
                    <button
                      type="button"
                      onClick={() => openTarik(jenis)}
                      className="flex flex-1 items-center justify-center gap-1 rounded-md bg-amber-500/10 px-2 py-1.5 text-xs font-medium text-amber-600 hover:bg-amber-500/20"
                    >
                      <ArrowDownToLine className="size-3.5" />
                      Tarik
                    </button>
                    <button
                      type="button"
                      onClick={() => openMin(jenis)}
                      aria-label="Atur ambang batas"
                      title="Atur Ambang Batas"
                      className="flex size-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-amber-600"
                    >
                      <Settings2 className="size-3.5" />
                    </button>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      {/* Search/filter bar */}
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
          <table className="w-full min-w-[560px] text-sm">
            <thead>
              <tr className="border-b text-left text-xs text-muted-foreground">
                <th className="p-3 font-medium">Produk</th>
                <th className="p-3 font-medium">Jenis</th>
                <th className="p-3 text-right font-medium">Harga Beli</th>
                <th className="p-3 text-right font-medium">Harga Jual</th>
                <th className="p-3 text-right font-medium">Stok Tersedia</th>
                <th className="p-3 font-medium">Update</th>
                {canManage && <th className="p-3 text-right font-medium">Aksi</th>}
              </tr>
            </thead>
            <tbody>
              {rows.map((p) => {
                const jenisMeta = JENIS_META[p.jenis as JenisSampingan] ?? null;
                const isPulsa = p.jenis === "pulsa";
                const stokTersedia = isPulsa && p.harga_beli > 0
                  ? Math.floor((saldo.pulsa ?? 0) / p.harga_beli)
                  : null;
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
                    <td className="p-3 text-right font-mono tabular-nums text-muted-foreground">
                      {formatRupiah(p.harga_beli)}
                      {!isPulsa && (
                        <p className="text-xs">/liter</p>
                      )}
                    </td>
                    <td className="p-3 text-right font-mono tabular-nums">
                      {formatRupiah(p.harga_jual)}
                      {!isPulsa && (
                        <p className="text-xs text-muted-foreground">/liter</p>
                      )}
                    </td>
                    <td className="p-3 text-right tabular-nums">
                      {stokTersedia !== null ? (
                        <span className={cn("font-semibold", stokTersedia === 0 ? "text-destructive" : "text-foreground")}>
                          {stokTersedia}
                        </span>
                      ) : (
                        <span className="text-muted-foreground">—</span>
                      )}
                    </td>
                    <td className="p-3 text-xs text-muted-foreground">
                      {formatTanggal(p.last_update)}
                    </td>
                    {canManage && (
                      <td className="p-3">
                        <div className="flex items-center justify-end gap-1">
                          <button
                            type="button"
                            onClick={() => setDialog({ mode: "edit", produk: p })}
                            aria-label={`Edit ${p.nama}`}
                            title="Edit produk"
                            className="flex size-7 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
                          >
                            <Pencil className="size-3.5" />
                          </button>
                          <button
                            type="button"
                            onClick={() => setDeleteTarget(p)}
                            aria-label={`Hapus ${p.nama}`}
                            title="Hapus produk"
                            className="flex size-7 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                          >
                            <Trash2 className="size-3.5" />
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

      {/* Add / Edit modal */}
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

      {/* Delete modal */}
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

      {/* Topup / Tambah Stok modal */}
      {topupJenis && (
        <Modal
          open={true}
          onClose={() => !pendingTopup && setTopupJenis(null)}
          title={
            topupJenis === "pulsa"
              ? `Topup Saldo ${JENIS_META[topupJenis].label}`
              : `Tambah Stok ${JENIS_META[topupJenis].label}`
          }
          description={`Saldo saat ini: ${formatSaldo(topupJenis, saldo[topupJenis] as number)}`}
          className="max-w-sm"
        >
          <form onSubmit={runTopup} className="space-y-4">
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
                  <p className="font-medium">Dari Penghasilan</p>
                  <p className="text-xs opacity-70">
                    Tersedia: {formatRupiah(topupJenis ? (labaTersediaPerJenis[topupJenis] ?? 0) : 0)}
                  </p>
                </button>
              </div>
              {topupFromLaba && topupJenis && (labaTersediaPerJenis[topupJenis] ?? 0) <= 0 && (
                <p className="text-xs text-destructive">Penghasilan tersedia tidak cukup.</p>
              )}
            </div>

            {topupJenis !== "pulsa" && (
              <div className="space-y-1.5">
                <Label htmlFor="topup-harga-beli">
                  Harga Beli dari Pemasok (Rp/liter)
                  {topupFromLaba && <span className="text-destructive"> *</span>}
                </Label>
                <Input
                  id="topup-harga-beli"
                  type="number"
                  inputMode="numeric"
                  min={1}
                  value={topupHargaBeli}
                  onChange={(e) => setTopupHargaBeli(e.target.value)}
                  placeholder="mis. 10000 (Pertalite/Pertamax)"
                  className="font-mono tabular-nums"
                />
                {topupFromLaba && Number(topupJumlah) > 0 && Number(topupHargaBeli) > 0 && (
                  <p className="text-xs text-muted-foreground">
                    Total biaya: {formatRupiah(Number(topupJumlah) * Number(topupHargaBeli))}
                    {" "}({topupJumlah} L × {formatRupiah(Number(topupHargaBeli))}/L)
                  </p>
                )}
              </div>
            )}

            <div className="space-y-1.5">
              <Label htmlFor="topup-jumlah">
                {topupJenis === "pulsa" ? "Jumlah Saldo (Rp)" : "Jumlah (Liter)"}
                <span className="text-destructive"> *</span>
              </Label>
              <Input
                id="topup-jumlah"
                type="number"
                inputMode="numeric"
                min={1}
                value={topupJumlah}
                onChange={(e) => setTopupJumlah(e.target.value)}
                placeholder={topupJenis === "pulsa" ? "mis. 100000" : "mis. 50"}
                className="font-mono tabular-nums"
                autoFocus
                required
              />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="topup-catatan">Catatan (opsional)</Label>
              <Input
                id="topup-catatan"
                value={topupCatatan}
                onChange={(e) => setTopupCatatan(e.target.value)}
                placeholder={topupFromLaba ? "mis. reinvest laba bulan ini" : "mis. dari agen"}
              />
            </div>

            <div className="flex justify-end gap-2 pt-1">
              <Button type="button" variant="outline" size="sm" onClick={() => setTopupJenis(null)} disabled={pendingTopup}>
                Batal
              </Button>
              <Button
                type="submit"
                variant="accent"
                size="sm"
                disabled={
                  pendingTopup ||
                  !topupJumlah ||
                  (topupFromLaba && topupJenis !== null && (labaTersediaPerJenis[topupJenis] ?? 0) <= 0) ||
                  (topupFromLaba && topupJenis !== "pulsa" && !topupHargaBeli)
                }
              >
                {pendingTopup && <Loader2 className="size-4 animate-spin" />}
                {topupJenis === "pulsa" ? "Topup Saldo" : "Tambah Stok"}
              </Button>
            </div>
          </form>
        </Modal>
      )}

      {/* Tarik Hasil modal */}
      {tarikJenis && (
        <Modal
          open={true}
          onClose={() => !pendingTarik && setTarikJenis(null)}
          title={`Tarik Penghasilan — ${JENIS_META[tarikJenis].label}`}
          description={`Penghasilan tersedia: ${formatRupiah(labaTersediaPerJenis[tarikJenis] ?? 0)}`}
          className="max-w-sm"
        >
          <form onSubmit={runTarik} className="space-y-4">
            {(labaTersediaPerJenis[tarikJenis] ?? 0) <= 0 ? (
              <p className="rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2.5 text-sm text-destructive">
                Belum ada penghasilan yang bisa ditarik saat ini.
              </p>
            ) : (
              <>
                <div className="space-y-1.5">
                  <Label htmlFor="tarik-jumlah">
                    Jumlah yang ditarik (Rp) <span className="text-destructive">*</span>
                  </Label>
                  <Input
                    id="tarik-jumlah"
                    type="number"
                    inputMode="numeric"
                    min={1}
                    max={labaTersediaPerJenis[tarikJenis] ?? 0}
                    value={tarikJumlah}
                    onChange={(e) => setTarikJumlah(e.target.value)}
                    placeholder={`maks ${formatRupiah(labaTersediaPerJenis[tarikJenis] ?? 0)}`}
                    className="font-mono tabular-nums"
                    autoFocus
                    required
                  />
                  {Number(tarikJumlah) > (labaTersediaPerJenis[tarikJenis] ?? 0) && (
                    <p className="text-xs text-destructive">Melebihi penghasilan tersedia.</p>
                  )}
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="tarik-catatan">Catatan (opsional)</Label>
                  <Input
                    id="tarik-catatan"
                    value={tarikCatatan}
                    onChange={(e) => setTarikCatatan(e.target.value)}
                    placeholder="mis. ambil tunai untuk kebutuhan"
                  />
                </div>
              </>
            )}
            <div className="flex justify-end gap-2 pt-1">
              <Button type="button" variant="outline" size="sm" onClick={() => setTarikJenis(null)} disabled={pendingTarik}>
                Batal
              </Button>
              {(labaTersediaPerJenis[tarikJenis] ?? 0) > 0 && (
                <Button
                  type="submit"
                  variant="accent"
                  size="sm"
                  disabled={pendingTarik || !tarikJumlah || Number(tarikJumlah) > (labaTersediaPerJenis[tarikJenis] ?? 0)}
                >
                  {pendingTarik && <Loader2 className="size-4 animate-spin" />}
                  Tarik Penghasilan
                </Button>
              )}
            </div>
          </form>
        </Modal>
      )}

      {/* Atur Ambang Batas modal */}
      {minJenis && (
        <Modal
          open={true}
          onClose={() => !pendingMin && setMinJenis(null)}
          title={`Atur Ambang Batas — ${JENIS_META[minJenis].label}`}
          description={
            minJenis === "pulsa"
              ? "Alert muncul jika saldo modal turun di bawah nilai ini."
              : "Alert muncul jika stok turun di bawah nilai ini."
          }
          className="max-w-sm"
        >
          <form onSubmit={runSetMin} className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="min-nilai">
                {minJenis === "pulsa" ? "Minimal Saldo (Rp)" : "Minimal Stok (liter)"}
              </Label>
              <Input
                id="min-nilai"
                type="number"
                inputMode="numeric"
                min={0}
                value={minNilai}
                onChange={(e) => setMinNilai(e.target.value)}
                placeholder={minJenis === "pulsa" ? "mis. 50000" : "mis. 20"}
                className="font-mono tabular-nums"
                autoFocus
              />
              <p className="text-xs text-muted-foreground">Isi 0 untuk menonaktifkan alert.</p>
            </div>
            <div className="flex justify-end gap-2 pt-1">
              <Button type="button" variant="outline" size="sm" onClick={() => setMinJenis(null)} disabled={pendingMin}>
                Batal
              </Button>
              <Button type="submit" variant="accent" size="sm" disabled={pendingMin || minNilai === ""}>
                {pendingMin && <Loader2 className="size-4 animate-spin" />}
                Simpan
              </Button>
            </div>
          </form>
        </Modal>
      )}

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
