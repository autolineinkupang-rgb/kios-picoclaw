import type { Metadata } from "next";
import { getAllPesanan } from "@/lib/kios";
import { ConnectionError } from "@/components/connection-error";
import { PesananInbox } from "@/components/pesanan/pesanan-inbox";

export const metadata: Metadata = { title: "Pesanan" };
export const dynamic = "force-dynamic";

export default async function PesananPage() {
  let pesanan;
  try {
    pesanan = await getAllPesanan();
  } catch (e) {
    return <ConnectionError message={e instanceof Error ? e.message : String(e)} />;
  }

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold">Pesanan Masuk</h2>
        <p className="text-sm text-muted-foreground">
          Pesanan dari halaman toko pembeli. Proses untuk mencatat penjualan &amp; mengurangi stok.
        </p>
      </div>
      <PesananInbox pesanan={pesanan} />
    </div>
  );
}
