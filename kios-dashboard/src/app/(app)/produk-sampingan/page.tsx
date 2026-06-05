import type { Metadata } from "next";
import { getAllProduk } from "@/lib/kios";
import { getSession } from "@/lib/auth";
import { ConnectionError } from "@/components/connection-error";
import { SampinganTable } from "@/components/produk-sampingan/sampingan-table";

export const metadata: Metadata = { title: "Produk Sampingan" };
export const dynamic = "force-dynamic";

const JENIS_SAMPINGAN = new Set(["pulsa", "bensin", "solar", "minyak_tanah"]);

export default async function ProdukSampinganPage() {
  const session = await getSession();
  let produk;
  try {
    const all = await getAllProduk();
    produk = all.filter((p) => JENIS_SAMPINGAN.has(p.jenis ?? ""));
  } catch (e) {
    return <ConnectionError message={e instanceof Error ? e.message : String(e)} />;
  }

  const canManage = session?.role === "owner";

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold">Produk Sampingan</h2>
        <p className="text-sm text-muted-foreground">
          {canManage
            ? "Kelola pulsa, bensin, solar, dan minyak tanah."
            : "Lihat stok produk sampingan. Pengelolaan khusus pemilik."}
        </p>
      </div>
      <SampinganTable produk={produk} canManage={canManage} />
    </div>
  );
}
