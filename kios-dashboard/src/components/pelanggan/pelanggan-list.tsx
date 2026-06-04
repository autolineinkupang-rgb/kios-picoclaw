"use client";

import { useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { Search, UserRound } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { formatRupiah } from "@/lib/format";
import { matchesQuery } from "@/lib/utils";
import type { Pelanggan } from "@/lib/types";

export function PelangganList({ pelanggan }: { pelanggan: Pelanggan[] }) {
  const router = useRouter();
  const [query, setQuery] = useState("");

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return pelanggan;
    return pelanggan.filter((p) => matchesQuery(q, p.nama, p.phone));
  }, [pelanggan, query]);

  return (
    <div className="space-y-3">
      <div className="relative max-w-sm">
        <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Cari nama atau nomor WA…"
          className="pl-9"
          aria-label="Cari pelanggan"
        />
      </div>

      {filtered.length === 0 ? (
        <EmptyState icon={UserRound} title="Belum ada pelanggan" description="Pelanggan akan muncul saat ada pesanan masuk dari storefront." />
      ) : (
        <div className="overflow-x-auto rounded-lg border">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/40 text-left">
                <th className="px-4 py-3 font-medium text-muted-foreground">Nama</th>
                <th className="px-4 py-3 font-medium text-muted-foreground">No. WA</th>
                <th className="px-4 py-3 text-right font-medium text-muted-foreground">Pesanan</th>
                <th className="px-4 py-3 text-right font-medium text-muted-foreground">Total Belanja</th>
                <th className="px-4 py-3 text-right font-medium text-muted-foreground">Piutang</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((p) => (
                <tr
                  key={p.id}
                  onClick={() => router.push(`/pelanggan/${encodeURIComponent(p.phone)}`)}
                  className="cursor-pointer border-b last:border-0 hover:bg-muted/30 transition-colors"
                >
                  <td className="px-4 py-3 font-medium">{p.nama}</td>
                  <td className="px-4 py-3 font-mono text-muted-foreground">{p.phone}</td>
                  <td className="px-4 py-3 text-right tabular-nums">{p.total_pesanan}</td>
                  <td className="px-4 py-3 text-right font-mono tabular-nums">{formatRupiah(p.total_belanja)}</td>
                  <td className="px-4 py-3 text-right">
                    {p.total_utang > 0 ? (
                      <Badge variant="destructive">{formatRupiah(p.total_utang)}</Badge>
                    ) : (
                      <span className="text-muted-foreground">—</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
