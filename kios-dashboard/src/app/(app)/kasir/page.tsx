import type { Metadata } from "next";
import { getAllProduk, getShift, getShiftHistory } from "@/lib/kios";
import { ConnectionError } from "@/components/connection-error";
import { KasirTabs } from "@/components/kasir/kasir-tabs";

export const metadata: Metadata = { title: "Kasir" };
export const dynamic = "force-dynamic";

export default async function KasirPage() {
  let produk, shift, shiftHistory;
  try {
    [produk, shift, shiftHistory] = await Promise.all([
      getAllProduk(),
      getShift(),
      getShiftHistory(10),
    ]);
  } catch (e) {
    return <ConnectionError message={e instanceof Error ? e.message : String(e)} />;
  }

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold">Kasir</h2>
        <p className="text-sm text-muted-foreground">
          Catat penjualan cepat. Stok &amp; laporan otomatis ter-update.
        </p>
      </div>
      <KasirTabs produk={produk} shift={shift} shiftHistory={shiftHistory} />
    </div>
  );
}
