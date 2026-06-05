import type { Metadata } from "next";
import { getAllProduk, getAllTransaksi, getSampinganSaldo } from "@/lib/kios";
import { ConnectionError } from "@/components/connection-error";
import { LaporanView } from "@/components/laporan/laporan-view";

export const metadata: Metadata = { title: "Laporan" };
export const dynamic = "force-dynamic";

export default async function LaporanPage() {
  let produk, transaksi, saldo;
  try {
    [produk, transaksi, saldo] = await Promise.all([
      getAllProduk(),
      getAllTransaksi(),
      getSampinganSaldo(),
    ]);
  } catch (e) {
    return <ConnectionError message={e instanceof Error ? e.message : String(e)} />;
  }

  return <LaporanView transaksi={transaksi} produk={produk} saldoKategori={saldo ?? undefined} />;
}
