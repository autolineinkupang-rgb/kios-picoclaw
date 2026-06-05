import type { Metadata } from "next";
import { getAllProduk } from "@/lib/kios";
import { getSession } from "@/lib/auth";
import { ConnectionError } from "@/components/connection-error";
import { ProdukTable } from "@/components/produk/produk-table";

export const metadata: Metadata = { title: "Produk & Stok" };
export const dynamic = "force-dynamic";

export default async function ProdukPage() {
  const session = await getSession();
  let produk;
  try {
    const all = await getAllProduk();
    produk = all.filter((p) => !p.jenis || p.jenis === "biasa");
  } catch (e) {
    return <ConnectionError message={e instanceof Error ? e.message : String(e)} />;
  }

  const canManage = session?.role === "owner";

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold">Produk &amp; Stok</h2>
        <p className="text-sm text-muted-foreground">
          {canManage
            ? "Kelola produk, harga, dan tingkat stok kios."
            : "Lihat daftar produk dan stok. Pengelolaan khusus pemilik."}
        </p>
      </div>
      <ProdukTable produk={produk} canManage={canManage} />
    </div>
  );
}
