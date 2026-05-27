import type { Metadata } from "next";
import { getAllTransaksi } from "@/lib/kios";
import { ConnectionError } from "@/components/connection-error";
import { PenjualanView } from "@/components/penjualan/penjualan-view";

export const metadata: Metadata = { title: "Penjualan" };
export const dynamic = "force-dynamic";

export default async function PenjualanPage() {
  let transaksi;
  try {
    transaksi = await getAllTransaksi();
  } catch (e) {
    return <ConnectionError message={e instanceof Error ? e.message : String(e)} />;
  }

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold">Penjualan</h2>
        <p className="text-sm text-muted-foreground">
          Riwayat transaksi kasir, tersinkron langsung dari bot Telegram.
        </p>
      </div>
      <PenjualanView transaksi={transaksi} />
    </div>
  );
}
