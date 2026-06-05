import type { Metadata } from "next";
import { getAllProduk, getAllTransaksi } from "@/lib/kios";
import { getSession } from "@/lib/auth";
import { ConnectionError } from "@/components/connection-error";
import { SampinganTable } from "@/components/produk-sampingan/sampingan-table";
import { LaporanPulsaBensin } from "@/components/laporan/laporan-pulsa-bensin";

export const metadata: Metadata = { title: "Produk Sampingan" };
export const dynamic = "force-dynamic";

const JENIS_SAMPINGAN = new Set(["pulsa", "bensin", "solar", "minyak_tanah"]);

export default async function ProdukSampinganPage() {
  const session = await getSession();
  let produk, transaksi;
  try {
    const [all, txs] = await Promise.all([getAllProduk(), getAllTransaksi()]);
    produk = all.filter((p) => JENIS_SAMPINGAN.has(p.jenis ?? ""));
    transaksi = txs;
  } catch (e) {
    return <ConnectionError message={e instanceof Error ? e.message : String(e)} />;
  }

  const canManage = session?.role === "owner";

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold">Produk Sampingan</h2>
        <p className="text-sm text-muted-foreground">
          {canManage
            ? "Kelola pulsa, bensin, solar, dan minyak tanah."
            : "Lihat stok produk sampingan. Pengelolaan khusus pemilik."}
        </p>
      </div>
      <LaporanPulsaBensin transaksi={transaksi} produk={produk} />
      <SampinganTable produk={produk} canManage={canManage} />
    </div>
  );
}
