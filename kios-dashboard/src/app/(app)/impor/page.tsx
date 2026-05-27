import type { Metadata } from "next";
import { getSession } from "@/lib/auth";
import { ImporView } from "@/components/impor/impor-view";

export const metadata: Metadata = { title: "Impor Data" };
export const dynamic = "force-dynamic";

export default async function ImporPage() {
  const session = await getSession();
  const role = session?.role ?? "kasir";

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold">Impor Data</h2>
        <p className="text-sm text-muted-foreground">
          {role === "owner"
            ? "Unggah Excel/CSV untuk memperbarui produk & stok secara massal."
            : "Unggah Excel/CSV hasil hitung stok untuk memperbarui jumlah stok."}
        </p>
      </div>
      <ImporView role={role} />
    </div>
  );
}
