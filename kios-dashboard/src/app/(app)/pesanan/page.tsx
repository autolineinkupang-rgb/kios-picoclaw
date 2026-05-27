import type { Metadata } from "next";
import { getAllPesanan, getConfig } from "@/lib/kios";
import { ConnectionError } from "@/components/connection-error";
import { PesananInbox } from "@/components/pesanan/pesanan-inbox";
import type { QrisLite } from "@/lib/wa";

export const metadata: Metadata = { title: "Pesanan" };
export const dynamic = "force-dynamic";

export default async function PesananPage() {
  let pesanan;
  let qris: QrisLite = { enabled: false };
  try {
    const [orders, cfg] = await Promise.all([getAllPesanan(), getConfig()]);
    pesanan = orders;
    qris =
      cfg.qris_enabled && cfg.qris_image_url
        ? { enabled: true, nama: cfg.qris_nama || "Kios Cerdas", image_url: cfg.qris_image_url }
        : { enabled: false };
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
      <PesananInbox pesanan={pesanan} qris={qris} />
    </div>
  );
}
