"use client";

import { MessageCircle } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { formatRupiah, formatTanggal } from "@/lib/format";
import type { Pelanggan, Pesanan } from "@/lib/types";

const STATUS_LABEL: Record<string, string> = {
  pending: "Menunggu",
  diproses: "Diproses",
  ditolak: "Ditolak",
};

const STATUS_VARIANT: Record<string, "warning" | "success" | "secondary"> = {
  pending: "warning",
  diproses: "success",
  ditolak: "secondary",
};

export function PelangganDetail({
  pelanggan,
  riwayat,
}: {
  pelanggan: Pelanggan;
  riwayat: Pesanan[];
}) {
  return (
    <div className="space-y-6">
      {/* Ringkasan */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <Card className="p-4 text-center">
          <p className="text-2xl font-bold tabular-nums">{pelanggan.total_pesanan}</p>
          <p className="text-xs text-muted-foreground">Total Pesanan</p>
        </Card>
        <Card className="p-4 text-center">
          <p className="font-mono text-xl font-bold tabular-nums">{formatRupiah(pelanggan.total_belanja)}</p>
          <p className="text-xs text-muted-foreground">Total Belanja</p>
        </Card>
        <Card className="p-4 text-center">
          <p className={`font-mono text-xl font-bold tabular-nums ${pelanggan.total_utang > 0 ? "text-destructive" : ""}`}>
            {formatRupiah(pelanggan.total_utang)}
          </p>
          <p className="text-xs text-muted-foreground">Piutang</p>
        </Card>
        <Card className="p-4 text-center">
          <p className="text-sm font-medium">{pelanggan.last_order || "—"}</p>
          <p className="text-xs text-muted-foreground">Terakhir Order</p>
        </Card>
      </div>

      {/* WA link */}
      {pelanggan.phone && (
        <a
          href={`https://wa.me/${pelanggan.phone}`}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex h-10 items-center gap-2 rounded-lg bg-[#25D366] px-4 text-sm font-medium text-white transition-colors hover:bg-[#1ebe5b]"
        >
          <MessageCircle className="size-4" />
          Hubungi via WhatsApp
        </a>
      )}

      {/* Riwayat pesanan */}
      <div>
        <h3 className="mb-3 text-base font-semibold">Riwayat Pesanan</h3>
        {riwayat.length === 0 ? (
          <EmptyState icon={MessageCircle} title="Belum ada pesanan" />
        ) : (
          <div className="space-y-2">
            {riwayat.map((p) => (
              <Card key={p.id} className="flex items-center justify-between gap-3 p-3">
                <div className="min-w-0">
                  <p className="font-mono text-sm font-semibold">{p.id}</p>
                  <p className="text-xs text-muted-foreground">
                    {formatTanggal(p.tanggal)} · {p.metode_bayar}
                  </p>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {p.items.map((it) => `${it.nama_produk} ×${it.qty}`).join(", ")}
                  </p>
                </div>
                <div className="flex shrink-0 flex-col items-end gap-1.5">
                  <span className="font-mono text-sm font-semibold tabular-nums">{formatRupiah(p.total)}</span>
                  <Badge variant={STATUS_VARIANT[p.status] ?? "secondary"}>
                    {STATUS_LABEL[p.status] ?? p.status}
                  </Badge>
                </div>
              </Card>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
