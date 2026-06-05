import type { Metadata } from "next";
import { getAllPenarikan, getAllProduk, getAllTransaksi, getSampinganSaldo } from "@/lib/kios";
import type { SampinganSaldo } from "@/lib/types";
import { getSession } from "@/lib/auth";
import { ConnectionError } from "@/components/connection-error";
import { SampinganTable } from "@/components/produk-sampingan/sampingan-table";
import { LaporanPulsaBensin } from "@/components/laporan/laporan-pulsa-bensin";
import { hitungLaba } from "@/lib/analytics";

export const metadata: Metadata = { title: "Produk Sampingan" };
export const dynamic = "force-dynamic";

const JENIS_SAMPINGAN = new Set(["pulsa", "bensin", "solar", "minyak_tanah"]);

const DEFAULT_SALDO: SampinganSaldo = { pulsa: 0, bensin: 0, solar: 0, minyak_tanah: 0 };

export default async function ProdukSampinganPage() {
  const session = await getSession();
  let produk, transaksi, labaTersedia: number, saldoKat: SampinganSaldo;
  try {
    const [allProduk, txs, penarikan, saldo] = await Promise.all([
      getAllProduk(),
      getAllTransaksi(),
      getAllPenarikan(),
      getSampinganSaldo(),
    ]);
    produk = allProduk.filter((p) => JENIS_SAMPINGAN.has(p.jenis ?? ""));
    transaksi = txs;
    const totalLaba = hitungLaba(txs, allProduk).laba;
    const totalTarik = penarikan.reduce((s, p) => s + p.jumlah, 0);
    labaTersedia = Math.max(0, totalLaba - totalTarik);
    saldoKat = saldo ?? DEFAULT_SALDO;
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
      <LaporanPulsaBensin transaksi={transaksi} produk={produk} saldoKategori={saldoKat} />
      <SampinganTable produk={produk} canManage={canManage} labaTersedia={labaTersedia} saldo={saldoKat} />
    </div>
  );
}
