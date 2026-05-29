"use client";

import { useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { CheckCircle2, Clock, Loader2, TriangleAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { formatRupiah } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { Shift } from "@/lib/types";
import {
  bukaShiftAction,
  tutupShiftAction,
  type ShiftResult,
} from "@/app/(app)/kasir/actions";

interface Props {
  shift: Shift | null;
  history: Shift[];
}

export function ShiftTab({ shift, history }: Props) {
  const router = useRouter();
  const [pending, start] = useTransition();
  const [toast, setToast] = useState<ShiftResult | null>(null);

  // form state buka shift
  const [kasir, setKasir] = useState("");
  const [saldoAwal, setSaldoAwal] = useState("");
  // form state tutup shift
  const [saldoAkhir, setSaldoAkhir] = useState("");

  function showToast(r: ShiftResult) {
    setToast(r);
    window.setTimeout(() => setToast(null), 4000);
  }

  function handleBuka() {
    const nominal = Number(saldoAwal.replace(/\D/g, "")) || 0;
    start(async () => {
      const r = await bukaShiftAction(kasir, nominal);
      showToast(r);
      if (r.ok) {
        setKasir("");
        setSaldoAwal("");
        router.refresh();
      }
    });
  }

  function handleTutup() {
    const nominal = Number(saldoAkhir.replace(/\D/g, "")) || 0;
    start(async () => {
      const r = await tutupShiftAction(nominal);
      showToast(r);
      if (r.ok) {
        setSaldoAkhir("");
        router.refresh();
      }
    });
  }

  const shiftAktif = shift?.status === "buka";

  return (
    <div className="space-y-6">
      {/* Status shift berjalan */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Clock className="size-4" />
            {shiftAktif ? "Shift Sedang Buka" : "Tidak Ada Shift Aktif"}
          </CardTitle>
        </CardHeader>
        <CardContent>
          {shiftAktif && shift ? (
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-3 text-sm sm:grid-cols-3">
                <div>
                  <p className="text-xs text-muted-foreground">Kasir</p>
                  <p className="font-medium">{shift.kasir}</p>
                </div>
                <div>
                  <p className="text-xs text-muted-foreground">Waktu Buka</p>
                  <p className="font-medium">{shift.waktu_buka}</p>
                </div>
                <div>
                  <p className="text-xs text-muted-foreground">Saldo Awal</p>
                  <p className="font-medium font-mono">{formatRupiah(shift.saldo_awal)}</p>
                </div>
              </div>
              <div className="space-y-1.5 border-t pt-4">
                <Label htmlFor="saldo-akhir">Saldo Akhir (saat tutup)</Label>
                <div className="flex gap-2">
                  <Input
                    id="saldo-akhir"
                    value={saldoAkhir}
                    onChange={(e) => setSaldoAkhir(e.target.value)}
                    placeholder="mis. 500000"
                    inputMode="numeric"
                    className="max-w-xs font-mono"
                  />
                  <Button
                    variant="outline"
                    size="md"
                    onClick={handleTutup}
                    disabled={pending}
                  >
                    {pending ? <Loader2 className="size-4 animate-spin" /> : null}
                    Tutup Shift
                  </Button>
                </div>
              </div>
            </div>
          ) : (
            <div className="space-y-3">
              <div className="grid gap-3 sm:grid-cols-2">
                <div className="space-y-1.5">
                  <Label htmlFor="kasir-nama">Nama Kasir</Label>
                  <Input
                    id="kasir-nama"
                    value={kasir}
                    onChange={(e) => setKasir(e.target.value)}
                    placeholder="Kosong = nama akun login"
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="saldo-awal">Saldo Awal</Label>
                  <Input
                    id="saldo-awal"
                    value={saldoAwal}
                    onChange={(e) => setSaldoAwal(e.target.value)}
                    placeholder="mis. 200000"
                    inputMode="numeric"
                    className="font-mono"
                  />
                </div>
              </div>
              <Button variant="accent" size="md" onClick={handleBuka} disabled={pending}>
                {pending ? <Loader2 className="size-4 animate-spin" /> : null}
                Buka Shift
              </Button>
            </div>
          )}

          {/* Toast */}
          {toast && (
            <p
              className={cn(
                "mt-3 flex items-center gap-1.5 text-sm",
                toast.ok ? "text-success" : "text-destructive",
              )}
            >
              {toast.ok ? (
                <CheckCircle2 className="size-4" />
              ) : (
                <TriangleAlert className="size-4" />
              )}
              {toast.ok ? "Berhasil." : toast.error}
            </p>
          )}
        </CardContent>
      </Card>

      {/* Riwayat shift */}
      {history.length > 0 && (
        <div>
          <h3 className="mb-2 text-sm font-semibold text-muted-foreground">
            Riwayat Shift Terakhir
          </h3>
          <div className="overflow-x-auto rounded-xl border bg-card">
            <table className="w-full min-w-[600px] text-sm">
              <thead>
                <tr className="border-b text-left text-xs text-muted-foreground">
                  <th className="p-3 font-medium">Kasir</th>
                  <th className="p-3 font-medium">Waktu Buka</th>
                  <th className="p-3 font-medium">Waktu Tutup</th>
                  <th className="p-3 text-right font-medium">Saldo Awal</th>
                  <th className="p-3 text-right font-medium">Saldo Akhir</th>
                </tr>
              </thead>
              <tbody>
                {history.map((s, i) => (
                  <tr
                    key={i}
                    className="border-b text-sm last:border-0 hover:bg-muted/40"
                  >
                    <td className="p-3 font-medium">{s.kasir}</td>
                    <td className="p-3 text-muted-foreground">{s.waktu_buka}</td>
                    <td className="p-3 text-muted-foreground">{s.waktu_tutup || "–"}</td>
                    <td className="p-3 text-right font-mono tabular-nums">
                      {formatRupiah(s.saldo_awal)}
                    </td>
                    <td className="p-3 text-right font-mono tabular-nums">
                      {s.saldo_akhir > 0 ? formatRupiah(s.saldo_akhir) : "–"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
