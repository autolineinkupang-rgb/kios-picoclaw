import type { Metadata } from "next";
import { getAllPromo, getAllProduk } from "@/lib/kios";
import { getSession } from "@/lib/auth";
import { ConnectionError } from "@/components/connection-error";
import { PromoTable } from "@/components/promo/promo-table";

export const metadata: Metadata = { title: "Promo" };
export const dynamic = "force-dynamic";

export default async function PromoPage() {
  const session = await getSession();
  let promo, produk;
  try {
    [promo, produk] = await Promise.all([getAllPromo(), getAllProduk()]);
  } catch (e) {
    return <ConnectionError message={e instanceof Error ? e.message : String(e)} />;
  }

  const isOwner = session?.role === "owner";

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold">Promo &amp; Diskon</h2>
        <p className="text-sm text-muted-foreground">
          {isOwner
            ? "Kelola promo diskon. Promo dari kasir perlu diaktifkan dulu."
            : "Ajukan promo diskon. Promo aktif setelah disetujui owner."}
        </p>
      </div>
      <PromoTable promo={promo} produk={produk} isOwner={isOwner} />
    </div>
  );
}
