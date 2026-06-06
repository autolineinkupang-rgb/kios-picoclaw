import type { Metadata } from "next";
import { getAllProduk, getSampinganSaldo } from "@/lib/kios";
import { ConnectionError } from "@/components/connection-error";
import { KasirForm } from "@/components/kasir/kasir-form";

export const metadata: Metadata = { title: "Kasir" };
export const dynamic = "force-dynamic";

export default async function KasirPage() {
  let produk, saldo;
  try {
    [produk, saldo] = await Promise.all([getAllProduk(), getSampinganSaldo()]);
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
      <KasirForm produk={produk} saldo={saldo ?? undefined} />
    </div>
  );
}
