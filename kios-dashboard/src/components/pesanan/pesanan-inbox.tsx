"use client";

import { useMemo, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import {
  CheckCircle2,
  ClipboardList,
  Loader2,
  MessageCircle,
  Phone,
  QrCode,
  TriangleAlert,
  User,
  Wallet,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Modal } from "@/components/ui/modal";
import { EmptyState } from "@/components/ui/empty-state";
import { formatRupiah, formatTanggal } from "@/lib/format";
import { cn } from "@/lib/utils";
import { buildStrukWa, normalizeWaNumber, waLink } from "@/lib/wa";
import type { Pesanan, PesananStatus } from "@/lib/types";
import {
  prosesPesananAction,
  tolakPesananAction,
  type ActionResult,
} from "@/app/(app)/pesanan/actions";

const STATUS_META: Record<PesananStatus, { label: string; variant: "warning" | "success" | "secondary" }> = {
  pending: { label: "Menunggu", variant: "warning" },
  diproses: { label: "Diproses", variant: "success" },
  ditolak: { label: "Ditolak", variant: "secondary" },
};

type Filter = PesananStatus | "semua";

const FILTERS: { value: Filter; label: string }[] = [
  { value: "pending", label: "Menunggu" },
  { value: "diproses", label: "Diproses" },
  { value: "ditolak", label: "Ditolak" },
  { value: "semua", label: "Semua" },
];

export function PesananInbox({ pesanan }: { pesanan: Pesanan[] }) {
  const router = useRouter();
  const [filter, setFilter] = useState<Filter>("pending");
  const [confirm, setConfirm] = useState<{ kind: "proses" | "tolak"; order: Pesanan } | null>(null);
  const [toast, setToast] = useState<ActionResult | null>(null);
  const [pending, start] = useTransition();

  const counts = useMemo(() => {
    const c: Record<string, number> = { pending: 0, diproses: 0, ditolak: 0 };
    for (const p of pesanan) c[p.status] = (c[p.status] ?? 0) + 1;
    return c;
  }, [pesanan]);

  const rows = useMemo(
    () => (filter === "semua" ? pesanan : pesanan.filter((p) => p.status === filter)),
    [pesanan, filter],
  );

  function showToast(r: ActionResult) {
    setToast(r);
    window.setTimeout(() => setToast(null), 4500);
  }

  function runConfirm() {
    if (!confirm) return;
    const { kind, order } = confirm;
    start(async () => {
      const r = kind === "proses" ? await prosesPesananAction(order.id) : await tolakPesananAction(order.id);
      setConfirm(null);
      showToast(r);
      if (r.ok) router.refresh();
    });
  }

  return (
    <div className="space-y-4">
      {/* Status filter */}
      <div role="tablist" aria-label="Filter status" className="flex flex-wrap gap-2">
        {FILTERS.map((f) => {
          const active = filter === f.value;
          const n = f.value === "semua" ? pesanan.length : counts[f.value] ?? 0;
          return (
            <button
              key={f.value}
              role="tab"
              aria-selected={active}
              type="button"
              onClick={() => setFilter(f.value)}
              className={cn(
                "cursor-pointer rounded-lg border px-3 py-1.5 text-sm font-medium transition-colors",
                active ? "border-accent bg-accent/10 text-accent" : "border-border text-muted-foreground hover:bg-muted",
              )}
            >
              {f.label}
              <span className="ml-1.5 tabular-nums opacity-70">{n}</span>
            </button>
          );
        })}
      </div>

      {rows.length === 0 ? (
        <EmptyState
          icon={ClipboardList}
          title={filter === "pending" ? "Tidak ada pesanan menunggu" : "Tidak ada pesanan"}
          description="Pesanan dari halaman toko pembeli akan muncul di sini."
        />
      ) : (
        <ul className="grid grid-cols-1 gap-3 lg:grid-cols-2">
          {rows.map((o) => {
            const st = STATUS_META[o.status];
            return (
              <li key={o.id}>
                <Card className="flex h-full flex-col p-4">
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <p className="font-mono text-sm font-semibold">{o.id}</p>
                      <p className="text-xs text-muted-foreground">
                        {formatTanggal(o.tanggal)} · {o.jam?.slice(0, 5)}
                      </p>
                    </div>
                    <Badge variant={st.variant}>{st.label}</Badge>
                  </div>

                  <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
                    <span className="flex items-center gap-1">
                      <User className="size-3.5" /> {o.nama_pembeli || "Pembeli"}
                    </span>
                    {o.kontak && (
                      <span className="flex items-center gap-1">
                        <Phone className="size-3.5" /> {o.kontak}
                      </span>
                    )}
                    {o.metode_bayar === "qris" ? (
                      <Badge variant="accent" className="gap-1">
                        <QrCode className="size-3" /> QRIS
                      </Badge>
                    ) : (
                      <span className="flex items-center gap-1">
                        <Wallet className="size-3.5" /> Tunai
                      </span>
                    )}
                  </div>

                  <ul className="mt-3 flex-1 space-y-1 border-t pt-3 text-sm">
                    {o.items.map((it, i) => (
                      <li key={i} className="flex justify-between gap-2">
                        <span className="min-w-0 truncate">
                          {it.nama_produk} <span className="text-muted-foreground">x{it.qty}</span>
                        </span>
                        <span className="font-mono tabular-nums text-muted-foreground">
                          {formatRupiah(it.subtotal)}
                        </span>
                      </li>
                    ))}
                  </ul>

                  {o.catatan && (
                    <p className="mt-2 rounded-md bg-muted/50 px-2 py-1.5 text-xs text-muted-foreground">
                      Catatan: {o.catatan}
                    </p>
                  )}

                  <div className="mt-3 flex items-center justify-between border-t pt-3">
                    <span className="font-mono text-base font-semibold tabular-nums">
                      {formatRupiah(o.total)}
                    </span>
                    {o.status === "pending" && (
                      <div className="flex gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          disabled={pending}
                          onClick={() => setConfirm({ kind: "tolak", order: o })}
                        >
                          Tolak
                        </Button>
                        <Button
                          variant="accent"
                          size="sm"
                          disabled={pending}
                          onClick={() => setConfirm({ kind: "proses", order: o })}
                        >
                          Proses
                        </Button>
                      </div>
                    )}
                  </div>

                  {normalizeWaNumber(o.kontak) && o.status !== "ditolak" && (
                    <div className="mt-2 space-y-1.5">
                      {o.metode_bayar === "qris" && (
                        <p className="flex items-start gap-1.5 rounded-md bg-accent/10 px-2 py-1.5 text-xs text-accent">
                          <QrCode className="mt-0.5 size-3.5 shrink-0" />
                          <span>
                            Bayar QRIS: screenshot QR dinamis (sesuai total) dari aplikasimu, lalu
                            lampirkan sebagai foto bersama struk ini di WhatsApp.
                          </span>
                        </p>
                      )}
                      <a
                        href={waLink(o.kontak, buildStrukWa(o))}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-flex h-9 w-full items-center justify-center gap-1.5 rounded-lg bg-[#25D366] px-3 text-sm font-medium text-white transition-colors hover:bg-[#1ebe5b]"
                      >
                        <MessageCircle className="size-4" />
                        Kirim struk via WhatsApp
                      </a>
                    </div>
                  )}
                </Card>
              </li>
            );
          })}
        </ul>
      )}

      <Modal
        open={confirm !== null}
        onClose={() => !pending && setConfirm(null)}
        title={confirm?.kind === "proses" ? "Proses pesanan?" : "Tolak pesanan?"}
        description={
          confirm?.kind === "proses"
            ? "Stok akan dikurangi & pesanan dicatat sebagai penjualan."
            : "Pesanan ditandai ditolak. Stok tidak berubah."
        }
        className="max-w-md"
      >
        {confirm && (
          <div className="space-y-4">
            <p className="text-sm">
              {confirm.kind === "proses" ? "Proses" : "Tolak"} pesanan{" "}
              <span className="font-mono font-medium">{confirm.order.id}</span> (
              {formatRupiah(confirm.order.total)})?
            </p>
            <div className="flex justify-end gap-2">
              <Button variant="outline" size="sm" disabled={pending} onClick={() => setConfirm(null)}>
                Batal
              </Button>
              <Button
                variant={confirm.kind === "proses" ? "accent" : "destructive"}
                size="sm"
                disabled={pending}
                onClick={runConfirm}
              >
                {pending && <Loader2 className="size-4 animate-spin" />}
                {confirm.kind === "proses" ? "Proses" : "Tolak"}
              </Button>
            </div>
          </div>
        )}
      </Modal>

      {toast && (
        <div
          role="status"
          aria-live="polite"
          className={cn(
            "fixed bottom-4 left-1/2 z-[60] flex -translate-x-1/2 items-center gap-2 rounded-lg border bg-card px-4 py-2.5 text-sm shadow-lg",
            toast.ok ? "border-success/30 text-foreground" : "border-destructive/30 text-destructive",
          )}
        >
          {toast.ok ? <CheckCircle2 className="size-4 text-success" /> : <TriangleAlert className="size-4" />}
          {toast.ok ? toast.message : toast.error}
        </div>
      )}
    </div>
  );
}
