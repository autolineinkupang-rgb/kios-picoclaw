import type { Metadata } from "next";
import { getAllPelanggan } from "@/lib/kios";
import { ConnectionError } from "@/components/connection-error";
import { PelangganList } from "@/components/pelanggan/pelanggan-list";

export const metadata: Metadata = { title: "Pelanggan" };
export const dynamic = "force-dynamic";

export default async function PelangganPage() {
  let pelanggan;
  try {
    pelanggan = await getAllPelanggan();
  } catch (e) {
    return <ConnectionError message={e instanceof Error ? e.message : String(e)} />;
  }

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold">Pelanggan</h2>
        <p className="text-sm text-muted-foreground">
          Daftar pembeli yang pernah memesan. Klik baris untuk melihat riwayat pesanan.
        </p>
      </div>
      <PelangganList pelanggan={pelanggan} />
    </div>
  );
}
