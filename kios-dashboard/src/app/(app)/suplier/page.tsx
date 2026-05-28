import type { Metadata } from "next";
import { getAllSuplier, getAllProduk, getAllPembelian, getAllHargaSupplier } from "@/lib/kios";
import { getSession } from "@/lib/auth";
import { ConnectionError } from "@/components/connection-error";
import { SuplierTable } from "@/components/suplier/suplier-table";
import { BandingHarga } from "@/components/suplier/banding-harga";

export const metadata: Metadata = { title: "Supplier" };
export const dynamic = "force-dynamic";

export default async function SuplierPage() {
  const session = await getSession();
  let suplier;
  let produkList;
  let pembelianList;
  let hargaOverrides;
  try {
    [suplier, produkList, pembelianList, hargaOverrides] = await Promise.all([
      getAllSuplier(),
      getAllProduk(),
      getAllPembelian(),
      getAllHargaSupplier(),
    ]);
  } catch (e) {
    return <ConnectionError message={e instanceof Error ? e.message : String(e)} />;
  }

  const canManage = session?.role === "owner" || session?.role === "kasir";
  const canDelete = session?.role === "owner";

  return (
    <div className="space-y-8">
      <div>
        <h2 className="text-lg font-semibold">Supplier</h2>
        <p className="text-sm text-muted-foreground">
          {canManage
            ? "Kelola data supplier dan harga beli produk."
            : "Lihat daftar supplier kios."}
        </p>
      </div>
      <SuplierTable suplier={suplier} canManage={canManage} canDelete={canDelete} />
      <BandingHarga
        produkList={produkList}
        pembelianList={pembelianList}
        hargaOverrides={hargaOverrides}
        canManage={canManage}
      />
    </div>
  );
}
